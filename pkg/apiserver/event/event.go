package event

import (
	"KubeMin-Cli/pkg/apiserver/event/workflow"
	"context"
)

var workers []Worker

// Worker handle events through rotation training, listener and crontab.
type Worker interface {
	Start(ctx context.Context, errChan chan error)
}

// LeaderAware marks a worker that reacts to leader state changes.
// Implemented by distributed workers to switch between write and execute roles.
type LeaderAware interface {
	SetAsLeader(isLeader bool)
}

// InitEvent init all event worker
func InitEvent() []interface{} {
	workflowCol := &workflow.Workflow{}
	workers = append(workers, workflowCol)
	return []interface{}{workflowCol}
}

// StartEventWorker start all event worker
func StartEventWorker(ctx context.Context, errChan chan error) {
	for i := range workers {
		go workers[i].Start(ctx, errChan)
	}
}

// GetWorkers get all registered workers
func GetWorkers() []Worker {
	return workers
}
