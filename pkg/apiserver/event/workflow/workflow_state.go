package workflow

import (
	"context"
	"time"

	"KubeMin-Cli/pkg/apiserver/config"
	"KubeMin-Cli/pkg/apiserver/domain/model"
)

func (w *Workflow) waitingTasks(ctx context.Context) ([]*model.WorkflowQueue, error) {
	queryCtx, cancel := context.WithTimeout(ctx, config.WaitingTasksQueryTimeout)
	defer cancel()
	return w.WorkflowService.WaitingTasks(queryCtx)
}

func (w *Workflow) markTaskStatus(ctx context.Context, taskID string, from, to config.Status) (bool, error) {
	statusCtx, cancel := context.WithTimeout(ctx, config.TaskStateTransitionTimeout)
	defer cancel()
	return w.WorkflowService.MarkTaskStatus(statusCtx, taskID, from, to)
}

func (w *Workflow) updateTask(ctx context.Context, task *model.WorkflowQueue) bool {
	updateCtx, cancel := context.WithTimeout(ctx, config.TaskStateTransitionTimeout)
	defer cancel()
	return w.WorkflowService.UpdateTask(updateCtx, task)
}

func (w *Workflow) enqueueDispatch(ctx context.Context, payload []byte) (string, error) {
	enqueueCtx, cancel := context.WithTimeout(ctx, config.QueueDispatchTimeout)
	defer cancel()
	return w.Queue.Enqueue(enqueueCtx, payload)
}

func (w *Workflow) localPollInterval() time.Duration {
	if w.Cfg != nil && w.Cfg.Workflow.LocalPollInterval > 0 {
		return w.Cfg.Workflow.LocalPollInterval
	}
	return config.DefaultLocalPollInterval
}

func (w *Workflow) dispatchPollInterval() time.Duration {
	if w.Cfg != nil && w.Cfg.Workflow.DispatchPollInterval > 0 {
		return w.Cfg.Workflow.DispatchPollInterval
	}
	return config.DefaultDispatchPollInterval
}

func (w *Workflow) workerStaleInterval() time.Duration {
	if w.Cfg != nil && w.Cfg.Workflow.WorkerStaleInterval > 0 {
		return w.Cfg.Workflow.WorkerStaleInterval
	}
	return config.DefaultWorkerStaleInterval
}

func (w *Workflow) workerAutoClaimMinIdle() time.Duration {
	if w.Cfg != nil && w.Cfg.Workflow.WorkerAutoClaimMinIdle > 0 {
		return w.Cfg.Workflow.WorkerAutoClaimMinIdle
	}
	return config.DefaultWorkerAutoClaimIdle
}

func (w *Workflow) workerAutoClaimCount() int {
	if w.Cfg != nil && w.Cfg.Workflow.WorkerAutoClaimCount > 0 {
		return w.Cfg.Workflow.WorkerAutoClaimCount
	}
	return config.DefaultWorkerAutoClaimCount
}

func (w *Workflow) workerReadCount() int {
	if w.Cfg != nil && w.Cfg.Workflow.WorkerReadCount > 0 {
		return w.Cfg.Workflow.WorkerReadCount
	}
	return config.DefaultWorkerReadCount
}

func (w *Workflow) workerReadBlock() time.Duration {
	if w.Cfg != nil && w.Cfg.Workflow.WorkerReadBlock > 0 {
		return w.Cfg.Workflow.WorkerReadBlock
	}
	return config.DefaultWorkerReadBlock
}
