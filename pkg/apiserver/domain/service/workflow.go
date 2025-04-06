package service

import (
	v1beta1 "KubeMin-Cli/apis/core.kubemincli.dev/v1alpha1"
	"KubeMin-Cli/pkg/apiserver/config"
	"KubeMin-Cli/pkg/apiserver/domain/model"
	"KubeMin-Cli/pkg/apiserver/domain/repository"
	"KubeMin-Cli/pkg/apiserver/infrastructure/datastore"
	apis "KubeMin-Cli/pkg/apiserver/interfaces/api/dto/v1"
	"KubeMin-Cli/pkg/apiserver/utils"
	"KubeMin-Cli/pkg/apiserver/utils/bcode"
	"KubeMin-Cli/pkg/apiserver/utils/cache"
	wf "KubeMin-Cli/pkg/apiserver/workflow"
	"context"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type WorkflowService interface {
	ListApplicationWorkflow(ctx context.Context, app *model.Applications) error
	SyncWorkflowRecord(ctx context.Context, appKey, recordName string, app *v1beta1.Applications, workflowContext map[string]string) error
	CreateWorkflowTask(ctx context.Context, workflow apis.CreateWorkflowRequest) (*apis.CreateWorkflowResponse, error)
	ExecWorkflowTask(ctx context.Context, workflowId string) (*apis.ExecWorkflowResponse, error)
	WaitingTasks(ctx context.Context) ([]*model.WorkflowQueue, error)
	UpdateTask(ctx context.Context, queue *model.WorkflowQueue) bool
	TaskRunning(ctx context.Context) ([]*model.WorkflowQueue, error)
	CancelWorkflowTask(ctx context.Context, userName, workId string) error
}

type workflowServiceImpl struct {
	Store      datastore.DataStore `inject:"datastore"`
	KubeClient client.Client       `inject:"kubeClient"`
	KubeConfig *rest.Config        `inject:"kubeConfig"`
	Cache      cache.ICache        `inject:"cache"`
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
			klog.Errorf("new trait failure,%s", err.Error())
			return nil, bcode.ErrInvalidProperties
		}
		nComponent.Properties = properties

		err = repository.CreateComponents(ctx, w.Store, nComponent)
		if err != nil {
			klog.Errorf("Create Compoents err:", err)
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
		Name:        req.Name,
		Alias:       req.Alias,
		Disabled:    true,
		ProjectId:   req.Project,
		Description: req.Description,
	}
}

func ConvertComponent(req *apis.CreateComponentRequest, appID string) *model.ApplicationComponent {
	return &model.ApplicationComponent{
		Name:          req.Name,
		AppId:         appID,
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
		WorkflowId: workflowTask.TaskID,
	}, nil
}

func (w *workflowServiceImpl) ListApplicationWorkflow(ctx context.Context, app *model.Applications) error {
	//TODO implement me
	panic("implement me")
}

func (w *workflowServiceImpl) SyncWorkflowRecord(ctx context.Context, appKey, recordName string, app *v1beta1.Applications, workflowContext map[string]string) error {
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
		klog.Errorf("%s:%d update t status error", task.WorkflowName, task.TaskID)
		return false
	}
	return true
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
