package event

import (
	"KubeMin-Cli/pkg/apiserver/event/collect"
	"context"
	"k8s.io/client-go/util/workqueue"
)

var workers []Worker

// Worker handle events through rotation training, listener and crontab.
type Worker interface {
	Start(ctx context.Context, errChan chan error)
}

// InitEvent init all event worker
func InitEvent() []interface{} {
	application := &ApplicationSync{
		Queue: workqueue.NewTypedRateLimitingQueue[any](workqueue.DefaultTypedControllerRateLimiter[any]()),
	}

	collect := &collect.InfoCalculateCronJob{}
	workers = append(workers, application, collect)
	return []interface{}{application, collect}
}

// StartEventWorker start all event worker
func StartEventWorker(ctx context.Context, errChan chan error) {
	for i := range workers {
		go workers[i].Start(ctx, errChan)
	}
}
