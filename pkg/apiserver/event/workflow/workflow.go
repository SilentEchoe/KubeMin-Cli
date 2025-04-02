package workflow

import (
	"KubeMin-Cli/pkg/apiserver/config"
	"KubeMin-Cli/pkg/apiserver/domain/model"
	"KubeMin-Cli/pkg/apiserver/domain/service"
	"KubeMin-Cli/pkg/apiserver/infrastructure/datastore"
	"context"
	"fmt"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sync"
	"time"
)

type Workflow struct {
	KubeClient      client.Client `inject:"kubeClient"`
	KubeConfig      *rest.Config  `inject:"kubeConfig"`
	Store           datastore.DataStore
	workflowService service.WorkflowService
}

func (w *Workflow) Start(ctx context.Context, errChan chan error) {
	w.InitQueue()
	go w.WorkflowTaskSender()
}

func (w *Workflow) InitQueue() {
	ctx := context.Background()
	// 从数据库中查找未完成的任务
	tasks, err := w.workflowService.TaskRunning(ctx)
	if err != nil {
		klog.Errorf(fmt.Sprintf("find task running error:%s", err))
		return
	}
	// 如果重启Queue，则取消所有正在运行的tasks
	for _, task := range tasks {
		err := w.workflowService.CancelWorkflowTask(ctx, config.DefaultTaskRevoker, task.ID)
		if err != nil {
			klog.Errorf(fmt.Sprintf("cance task error:%s", err))
			return
		}
	}
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

			if success := w.workflowService.UpdateTask(ctx, task); !success {
				continue
			}
			// go

		}

	}
}

type workflowCtl struct {
	workflowTask      *model.WorkflowQueue
	workflowTaskMutex sync.RWMutex
	Store             datastore.DataStore
	prefix            string
	ack               func()
}

func NewWorkflowController(workflowTask *model.WorkflowQueue, store datastore.DataStore) *workflowCtl {
	ctl := &workflowCtl{
		workflowTask: workflowTask,
		Store:        store,
		prefix:       fmt.Sprintf("workflowctl-%s-%d", workflowTask.WorkflowName, workflowTask.TaskID),
	}
	ctl.ack = ctl.updateWorkflowTask
	return ctl
}

func (w *workflowCtl) updateWorkflowTask() {
	taskInColl := w.workflowTask
	if taskInColl.Status == config.StatusPassed || taskInColl.Status == config.StatusFailed || taskInColl.Status == config.StatusTimeout || taskInColl.Status == config.StatusReject {
		klog.Info(fmt.Sprintf("%s:%d:%s task already done", taskInColl.WorkflowName, taskInColl.TaskID, taskInColl.Status))
		return
	}
}

func (w *workflowCtl) Run(ctx context.Context, concurrency int) {
	w.workflowTask.Status = config.StatusRunning
	w.workflowTask.CreateTime = time.Now()

	w.ack() // 通知工作流
	klog.Infof(fmt.Sprintf("start workflow: %s,status: %s", w.workflowTask.WorkflowName, w.workflowTask.Status))

	defer func() {
		klog.Infof(fmt.Sprintf("finish workflow: %s,status: %s", w.workflowTask.WorkflowName, w.workflowTask.Status))
		w.ack()
	}()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	// 从redis中订阅取消信号
	//cancelChan, closeFunc := cache.NewRedisCache(config2.RedisCommonCacheTokenDB()).Subscribe(fmt.Sprintf("workflowctl-cancel-%s-%d", c.workflowTask.WorkflowName, c.workflowTask.TaskID))

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			}
		}
	}()

}

func RunStages(ctx context.Context, stages []*model.WorkflowQueue, concurrency int, ack func()) {
	// 执行workflow的每个阶段
	for _, stage := range stages {
		// 当工作流任务重新启动时，是否应该跳过已通过的阶段
		if stage.Status == config.StatusPassed {
			continue
		}
		runStage(ctx)
		if statusStopped(stage.Status) {
			return
		}
	}

}

func runStage(ctx context.Context) {
	// 将状态更改为
	//stage.Status = config.StatusRunning

}

func statusStopped(status config.Status) bool {
	if status == config.StatusCancelled || status == config.StatusFailed ||
		status == config.StatusTimeout || status == config.StatusReject ||
		status == config.StatusPause {
		return true
	}
	return false
}
