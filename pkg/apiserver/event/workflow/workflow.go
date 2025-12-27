package workflow

import (
	"context"
	"fmt"

	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"

	"kubemin-cli/pkg/apiserver/config"
	"kubemin-cli/pkg/apiserver/domain/model"
	"kubemin-cli/pkg/apiserver/domain/service"
	"kubemin-cli/pkg/apiserver/infrastructure/datastore"
	msg "kubemin-cli/pkg/apiserver/infrastructure/messaging"
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

// InitQueue 在服务启动时调用，将所有"运行中"的任务重新入队
// 因为 Job 是通过 goroutine 执行的，进程重启后所有 goroutine 都会死亡
// 分布式场景下，Redis Streams 的 AutoClaim 机制会自动处理 pending 消息
func (w *Workflow) InitQueue(ctx context.Context) {
	if w.Store == nil {
		klog.Error("datastore is nil")
		return
	}

	// 从数据库中查找所有"运行中"的任务
	tasks, err := w.WorkflowService.TaskRunning(ctx)
	if err != nil {
		klog.Errorf("find task running error: %v", err)
		return
	}

	if len(tasks) == 0 {
		klog.Info("InitQueue: no running tasks to re-queue")
		return
	}

	// 进程重启，所有 running 的任务都需要重新入队
	var requeued, failed int
	for _, task := range tasks {
		task.Status = config.StatusWaiting
		if err := w.Store.Put(ctx, task); err != nil {
			klog.Errorf("re-queue task %s error: %v", task.TaskID, err)
			failed++
			continue
		}
		klog.Infof("re-queued task: %s (workflow=%s)", task.TaskID, task.WorkflowName)
		requeued++
	}

	klog.Infof("InitQueue completed: requeued=%d failed=%d", requeued, failed)
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

// reportTaskError logs workflow task errors.
// Note: Workflow task failures are expected business errors (e.g., deployment failures,
// validation errors) and should NOT cause the server to exit. Only infrastructure errors
// (e.g., Redis connection failures, database errors) should trigger service termination.
// Therefore, we only log the error instead of sending it to errChan.
func (w *Workflow) reportTaskError(err error) {
	if err == nil {
		return
	}
	klog.Errorf("workflow task error: %v", err)
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
