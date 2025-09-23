package service

import (
	"context"
	"strings"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"

	"KubeMin-Cli/pkg/apiserver/config"
	"KubeMin-Cli/pkg/apiserver/domain/model"
	"KubeMin-Cli/pkg/apiserver/domain/repository"
	"KubeMin-Cli/pkg/apiserver/infrastructure/datastore"
	apis "KubeMin-Cli/pkg/apiserver/interfaces/api/dto/v1"
	"KubeMin-Cli/pkg/apiserver/utils"
	"KubeMin-Cli/pkg/apiserver/utils/bcode"
	"KubeMin-Cli/pkg/apiserver/utils/cache"
	wf "KubeMin-Cli/pkg/apiserver/workflow"
)

type WorkflowService interface {
	ListApplicationWorkflow(ctx context.Context, app *model.Applications) error
	CreateWorkflowTask(ctx context.Context, workflow apis.CreateWorkflowRequest) (*apis.CreateWorkflowResponse, error)
	ExecWorkflowTask(ctx context.Context, workflowId string) (*apis.ExecWorkflowResponse, error)
	WaitingTasks(ctx context.Context) ([]*model.WorkflowQueue, error)
	UpdateTask(ctx context.Context, queue *model.WorkflowQueue) bool
	TaskRunning(ctx context.Context) ([]*model.WorkflowQueue, error)
	CancelWorkflowTask(ctx context.Context, userName, workId string) error
	MarkTaskStatus(ctx context.Context, taskID string, from, to config.Status) (bool, error)
}

type workflowServiceImpl struct {
	Store      datastore.DataStore   `inject:"datastore"`
	KubeClient *kubernetes.Clientset `inject:"kubeClient"`
	KubeConfig *rest.Config          `inject:"kubeConfig"`
	Cache      cache.ICache          `inject:"cache"`
}

// NewWorkflowService new workflow service
func NewWorkflowService() WorkflowService {
	return &workflowServiceImpl{}
}

// CreateWorkflowTask 创建工作流任务(执行)
func (w *workflowServiceImpl) CreateWorkflowTask(ctx context.Context, req apis.CreateWorkflowRequest) (*apis.CreateWorkflowResponse, error) {
	workflow := &model.Workflow{
		Name: req.Name,
	}
	exist, err := w.Store.IsExist(ctx, workflow)
	if err != nil {
		klog.Errorf("check workflow name is exist failure %s", err.Error())
		return nil, bcode.ErrWorkflowExist
	}
	if exist {
		return nil, bcode.ErrWorkflowExist
	}
	workflow = ConvertWorkflow(&req)

	// 校验工作流信息
	if err = wf.LintWorkflow(workflow); err != nil {
		return nil, err
	}

	err = repository.CreateWorkflow(ctx, w.Store, workflow)
	if err != nil {
		return nil, bcode.ErrCreateWorkflow
	}

	// 创建组件
	for _, component := range req.Component {
		nComponent := ConvertComponent(&component, workflow.ID)
		properties, err := model.NewJSONStructByStruct(component.Properties)
		if err != nil {
			dErr := repository.DelWorkflow(ctx, w.Store, workflow)
			if dErr != nil {
				klog.Errorf("del workflow failure,%s", dErr.Error())
				return nil, bcode.ErrInvalidProperties
			}
			klog.Errorf("new trait failure,%s", err.Error())
			return nil, bcode.ErrInvalidProperties
		}

		nComponent.Properties = properties

		err = repository.CreateComponents(ctx, w.Store, nComponent)
		if err != nil {
			dErr := repository.DelWorkflow(ctx, w.Store, workflow)
			if dErr != nil {
				klog.Errorf("Create Components err: %s", err)
				return nil, err
			}
			klog.Errorf("Create Components err: %s", err)
			return nil, bcode.ErrCreateComponents
		}
	}
	return &apis.CreateWorkflowResponse{
		WorkflowId: workflow.ID,
	}, nil
}

func ConvertWorkflow(req *apis.CreateWorkflowRequest) *model.Workflow {
	return &model.Workflow{
		ID:          utils.RandStringByNumLowercase(24),
		Name:        strings.ToLower(req.Name),
		Alias:       req.Alias,
		Disabled:    true,
		ProjectId:   strings.ToLower(req.Project),
		Description: req.Description,
	}
}

func ConvertComponent(req *apis.CreateComponentRequest, appID string) *model.ApplicationComponent {
	if req.Replicas <= 0 {
		req.Replicas = 1
	}

	return &model.ApplicationComponent{
		Name:          req.Name,
		AppId:         appID,
		Namespace:     req.NameSpace,
		Image:         req.Image,
		Replicas:      req.Replicas,
		ComponentType: req.ComponentType,
	}
}

// ExecWorkflowTask 执行工作流的任务
func (w *workflowServiceImpl) ExecWorkflowTask(ctx context.Context, workflowId string) (*apis.ExecWorkflowResponse, error) {
	//查询该工作流是否存在
	workflow, err := repository.WorkflowById(ctx, w.Store, workflowId)
	if err != nil {
		return nil, err
	}

	if workflow.Steps == nil {
		return nil, bcode.ErrExecWorkflow
	}
	//验证并解析工作流，生成Job并放入消息队列(WorkflowQueue表)中
	workflowTask := &model.WorkflowQueue{
		TaskID:              utils.RandStringByNumLowercase(24),
		AppID:               workflow.AppID,
		WorkflowId:          workflowId,
		ProjectId:           workflow.ProjectId,
		WorkflowName:        workflow.Name,
		WorkflowDisplayName: workflow.Alias,
		Type:                workflow.WorkflowType,
		Status:              config.StatusWaiting,
	}

	err = repository.CreateWorkflowQueue(ctx, w.Store, workflowTask)
	if err != nil {
		return nil, err
	}

	return &apis.ExecWorkflowResponse{
		TaskId: workflowTask.TaskID,
	}, nil
}

func (w *workflowServiceImpl) ListApplicationWorkflow(ctx context.Context, app *model.Applications) error {
	//TODO implement me
	panic("implement me")
}

func (w *workflowServiceImpl) WaitingTasks(ctx context.Context) ([]*model.WorkflowQueue, error) {
	list, err := repository.WaitingTasks(ctx, w.Store)
	if err != nil {
		return nil, err
	}
	return list, err
}

func (w *workflowServiceImpl) UpdateTask(ctx context.Context, task *model.WorkflowQueue) bool {
	err := repository.UpdateTask(ctx, w.Store, task)
	if err != nil {
		klog.Errorf("%s:%s update t status error", task.WorkflowName, task.TaskID)
		return false
	}
	return true
}

func (w *workflowServiceImpl) MarkTaskStatus(ctx context.Context, taskID string, from, to config.Status) (bool, error) {
	return repository.UpdateTaskStatus(ctx, w.Store, taskID, from, to)
}

// TaskRunning 所有正在运行的Task
func (w *workflowServiceImpl) TaskRunning(ctx context.Context) ([]*model.WorkflowQueue, error) {
	list, err := repository.TaskRunning(ctx, w.Store)
	if err != nil {
		return nil, err
	}
	return list, err
}

func (w *workflowServiceImpl) CancelWorkflowTask(ctx context.Context, userName, taskId string) error {
	task, err := repository.TaskById(ctx, w.Store, taskId)
	if err != nil {
		return err
	}

	task.TaskRevoker = userName
	task.Status = config.StatusCancelled

	err = repository.UpdateTask(ctx, w.Store, task)
	if err != nil {
		return err
	}
	return nil
}
