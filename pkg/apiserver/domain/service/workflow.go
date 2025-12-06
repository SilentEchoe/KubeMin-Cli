package service

import (
	"context"
	"encoding/json"
	"errors"
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
	GetTaskStatus(ctx context.Context, taskID string) (*apis.TaskStatusResponse, error)
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

	if err = repository.CreateWorkflow(ctx, w.Store, workflow); err != nil {
		return nil, bcode.ErrCreateWorkflow
	}

	// 创建组件
	for _, component := range req.Component {
		nComponent := ConvertComponent(&component, workflow.ID)
		properties, err := model.NewJSONStructByStruct(component.Properties)
		if err != nil {
			w.rollbackWorkflowCreation(ctx, workflow)
			klog.Errorf("new trait failure,%s", err.Error())
			return nil, bcode.ErrInvalidProperties
		}

		nComponent.Properties = properties

		err = repository.CreateComponents(ctx, w.Store, nComponent)
		if err != nil {
			w.rollbackWorkflowCreation(ctx, workflow)
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

func (w *workflowServiceImpl) GetTaskStatus(ctx context.Context, taskID string) (*apis.TaskStatusResponse, error) {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return nil, bcode.ErrWorkflowTaskNotExist
	}
	task, err := repository.TaskByID(ctx, w.Store, taskID)
	if err != nil {
		if errors.Is(err, datastore.ErrRecordNotExist) {
			return nil, bcode.ErrWorkflowTaskNotExist
		}
		return nil, err
	}

	componentAggregates := make(map[string]*apis.ComponentTaskStatus)
	jobEntities, err := w.Store.List(ctx, &model.JobInfo{TaskID: taskID}, &datastore.ListOptions{
		SortBy: []datastore.SortOption{
			{Key: "updatetime", Order: datastore.SortOrderAscending},
			{Key: "createtime", Order: datastore.SortOrderAscending},
		},
	})
	if err != nil && !errors.Is(err, datastore.ErrRecordNotExist) {
		klog.Errorf("list job info for task %s failed: %v", taskID, err)
	} else {
		for _, entity := range jobEntities {
			j, ok := entity.(*model.JobInfo)
			if !ok {
				continue
			}
			key := strings.ToLower(j.ServiceName)
			agg, exists := componentAggregates[key]
			if !exists {
				agg = &apis.ComponentTaskStatus{
					Name:      j.ServiceName,
					Type:      j.Type,
					Status:    j.Status,
					Error:     j.Error,
					StartTime: j.StartTime,
					EndTime:   j.EndTime,
				}
				componentAggregates[key] = agg
				continue
			}
			// Aggregate: prefer the most severe status; capture first error message.
			agg.Status = chooseAggStatus(agg.Status, j.Status)
			if agg.Error == "" && j.Error != "" {
				agg.Error = j.Error
			}
			if agg.StartTime == 0 || (j.StartTime != 0 && j.StartTime < agg.StartTime) {
				agg.StartTime = j.StartTime
			}
			if j.EndTime > agg.EndTime {
				agg.EndTime = j.EndTime
			}
		}
	}

	// Fill in missing components from workflow definition so the caller can see
	// waiting/queued/cancelled components even before job records exist.
	if workflow, wfErr := repository.WorkflowByID(ctx, w.Store, task.WorkflowID); wfErr == nil {
		names := collectWorkflowComponentNames(workflow)
		defaultStatus := defaultComponentStatus(task.Status)
		for _, name := range names {
			key := strings.ToLower(name)
			if _, exists := componentAggregates[key]; exists {
				continue
			}
			componentAggregates[key] = &apis.ComponentTaskStatus{
				Name:   name,
				Status: defaultStatus,
			}
		}
	} else if !errors.Is(wfErr, datastore.ErrRecordNotExist) {
		klog.V(4).Infof("load workflow %s for task %s failed: %v", task.WorkflowID, taskID, wfErr)
	}

	componentStatuses := make([]apis.ComponentTaskStatus, 0, len(componentAggregates))
	for _, cs := range componentAggregates {
		componentStatuses = append(componentStatuses, *cs)
	}

	return &apis.TaskStatusResponse{
		TaskID:       task.TaskID,
		Status:       string(task.Status),
		WorkflowID:   task.WorkflowID,
		WorkflowName: task.WorkflowName,
		AppID:        task.AppID,
		Type:         task.Type,
		Components:   componentStatuses,
	}, nil
}

// chooseAggStatus merges two statuses, preferring failure/timeouts over running, over waiting.
func chooseAggStatus(current, incoming string) string {
	priority := func(status string) int {
		switch config.Status(status) {
		case config.StatusFailed, config.StatusTimeout, config.StatusReject:
			return 4
		case config.StatusCancelled:
			return 3
		case config.StatusRunning, config.StatusPrepare, config.StatusDistributed, config.StatusDebugBefore, config.StatusDebugAfter:
			return 2
		case config.StatusCompleted, config.StatusPassed:
			return 1
		default:
			return 0
		}
	}
	if priority(incoming) > priority(current) {
		return incoming
	}
	return current
}

// collectWorkflowComponentNames extracts all unique component names declared in a workflow.
func collectWorkflowComponentNames(workflow *model.Workflow) []string {
	if workflow == nil || workflow.Steps == nil {
		return nil
	}
	raw, err := json.Marshal(workflow.Steps)
	if err != nil {
		klog.Errorf("marshal workflow steps for %s failed: %v", workflow.ID, err)
		return nil
	}
	var steps model.WorkflowSteps
	if err := json.Unmarshal(raw, &steps); err != nil {
		klog.Errorf("unmarshal workflow steps for %s failed: %v", workflow.ID, err)
		return nil
	}
	seen := make(map[string]struct{})
	var names []string
	add := func(name string) {
		if name == "" {
			return
		}
		key := strings.ToLower(name)
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		names = append(names, name)
	}
	for _, step := range steps.Steps {
		for _, n := range step.ComponentNames() {
			add(n)
		}
		for _, sub := range step.SubSteps {
			for _, n := range sub.ComponentNames() {
				add(n)
			}
		}
	}
	return names
}

func defaultComponentStatus(taskStatus config.Status) string {
	switch taskStatus {
	case config.StatusCancelled:
		return string(config.StatusCancelled)
	case config.StatusFailed, config.StatusTimeout, config.StatusReject:
		return string(taskStatus)
	case config.StatusCompleted:
		return string(config.StatusCompleted)
	default:
		return string(config.StatusWaiting)
	}
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
	// Audit log: record cancel operation with full context
	klog.Infof("AUDIT: cancel workflow task taskID=%s workflowID=%s workflowName=%s user=%s reason=%s prevStatus=%s",
		task.TaskID, task.WorkflowID, task.WorkflowName, userName, reason, task.Status)

	task.TaskRevoker = userName
	task.Status = config.StatusCancelled

	if err := repository.UpdateTask(ctx, w.Store, task); err != nil {
		klog.Errorf("AUDIT: cancel workflow task failed taskID=%s user=%s error=%v", task.TaskID, userName, err)
		return err
	}
	if reason == "" {
		reason = fmt.Sprintf("cancelled by %s", userName)
	}
	if err := signal.Cancel(ctx, task.TaskID, reason); err != nil {
		klog.Errorf("AUDIT: signal cancel failed taskID=%s user=%s error=%v", task.TaskID, userName, err)
		return err
	}

	klog.Infof("AUDIT: cancel workflow task completed taskID=%s user=%s", task.TaskID, userName)
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

func (w *workflowServiceImpl) rollbackWorkflowCreation(ctx context.Context, workflow *model.Workflow) {
	if workflow == nil {
		return
	}
	if err := repository.DelComponentsByAppID(ctx, w.Store, workflow.ID); err != nil {
		klog.Errorf("cleanup components for workflow %s failed: %v", workflow.ID, err)
	}
	if err := repository.DelWorkflow(ctx, w.Store, workflow); err != nil {
		klog.Errorf("cleanup workflow %s failed: %v", workflow.ID, err)
	}
}
