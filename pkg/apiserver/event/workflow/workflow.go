package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"KubeMin-Cli/pkg/apiserver/config"
	"KubeMin-Cli/pkg/apiserver/domain/model"
	"KubeMin-Cli/pkg/apiserver/domain/service"
	"KubeMin-Cli/pkg/apiserver/event/workflow/job"
	"KubeMin-Cli/pkg/apiserver/infrastructure/datastore"
)

type Workflow struct {
	KubeClient      *kubernetes.Clientset   `inject:"kubeClient"`
	KubeConfig      *rest.Config            `inject:"kubeConfig"`
	Store           datastore.DataStore     `inject:"datastore"`
	WorkflowService service.WorkflowService `inject:""`
}

func (w *Workflow) Start(ctx context.Context, errChan chan error) {
	w.InitQueue(ctx)
	go w.WorkflowTaskSender()
}

func (w *Workflow) InitQueue(ctx context.Context) {
	if w.Store == nil {
		klog.Errorf("datastore is nil")
		return
	}
	// 从数据库中查找未完成的任务
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
		klog.Infof(fmt.Sprintf("cance task :%s", task.TaskID))
	}
}

func (w *Workflow) WorkflowTaskSender() {
	for {
		time.Sleep(time.Second * 3)
		ctx := context.Background()
		//获取等待的任务
		waitingTasks, err := w.WorkflowService.WaitingTasks(ctx)
		if err != nil || len(waitingTasks) == 0 {
			continue
		}
		for _, task := range waitingTasks {
			// TODO jobConcurrency 默认为1，表示串行执行
			if err := w.updateQueueAndRunTask(ctx, task, 1); err != nil {
				continue
			}
		}
	}
}

type WorkflowCtl struct {
	workflowTask      *model.WorkflowQueue
	workflowTaskMutex sync.RWMutex
	Client            *kubernetes.Clientset
	Store             datastore.DataStore
	prefix            string
	ack               func()
}

func NewWorkflowController(workflowTask *model.WorkflowQueue, client *kubernetes.Clientset, store datastore.DataStore) *WorkflowCtl {
	ctl := &WorkflowCtl{
		workflowTask: workflowTask,
		Store:        store,
		Client:       client,
		prefix:       fmt.Sprintf("workflowctl-%s-%d", workflowTask.WorkflowName, workflowTask.TaskID),
	}
	ctl.ack = ctl.updateWorkflowTask
	return ctl
}

// 更改工作流的状态或信息
func (w *WorkflowCtl) updateWorkflowTask() {
	taskInColl := w.workflowTask
	// 如果当前的task状态为：通过，暂停，超时，拒绝；则不处理，直接返回
	if taskInColl.Status == config.StatusPassed || taskInColl.Status == config.StatusFailed || taskInColl.Status == config.StatusTimeout || taskInColl.Status == config.StatusReject {
		klog.Info(fmt.Sprintf("%s:%s:%s task already done", taskInColl.WorkflowName, taskInColl.TaskID, taskInColl.Status))
		return
	}
	if err := w.Store.Put(context.Background(), w.workflowTask); err != nil {
		klog.Errorf("%s:%s update t status error", w.workflowTask.WorkflowName, w.workflowTask.TaskID)
	}
}

func (w *WorkflowCtl) Run(ctx context.Context, concurrency int) {
	// 将工作流的状态更改为运行中
	w.workflowTask.Status = config.StatusRunning
	w.workflowTask.CreateTime = time.Now()
	w.ack()
	klog.Infof(fmt.Sprintf("start workflow: %s,status: %s", w.workflowTask.WorkflowName, w.workflowTask.Status))

	defer func() {
		klog.Infof(fmt.Sprintf("finish workflow: %s,status: %s", w.workflowTask.WorkflowName, w.workflowTask.Status))
		w.ack()
	}()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// sub cancel signal from redis
	// 从redis中订阅取消信号
	//cancelChan, closeFunc := cache.NewRedisCache(config2.RedisCommonCacheTokenDB()).Subscribe(fmt.Sprintf("workflowctl-cancel-%s-%d", c.workflowTask.WorkflowName, c.workflowTask.TaskID))
	//defer func() {
	//	log.Infof("pubsub channel: %s/%d closed", c.workflowTask.WorkflowName, c.workflowTask.TaskID)
	//	_ = closeFunc()

	//}()

	go func() {
		for {
			select {
			//case <-cancelChan:
			//	cancel()
			//	return
			case <-ctx.Done():
				return
			}
		}
	}()

	tasks := GenerateJobTask(ctx, w.workflowTask, w.Store)
	job.RunJobs(ctx, tasks, concurrency, w.Client, w.Store, w.ack)
	w.updateWorkflowStatus(ctx)
}

func GenerateJobTask(ctx context.Context, task *model.WorkflowQueue, ds datastore.DataStore) []*model.JobTask {
	// Step1.根据 appId 查询所有组件
	workflow := model.Workflow{
		ID: task.WorkflowId,
	}
	err := ds.Get(ctx, &workflow)
	if err != nil {
		klog.Errorf("Generate JobTask Components error: %s", err)
		return nil
	}

	// 将 JSONStruct 序列化为字节切片
	steps, err := json.Marshal(workflow.Steps)
	if err != nil {
		klog.Errorf("Workflow.Steps deserialization failure: %s", err)
		return nil
	}

	// Step2.对阶段进行反序列化
	var workflowStep model.WorkflowSteps
	err = json.Unmarshal(steps, &workflowStep)
	if err != nil {
		klog.Errorf("WorkflowSteps deserialization failure: %s", err)
		return nil
	}

	// Step3.根据 appId 查询所有组件信息
	component, err := ds.List(ctx, &model.ApplicationComponent{AppId: task.AppID}, &datastore.ListOptions{})

	if err != nil {
		klog.Errorf("Generate JobTask Components error: %s", err)
		return nil
	}
	var ComponentList []*model.ApplicationComponent
	for _, v := range component {
		ac := v.(*model.ApplicationComponent)
		ComponentList = append(ComponentList, ac)
	}

	// 构建Jobs
	var jobs []*model.JobTask
	for _, step := range workflowStep.Steps {
		componentSteps := FindComponents(ComponentList, step.Name)
		if componentSteps == nil {
			continue
		}
		jobTask := NewJobTask(componentSteps.Name, componentSteps.Namespace, task.WorkflowId, task.ProjectId, task.AppID)
		properties := ParseProperties(componentSteps.Properties)

		switch componentSteps.ComponentType {
		case config.ServerJob:
			jobTask.JobType = string(config.JobDeploy)
			// webservice 默认为无状态服务，使用Deployment 构建
			jobTask.JobInfo = job.GenerateWebService(componentSteps, &properties)
		case config.StoreJob:
			jobTask.JobType = string(config.JobDeployStore)
			storeJobs := job.GenerateStoreService(componentSteps)
			if storeJobs != nil {
				jobTask.JobInfo = storeJobs.StatefulSet
				var err error
				jobs, err = CreatePVCJobsFromResult(storeJobs.AdditionalObjects, componentSteps, task, jobs)
				if err != nil {
					klog.Errorf("failed to create PVC jobs for component %s: %v", componentSteps.Name, err)
				}
			}
		case config.ConfJob:
			jobTask.JobType = string(config.JobDeployConfigMap)
			jobTask.JobInfo = job.GenerateConfigMap(componentSteps, &properties)
		case config.SecretJob:
			jobTask.JobType = string(config.JobDeploySecret)
			jobTask.JobInfo = job.GenerateSecret(componentSteps, &properties)
		}

		// 创建Service
		if len(properties.Ports) > 0 {
			jobTaskService := NewJobTask(fmt.Sprintf("%s", componentSteps.Name), "default", task.WorkflowId, task.ProjectId, task.AppID)
			jobTaskService.JobType = string(config.JobDeployService)
			jobTaskService.JobInfo = job.GenerateService(fmt.Sprintf("%s", componentSteps.Name), "default", nil, properties.Ports)
			jobs = append(jobs, jobTaskService)
		}
		jobs = append(jobs, jobTask)
	}
	return jobs
}

func NewJobTask(name, namespace, workflowId, projectId, appId string) *model.JobTask {
	return &model.JobTask{
		Name:       name,
		Namespace:  namespace,
		WorkflowId: workflowId,
		ProjectId:  projectId,
		AppId:      appId,
		Status:     config.StatusQueued,
		Timeout:    60,
	}
}

func FindComponents(components []*model.ApplicationComponent, name string) *model.ApplicationComponent {
	for _, v := range components {
		if v.Name == name {
			return v
		}
	}
	return nil
}

// 更改工作流队列的状态，并运行它
func (w *Workflow) updateQueueAndRunTask(ctx context.Context, task *model.WorkflowQueue, jobConcurrency int) error {
	//将状态更改为队列中
	task.Status = config.StatusQueued
	if success := w.WorkflowService.UpdateTask(ctx, task); !success {
		klog.Errorf("%s:%d update t status error", task.WorkflowName, task.TaskID)
		return fmt.Errorf("%s:%d update t status error", task.WorkflowName, task.TaskID)
	}
	// 执行新的任务
	go NewWorkflowController(task, w.KubeClient, w.Store).Run(ctx, jobConcurrency)
	return nil
}

func (w *WorkflowCtl) updateWorkflowStatus(ctx context.Context) {
	w.workflowTask.Status = config.StatusCompleted
	err := w.Store.Put(ctx, w.workflowTask)
	if err != nil {
		klog.Errorf("update Workflow status err:%s", err)
	}
}

func ParseProperties(properties *model.JSONStruct) model.Properties {
	cProperties, err := json.Marshal(properties)
	if err != nil {
		klog.Errorf("Component.Properties deserialization failure: %s", err)
		return model.Properties{}
	}

	var propertied model.Properties
	err = json.Unmarshal(cProperties, &propertied)
	if err != nil {
		klog.Errorf("WorkflowSteps deserialization failure: %s", err)
		return model.Properties{}
	}
	return propertied
}

func CreatePVCJobsFromResult(additionalObjects []client.Object, component *model.ApplicationComponent, task *model.WorkflowQueue, jobs []*model.JobTask) ([]*model.JobTask, error) {
	if len(additionalObjects) == 0 {
		return jobs, nil
	}

	for _, obj := range additionalObjects {
		if pvc, ok := obj.(*corev1.PersistentVolumeClaim); ok {
			// 创建PVC Job
			pvcJob := NewJobTask(
				fmt.Sprintf("%s-pvc-%s", component.Name, pvc.Name),
				component.Namespace,
				task.WorkflowId,
				task.ProjectId,
				task.AppID,
			)
			pvcJob.JobType = string(config.JobDeployPVC)
			pvcJob.JobInfo = pvc
			pvcJob.Timeout = 60 * 5 // 5分钟超时

			jobs = append(jobs, pvcJob)
			klog.Infof("Created PVC job for component %s: %s", component.Name, pvc.Name)
		}
	}
	return jobs, nil
}
