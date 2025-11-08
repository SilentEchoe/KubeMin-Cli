package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"KubeMin-Cli/pkg/apiserver/config"
	"KubeMin-Cli/pkg/apiserver/domain/model"
	"KubeMin-Cli/pkg/apiserver/domain/repository"
	"KubeMin-Cli/pkg/apiserver/domain/service"
	"KubeMin-Cli/pkg/apiserver/event/workflow/job"
	"KubeMin-Cli/pkg/apiserver/infrastructure/datastore"
	msg "KubeMin-Cli/pkg/apiserver/infrastructure/messaging"
)

type Workflow struct {
	KubeClient      kubernetes.Interface    `inject:"kubeClient"`
	KubeConfig      *rest.Config            `inject:"kubeConfig"`
	Store           datastore.DataStore     `inject:"datastore"`
	WorkflowService service.WorkflowService `inject:""`
	Queue           msg.Queue               `inject:"queue"`
	Cfg             *config.Config          `inject:""`
}

// TaskDispatch is the minimal payload for dispatching a workflow task to a worker.
type TaskDispatch struct {
	TaskID     string `json:"taskId"`
	WorkflowID string `json:"workflowId"`
	ProjectID  string `json:"projectId"`
	AppID      string `json:"appId"`
}

func MarshalTaskDispatch(t TaskDispatch) ([]byte, error) { return json.Marshal(t) }
func UnmarshalTaskDispatch(b []byte) (TaskDispatch, error) {
	var t TaskDispatch
	err := json.Unmarshal(b, &t)
	return t, err
}

func (w *Workflow) Start(ctx context.Context, errChan chan error) {
	w.InitQueue(ctx)
	// If queue is noop (local mode), fall back to direct DB scan executor for functionality.
	if _, ok := w.Queue.(*msg.NoopQueue); ok {
		go w.WorkflowTaskSender(ctx)
		return
	}
	// Redis Streams path: leader runs dispatcher; workers managed by server callbacks.
	go w.Dispatcher(ctx)
}

func (w *Workflow) InitQueue(ctx context.Context) {
	if w.Store == nil {
		klog.Error("datastore is nil")
		return
	}
	// 从数据库中查找未完成的任务
	tasks, err := w.WorkflowService.TaskRunning(ctx)
	if err != nil {
		klog.Errorf("find task running error: %v", err)
		return
	}
	// 如果重启Queue，则取消所有正在运行的tasks（尽最大努力取消并收集错误）
	var cancelErrs []error
	for _, task := range tasks {
		if err := w.WorkflowService.CancelWorkflowTask(ctx, config.DefaultTaskRevoker, task.TaskID, ""); err != nil {
			klog.Errorf("cancel task %s error: %v", task.TaskID, err)
			cancelErrs = append(cancelErrs, err)
			continue
		}
		klog.Infof("cancel task: %s", task.TaskID)
	}
	if len(cancelErrs) > 0 {
		klog.Warningf("cancel running tasks finished with errors: failed=%d total=%d", len(cancelErrs), len(tasks))
	}
}

// WorkflowTaskSender is the original local executor scanning DB and running tasks.
func (w *Workflow) WorkflowTaskSender(ctx context.Context) {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			klog.V(3).Info("workflow task sender stopped: context cancelled")
			return
		case <-ticker.C:
		}

		// 获取等待的任务
		waitingTasks, err := w.WorkflowService.WaitingTasks(context.Background())
		if err != nil || len(waitingTasks) == 0 {
			continue
		}
		for _, task := range waitingTasks {
			if ctx.Err() != nil {
				return
			}
			claimed, err := w.WorkflowService.MarkTaskStatus(context.Background(), task.TaskID, config.StatusWaiting, config.StatusQueued)
			if err != nil {
				klog.Errorf("mark task queued failed: %v", err)
				continue
			}
			if !claimed {
				continue
			}
			// TODO jobConcurrency 默认为1，表示串行执行
			if err := w.updateQueueAndRunTask(ctx, task, 1); err != nil {
				klog.Errorf("run task %s failed after mark queued: %v", task.TaskID, err)
				if reverted, revertErr := w.WorkflowService.MarkTaskStatus(context.Background(), task.TaskID, config.StatusQueued, config.StatusWaiting); revertErr != nil {
					klog.Errorf("revert task %s status to waiting failed: %v", task.TaskID, revertErr)
				} else if !reverted {
					klog.V(4).Infof("task %s status already changed before revert", task.TaskID)
				}
				continue
			}
		}
	}
}

// Dispatcher scans waiting tasks and publishes dispatch messages.
func (w *Workflow) Dispatcher(ctx context.Context) {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			klog.V(3).Info("workflow dispatcher stopped: context cancelled")
			return
		case <-ticker.C:
		}

		// 获取等待的任务
		waitingTasks, err := w.WorkflowService.WaitingTasks(context.Background())
		if err != nil || len(waitingTasks) == 0 {
			continue
		}
		for _, task := range waitingTasks {
			if ctx.Err() != nil {
				return
			}
			claimed, err := w.WorkflowService.MarkTaskStatus(context.Background(), task.TaskID, config.StatusWaiting, config.StatusQueued)
			if err != nil {
				klog.Errorf("mark task queued failed: %v", err)
				continue
			}
			if !claimed {
				continue
			}
			payload := TaskDispatch{TaskID: task.TaskID, WorkflowID: task.WorkflowID, ProjectID: task.ProjectID, AppID: task.AppID}
			b, err := MarshalTaskDispatch(payload)
			if err != nil {
				klog.Errorf("marshal task dispatch failed: %v", err)
				if reverted, revertErr := w.WorkflowService.MarkTaskStatus(ctx, task.TaskID, config.StatusQueued, config.StatusWaiting); revertErr != nil {
					klog.Errorf("revert task %s status to waiting failed: %v", task.TaskID, revertErr)
				} else if !reverted {
					klog.V(4).Infof("task %s status already changed before revert", task.TaskID)
				}
				continue
			}
			if id, err := w.Queue.Enqueue(ctx, b); err != nil {
				klog.Errorf("enqueue task dispatch failed: %v", err)
				if reverted, revertErr := w.WorkflowService.MarkTaskStatus(context.Background(), task.TaskID, config.StatusQueued, config.StatusWaiting); revertErr != nil {
					klog.Errorf("revert task %s status to waiting failed: %v", task.TaskID, revertErr)
				} else if !reverted {
					klog.V(4).Infof("task %s status already changed before revert", task.TaskID)
				}
				continue
			} else {
				klog.Infof("dispatched task: %s, streamID: %s", task.TaskID, id)
			}
		}
	}
}

// StartWorker subscribes to task dispatch topic and executes tasks.
func (w *Workflow) StartWorker(ctx context.Context, errChan chan error) {
	group := w.consumerGroup()
	consumer := w.consumerName()
	klog.Infof("worker reading stream: %s, group: %s, consumer: %s", w.dispatchTopic(), group, consumer)
	// Ensure consumer group exists once on worker start to avoid per-read overhead.
	if err := w.Queue.EnsureGroup(ctx, group); err != nil {
		klog.V(4).Infof("ensure group error: %v", err)
	}
	staleTicker := time.NewTicker(15 * time.Second)
	defer staleTicker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-staleTicker.C:
			// periodically claim stale pending messages
			mags, err := w.Queue.AutoClaim(ctx, group, consumer, 60*time.Second, 50)
			if err != nil {
				klog.V(4).Infof("auto-claim error: %v", err)
				continue
			}
			for _, m := range mags {
				td, err := UnmarshalTaskDispatch(m.Payload)
				if err != nil {
					klog.Errorf("decode dispatch (claim) failed: %v", err)
					_ = w.Queue.Ack(ctx, group, m.ID)
					continue
				}
				task, err := repository.TaskByID(ctx, w.Store, td.TaskID)
				if err != nil {
					klog.Errorf("load task %s failed: %v", td.TaskID, err)
					_ = w.Queue.Ack(ctx, group, m.ID)
					continue
				}
				if err := w.updateQueueAndRunTask(ctx, task, 1); err != nil {
					klog.Errorf("run task %s failed: %v", td.TaskID, err)
					_ = w.Queue.Ack(ctx, group, m.ID)
					continue
				}
				klog.Infof("consumer=%s acked message id=%s task=%s", consumer, m.ID, td.TaskID)
				_ = w.Queue.Ack(ctx, group, m.ID)
			}
		default:
			msgs, err := w.Queue.ReadGroup(ctx, group, consumer, 10, 2*time.Second)
			if err != nil {
				klog.V(4).Infof("read group error: %v", err)
				continue
			}
			for _, m := range msgs {
				td, err := UnmarshalTaskDispatch(m.Payload)
				if err != nil {
					klog.Errorf("decode dispatch failed: %v", err)
					_ = w.Queue.Ack(ctx, group, m.ID)
					continue
				}
				task, err := repository.TaskByID(ctx, w.Store, td.TaskID)
				if err != nil {
					klog.Errorf("load task %s failed: %v", td.TaskID, err)
					_ = w.Queue.Ack(ctx, group, m.ID)
					continue
				}
				if err := w.updateQueueAndRunTask(ctx, task, 1); err != nil {
					klog.Errorf("run task %s failed: %v", td.TaskID, err)
					_ = w.Queue.Ack(ctx, group, m.ID)
					continue
				}
				klog.Infof("consumer=%s acked claimed message id=%s task=%s", consumer, m.ID, td.TaskID)
				_ = w.Queue.Ack(ctx, group, m.ID)
			}
		}
	}
}

func (w *Workflow) dispatchTopic() string {
	prefix := ""
	if w.Cfg != nil {
		prefix = w.Cfg.Messaging.ChannelPrefix
	}
	if prefix == "" {
		prefix = "kubemin"
	}
	return fmt.Sprintf("%s.workflow.dispatch", prefix)
}

func (w *Workflow) consumerGroup() string { return "workflow-workers" }
func (w *Workflow) consumerName() string {
	if w.Cfg != nil {
		return w.Cfg.LeaderConfig.ID
	}
	return "worker"
}

type WorkflowCtl struct {
	workflowTask      *model.WorkflowQueue
	workflowTaskMutex sync.RWMutex
	Client            kubernetes.Interface
	Store             datastore.DataStore
	prefix            string
	ack               func()
}

type StepExecution struct {
	Name string
	Mode config.WorkflowMode
	Jobs map[int][]*model.JobTask
}

func NewWorkflowController(workflowTask *model.WorkflowQueue, client kubernetes.Interface, store datastore.DataStore) *WorkflowCtl {
	ctl := &WorkflowCtl{
		workflowTask: workflowTask,
		Store:        store,
		Client:       client,
		prefix:       fmt.Sprintf("workflowctl-%s-%s", workflowTask.WorkflowName, workflowTask.TaskID),
	}
	ctl.ack = ctl.updateWorkflowTask
	return ctl
}

// 更改工作流的状态或信息
func (w *WorkflowCtl) updateWorkflowTask() {
	taskInColl := w.workflowTask
	// 如果当前的task状态为：通过，暂停，超时，拒绝；则不处理，直接返回
	if taskInColl.Status == config.StatusPassed || taskInColl.Status == config.StatusFailed || taskInColl.Status == config.StatusTimeout || taskInColl.Status == config.StatusReject {
		klog.Infof("workflow %s, task %s, status %s: task already done, skipping update", taskInColl.WorkflowName, taskInColl.TaskID, taskInColl.Status)
		return
	}
	if err := w.Store.Put(context.Background(), w.workflowTask); err != nil {
		klog.Errorf("update task status error for workflow %s, task %s: %v", w.workflowTask.WorkflowName, w.workflowTask.TaskID, err)
	}
}

func (w *WorkflowCtl) Run(ctx context.Context, concurrency int) {
	// 1. Start a new trace for this workflow execution
	tracer := otel.Tracer("workflow-runner")
	ctx, span := tracer.Start(ctx, w.workflowTask.WorkflowName, trace.WithAttributes(
		attribute.String("workflow.name", w.workflowTask.WorkflowName),
		attribute.String("workflow.task_id", w.workflowTask.TaskID),
	))
	defer span.End()

	// 2. Create a logger with the traceID and put it in the context
	logger := klog.FromContext(ctx).WithValues("traceID", span.SpanContext().TraceID().String())
	ctx = klog.NewContext(ctx, logger)
	ctx = job.WithTaskMetadata(ctx, w.workflowTask.TaskID)

	// 将工作流的状态更改为运行中
	w.workflowTask.Status = config.StatusRunning
	w.workflowTask.CreateTime = time.Now()
	w.ack()
	logger.Info("Starting workflow", "workflowName", w.workflowTask.WorkflowName, "status", w.workflowTask.Status)

	defer func() {
		logger.Info("Finished workflow", "workflowName", w.workflowTask.WorkflowName, "status", w.workflowTask.Status)
		w.ack()
	}()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	stepExecutions := GenerateJobTasks(ctx, w.workflowTask, w.Store)
	seqLimit := 1
	if concurrency > 0 {
		seqLimit = concurrency
	}

	for _, stepExec := range stepExecutions {
		if stepExec.Jobs == nil {
			continue
		}
		priorities := sortedPriorities(stepExec.Jobs)
		for _, priority := range priorities {
			tasksInPriority := stepExec.Jobs[priority]
			if len(tasksInPriority) == 0 {
				continue
			}
			stepConcurrency := determineStepConcurrency(stepExec.Mode, len(tasksInPriority), seqLimit)
			logger.Info("Executing workflow step", "workflowName", w.workflowTask.WorkflowName, "step", stepExec.Name, "mode", stepExec.Mode, "priority", priority, "jobCount", len(tasksInPriority), "concurrency", stepConcurrency)

			job.RunJobs(ctx, tasksInPriority, stepConcurrency, w.Client, w.Store, w.ack)

			for _, task := range tasksInPriority {
				if task.Status != config.StatusCompleted {
					logger.Error(nil, "Workflow failed at job, aborting.", "workflowName", w.workflowTask.WorkflowName, "step", stepExec.Name, "priority", priority, "jobName", task.Name, "jobStatus", task.Status)
					w.workflowTask.Status = config.StatusFailed
					span.SetStatus(codes.Error, "Workflow failed")
					span.RecordError(fmt.Errorf("job %s failed with status %s", task.Name, task.Status))
					return
				}
			}
		}
		logger.Info("Workflow step completed successfully", "workflowName", w.workflowTask.WorkflowName, "step", stepExec.Name)
	}

	span.SetStatus(codes.Ok, "Workflow completed successfully")
	w.updateWorkflowStatus(ctx)
}

func GenerateJobTasks(ctx context.Context, task *model.WorkflowQueue, ds datastore.DataStore) []StepExecution {
	logger := klog.FromContext(ctx)
	workflow := model.Workflow{ID: task.WorkflowID}
	if err := ds.Get(ctx, &workflow); err != nil {
		logger.Error(err, "Failed to get workflow for generating job tasks", "workflowID", task.WorkflowID)
		return nil
	}

	stepsBytes, err := json.Marshal(workflow.Steps)
	if err != nil {
		logger.Error(err, "Failed to marshal workflow steps")
		return nil
	}

	var workflowSteps model.WorkflowSteps
	if err := json.Unmarshal(stepsBytes, &workflowSteps); err != nil {
		logger.Error(err, "Failed to unmarshal workflow steps")
		return nil
	}

	componentEntities, err := ds.List(ctx, &model.ApplicationComponent{AppID: task.AppID}, &datastore.ListOptions{})
	if err != nil {
		logger.Error(err, "Failed to list application components", "appID", task.AppID)
		return nil
	}
	componentMap := make(map[string]*model.ApplicationComponent)
	for _, entity := range componentEntities {
		if component, ok := entity.(*model.ApplicationComponent); ok {
			componentMap[component.Name] = component
		}
	}

	var executions []StepExecution
	totalJobs := 0

	for _, step := range workflowSteps.Steps {
		mode := step.Mode
		if mode == "" {
			mode = config.WorkflowModeStepByStep
		}
		if len(step.SubSteps) > 0 {
			if mode.IsParallel() {
				buckets := newJobBuckets()
				for _, sub := range step.SubSteps {
					subComponents := sub.ComponentNames()
					appendComponentGroup(ctx, buckets, subComponents, componentMap, task)
				}
				if !bucketsEmpty(buckets) {
					totalJobs += countJobs(buckets)
					stepName := step.Name
					if stepName == "" {
						stepName = "parallel-group"
					}
					executions = append(executions, StepExecution{Name: stepName, Mode: mode, Jobs: buckets})
					logGeneratedJobs(logger, task.WorkflowName, stepName, mode, buckets)
				}
			} else {
				for _, sub := range step.SubSteps {
					buckets := newJobBuckets()
					subComponents := sub.ComponentNames()
					appendComponentGroup(ctx, buckets, subComponents, componentMap, task)
					if bucketsEmpty(buckets) {
						continue
					}
					totalJobs += countJobs(buckets)
					displayName := sub.Name
					if displayName == "" && len(subComponents) == 1 {
						displayName = subComponents[0]
					}
					executions = append(executions, StepExecution{Name: displayName, Mode: config.WorkflowModeStepByStep, Jobs: buckets})
					logGeneratedJobs(logger, task.WorkflowName, displayName, config.WorkflowModeStepByStep, buckets)
				}
			}
			continue
		}

		componentNames := step.ComponentNames()
		if len(componentNames) == 0 {
			continue
		}
		if mode.IsParallel() && len(componentNames) > 1 {
			buckets := newJobBuckets()
			appendComponentGroup(ctx, buckets, componentNames, componentMap, task)
			if !bucketsEmpty(buckets) {
				totalJobs += countJobs(buckets)
				stepName := step.Name
				if stepName == "" && len(componentNames) > 1 {
					stepName = "parallel-group"
				}
				executions = append(executions, StepExecution{Name: stepName, Mode: mode, Jobs: buckets})
				logGeneratedJobs(logger, task.WorkflowName, stepName, mode, buckets)
			}
			continue
		}
		for _, name := range componentNames {
			buckets := newJobBuckets()
			appendComponentGroup(ctx, buckets, []string{name}, componentMap, task)
			if bucketsEmpty(buckets) {
				continue
			}
			totalJobs += countJobs(buckets)
			executions = append(executions, StepExecution{Name: name, Mode: config.WorkflowModeStepByStep, Jobs: buckets})
			logGeneratedJobs(logger, task.WorkflowName, name, config.WorkflowModeStepByStep, buckets)
		}
	}

	logger.Info("Generated total jobs for workflow", "totalJobs", totalJobs, "workflowName", task.WorkflowName)
	return executions
}

func NewJobTask(name, namespace, workflowID, projectID, appID string) *model.JobTask {
	return &model.JobTask{
		Name:       name,
		Namespace:  namespace,
		WorkflowID: workflowID,
		ProjectID:  projectID,
		AppID:      appID,
		Status:     config.StatusQueued,
		Timeout:    60,
	}
}

// 更改工作流队列的状态，并运行它
func (w *Workflow) updateQueueAndRunTask(ctx context.Context, task *model.WorkflowQueue, jobConcurrency int) error {
	//将状态更改为队列中
	task.Status = config.StatusQueued
	if success := w.WorkflowService.UpdateTask(ctx, task); !success {
		klog.Errorf("update task status error for workflow %s, task %s", task.WorkflowName, task.TaskID)
		return fmt.Errorf("update task status error for workflow %s, task %s", task.WorkflowName, task.TaskID)
	}

	sequentialConcurrency := jobConcurrency
	if w.Cfg != nil && w.Cfg.Workflow.SequentialMaxConcurrency > 0 {
		sequentialConcurrency = w.Cfg.Workflow.SequentialMaxConcurrency
	}
	// 执行新的任务
	go NewWorkflowController(task, w.KubeClient, w.Store).Run(ctx, sequentialConcurrency)
	return nil
}

func (w *WorkflowCtl) updateWorkflowStatus(ctx context.Context) {
	w.workflowTask.Status = config.StatusCompleted
	err := w.Store.Put(ctx, w.workflowTask)
	if err != nil {
		klog.Errorf("update Workflow status err: %v", err)
	}
}

func ParseProperties(ctx context.Context, properties *model.JSONStruct) model.Properties {
	logger := klog.FromContext(ctx)
	cProperties, err := json.Marshal(properties)
	if err != nil {
		logger.Error(err, "Component.Properties deserialization failure")
		return model.Properties{}
	}

	var propertied model.Properties
	err = json.Unmarshal(cProperties, &propertied)
	if err != nil {
		logger.Error(err, "WorkflowSteps deserialization failure")
		return model.Properties{}
	}
	return propertied
}

func CreateObjectJobsFromResult(additionalObjects []client.Object, component *model.ApplicationComponent, task *model.WorkflowQueue, jobs []*model.JobTask) ([]*model.JobTask, error) {
	if len(additionalObjects) == 0 {
		return jobs, nil
	}

	for _, obj := range additionalObjects {
		if pvc, ok := obj.(*corev1.PersistentVolumeClaim); ok {
			ns := pvc.Namespace
			if ns == "" {
				ns = component.Namespace
				pvc.Namespace = ns
			}
			pvcJob := NewJobTask(
				pvc.Name,
				ns,
				task.WorkflowID,
				task.ProjectID,
				task.AppID,
			)
			pvcJob.JobType = string(config.JobDeployPVC)
			pvcJob.JobInfo = pvc

			jobs = append(jobs, pvcJob)
			klog.Infof("Created PVC job for component %s: %s", component.Name, pvc.Name)
		}
		if ingress, ok := obj.(*networkingv1.Ingress); ok {
			baseName := nameOrFallback(ingress.Name, component.Name)
			normalizedName := job.BuildIngressName(baseName, component.AppID)
			ingress.Name = normalizedName
			if ingress.Namespace == "" {
				ingress.Namespace = component.Namespace
			}
			ingressJob := NewJobTask(
				ingress.Name,
				ingress.Namespace,
				task.WorkflowID,
				task.ProjectID,
				task.AppID,
			)
			ingressJob.JobType = string(config.JobDeployIngress)
			ingressJob.JobInfo = ingress
			jobs = append(jobs, ingressJob)
			klog.Infof("Created Ingress job for component %s: %s", component.Name, ingress.Name)
		}
		if sa, ok := obj.(*corev1.ServiceAccount); ok {
			ns := sa.Namespace
			if ns == "" {
				ns = component.Namespace
				sa.Namespace = ns
			}
			jobTask := NewJobTask(
				sa.Name,
				ns,
				task.WorkflowID,
				task.ProjectID,
				task.AppID,
			)
			jobTask.JobType = string(config.JobDeployServiceAccount)
			jobTask.JobInfo = sa.DeepCopy()
			jobs = append(jobs, jobTask)
			klog.Infof("Created ServiceAccount job for component %s: %s/%s", component.Name, ns, sa.Name)
		}
		if role, ok := obj.(*rbacv1.Role); ok {
			ns := role.Namespace
			if ns == "" {
				ns = component.Namespace
				role.Namespace = ns
			}
			jobTask := NewJobTask(
				role.Name,
				ns,
				task.WorkflowID,
				task.ProjectID,
				task.AppID,
			)
			jobTask.JobType = string(config.JobDeployRole)
			jobTask.JobInfo = role.DeepCopy()
			jobs = append(jobs, jobTask)
			klog.Infof("Created Role job for component %s: %s/%s", component.Name, ns, role.Name)
		}
		if binding, ok := obj.(*rbacv1.RoleBinding); ok {
			ns := binding.Namespace
			if ns == "" {
				ns = component.Namespace
				binding.Namespace = ns
			}
			jobTask := NewJobTask(
				binding.Name,
				ns,
				task.WorkflowID,
				task.ProjectID,
				task.AppID,
			)
			jobTask.JobType = string(config.JobDeployRoleBinding)
			jobTask.JobInfo = binding.DeepCopy()
			jobs = append(jobs, jobTask)
			klog.Infof("Created RoleBinding job for component %s: %s/%s", component.Name, ns, binding.Name)
		}
		if clusterRole, ok := obj.(*rbacv1.ClusterRole); ok {
			jobTask := NewJobTask(
				clusterRole.Name,
				component.Namespace,
				task.WorkflowID,
				task.ProjectID,
				task.AppID,
			)
			jobTask.JobType = string(config.JobDeployClusterRole)
			jobTask.JobInfo = clusterRole.DeepCopy()
			jobs = append(jobs, jobTask)
			klog.Infof("Created ClusterRole job for component %s: %s", component.Name, clusterRole.Name)
		}
		if clusterBinding, ok := obj.(*rbacv1.ClusterRoleBinding); ok {
			jobTask := NewJobTask(
				clusterBinding.Name,
				component.Namespace,
				task.WorkflowID,
				task.ProjectID,
				task.AppID,
			)
			jobTask.JobType = string(config.JobDeployClusterRoleBinding)
			jobTask.JobInfo = clusterBinding.DeepCopy()
			jobs = append(jobs, jobTask)
			klog.Infof("Created ClusterRoleBinding job for component %s: %s", component.Name, clusterBinding.Name)
		}
	}
	return jobs, nil
}

func appendComponentGroup(ctx context.Context, buckets map[int][]*model.JobTask, componentNames []string, componentMap map[string]*model.ApplicationComponent, task *model.WorkflowQueue) {
	logger := klog.FromContext(ctx)
	for _, name := range componentNames {
		component, ok := componentMap[name]
		if !ok {
			logger.Info("Component referenced in workflow step not found", "componentName", name)
			continue
		}
		componentBuckets := buildJobsForComponent(ctx, component, task)
		mergeJobBuckets(buckets, componentBuckets)
	}
}

func buildJobsForComponent(ctx context.Context, component *model.ApplicationComponent, task *model.WorkflowQueue) map[int][]*model.JobTask {
	logger := klog.FromContext(ctx)
	buckets := newJobBuckets()
	if component == nil {
		return buckets
	}

	namespace := component.Namespace
	if namespace == "" {
		namespace = config.DefaultNamespace
		component.Namespace = namespace
	}

	properties := ParseProperties(ctx, component.Properties)

	switch component.ComponentType {
	case config.ServerJob:
		serviceJobs := job.GenerateWebService(component, &properties)
		queueServiceJobs(logger, buckets, component, task, namespace, config.JobDeploy, serviceJobs)
	case config.StoreJob:
		storeJobs := job.GenerateStoreService(component)
		queueServiceJobs(logger, buckets, component, task, namespace, config.JobDeployStore, storeJobs)

	case config.ConfJob:
		jobTask := NewJobTask(component.Name, namespace, task.WorkflowID, task.ProjectID, task.AppID)
		jobTask.JobType = string(config.JobDeployConfigMap)
		jobTask.JobInfo = job.GenerateConfigMap(component, &properties)
		buckets[config.JobPriorityMaxHigh] = append(buckets[config.JobPriorityMaxHigh], jobTask)

	case config.SecretJob:
		jobTask := NewJobTask(component.Name, namespace, task.WorkflowID, task.ProjectID, task.AppID)
		jobTask.JobType = string(config.JobDeploySecret)
		jobTask.JobInfo = job.GenerateSecret(component, &properties)
		buckets[config.JobPriorityMaxHigh] = append(buckets[config.JobPriorityMaxHigh], jobTask)
	}

	if len(properties.Ports) > 0 {
		svcJob := NewJobTask(component.Name, namespace, task.WorkflowID, task.ProjectID, task.AppID)
		svcJob.JobType = string(config.JobDeployService)
		svcJob.JobInfo = job.GenerateService(component, &properties)
		buckets[config.JobPriorityNormal] = append(buckets[config.JobPriorityNormal], svcJob)
	}

	return buckets
}

func queueServiceJobs(
	logger klog.Logger,
	buckets map[int][]*model.JobTask,
	component *model.ApplicationComponent,
	task *model.WorkflowQueue,
	namespace string,
	jobType config.JobType,
	result *job.GenerateServiceResult,
) {
	if result == nil {
		return
	}

	appendJob := func(priority int, job *model.JobTask) {
		if job == nil {
			return
		}
		buckets[priority] = append(buckets[priority], job)
	}

	// Traits may emit extra Kubernetes objects (PVC, Ingress, etc.). Schedule them
	// ahead of the base workload so dependencies are ready before the deployment runs.
	if len(result.AdditionalObjects) > 0 {
		jobs, err := CreateObjectJobsFromResult(result.AdditionalObjects, component, task, nil)
		if err != nil {
			logger.Error(err, "Failed to create additional resource jobs", "componentName", component.Name)
		} else {
			for _, jt := range jobs {
				appendJob(config.JobPriorityHigh, jt)
			}
		}
	}

	jobTask := NewJobTask(component.Name, namespace, task.WorkflowID, task.ProjectID, task.AppID)
	jobTask.JobType = string(jobType)
	jobTask.JobInfo = result.Service
	appendJob(config.JobPriorityNormal, jobTask)
}

func newJobBuckets() map[int][]*model.JobTask {
	return map[int][]*model.JobTask{
		config.JobPriorityMaxHigh: {},
		config.JobPriorityHigh:    {},
		config.JobPriorityNormal:  {},
		config.JobPriorityLow:     {},
	}
}

func mergeJobBuckets(dst, src map[int][]*model.JobTask) {
	for priority, jobs := range src {
		if len(jobs) == 0 {
			continue
		}
		dst[priority] = append(dst[priority], jobs...)
	}
}

func bucketsEmpty(buckets map[int][]*model.JobTask) bool {
	for _, jobs := range buckets {
		if len(jobs) > 0 {
			return false
		}
	}
	return true
}

func countJobs(buckets map[int][]*model.JobTask) int {
	count := 0
	for _, jobs := range buckets {
		count += len(jobs)
	}
	return count
}

func logGeneratedJobs(logger klog.Logger, workflowName, stepName string, mode config.WorkflowMode, buckets map[int][]*model.JobTask) {
	for priority, jobs := range buckets {
		if len(jobs) == 0 {
			continue
		}
		logger.Info("Generated jobs for workflow step", "workflowName", workflowName, "step", stepName, "mode", mode, "priority", priority, "jobCount", len(jobs))
		for _, j := range jobs {
			logger.Info("Generated job details", "workflowName", workflowName, "step", stepName, "jobName", j.Name, "jobType", j.JobType, "priority", priority)
		}
	}
}

func determineStepConcurrency(mode config.WorkflowMode, jobCount, sequentialLimit int) int {
	if jobCount <= 0 {
		return 0
	}
	if mode.IsParallel() {
		return jobCount
	}
	if sequentialLimit < 1 {
		sequentialLimit = 1
	}
	if jobCount < sequentialLimit {
		return jobCount
	}
	return sequentialLimit
}

func sortedPriorities(jobs map[int][]*model.JobTask) []int {
	var priorities []int
	for priority := range jobs {
		priorities = append(priorities, priority)
	}
	sort.Ints(priorities)
	return priorities
}

func nameOrFallback(name, fallback string) string {
	if name != "" {
		return name
	}
	return fallback
}
