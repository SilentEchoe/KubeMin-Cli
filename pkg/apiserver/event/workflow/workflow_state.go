package workflow

import (
	"KubeMin-Cli/pkg/apiserver/config"
	"KubeMin-Cli/pkg/apiserver/domain/model"
	"context"
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
