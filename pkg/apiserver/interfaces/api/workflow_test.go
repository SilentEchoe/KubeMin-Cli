package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"KubeMin-Cli/pkg/apiserver/config"
	"KubeMin-Cli/pkg/apiserver/domain/model"
	apis "KubeMin-Cli/pkg/apiserver/interfaces/api/dto/v1"
)

type fakeWorkflowService struct {
	execResp           *apis.ExecWorkflowResponse
	execForAppCalled   bool
	lastExecAppID      string
	lastExecWorkflowID string
	cancelForAppCalled bool
	lastCancelAppID    string
	lastCancelUser     string
	lastCancelReason   string
	lastCancelTaskID   string
	cancelCalled       bool
	lastUser           string
	lastReason         string
	taskStatusResp     *apis.TaskStatusResponse
}

func (f *fakeWorkflowService) ListApplicationWorkflow(context.Context, *model.Applications) error {
	return nil
}

func (f *fakeWorkflowService) CreateWorkflowTask(context.Context, apis.CreateWorkflowRequest) (*apis.CreateWorkflowResponse, error) {
	return nil, nil
}

func (f *fakeWorkflowService) ExecWorkflowTask(context.Context, string) (*apis.ExecWorkflowResponse, error) {
	return nil, nil
}

func (f *fakeWorkflowService) ExecWorkflowTaskForApp(_ context.Context, appID, workflowID string) (*apis.ExecWorkflowResponse, error) {
	f.execForAppCalled = true
	f.lastExecAppID = appID
	f.lastExecWorkflowID = workflowID
	if f.execResp == nil {
		f.execResp = &apis.ExecWorkflowResponse{TaskID: "test-task"}
	}
	return f.execResp, nil
}

func (f *fakeWorkflowService) WaitingTasks(context.Context) ([]*model.WorkflowQueue, error) {
	return nil, nil
}

func (f *fakeWorkflowService) UpdateTask(context.Context, *model.WorkflowQueue) bool { return true }

func (f *fakeWorkflowService) TaskRunning(context.Context) ([]*model.WorkflowQueue, error) {
	return nil, nil
}

func (f *fakeWorkflowService) CancelWorkflowTask(ctx context.Context, userName, taskID, reason string) error {
	f.cancelCalled = true
	f.lastUser = userName
	f.lastReason = reason
	f.lastCancelTaskID = taskID
	return nil
}

func (f *fakeWorkflowService) CancelWorkflowTaskForApp(_ context.Context, appID, userName, taskID, reason string) error {
	f.cancelForAppCalled = true
	f.lastCancelAppID = appID
	f.lastCancelUser = userName
	f.lastCancelTaskID = taskID
	f.lastCancelReason = reason
	return nil
}

func (f *fakeWorkflowService) MarkTaskStatus(context.Context, string, config.Status, config.Status) (bool, error) {
	return false, nil
}

func (f *fakeWorkflowService) GetTaskStatus(context.Context, string) (*apis.TaskStatusResponse, error) {
	if f.taskStatusResp == nil {
		return &apis.TaskStatusResponse{TaskID: "task-123", Status: string(config.StatusRunning)}, nil
	}
	return f.taskStatusResp, nil
}

type noopApplicationsService struct{}

func (noopApplicationsService) CreateApplications(context.Context, apis.CreateApplicationsRequest) (*apis.ApplicationBase, error) {
	return nil, nil
}
func (noopApplicationsService) GetApplication(context.Context, string) (*model.Applications, error) {
	return nil, nil
}
func (noopApplicationsService) ListApplications(context.Context) ([]*apis.ApplicationBase, error) {
	return nil, nil
}
func (noopApplicationsService) ListTemplateApplications(context.Context) ([]*apis.ApplicationBase, error) {
	return nil, nil
}
func (noopApplicationsService) DeleteApplication(context.Context, *model.Applications) error {
	return nil
}
func (noopApplicationsService) CleanupApplicationResources(context.Context, string) (*apis.CleanupApplicationResourcesResponse, error) {
	return nil, nil
}
func (noopApplicationsService) UpdateApplicationWorkflow(context.Context, string, apis.UpdateApplicationWorkflowRequest) (*apis.UpdateWorkflowResponse, error) {
	return nil, nil
}
func (noopApplicationsService) ListApplicationWorkflows(context.Context, string) ([]*model.Workflow, error) {
	return nil, nil
}
func (noopApplicationsService) ListApplicationComponents(context.Context, string) ([]*model.ApplicationComponent, error) {
	return nil, nil
}

type workflowListApplicationService struct {
	noopApplicationsService
	workflows []*model.Workflow
	err       error
}

func (s workflowListApplicationService) ListApplicationWorkflows(context.Context, string) ([]*model.Workflow, error) {
	return s.workflows, s.err
}

type templateApplicationService struct {
	noopApplicationsService
	templates []*apis.ApplicationBase
	err       error
}

func (s templateApplicationService) ListTemplateApplications(context.Context) ([]*apis.ApplicationBase, error) {
	return s.templates, s.err
}

func TestExecApplicationWorkflowEndpoint(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &fakeWorkflowService{}
	appHandler := &applications{
		ApplicationService: noopApplicationsService{},
		WorkflowService:    svc,
	}
	r := gin.New()
	r.POST("/applications/:appID/workflow/exec", appHandler.execApplicationWorkflow)

	body := `{"workflowId":"wf-123"}`
	req := httptest.NewRequest(http.MethodPost, "/applications/app-1/workflow/exec", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	r.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("unexpected status code: %d", resp.Code)
	}

	var payload apis.ExecWorkflowResponse
	if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.TaskID != "test-task" {
		t.Fatalf("unexpected taskID %s", payload.TaskID)
	}
	if !svc.execForAppCalled || svc.lastExecAppID != "app-1" || svc.lastExecWorkflowID != "wf-123" {
		t.Fatalf("expected exec workflow for app to be invoked")
	}
}

func TestCancelApplicationWorkflowEndpoint(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &fakeWorkflowService{}
	appHandler := &applications{
		ApplicationService: noopApplicationsService{},
		WorkflowService:    svc,
	}
	r := gin.New()
	r.POST("/applications/:appID/workflow/cancel", appHandler.cancelApplicationWorkflow)

	body := `{"taskId":"demo-task","user":"tester","reason":"manual stop"}`
	req := httptest.NewRequest(http.MethodPost, "/applications/app-2/workflow/cancel", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	r.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("unexpected status code: %d", resp.Code)
	}

	var payload apis.CancelWorkflowResponse
	if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if payload.TaskID != "demo-task" {
		t.Fatalf("expected taskId demo-task, got %s", payload.TaskID)
	}
	if payload.Status != string(config.StatusCancelled) {
		t.Fatalf("expected status cancelled, got %s", payload.Status)
	}
	if !svc.cancelForAppCalled || svc.lastCancelAppID != "app-2" || svc.lastCancelUser != "tester" || svc.lastCancelTaskID != "demo-task" {
		t.Fatalf("expected cancel for app to be invoked")
	}
	if svc.lastCancelReason != "manual stop" {
		t.Fatalf("unexpected cancel reason: %s", svc.lastCancelReason)
	}
}

func TestGetWorkflowTaskStatusEndpoint(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &fakeWorkflowService{
		taskStatusResp: &apis.TaskStatusResponse{
			TaskID:       "task-abc",
			Status:       string(config.StatusQueued),
			WorkflowID:   "wf-1",
			WorkflowName: "deploy",
			AppID:        "app-1",
		},
	}
	appHandler := &applications{
		ApplicationService: noopApplicationsService{},
		WorkflowService:    svc,
	}
	r := gin.New()
	r.GET("/workflow/tasks/:taskID/status", appHandler.getWorkflowTaskStatus)

	req := httptest.NewRequest(http.MethodGet, "/workflow/tasks/task-abc/status", nil)
	resp := httptest.NewRecorder()

	r.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("unexpected status code: %d", resp.Code)
	}

	var payload apis.TaskStatusResponse
	if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.TaskID != "task-abc" || payload.Status != string(config.StatusQueued) {
		t.Fatalf("unexpected payload: %+v", payload)
	}
	if payload.WorkflowID != "wf-1" || payload.WorkflowName != "deploy" || payload.AppID != "app-1" {
		t.Fatalf("unexpected workflow info: %+v", payload)
	}
}

func TestListApplicationWorkflowsEndpoint(t *testing.T) {
	gin.SetMode(gin.TestMode)
	steps := &model.WorkflowSteps{
		Steps: []*model.WorkflowStep{
			{
				Name:       "deploy-nginx",
				Mode:       config.WorkflowModeStepByStep,
				Properties: []model.Policies{{Policies: []string{"nginx"}}},
			},
		},
	}
	stepStruct, err := model.NewJSONStructByStruct(steps)
	if err != nil {
		t.Fatalf("build steps json: %v", err)
	}
	appSvc := workflowListApplicationService{
		workflows: []*model.Workflow{
			{
				ID:          "wf-1",
				Name:        "deploy",
				Alias:       "Deploy",
				Namespace:   "default",
				ProjectID:   "proj",
				Description: "desc",
				Status:      config.StatusRunning,
				Steps:       stepStruct,
			},
		},
	}
	appHandler := &applications{
		ApplicationService: appSvc,
		WorkflowService:    &fakeWorkflowService{},
	}
	r := gin.New()
	r.GET("/applications/:appID/workflows", appHandler.listApplicationWorkflows)

	req := httptest.NewRequest(http.MethodGet, "/applications/app-1/workflows", nil)
	resp := httptest.NewRecorder()

	r.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("unexpected status code: %d", resp.Code)
	}
	var payload apis.ListApplicationWorkflowsResponse
	if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(payload.Workflows) != 1 {
		t.Fatalf("expected one workflow, got %d", len(payload.Workflows))
	}
	if payload.Workflows[0].ID != "wf-1" {
		t.Fatalf("unexpected workflow ID %s", payload.Workflows[0].ID)
	}
	if len(payload.Workflows[0].Steps) != 1 {
		t.Fatalf("expected one workflow step")
	}
	if len(payload.Workflows[0].Steps[0].Components) != 1 || payload.Workflows[0].Steps[0].Components[0] != "nginx" {
		t.Fatalf("unexpected workflow step components: %+v", payload.Workflows[0].Steps[0].Components)
	}
}

func TestListTemplateApplicationsEndpoint(t *testing.T) {
	gin.SetMode(gin.TestMode)
	appSvc := templateApplicationService{
		templates: []*apis.ApplicationBase{
			{ID: "app-tmpl-1", Name: "tmpl-1", TmpEnable: true},
			{ID: "app-tmpl-2", Name: "tmpl-2", TmpEnable: true},
		},
	}
	appHandler := &applications{
		ApplicationService: appSvc,
		WorkflowService:    &fakeWorkflowService{},
	}
	r := gin.New()
	r.GET("/applications/templates", appHandler.listTemplateApplications)

	req := httptest.NewRequest(http.MethodGet, "/applications/templates", nil)
	resp := httptest.NewRecorder()

	r.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("unexpected status code: %d", resp.Code)
	}

	var payload apis.ListApplicationResponse
	if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(payload.Applications) != 2 {
		t.Fatalf("expected 2 templates, got %d", len(payload.Applications))
	}
	if payload.Applications[0].ID != "app-tmpl-1" {
		t.Fatalf("unexpected first template: %+v", payload.Applications[0])
	}
}

func TestWorkflowCancelEndpointNotImplemented(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &fakeWorkflowService{}
	w := &workflow{WorkflowService: svc}
	r := gin.New()
	r.POST("/workflow/cancel", w.cancelWorkflowTask)

	body := `{"taskId":"demo-task","user":"tester","reason":"manual stop"}`
	req := httptest.NewRequest(http.MethodPost, "/workflow/cancel", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	r.ServeHTTP(resp, req)

	if resp.Code != http.StatusNotImplemented {
		t.Fatalf("expected 501 status code, got %d", resp.Code)
	}
}
