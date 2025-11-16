package workflow

import (
	"context"
	"fmt"

	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"

	"KubeMin-Cli/pkg/apiserver/config"
	"KubeMin-Cli/pkg/apiserver/domain/model"
	"KubeMin-Cli/pkg/apiserver/domain/service"
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
	taskGroup       *errgroup.Group
	taskGroupCtx    context.Context
	errChan         chan error
	workflowLimiter *semaphore.Weighted
}

func (w *Workflow) Start(ctx context.Context, errChan chan error) {
	w.InitQueue(ctx)
	w.errChan = errChan
	w.taskGroup, w.taskGroupCtx = errgroup.WithContext(ctx)
	if max := w.maxWorkflowConcurrency(); max > 0 {
		w.workflowLimiter = semaphore.NewWeighted(max)
	}
	go func() {
		<-ctx.Done()
		if w.taskGroup != nil {
			if err := w.taskGroup.Wait(); err != nil {
				w.reportTaskError(err)
			}
		}
	}()
	klog.Infof("workflow runtime config: localPoll=%s dispatchPoll=%s workerStale=%s autoClaimIdle=%s autoClaimCount=%d workerReadCount=%d workerReadBlock=%s",
		w.localPollInterval(),
		w.dispatchPollInterval(),
		w.workerStaleInterval(),
		w.workerAutoClaimMinIdle(),
		w.workerAutoClaimCount(),
		w.workerReadCount(),
		w.workerReadBlock(),
	)
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

func (w *Workflow) runWorkflowTask(ctx context.Context, task *model.WorkflowQueue, concurrency int) {
	runnerCtx := ctx
	if w.taskGroupCtx != nil {
		runnerCtx = w.taskGroupCtx
	}
	acquired := false
	if w.workflowLimiter != nil {
		if err := w.workflowLimiter.Acquire(runnerCtx, 1); err != nil {
			w.reportTaskError(fmt.Errorf("acquire workflow slot: %w", err))
			return
		}
		acquired = true
	}
	if w.taskGroup != nil {
		taskCopy := task
		w.taskGroup.Go(func() error {
			controller := NewWorkflowController(taskCopy, w.KubeClient, w.Store, w.Cfg)
			err := controller.Run(runnerCtx, concurrency)
			if acquired {
				w.workflowLimiter.Release(1)
			}
			if err != nil {
				w.reportTaskError(err)
			}
			return err
		})
		return
	}
	go func() {
		controller := NewWorkflowController(task, w.KubeClient, w.Store, w.Cfg)
		err := controller.Run(runnerCtx, concurrency)
		if acquired {
			w.workflowLimiter.Release(1)
		}
		if err != nil {
			w.reportTaskError(err)
		}
	}()
}

func (w *Workflow) reportTaskError(err error) {
	if err == nil {
		return
	}
	if w.errChan != nil {
		select {
		case w.errChan <- err:
		default:
			klog.Errorf("workflow task error: %v", err)
		}
	} else {
		klog.Errorf("workflow task error: %v", err)
	}
}

func (w *Workflow) maxWorkflowConcurrency() int64 {
	if w.Cfg != nil && w.Cfg.Workflow.MaxConcurrentWorkflows > 0 {
		return int64(w.Cfg.Workflow.MaxConcurrentWorkflows)
	}
	return int64(config.DefaultMaxConcurrentWorkflows)
}

// 更改工作流队列的状态，并运行它
func (w *Workflow) updateQueueAndRunTask(ctx context.Context, task *model.WorkflowQueue, jobConcurrency int) error {
	//将状态更改为队列中
	task.Status = config.StatusQueued
	if success := w.updateTask(ctx, task); !success {
		klog.Errorf("update task status error for workflow %s, task %s", task.WorkflowName, task.TaskID)
		return fmt.Errorf("update task status error for workflow %s, task %s", task.WorkflowName, task.TaskID)
	}

	sequentialConcurrency := jobConcurrency
	if w.Cfg != nil && w.Cfg.Workflow.SequentialMaxConcurrency > 0 {
		sequentialConcurrency = w.Cfg.Workflow.SequentialMaxConcurrency
	}
	w.runWorkflowTask(ctx, task, sequentialConcurrency)
	return nil
}
