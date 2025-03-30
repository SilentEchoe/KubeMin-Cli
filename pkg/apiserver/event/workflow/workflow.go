package workflow

import (
	"KubeMin-Cli/pkg/apiserver/domain/model"
	"KubeMin-Cli/pkg/apiserver/domain/service"
	"KubeMin-Cli/pkg/apiserver/infrastructure/datastore"
	"context"
	"fmt"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sync"
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
		ctx := context.Background()
		waitingTasks, err := w.workflowService.WaitingTasks(ctx)
		if err != nil || len(waitingTasks) == 0 {
			continue
		}

		for _, task := range waitingTasks {
			// TODO 判断Task是否符合执行条件

			if success := w.workflowService.UpdateQueue(ctx, task); !success {
				continue
			}
			// go

		}

	}
}

type workflowCtl struct {
	workflowTask      *model.WorkflowQueue
	workflowTaskMutex sync.RWMutex
	prefix            string
	ack               func()
}

func NewWorkflowController(workflowTask *model.WorkflowQueue) *workflowCtl {
	ctl := &workflowCtl{
		workflowTask: workflowTask,
		prefix:       fmt.Sprintf("workflowctl-%s-%d", workflowTask.WorkflowName, workflowTask.TaskID),
	}
	//ctl.ack = ctl
	return ctl
}

func (w *workflowCtl) updateWorkflowTask() {

}
