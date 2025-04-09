package workflow

import (
	"KubeMin-Cli/pkg/apiserver/event/workflow/job"
	"context"
	"encoding/json"
	"fmt"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	"sync"
	"time"

	"KubeMin-Cli/pkg/apiserver/config"
	"KubeMin-Cli/pkg/apiserver/domain/model"
	"KubeMin-Cli/pkg/apiserver/domain/service"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"

	"KubeMin-Cli/pkg/apiserver/infrastructure/datastore"
)

type Workflow struct {
	KubeClient      *kubernetes.Clientset   `inject:"kubeClient"`
	KubeConfig      *rest.Config            `inject:"kubeConfig"`
	Store           datastore.DataStore     `inject:"datastore"`
	WorkflowService service.WorkflowService `inject:""`
}

func (w *Workflow) Start(ctx context.Context, errChan chan error) {
	//w.InitQueue(ctx)
	go w.WorkflowTaskSender()
}

func (w *Workflow) InitQueue(ctx context.Context) {
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

	w.ack()
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

	task := GenerateJobTask(ctx, w.workflowTask, w.Store)
	job.RunJobs(ctx, task, concurrency, w.Client, w.Store, w.ack)
}

func GenerateJobTask(ctx context.Context, task *model.WorkflowQueue, ds datastore.DataStore) []*model.JobTask {
	// Step1.根据 appId 查询所有组件
	workflow := model.Workflow{
		ID: task.WorkflowId,
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
	err = json.Unmarshal(steps, &workflowStep)
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

	var jobs []*model.JobTask
	// 构建Jobs
	for _, step := range workflowStep.Steps {
		component := FindComponents(ComponentList, step.Name)
		jobTask := NewJobTask(component.Name, "default", task.WorkflowId, task.ProjectId, task.AppID)

		cProperties, err := json.Marshal(component.Properties)
		if err != nil {
			klog.Errorf("Component.Properties deserialization failure:", err)
			return nil
		}

		var properties model.Properties
		err = json.Unmarshal(cProperties, &properties)
		if err != nil {
			klog.Errorf("WorkflowSteps deserialization failure:", err)
			return nil
		}

		switch component.ComponentType {
		case config.ServerJob:
			jobTask.JobType = string(config.JobDeploy)
			// webservice 默认为无状态服务，使用Deployment 构建
			jobTask.JobInfo = GenerateWebService(component, &properties)
		}

		//// 创建Service
		//if len(properties.Ports) > 0 {
		//	jobTaskService := NewJobTask(fmt.Sprintf("%s-service", component.Name), "default", task.WorkflowId, task.ProjectId, task.AppID)
		//	jobTaskService.JobType = string(config.JobDeployService)
		//	jobTaskService.JobInfo = GenerateService(fmt.Sprintf("%s-service", component.Name), "default", nil, properties.Ports)
		//	jobs = append(jobs, jobTaskService)
		//}
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

func GenerateWebService(component *model.ApplicationComponent, properties *model.Properties) interface{} {
	serviceName := component.Name
	labels := make(map[string]string)
	labels["kube-min-cli"] = fmt.Sprintf("%s-%s", component.AppId, component.Name)
	labels["kube-min-cli-appId"] = component.AppId
	if component.Labels != nil {
		for k, v := range component.Labels {
			labels[k] = v
		}
	}

	var ContainerPort []corev1.ContainerPort
	for _, v := range properties.Ports {
		ContainerPort = append(ContainerPort, corev1.ContainerPort{
			ContainerPort: v.Port,
		})
	}
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceName,
			Namespace: "default",
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &component.Replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  serviceName,
							Image: properties.Image,
							Ports: ContainerPort,
						},
					},
				},
			},
		},
	}

	return deployment
}

func GenerateService(name, namespace string, lab map[string]string, ports []model.Ports) interface{} {
	var servicePort []corev1.ServicePort
	for _, v := range ports {
		servicePort = append(servicePort, corev1.ServicePort{
			Port:       v.Port,
			TargetPort: intstr.FromInt32(v.Port),
			Protocol:   corev1.ProtocolTCP,
		})
	}
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-service", name),
			Namespace: namespace,
		},
		Spec: corev1.ServiceSpec{
			Selector: lab,
			Ports:    servicePort,
			Type:     corev1.ServiceTypeClusterIP,
		},
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
	task.Status = config.StatusQueued
	if success := w.WorkflowService.UpdateTask(ctx, task); !success {
		klog.Errorf("%s:%d update t status error", task.WorkflowName, task.TaskID)
		return fmt.Errorf("%s:%d update t status error", task.WorkflowName, task.TaskID)
	}

	go NewWorkflowController(task, w.KubeClient, w.Store).Run(ctx, jobConcurrency)
	return nil
}
