package event

import (
	"context"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/util/workqueue"
)

type Task struct {
	ID   string
	Name string
}

// ApplicationSync sync application from cluster to database
type ApplicationSync struct {
	KubeConfig *rest.Config `inject:"kubeConfig"`
	Queue      workqueue.TypedRateLimitingInterface[Task]
}

// Start prepares watchers and run their controllers, then waits for process termination signals
func (a *ApplicationSync) Start(ctx context.Context, errChan chan error) {
	//dynamicClient, err := dynamic.NewForConfig(a.KubeConfig)
	//if err != nil {
	//	errChan <- err
	//}
	// TODO 这里创建一个informerFactory的实例，然后监听Pod的变化

}
