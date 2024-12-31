package event

import (
	"context"
	"k8s.io/client-go/util/workqueue"
)

type Task struct {
	ID   string
	Name string
}

// ApplicationSync sync application from cluster to database
// TODO RateLimitingInterface Replace TypedRateLimitingInterface
type ApplicationSync struct {
	Queue workqueue.TypedRateLimitingInterface[Task]
}

func (a ApplicationSync) Start(ctx context.Context, errChan chan error) {
	//TODO implement me
	panic("implement me")
}
