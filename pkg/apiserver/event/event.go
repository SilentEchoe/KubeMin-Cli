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

// WorkerSubscriber is optional interface for workers that can subscribe to a message bus.
type WorkerSubscriber interface {
    StartWorker(ctx context.Context, errChan chan error)
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

// StartWorkerSubscriber starts message subscribers for workers that implement WorkerSubscriber.
func StartWorkerSubscriber(ctx context.Context, errChan chan error) {
    for i := range workers {
        if ws, ok := workers[i].(WorkerSubscriber); ok {
            go ws.StartWorker(ctx, errChan)
        }
    }
}
