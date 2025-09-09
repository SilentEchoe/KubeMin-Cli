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

	"KubeMin-Cli/pkg/apiserver/config"
	"KubeMin-Cli/pkg/apiserver/domain/model"
	"KubeMin-Cli/pkg/apiserver/infrastructure/datastore"
)

type JobCtl interface {
	Run(ctx context.Context)
	Clean(ctx context.Context)
	SaveInfo(ctx context.Context) error
}

func initJobCtl(job *model.JobTask, client *kubernetes.Clientset, store datastore.DataStore, ack func()) JobCtl {
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
	default:
		klog.Errorf("unknown job type: %s", job.JobType)
		return nil
	}
	return jobCtl
}

func RunJobs(ctx context.Context, jobs []*model.JobTask, concurrency int, client *kubernetes.Clientset, store datastore.DataStore, ack func()) {
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
	jobPool := NewPool(ctx, jobs, concurrency, client, ack)
	jobPool.Run()
}

func runJob(ctx context.Context, job *model.JobTask, client *kubernetes.Clientset, store datastore.DataStore, ack func()) {
	// Start a new span for this specific job, it will be a child of the workflow span
	tracer := otel.Tracer("job-runner")
	ctx, span := tracer.Start(ctx, job.Name, trace.WithAttributes(
		attribute.String("job.name", job.Name),
		attribute.String("job.type", job.JobType),
	))
	defer span.End()

	// Create a logger with both workflow traceID and the new job spanID
	logger := klog.FromContext(ctx).WithValues(
		"spanID", span.SpanContext().SpanID().String(),
		"jobName", job.Name,
	)
	ctx = klog.NewContext(ctx, logger)

	if job == nil {
		klog.Error("runJob received nil job") // This log cannot have context
		return
	}

	if job.Status == config.StatusPassed || job.Status == config.StatusSkipped {
		logger.Info("Job skipped", "status", job.Status)
		return
	}
	job.Status = config.StatusPrepare
	job.StartTime = time.Now().Unix()
	ack()

	if store == nil {
		klog.Error("start job store is nil") // This log cannot have context
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

	defer func() {
		if r := recover(); r != nil {
			errMsg := fmt.Sprintf("job panic: %v", r)
			logger.Error(errors.New(errMsg), "Panic recovered in job execution")
			job.Status = config.StatusFailed
			job.Error = errMsg
			span.SetStatus(codes.Error, "Panic in job execution")
			span.RecordError(errors.New(errMsg))
		}
		job.EndTime = time.Now().Unix()
		if job.Error != "" {
			logger.Info("Finished job with error", "status", job.Status, "error", job.Error)
		} else {
			logger.Info("Finished job successfully", "status", job.Status)
		}
		ack()
		logger.Info("Updating job info in db...")
		if err := jobCtl.SaveInfo(ctx); err != nil {
			logger.Error(err, "Failed to update job info in db")
		}
	}()
	// 执行对应的JOb任务
	jobCtl.Run(ctx)
}

func jobStatusFailed(status config.Status) bool {
	if status == config.StatusCancelled || status == config.StatusFailed || status == config.StatusTimeout || status == config.StatusReject {
		return true
	}
	return false
}

type Pool struct {
	Jobs        []*model.JobTask
	concurrency int
	client      *kubernetes.Clientset
	store       datastore.DataStore
	jobsChan    chan *model.JobTask
	ack         func()
	ctx         context.Context
	wg          sync.WaitGroup
}

func (p *Pool) Run() {
	for i := 0; i < p.concurrency; i++ {
		go p.work()
	}
	p.wg.Add(len(p.Jobs))
	for _, task := range p.Jobs {
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
		p.wg.Done()
	}
}

// NewPool initializes a new pool with the given tasks and
// at the given concurrency.
func NewPool(ctx context.Context, jobs []*model.JobTask, concurrency int, client *kubernetes.Clientset, ack func()) *Pool {
	return &Pool{
		Jobs:        jobs,
		client:      client,
		concurrency: concurrency,
		jobsChan:    make(chan *model.JobTask),
		ack:         ack,
		ctx:         ctx,
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
		config.LabelCli:         fmt.Sprintf("%s-%s", c.AppId, c.Name),
		config.LabelComponentId: fmt.Sprintf("%d", c.ID),
		config.LabelAppId:       c.AppId,
	}
	if p != nil {
		for k, v := range p.Labels {
			labels[k] = v
		}
	}
	return labels
}
