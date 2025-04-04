package event

import (
	workflow "KubeMin-Cli/pkg/apiserver/event/workflow"
	"context"
)

var workers []Worker

// Worker handle events through rotation training, listener and crontab.
type Worker interface {
	Start(ctx context.Context, errChan chan error)
}

// InitEvent init all event worker
func InitEvent() []interface{} {
	//application := &sync.ApplicationSync{
	//	Queue: workqueue.NewTypedRateLimitingQueue[any](workqueue.DefaultTypedControllerRateLimiter[any]()),
	//}

	//collect := &collect.InfoCalculateCronJob{}
	//workers = append(workers, application, collect, workflow)

	workflow := &workflow.Workflow{}
	//debug code
	workers = append(workers, workflow)
	return []interface{}{workflow}
}

// StartEventWorker start all event worker
func StartEventWorker(ctx context.Context, errChan chan error) {
	for i := range workers {
		go workers[i].Start(ctx, errChan)
	}
}
