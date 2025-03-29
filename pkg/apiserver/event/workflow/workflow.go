package workflow

import (
	"KubeMin-Cli/pkg/apiserver/domain/service"
	"KubeMin-Cli/pkg/apiserver/infrastructure/datastore"
	"context"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"time"
)

type Workflow struct {
	KubeClient      client.Client `inject:"kubeClient"`
	Store           datastore.DataStore
	workflowService service.WorkflowService
}

func (w *Workflow) Start(ctx context.Context, errChan chan error) {
	w.InitQueue()
	go w.WorkflowTaskSender()
}

func (w *Workflow) InitQueue() {
	// 从数据库中查找未完成的任务
	// 如果重启Queue，则取消所有正在运行的tasks

}

func (w *Workflow) WorkflowTaskSender() {
	for {
		time.Sleep(time.Second * 3)
		//获取等待的任务
		//waitingTasks, err := w.workflowService.WaitingTasks()
	}
}
