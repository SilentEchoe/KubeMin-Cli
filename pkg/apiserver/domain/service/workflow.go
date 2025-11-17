package service

import (
	"context"
	"fmt"
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
	"KubeMin-Cli/pkg/apiserver/workflow/signal"
)

type WorkflowService interface {
	ListApplicationWorkflow(ctx context.Context, app *model.Applications) error
	CreateWorkflowTask(ctx context.Context, workflow apis.CreateWorkflowRequest) (*apis.CreateWorkflowResponse, error)
	ExecWorkflowTask(ctx context.Context, workflowID string) (*apis.ExecWorkflowResponse, error)
	ExecWorkflowTaskForApp(ctx context.Context, appID, workflowID string) (*apis.ExecWorkflowResponse, error)
	WaitingTasks(ctx context.Context) ([]*model.WorkflowQueue, error)
	UpdateTask(ctx context.Context, queue *model.WorkflowQueue) bool
	TaskRunning(ctx context.Context) ([]*model.WorkflowQueue, error)
	CancelWorkflowTask(ctx context.Context, userName, taskID, reason string) error
	CancelWorkflowTaskForApp(ctx context.Context, appID, userName, taskID, reason string) error
	MarkTaskStatus(ctx context.Context, taskID string, from, to config.Status) (bool, error)
}

type workflowServiceImpl struct {
	Store      datastore.DataStore  `inject:"datastore"`
	KubeClient kubernetes.Interface `inject:"kubeClient"`
	KubeConfig *rest.Config         `inject:"kubeConfig"`
	Cache      cache.ICache         `inject:"cache"`
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
		klog.Errorf("check workflow existence failure: %v", err)
		return nil, fmt.Errorf("check workflow existence: %w", err)
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
		WorkflowID: workflow.ID,
	}, nil
}

func ConvertWorkflow(req *apis.CreateWorkflowRequest) *model.Workflow {
	return &model.Workflow{
		ID:          utils.RandStringByNumLowercase(24),
		Name:        strings.ToLower(req.Name),
		Alias:       req.Alias,
		Disabled:    true,
		ProjectID:   strings.ToLower(req.Project),
		Description: req.Description,
	}
}

func ConvertComponent(req *apis.CreateComponentRequest, appID string) *model.ApplicationComponent {
	if req.Replicas <= 0 {
		req.Replicas = 1
	}

	return &model.ApplicationComponent{
		Name:          req.Name,
		AppID:         appID,
		Namespace:     req.NameSpace,
		Image:         req.Image,
		Replicas:      req.Replicas,
		ComponentType: req.ComponentType,
	}
}

// ExecWorkflowTask 执行工作流的任务
func (w *workflowServiceImpl) ExecWorkflowTask(ctx context.Context, workflowID string) (*apis.ExecWorkflowResponse, error) {
	workflow, err := repository.WorkflowByID(ctx, w.Store, workflowID)
	if err != nil {
		return nil, err
	}
	return w.enqueueWorkflowTask(ctx, workflow)
}

func (w *workflowServiceImpl) ExecWorkflowTaskForApp(ctx context.Context, appID, workflowID string) (*apis.ExecWorkflowResponse, error) {
	workflow, err := repository.WorkflowByID(ctx, w.Store, workflowID)
	if err != nil {
		return nil, err
	}
	if workflow.AppID == "" || workflow.AppID != appID {
		return nil, bcode.ErrWorkflowNotExist
	}
	return w.enqueueWorkflowTask(ctx, workflow)
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

func (w *workflowServiceImpl) CancelWorkflowTask(ctx context.Context, userName, taskID, reason string) error {
	task, err := repository.TaskByID(ctx, w.Store, taskID)
	if err != nil {
		return err
	}
	return w.cancelWorkflowTask(ctx, task, userName, reason)
}

func (w *workflowServiceImpl) CancelWorkflowTaskForApp(ctx context.Context, appID, userName, taskID, reason string) error {
	task, err := repository.TaskByID(ctx, w.Store, taskID)
	if err != nil {
		return err
	}
	if task.AppID == "" || task.AppID != appID {
		return bcode.ErrWorkflowNotExist
	}
	return w.cancelWorkflowTask(ctx, task, userName, reason)
}

func (w *workflowServiceImpl) cancelWorkflowTask(ctx context.Context, task *model.WorkflowQueue, userName, reason string) error {
	task.TaskRevoker = userName
	task.Status = config.StatusCancelled

	if err := repository.UpdateTask(ctx, w.Store, task); err != nil {
		return err
	}
	if reason == "" {
		reason = fmt.Sprintf("cancelled by %s", userName)
	}
	if err := signal.Cancel(ctx, task.TaskID, reason); err != nil {
		return err
	}
	return nil
}

func (w *workflowServiceImpl) enqueueWorkflowTask(ctx context.Context, workflow *model.Workflow) (*apis.ExecWorkflowResponse, error) {
	if workflow == nil || workflow.Steps == nil {
		return nil, bcode.ErrExecWorkflow
	}
	workflowTask := &model.WorkflowQueue{
		TaskID:              utils.RandStringByNumLowercase(24),
		AppID:               workflow.AppID,
		WorkflowID:          workflow.ID,
		ProjectID:           workflow.ProjectID,
		WorkflowName:        workflow.Name,
		WorkflowDisplayName: workflow.Alias,
		Type:                workflow.WorkflowType,
		Status:              config.StatusWaiting,
	}

	if err := repository.CreateWorkflowQueue(ctx, w.Store, workflowTask); err != nil {
		return nil, err
	}
	return &apis.ExecWorkflowResponse{TaskID: workflowTask.TaskID}, nil
}
