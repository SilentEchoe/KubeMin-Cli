package job

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"kubemin-cli/pkg/apiserver/config"
	"kubemin-cli/pkg/apiserver/domain/model"
	"kubemin-cli/pkg/apiserver/infrastructure/datastore"
	"kubemin-cli/pkg/apiserver/infrastructure/informer"
	"kubemin-cli/pkg/apiserver/workflow/signal"
)

type JobCtl interface {
	Run(ctx context.Context) error
	Clean(ctx context.Context)
	SaveInfo(ctx context.Context) error
}

type GenerateServiceResult struct {
	Service           interface{}
	AdditionalObjects []client.Object
}

type taskIDKey struct{}

// StatusError wraps an error with an explicit job status for persistence.
type StatusError struct {
	Status config.Status
	Err    error
}

func (s *StatusError) Error() string { return s.Err.Error() }

func (s *StatusError) Unwrap() error { return s.Err }

// NewStatusError constructs a StatusError with the provided status and error.
func NewStatusError(status config.Status, err error) error {
	if err == nil {
		return nil
	}
	return &StatusError{Status: status, Err: err}
}

// ExtractStatusError attempts to retrieve a StatusError from err.
func ExtractStatusError(err error) (*StatusError, bool) {
	if err == nil {
		return nil, false
	}
	var se *StatusError
	if errors.As(err, &se) {
		return se, true
	}
	return nil, false
}

// WithTaskMetadata injects workflow identifiers into context so job controllers
// can derive cancellation signals when needed.
func WithTaskMetadata(ctx context.Context, taskID string) context.Context {
	return context.WithValue(ctx, taskIDKey{}, taskID)
}

// TaskIDFromContext extracts the workflow task identifier from context.
func TaskIDFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(taskIDKey{}).(string); ok {
		return v
	}
	return ""
}

func initJobCtl(job *model.JobTask, client kubernetes.Interface, store datastore.DataStore, ack func()) JobCtl {
	if store == nil {
		klog.Errorf("initJobCtl store is nil")
		return nil
	}
	if job == nil {
		klog.Errorf("initJobCtl job is nil")
		return nil
	}
	if client == nil {
		klog.Errorf("initJobCtl client is nil")
		return nil
	}

	var jobCtl JobCtl
	switch job.JobType {
	case string(config.JobDeploy):
		jobCtl = NewDeployJobCtl(job, client, store, ack)
	case string(config.JobDeployService):
		jobCtl = NewDeployServiceJobCtl(job, client, store, ack)
	case string(config.JobDeployStore):
		jobCtl = NewDeployStatefulSetJobCtl(job, client, store, ack)
	case string(config.JobDeployPVC):
		jobCtl = NewDeployPVCJobCtl(job, client, store, ack)
	case string(config.JobDeployConfigMap):
		jobCtl = NewDeployConfigMapJobCtl(job, client, store, ack)
	case string(config.JobDeploySecret):
		jobCtl = NewDeploySecretJobCtl(job, client, store, ack)
	case string(config.JobDeployIngress):
		jobCtl = NewDeployIngressJobCtl(job, client, store, ack)
	case string(config.JobDeployServiceAccount):
		jobCtl = NewDeployServiceAccountJobCtl(job, client, store, ack)
	case string(config.JobDeployRole):
		jobCtl = NewDeployRoleJobCtl(job, client, store, ack)
	case string(config.JobDeployRoleBinding):
		jobCtl = NewDeployRoleBindingJobCtl(job, client, store, ack)
	case string(config.JobDeployClusterRole):
		jobCtl = NewDeployClusterRoleJobCtl(job, client, store, ack)
	case string(config.JobDeployClusterRoleBinding):
		jobCtl = NewDeployClusterRoleBindingJobCtl(job, client, store, ack)
	default:
		klog.Errorf("unknown job type: %s", job.JobType)
		return nil
	}
	return jobCtl
}

func RunJobs(ctx context.Context, jobs []*model.JobTask, concurrency int, client kubernetes.Interface, store datastore.DataStore, ack func(), stopOnFailure bool) {
	logger := klog.FromContext(ctx)
	if len(jobs) == 0 {
		logger.Info("no jobs to run")
		return
	}

	if concurrency == 1 {
		for _, job := range jobs {
			logger.Info("Job started", "jobName", job.Name, "jobType", job.JobType)
			runJob(ctx, job, client, store, ack)
			// DEBUG: Log job completion status before checking for failure.
			logger.Info("DEBUG: Job finished running", "jobName", job.Name, "status", job.Status)
			if jobStatusFailed(job.Status) {
				logger.Error(nil, "Job failed, stopping workflow execution.", "jobName", job.Name, "status", job.Status)
				return
			}
		}
		return
	}
	jobPool := NewPool(ctx, jobs, concurrency, client, store, ack, stopOnFailure)
	jobPool.Run()
}

func runJob(ctx context.Context, job *model.JobTask, client kubernetes.Interface, store datastore.DataStore, ack func()) {
	tracer := otel.Tracer("job-runner")
	ctx, span := tracer.Start(ctx, job.Name, trace.WithAttributes(
		attribute.String("job.name", job.Name),
		attribute.String("job.type", job.JobType),
	))
	defer span.End()

	logger := klog.FromContext(ctx).WithValues(
		"spanID", span.SpanContext().SpanID().String(),
		"jobName", job.Name,
	)
	ctx = klog.NewContext(ctx, logger)
	ctx = WithCleanupTracker(ctx)

	var (
		watcher  *signal.CancelWatcher
		cancelFn context.CancelFunc = func() {}
		jobCtx                      = ctx
	)

	if taskID := TaskIDFromContext(ctx); taskID != "" {
		var err error
		watcher, jobCtx, cancelFn, err = signal.Watch(ctx, taskID)
		if err != nil {
			logger.Error(err, "Failed to activate cancellation watcher", "taskID", taskID)
			watcher = nil
			jobCtx = ctx
			cancelFn = func() {}
		}
	}

	defer cancelFn()
	defer func() {
		releaseCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if watcher != nil {
			watcher.Stop(releaseCtx)
		}
	}()

	if job.Status == config.StatusPassed {
		logger.Info("Job skipped", "status", job.Status)
		return
	}
	if job.Status == config.StatusSkipped {
		logger.Info("Job skipped", "status", job.Status)
		job.Error = ""
		now := time.Now().Unix()
		if job.StartTime == 0 {
			job.StartTime = now
		}
		if job.EndTime == 0 {
			job.EndTime = now
		}
		if ack != nil {
			ack()
		}
		if store == nil {
			klog.Error("start job store is nil")
			return
		}
		jobCtl := initJobCtl(job, client, store, ack)
		if jobCtl == nil {
			logger.Error(nil, "Failed to initialize job controller for skipped job")
			return
		}
		if err := jobCtl.SaveInfo(ctx); err != nil {
			logger.Error(err, "Failed to update job info in db")
		}
		return
	}
	job.Status = config.StatusPrepare
	job.Error = ""
	job.StartTime = time.Now().Unix()
	ack()

	if store == nil {
		klog.Error("start job store is nil")
		return
	}
	logger.Info("Starting job", "jobType", job.JobType, "status", job.Status)
	jobCtl := initJobCtl(job, client, store, ack)
	if jobCtl == nil {
		errMsg := fmt.Sprintf("failed to initialize job controller for job: %s", job.Name)
		logger.Error(nil, errMsg)
		job.Status = config.StatusFailed
		job.Error = errMsg
		job.EndTime = time.Now().Unix()
		span.SetStatus(codes.Error, "Failed to initialize job controller")
		span.RecordError(errors.New(errMsg))
		ack()
		return
	}

	cleaned := false

	defer func() {
		if r := recover(); r != nil {
			if !cleaned {
				jobCtl.Clean(jobCtx)
				cleaned = true
			}
			errMsg := fmt.Sprintf("job panic: %v", r)
			logger.Error(errors.New(errMsg), "Panic recovered in job execution")
			job.Status = config.StatusFailed
			job.Error = errMsg
			span.SetStatus(codes.Error, "Panic in job execution")
			span.RecordError(errors.New(errMsg))
		}
		job.EndTime = time.Now().Unix()
		if job.Error != "" {
			logger.Error(errors.New(job.Error), "Finished job with error", "status", job.Status, "detail", job.Error)
		} else {
			logger.Info("Finished job successfully", "status", job.Status)
		}
		ack()
		logger.Info("Updating job info in db...")
		if err := jobCtl.SaveInfo(jobCtx); err != nil {
			logger.Error(err, "Failed to update job info in db")
		}
	}()

	if err := jobCtl.Run(jobCtx); err != nil {
		if !cleaned {
			jobCtl.Clean(jobCtx)
			cleaned = true
		}
		span.SetStatus(codes.Error, "Job execution failed")
		span.RecordError(err)
		if job.Error == "" {
			job.Error = err.Error()
		}
		if statusErr, ok := ExtractStatusError(err); ok {
			job.Status = statusErr.Status
		} else if errors.Is(err, context.Canceled) {
			reason := signal.ReasonFromContext(jobCtx)
			if reason != "" {
				job.Error = reason
			}
			job.Status = config.StatusCancelled
		} else if job.Status != config.StatusFailed && job.Status != config.StatusCancelled && job.Status != config.StatusTimeout {
			job.Status = config.StatusFailed
		}
	} else if job.Status == config.StatusPrepare || job.Status == config.StatusRunning {
		job.Status = config.StatusCompleted
	}

	if !cleaned && jobStatusFailed(job.Status) {
		jobCtl.Clean(jobCtx)
	}
}

func jobStatusFailed(status config.Status) bool {
	if status == config.StatusCancelled || status == config.StatusFailed || status == config.StatusTimeout || status == config.StatusReject {
		return true
	}
	return false
}

type Pool struct {
	Jobs          []*model.JobTask
	concurrency   int
	client        kubernetes.Interface
	store         datastore.DataStore
	jobsChan      chan *model.JobTask
	ack           func()
	ctx           context.Context
	cancel        context.CancelFunc
	stopOnFailure bool
	wg            sync.WaitGroup
	failureOnce   sync.Once
}

func (p *Pool) Run() {
	defer p.cancel()
	for i := 0; i < p.concurrency; i++ {
		go p.work()
	}
	for _, task := range p.Jobs {
		if p.stopOnFailure && p.ctx.Err() != nil {
			break
		}
		p.wg.Add(1)
		p.jobsChan <- task
	}
	// all workers return
	close(p.jobsChan)
	p.wg.Wait()
}

// The work loop for any single goroutine.
func (p *Pool) work() {
	for job := range p.jobsChan {
		runJob(p.ctx, job, p.client, p.store, p.ack)
		if p.stopOnFailure && jobStatusFailed(job.Status) {
			p.failureOnce.Do(func() {
				p.cancel()
			})
		}
		p.wg.Done()
	}
}

// NewPool initializes a new pool with the given tasks and
// at the given concurrency.
func NewPool(ctx context.Context, jobs []*model.JobTask, concurrency int, client kubernetes.Interface, store datastore.DataStore, ack func(), stopOnFailure bool) *Pool {
	ctxForPool := ctx
	cancel := func() {}
	if stopOnFailure {
		ctxForPool, cancel = context.WithCancel(ctx)
	}
	return &Pool{
		Jobs:          jobs,
		client:        client,
		store:         store,
		concurrency:   concurrency,
		jobsChan:      make(chan *model.JobTask),
		ack:           ack,
		ctx:           ctxForPool,
		cancel:        cancel,
		stopOnFailure: stopOnFailure,
	}
}

func ParseProperties(properties *model.JSONStruct) model.Properties {
	cProperties, err := json.Marshal(properties)
	if err != nil {
		klog.Errorf("Component.Properties deserialization failure: %s", err)
		return model.Properties{}
	}

	var propertied model.Properties
	err = json.Unmarshal(cProperties, &propertied)
	if err != nil {
		klog.Errorf("WorkflowSteps deserialization failure: %s", err)
		return model.Properties{}
	}
	return propertied
}

func BuildLabels(c *model.ApplicationComponent, p *model.Properties) map[string]string {
	labels := map[string]string{
		config.LabelCli:           fmt.Sprintf("%s-%s", c.AppID, c.Name),
		config.LabelComponentID:   fmt.Sprintf("%d", c.ID),
		config.LabelAppID:         c.AppID,
		config.LabelComponentName: c.Name,
	}
	if p != nil {
		for k, v := range p.Labels {
			labels[k] = v
		}
	}
	return labels
}

// globalWaiter 全局资源等待器（由 Informer Manager 初始化）
var globalWaiter *informer.ResourceReadyWaiter

// SetGlobalWaiter 设置全局等待器（在 server 启动时调用）
func SetGlobalWaiter(w *informer.ResourceReadyWaiter) {
	globalWaiter = w
	klog.V(2).Info("Global resource waiter has been set")
}

// GetGlobalWaiter 获取全局等待器
func GetGlobalWaiter() *informer.ResourceReadyWaiter {
	return globalWaiter
}
