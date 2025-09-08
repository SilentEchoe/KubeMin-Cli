package workflow

import (
    "context"
    "encoding/json"
    "fmt"
    "sort"
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
    "KubeMin-Cli/pkg/apiserver/domain/repository"
    "KubeMin-Cli/pkg/apiserver/event/workflow/job"
    "KubeMin-Cli/pkg/apiserver/infrastructure/datastore"
    qpkg "KubeMin-Cli/pkg/apiserver/queue"
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/attribute"
    "go.opentelemetry.io/otel/codes"
    "go.opentelemetry.io/otel/trace"
)

type Workflow struct {
    KubeClient      *kubernetes.Clientset   `inject:"kubeClient"`
    KubeConfig      *rest.Config            `inject:"kubeConfig"`
    Store           datastore.DataStore     `inject:"datastore"`
    WorkflowService service.WorkflowService `inject:""`
    Queue           qpkg.Queue               `inject:"queue"`
    Cfg             *config.Config           `inject:""`
}

// TaskDispatch is the minimal payload for dispatching a workflow task to a worker.
type TaskDispatch struct {
    TaskID     string `json:"taskId"`
    WorkflowID string `json:"workflowId"`
    ProjectID  string `json:"projectId"`
    AppID      string `json:"appId"`
}

func MarshalTaskDispatch(t TaskDispatch) ([]byte, error) { return json.Marshal(t) }
func UnmarshalTaskDispatch(b []byte) (TaskDispatch, error) {
    var t TaskDispatch
    err := json.Unmarshal(b, &t)
    return t, err
}

func (w *Workflow) Start(ctx context.Context, errChan chan error) {
    w.InitQueue(ctx)
    // If queue is noop (local mode), fall back to direct DB scan executor for functionality.
    if _, ok := w.Queue.(*qpkg.NoopQueue); ok {
        go w.WorkflowTaskSender()
        return
    }
    // Redis Streams path: leader runs dispatcher; workers managed by server callbacks.
    go w.Dispatcher()
}

func (w *Workflow) InitQueue(ctx context.Context) {
	if w.Store == nil {
		klog.Error("datastore is nil")
		return
	}
	// 从数据库中查找未完成的任务
	tasks, err := w.WorkflowService.TaskRunning(ctx)
	if err != nil {
		klog.Errorf("find task running error: %v", err)
		return
	}
	// 如果重启Queue，则取消所有正在运行的tasks
	for _, task := range tasks {
		err := w.WorkflowService.CancelWorkflowTask(ctx, config.DefaultTaskRevoker, task.TaskID)
		if err != nil {
			klog.Errorf("cancel task error: %v", err)
			return
		}
		klog.Infof("cancel task: %s", task.TaskID)
	}
}

// WorkflowTaskSender is the original local executor scanning DB and running tasks.
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

// Dispatcher scans waiting tasks and publishes dispatch messages.
func (w *Workflow) Dispatcher() {
    for {
        time.Sleep(time.Second * 3)
        ctx := context.Background()
        //获取等待的任务
        waitingTasks, err := w.WorkflowService.WaitingTasks(ctx)
        if err != nil || len(waitingTasks) == 0 {
            continue
        }
        for _, task := range waitingTasks {
            payload := TaskDispatch{TaskID: task.TaskID, WorkflowID: task.WorkflowId, ProjectID: task.ProjectId, AppID: task.AppID}
            b, err := MarshalTaskDispatch(payload)
            if err != nil {
                klog.Errorf("marshal task dispatch failed: %v", err)
                continue
            }
            if _, err := w.Queue.Enqueue(ctx, b); err != nil {
                klog.Errorf("enqueue task dispatch failed: %v", err)
                continue
            }
            klog.Infof("dispatched task: %s", task.TaskID)
        }
    }
}

// StartWorker subscribes to task dispatch topic and executes tasks.
func (w *Workflow) StartWorker(ctx context.Context, errChan chan error) {
    group := w.consumerGroup()
    consumer := w.consumerName()
    klog.Infof("worker reading stream: %s, group: %s, consumer: %s", w.dispatchTopic(), group, consumer)
    staleTicker := time.NewTicker(15 * time.Second)
    defer staleTicker.Stop()
    for {
        select {
        case <-ctx.Done():
            return
        case <-staleTicker.C:
            // periodically claim stale pending messages
            msgs, err := w.Queue.AutoClaim(ctx, group, consumer, 60*time.Second, 50)
            if err != nil {
                klog.V(4).Infof("auto-claim error: %v", err)
                continue
            }
            for _, m := range msgs {
                td, err := UnmarshalTaskDispatch(m.Payload)
                if err != nil {
                    klog.Errorf("decode dispatch (claim) failed: %v", err)
                    _ = w.Queue.Ack(ctx, group, m.ID)
                    continue
                }
                task, err := repository.TaskById(ctx, w.Store, td.TaskID)
                if err != nil {
                    klog.Errorf("load task %s failed: %v", td.TaskID, err)
                    _ = w.Queue.Ack(ctx, group, m.ID)
                    continue
                }
                if err := w.updateQueueAndRunTask(ctx, task, 1); err != nil {
                    klog.Errorf("run task %s failed: %v", td.TaskID, err)
                    _ = w.Queue.Ack(ctx, group, m.ID)
                    continue
                }
                _ = w.Queue.Ack(ctx, group, m.ID)
            }
        default:
            msgs, err := w.Queue.ReadGroup(ctx, group, consumer, 10, 2*time.Second)
            if err != nil {
                klog.V(4).Infof("read group error: %v", err)
                continue
            }
            for _, m := range msgs {
                td, err := UnmarshalTaskDispatch(m.Payload)
                if err != nil {
                    klog.Errorf("decode dispatch failed: %v", err)
                    _ = w.Queue.Ack(ctx, group, m.ID)
                    continue
                }
                task, err := repository.TaskById(ctx, w.Store, td.TaskID)
                if err != nil {
                    klog.Errorf("load task %s failed: %v", td.TaskID, err)
                    _ = w.Queue.Ack(ctx, group, m.ID)
                    continue
                }
                if err := w.updateQueueAndRunTask(ctx, task, 1); err != nil {
                    klog.Errorf("run task %s failed: %v", td.TaskID, err)
                    _ = w.Queue.Ack(ctx, group, m.ID)
                    continue
                }
                _ = w.Queue.Ack(ctx, group, m.ID)
            }
        }
    }
}

func (w *Workflow) dispatchTopic() string {
    prefix := ""
    if w.Cfg != nil {
        prefix = w.Cfg.Messaging.ChannelPrefix
    }
    if prefix == "" {
        prefix = "kubemin"
    }
    return fmt.Sprintf("%s.workflow.dispatch", prefix)
}

func (w *Workflow) consumerGroup() string { return "workflow-workers" }
func (w *Workflow) consumerName() string {
    if w.Cfg != nil {
        return w.Cfg.LeaderConfig.ID
    }
    return "worker"
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
		prefix:       fmt.Sprintf("workflowctl-%s-%s", workflowTask.WorkflowName, workflowTask.TaskID),
	}
	ctl.ack = ctl.updateWorkflowTask
	return ctl
}

// 更改工作流的状态或信息
func (w *WorkflowCtl) updateWorkflowTask() {
	taskInColl := w.workflowTask
	// 如果当前的task状态为：通过，暂停，超时，拒绝；则不处理，直接返回
	if taskInColl.Status == config.StatusPassed || taskInColl.Status == config.StatusFailed || taskInColl.Status == config.StatusTimeout || taskInColl.Status == config.StatusReject {
		klog.Infof("workflow %s, task %s, status %s: task already done, skipping update", taskInColl.WorkflowName, taskInColl.TaskID, taskInColl.Status)
		return
	}
	if err := w.Store.Put(context.Background(), w.workflowTask); err != nil {
		klog.Errorf("update task status error for workflow %s, task %s: %v", w.workflowTask.WorkflowName, w.workflowTask.TaskID, err)
	}
}

func (w *WorkflowCtl) Run(ctx context.Context, concurrency int) {
	// 1. Start a new trace for this workflow execution
	tracer := otel.Tracer("workflow-runner")
	ctx, span := tracer.Start(ctx, w.workflowTask.WorkflowName, trace.WithAttributes(
		attribute.String("workflow.name", w.workflowTask.WorkflowName),
		attribute.String("workflow.task_id", w.workflowTask.TaskID),
	))
	defer span.End()

	// 2. Create a logger with the traceID and put it in the context
	logger := klog.FromContext(ctx).WithValues("traceID", span.SpanContext().TraceID().String())
	ctx = klog.NewContext(ctx, logger)

	// 将工作流的状态更改为运行中
	w.workflowTask.Status = config.StatusRunning
	w.workflowTask.CreateTime = time.Now()
	w.ack()
	logger.Info("Starting workflow", "workflowName", w.workflowTask.WorkflowName, "status", w.workflowTask.Status)

	defer func() {
		logger.Info("Finished workflow", "workflowName", w.workflowTask.WorkflowName, "status", w.workflowTask.Status)
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

	stagedTasks := GenerateJobTasks(ctx, w.workflowTask, w.Store)

	var levels []int
	for level := range stagedTasks {
		levels = append(levels, level)
	}
	sort.Ints(levels)

	for _, level := range levels {
		tasksInLevel := stagedTasks[level]
		if len(tasksInLevel) == 0 {
			continue
		}
		logger.Info("Executing workflow level", "workflowName", w.workflowTask.WorkflowName, "level", level, "jobCount", len(tasksInLevel))

		job.RunJobs(ctx, tasksInLevel, concurrency, w.Client, w.Store, w.ack)

		for _, task := range tasksInLevel {
			if task.Status != config.StatusCompleted {
				logger.Error(nil, "Workflow failed at job, aborting.", "workflowName", w.workflowTask.WorkflowName, "level", level, "jobName", task.Name, "jobStatus", task.Status)
				w.workflowTask.Status = config.StatusFailed
				span.SetStatus(codes.Error, "Workflow failed")
				span.RecordError(fmt.Errorf("job %s failed with status %s", task.Name, task.Status))
				return
			}
		}
		logger.Info("Workflow level completed successfully", "workflowName", w.workflowTask.WorkflowName, "level", level)
	}

	span.SetStatus(codes.Ok, "Workflow completed successfully")
	w.updateWorkflowStatus(ctx)
}

func GenerateJobTasks(ctx context.Context, task *model.WorkflowQueue, ds datastore.DataStore) map[int][]*model.JobTask {
	logger := klog.FromContext(ctx)
	// Step1.根据 appId 查询所有组件
	workflow := model.Workflow{
		ID: task.WorkflowId,
	}
	err := ds.Get(ctx, &workflow)
	if err != nil {
		logger.Error(err, "Failed to get workflow for generating job tasks", "workflowID", task.WorkflowId)
		return nil
	}

	// 将 JSONStruct 序列化为字节切片
	steps, err := json.Marshal(workflow.Steps)
	if err != nil {
		logger.Error(err, "Failed to marshal workflow steps")
		return nil
	}

	// Step2.对阶段进行反序列化
	var workflowStep model.WorkflowSteps
	err = json.Unmarshal(steps, &workflowStep)
	if err != nil {
		logger.Error(err, "Failed to unmarshal workflow steps")
		return nil
	}

	// Step3.根据 appId 查询所有组件信息
	component, err := ds.List(ctx, &model.ApplicationComponent{AppId: task.AppID}, &datastore.ListOptions{})

	if err != nil {
		logger.Error(err, "Failed to list application components", "appID", task.AppID)
		return nil
	}
	var ComponentList []*model.ApplicationComponent
	for _, v := range component {
		ac := v.(*model.ApplicationComponent)
		ComponentList = append(ComponentList, ac)
	}

	// 构建Jobs
	stagedJobs := make(map[int][]*model.JobTask)
	stagedJobs[config.JobPriorityHigh] = []*model.JobTask{}
	stagedJobs[config.JobPriorityNormal] = []*model.JobTask{}
	stagedJobs[config.JobPriorityLow] = []*model.JobTask{}

	for _, step := range workflowStep.Steps {
		componentSteps := FindComponents(ComponentList, step.Name)
		if componentSteps == nil {
			continue
		}
		jobTask := NewJobTask(componentSteps.Name, componentSteps.Namespace, task.WorkflowId, task.ProjectId, task.AppID)
		properties := ParseProperties(ctx, componentSteps.Properties)

		switch componentSteps.ComponentType {
		case config.ServerJob:
			jobTask.JobType = string(config.JobDeploy)
			jobTask.JobInfo = job.GenerateWebService(componentSteps, &properties)
			stagedJobs[config.JobPriorityNormal] = append(stagedJobs[config.JobPriorityNormal], jobTask)

		case config.StoreJob:
			jobTask.JobType = string(config.JobDeployStore)
			storeJobs := job.GenerateStoreService(componentSteps)
			if storeJobs != nil {
				jobTask.JobInfo = storeJobs.StatefulSet
				stagedJobs[config.JobPriorityNormal] = append(stagedJobs[config.JobPriorityNormal], jobTask)

				var pvcJobs []*model.JobTask
				pvcJobs, err = CreatePVCJobsFromResult(storeJobs.AdditionalObjects, componentSteps, task, pvcJobs)
				if err != nil {
					logger.Error(err, "Failed to create PVC jobs", "componentName", componentSteps.Name)
				}
				stagedJobs[config.JobPriorityHigh] = append(stagedJobs[config.JobPriorityHigh], pvcJobs...)
			}
		case config.ConfJob:
			jobTask.JobType = string(config.JobDeployConfigMap)
			jobTask.JobInfo = job.GenerateConfigMap(componentSteps, &properties)
			stagedJobs[config.JobPriorityHigh] = append(stagedJobs[config.JobPriorityHigh], jobTask)
		case config.SecretJob:
			jobTask.JobType = string(config.JobDeploySecret)
			jobTask.JobInfo = job.GenerateSecret(componentSteps, &properties)
			stagedJobs[config.JobPriorityHigh] = append(stagedJobs[config.JobPriorityHigh], jobTask)
		}

		// 创建Service
		if len(properties.Ports) > 0 {
			jobTaskService := NewJobTask(fmt.Sprintf("%s", componentSteps.Name), "default", task.WorkflowId, task.ProjectId, task.AppID)
			jobTaskService.JobType = string(config.JobDeployService)
			jobTaskService.JobInfo = job.GenerateService(fmt.Sprintf("%s", componentSteps.Name), "default", nil, properties.Ports)
			stagedJobs[config.JobPriorityNormal] = append(stagedJobs[config.JobPriorityNormal], jobTaskService)
		}
	}
	totalJobs := 0
	for level, jobs := range stagedJobs {
		if len(jobs) > 0 {
			logger.Info("Generated jobs for workflow level", "jobCount", len(jobs), "workflowName", task.WorkflowName, "level", level)
			totalJobs += len(jobs)
			for _, j := range jobs {
				logger.Info("Generated job details", "jobName", j.Name, "jobType", j.JobType, "level", level)
			}
		}
	}
	logger.Info("Generated total jobs for workflow", "totalJobs", totalJobs, "workflowName", task.WorkflowName)
	return stagedJobs
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
        klog.Errorf("update task status error for workflow %s, task %s", task.WorkflowName, task.TaskID)
        return fmt.Errorf("update task status error for workflow %s, task %s", task.WorkflowName, task.TaskID)
    }
    // 执行新的任务
    go NewWorkflowController(task, w.KubeClient, w.Store).Run(ctx, jobConcurrency)
    return nil
}

func (w *WorkflowCtl) updateWorkflowStatus(ctx context.Context) {
	w.workflowTask.Status = config.StatusCompleted
	err := w.Store.Put(ctx, w.workflowTask)
	if err != nil {
		klog.Errorf("update Workflow status err: %v", err)
	}
}

func ParseProperties(ctx context.Context, properties *model.JSONStruct) model.Properties {
	logger := klog.FromContext(ctx)
	cProperties, err := json.Marshal(properties)
	if err != nil {
		logger.Error(err, "Component.Properties deserialization failure")
		return model.Properties{}
	}

	var propertied model.Properties
	err = json.Unmarshal(cProperties, &propertied)
	if err != nil {
		logger.Error(err, "WorkflowSteps deserialization failure")
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
