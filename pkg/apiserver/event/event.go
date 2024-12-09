package event

import "context"

var workers []Worker

// Worker handle events through rotation training, listener and crontab.
type Worker interface {
	Start(ctx context.Context, errChan chan error)
}

// InitEvent init all event worker
func InitEvent() []interface{} {
	return []interface{}{}
}

// StartEventWorker start all event worker
func StartEventWorker(ctx context.Context, errChan chan error) {
	for i := range workers {
		go workers[i].Start(ctx, errChan)
	}
}
