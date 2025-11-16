package workflow

import (
	"context"
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
	"KubeMin-Cli/pkg/apiserver/event/workflow/job"
	"KubeMin-Cli/pkg/apiserver/infrastructure/datastore"
)

type WorkflowCtl struct {
	workflowTask             *model.WorkflowQueue
	workflowTaskMutex        sync.RWMutex
	Client                   kubernetes.Interface
	Store                    datastore.DataStore
	prefix                   string
	ack                      func()
	defaultJobTimeoutSeconds int64
}

func NewWorkflowController(workflowTask *model.WorkflowQueue, client kubernetes.Interface, store datastore.DataStore, cfg *config.Config) *WorkflowCtl {
	ctl := &WorkflowCtl{
		workflowTask:             workflowTask,
		Store:                    store,
		Client:                   client,
		prefix:                   fmt.Sprintf("workflowctl-%s-%s", workflowTask.WorkflowName, workflowTask.TaskID),
		defaultJobTimeoutSeconds: resolveDefaultJobTimeout(cfg),
	}
	ctl.ack = ctl.updateWorkflowTask
	return ctl
}

// 更改工作流的状态或信息
func (w *WorkflowCtl) updateWorkflowTask() {
	taskSnapshot := w.snapshotTask()
	// 如果当前的task状态为：通过，暂停，超时，拒绝；则不处理，直接返回
	if isWorkflowTerminal(taskSnapshot.Status) {
		klog.Infof("workflow %s, task %s, status %s: task already done, skipping update", taskSnapshot.WorkflowName, taskSnapshot.TaskID, taskSnapshot.Status)
		return
	}
	if err := w.Store.Put(context.Background(), &taskSnapshot); err != nil {
		klog.Errorf("update task status error for workflow %s, task %s: %v", taskSnapshot.WorkflowName, taskSnapshot.TaskID, err)
	}
}

func (w *WorkflowCtl) Run(ctx context.Context, concurrency int) error {
	// 1. Start a new trace for this workflow execution
	tracer := otel.Tracer("workflow-runner")
	taskMeta := w.snapshotTask()
	workflowName := taskMeta.WorkflowName
	ctx, span := tracer.Start(ctx, workflowName, trace.WithAttributes(
		attribute.String("workflow.name", workflowName),
		attribute.String("workflow.task_id", taskMeta.TaskID),
	))
	defer span.End()

	// 2. Create a logger with the traceID and put it in the context
	logger := klog.FromContext(ctx).WithValues(
		"traceID", span.SpanContext().TraceID().String(),
		"workflowName", workflowName,
		"taskID", taskMeta.TaskID,
	)
	ctx = klog.NewContext(ctx, logger)
	ctx = job.WithTaskMetadata(ctx, taskMeta.TaskID)

	// 将工作流的状态更改为运行中
	w.mutateTask(func(task *model.WorkflowQueue) {
		task.Status = config.StatusRunning
		task.CreateTime = time.Now()
	})
	w.ack()
	logger.Info("Starting workflow", "status", w.snapshotTask().Status)

	defer func() {
		logger.Info("Finished workflow", "status", w.snapshotTask().Status)
		w.ack()
	}()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	taskForGeneration := w.snapshotTask()
	stepExecutions := GenerateJobTasks(ctx, &taskForGeneration, w.Store, w.defaultJobTimeoutSeconds)
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
			logger.Info("Executing workflow step", "workflowName", workflowName, "step", stepExec.Name, "mode", stepExec.Mode, "priority", priority, "jobCount", len(tasksInPriority), "concurrency", stepConcurrency)

			job.RunJobs(ctx, tasksInPriority, stepConcurrency, w.Client, w.Store, w.ack, stepExec.Mode.IsParallel())

			for _, task := range tasksInPriority {
				if task.Status != config.StatusCompleted {
					err := fmt.Errorf("workflow %s failed at job %s (status=%s)", workflowName, task.Name, task.Status)
					logger.Error(err, "Workflow failed at job, aborting.", "step", stepExec.Name, "priority", priority, "jobName", task.Name, "jobStatus", task.Status)
					w.setStatus(config.StatusFailed)
					span.SetStatus(codes.Error, "Workflow failed")
					span.RecordError(err)
					return err
				}
			}
		}
		logger.Info("Workflow step completed successfully", "workflowName", workflowName, "step", stepExec.Name)
	}

	span.SetStatus(codes.Ok, "Workflow completed successfully")
	w.updateWorkflowStatus(ctx)
	return nil
}

func (w *WorkflowCtl) updateWorkflowStatus(ctx context.Context) {
	w.setStatus(config.StatusCompleted)
	taskSnapshot := w.snapshotTask()
	err := w.Store.Put(ctx, &taskSnapshot)
	if err != nil {
		klog.Errorf("update Workflow status err: %v", err)
	}
}

func resolveDefaultJobTimeout(cfg *config.Config) int64 {
	if cfg != nil && cfg.Workflow.DefaultJobTimeout > 0 {
		seconds := int64(cfg.Workflow.DefaultJobTimeout / time.Second)
		if seconds > 0 {
			return seconds
		}
	}
	return config.DefaultJobTaskTimeoutSeconds
}

func (w *WorkflowCtl) mutateTask(mut func(task *model.WorkflowQueue)) {
	w.workflowTaskMutex.Lock()
	defer w.workflowTaskMutex.Unlock()
	mut(w.workflowTask)
}

func (w *WorkflowCtl) snapshotTask() model.WorkflowQueue {
	w.workflowTaskMutex.RLock()
	defer w.workflowTaskMutex.RUnlock()
	return *w.workflowTask
}

func (w *WorkflowCtl) setStatus(status config.Status) {
	w.mutateTask(func(task *model.WorkflowQueue) {
		task.Status = status
	})
}

func isWorkflowTerminal(status config.Status) bool {
	return status == config.StatusPassed ||
		status == config.StatusFailed ||
		status == config.StatusTimeout ||
		status == config.StatusReject
}
