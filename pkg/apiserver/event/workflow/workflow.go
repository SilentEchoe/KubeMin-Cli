package workflow

import (
	"KubeMin-Cli/pkg/apiserver/config"
	"KubeMin-Cli/pkg/apiserver/domain/model"
	"KubeMin-Cli/pkg/apiserver/domain/service"
	"KubeMin-Cli/pkg/apiserver/event/workflow/job"
	"KubeMin-Cli/pkg/apiserver/infrastructure/datastore"
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Workflow struct {
	KubeClient      client.Client           `inject:"kubeClient"`
	KubeConfig      *rest.Config            `inject:"kubeConfig"`
	Store           datastore.DataStore     `inject:"datastore"`
	WorkflowService service.WorkflowService `inject:""`
}

func (w *Workflow) Start(ctx context.Context, errChan chan error) {
	//w.InitQueue()
	go w.WorkflowTaskSender()
}

func (w *Workflow) InitQueue() {
	ctx := context.Background()
	// 从数据库中查找未完成的任务

	if w.Store == nil {
		klog.Errorf("datastore is nil")
		return
	}
	tasks, err := w.WorkflowService.TaskRunning(ctx)
	if err != nil {
		klog.Errorf(fmt.Sprintf("find task running error:%s", err))
		return
	}
	// 如果重启Queue，则取消所有正在运行的tasks
	for _, task := range tasks {
		err := w.WorkflowService.CancelWorkflowTask(ctx, config.DefaultTaskRevoker, task.TaskID)
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
		waitingTasks, err := w.WorkflowService.WaitingTasks(ctx)
		if err != nil || len(waitingTasks) == 0 {
			continue
		}
		for _, task := range waitingTasks {
			if err := w.updateQueueAndRunTask(ctx, task, 1); err != nil {
				continue
			}
		}
	}
}

type WorkflowCtl struct {
	workflowTask      *model.WorkflowQueue
	workflowTaskMutex sync.RWMutex
	Store             datastore.DataStore
	prefix            string
	ack               func()
}

func NewWorkflowController(workflowTask *model.WorkflowQueue, store datastore.DataStore) *WorkflowCtl {
	ctl := &WorkflowCtl{
		workflowTask: workflowTask,
		Store:        store,
		prefix:       fmt.Sprintf("workflowctl-%s-%d", workflowTask.WorkflowName, workflowTask.TaskID),
	}
	ctl.ack = ctl.updateWorkflowTask
	return ctl
}

func (w *WorkflowCtl) updateWorkflowTask() {
	taskInColl := w.workflowTask
	// 如果当前的task状态为：通过，暂停，超时，拒绝；则不处理，直接返回
	if taskInColl.Status == config.StatusPassed || taskInColl.Status == config.StatusFailed || taskInColl.Status == config.StatusTimeout || taskInColl.Status == config.StatusReject {
		klog.Info(fmt.Sprintf("%s:%d:%s task already done", taskInColl.WorkflowName, taskInColl.TaskID, taskInColl.Status))
		return
	}
	if err := w.Store.Put(context.Background(), w.workflowTask); err != nil {
		klog.Errorf("%s:%d update t status error", w.workflowTask.WorkflowName, w.workflowTask.TaskID)
	}
}

func (w *WorkflowCtl) Run(ctx context.Context, concurrency int) {
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

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			}
		}
	}()

	// TODO 将一个Task 组装成多个阶段(Job)
	task := GenerateJobTask(ctx, w.workflowTask, w.Store)
	RunStages(ctx, task, 1, w.ack)
}

func GenerateJobTask(ctx context.Context, task *model.WorkflowQueue, ds datastore.DataStore) []*job.JobTask {
	// Step1.根据 appId 查询所有组件

	workflow := model.Workflow{
		ID: task.TaskID,
	}
	err := ds.Get(ctx, &workflow)
	if err != nil {
		klog.Errorf("Generate JobTask Components error:", err)
		return nil
	}

	// 将 JSONStruct 序列化为字节切片
	steps, err := json.Marshal(workflow.Steps)
	if err != nil {
		klog.Errorf("Workflow.Steps deserialization failure:", err)
		return nil
	}

	// Step2.对阶段进行反序列化
	var workflowStep model.WorkflowSteps
	err = json.Unmarshal(steps, &workflowStep) // 注意传递指针
	if err != nil {
		klog.Errorf("WorkflowSteps deserialization failure:", err)
		return nil
	}

	// Step2.根据 appId 查询所有组件信息
	component, err := ds.List(ctx, &model.ApplicationComponent{AppId: task.AppID}, &datastore.ListOptions{})
	if err != nil {
		klog.Errorf("Generate JobTask Components error:", err)
		return nil
	}
	var ComponentList []*model.ApplicationComponent
	for _, v := range component {
		ac := v.(*model.ApplicationComponent)
		ComponentList = append(ComponentList, ac)
	}

	var jobs []*job.JobTask
	for _, step := range workflowStep.Steps {
		var jobTask *job.JobTask
		component := FindComponents(ComponentList, step.Name)

		switch component.ComponentType {
		case config.ServerJob:
			// 如果是服务器类型，那就默认部署
			jobTask.JobType = string(config.JobDeploy)
			// webservice 默认为无状态服务，使用Deployment 构建
			// TODO  可以在这个阶段直接构建JobInfo 是个Yaml类型
		}

	}
	return jobs
}

func FindComponents(components []*model.ApplicationComponent, name string) *model.ApplicationComponent {
	for _, v := range components {
		if v.Name == name {
			return v
		}
	}
	return nil
}

func RunStages(ctx context.Context, stages []*job.JobTask, concurrency int, ack func()) {
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

type StageTask struct {
	Name      string         `json:"name"`
	Status    config.Status  `json:"status"`
	StartTime int64          `json:"start_time,omitempty"`
	EndTime   int64          `json:"end_time,omitempty"`
	Parallel  bool           `json:"parallel,omitempty"`
	Jobs      []*job.JobTask `json:"jobs,omitempty"`
	Error     string         `json:"error"`
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

// 更改工作流队列的状态，并运行它
func (w *Workflow) updateQueueAndRunTask(ctx context.Context, task *model.WorkflowQueue, jobConcurrency int) error {
	task.Status = config.StatusQueued
	if success := w.WorkflowService.UpdateTask(ctx, task); !success {
		klog.Errorf("%s:%d update t status error", task.WorkflowName, task.TaskID)
		return fmt.Errorf("%s:%d update t status error", task.WorkflowName, task.TaskID)
	}

	go NewWorkflowController(task, w.Store).Run(ctx, jobConcurrency)
	return nil
}
