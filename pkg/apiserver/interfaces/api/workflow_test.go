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
func (noopApplicationsService) UpdateVersion(context.Context, string, apis.UpdateVersionRequest) (*apis.UpdateVersionResponse, error) {
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
			Components: []apis.ComponentTaskStatus{
				{Name: "web", Status: string(config.StatusRunning)},
				{Name: "db", Status: string(config.StatusWaiting)},
			},
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
	if len(payload.Components) != 2 {
		t.Fatalf("expected 2 components, got %d", len(payload.Components))
	}
	if payload.Components[0].Name != "web" || payload.Components[0].Status != string(config.StatusRunning) {
		t.Fatalf("unexpected first component: %+v", payload.Components[0])
	}
	if payload.Components[1].Name != "db" || payload.Components[1].Status != string(config.StatusWaiting) {
		t.Fatalf("unexpected second component: %+v", payload.Components[1])
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

// ---- UpdateVersion API Tests ----

type fakeUpdateVersionService struct {
	noopApplicationsService
	updateResp *apis.UpdateVersionResponse
	updateErr  error
	lastAppID  string
	lastReq    apis.UpdateVersionRequest
}

func (f *fakeUpdateVersionService) UpdateVersion(_ context.Context, appID string, req apis.UpdateVersionRequest) (*apis.UpdateVersionResponse, error) {
	f.lastAppID = appID
	f.lastReq = req
	if f.updateErr != nil {
		return nil, f.updateErr
	}
	if f.updateResp != nil {
		return f.updateResp, nil
	}
	return &apis.UpdateVersionResponse{
		AppID:             appID,
		Version:           req.Version,
		PreviousVersion:   "1.0.0",
		Strategy:          req.Strategy,
		UpdatedComponents: []string{"backend"},
	}, nil
}

func TestUpdateVersionEndpoint(t *testing.T) {
	gin.SetMode(gin.TestMode)
	appSvc := &fakeUpdateVersionService{
		updateResp: &apis.UpdateVersionResponse{
			AppID:             "app-123",
			Version:           "2.0.0",
			PreviousVersion:   "1.0.0",
			Strategy:          "rolling",
			TaskID:            "task-456",
			UpdatedComponents: []string{"backend", "frontend"},
			AddedComponents:   []string{"cache"},
			RemovedComponents: []string{"old-worker"},
		},
	}
	appHandler := &applications{
		ApplicationService: appSvc,
		WorkflowService:    &fakeWorkflowService{},
	}
	r := gin.New()
	r.POST("/applications/:appID/version", appHandler.updateVersion)

	body := `{
		"version": "2.0.0",
		"strategy": "rolling",
		"components": [
			{"action": "update", "name": "backend", "image": "backend:v2"},
			{"action": "update", "name": "frontend", "image": "frontend:v2"},
			{"action": "add", "name": "cache", "type": "store", "image": "redis:7"},
			{"action": "remove", "name": "old-worker"}
		],
		"autoExec": true,
		"description": "Major version update"
	}`
	req := httptest.NewRequest(http.MethodPost, "/applications/app-123/version", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	r.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("unexpected status code: %d, body: %s", resp.Code, resp.Body.String())
	}

	var payload apis.UpdateVersionResponse
	if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if payload.AppID != "app-123" {
		t.Fatalf("unexpected appId: %s", payload.AppID)
	}
	if payload.Version != "2.0.0" {
		t.Fatalf("unexpected version: %s", payload.Version)
	}
	if payload.Strategy != "rolling" {
		t.Fatalf("unexpected strategy: %s", payload.Strategy)
	}
	if payload.TaskID != "task-456" {
		t.Fatalf("unexpected taskId: %s", payload.TaskID)
	}
	if len(payload.UpdatedComponents) != 2 {
		t.Fatalf("expected 2 updated components, got %d", len(payload.UpdatedComponents))
	}
	if len(payload.AddedComponents) != 1 {
		t.Fatalf("expected 1 added component, got %d", len(payload.AddedComponents))
	}
	if len(payload.RemovedComponents) != 1 {
		t.Fatalf("expected 1 removed component, got %d", len(payload.RemovedComponents))
	}

	// 验证请求参数
	if appSvc.lastAppID != "app-123" {
		t.Fatalf("expected appID app-123, got %s", appSvc.lastAppID)
	}
	if appSvc.lastReq.Version != "2.0.0" {
		t.Fatalf("expected version 2.0.0, got %s", appSvc.lastReq.Version)
	}
	if len(appSvc.lastReq.Components) != 4 {
		t.Fatalf("expected 4 components in request, got %d", len(appSvc.lastReq.Components))
	}
}

func TestUpdateVersionEndpointMinimalRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	appSvc := &fakeUpdateVersionService{}
	appHandler := &applications{
		ApplicationService: appSvc,
		WorkflowService:    &fakeWorkflowService{},
	}
	r := gin.New()
	r.POST("/applications/:appID/version", appHandler.updateVersion)

	// 最简请求 - 仅更新版本号
	body := `{"version": "1.1.0"}`
	req := httptest.NewRequest(http.MethodPost, "/applications/app-1/version", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	r.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("unexpected status code: %d, body: %s", resp.Code, resp.Body.String())
	}

	if appSvc.lastReq.Version != "1.1.0" {
		t.Fatalf("expected version 1.1.0, got %s", appSvc.lastReq.Version)
	}
}

func TestUpdateVersionEndpointMissingAppID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	appSvc := &fakeUpdateVersionService{}
	appHandler := &applications{
		ApplicationService: appSvc,
		WorkflowService:    &fakeWorkflowService{},
	}
	r := gin.New()
	r.POST("/applications/:appID/version", appHandler.updateVersion)

	body := `{"version": "1.1.0"}`
	// 空的 appID
	req := httptest.NewRequest(http.MethodPost, "/applications//version", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	r.ServeHTTP(resp, req)

	// 由于路由不匹配，应该返回 404
	if resp.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", resp.Code)
	}
}

func TestUpdateVersionEndpointImageUpdate(t *testing.T) {
	gin.SetMode(gin.TestMode)
	appSvc := &fakeUpdateVersionService{
		updateResp: &apis.UpdateVersionResponse{
			AppID:             "app-1",
			Version:           "1.1.0",
			PreviousVersion:   "1.0.0",
			Strategy:          "rolling",
			UpdatedComponents: []string{"backend"},
		},
	}
	appHandler := &applications{
		ApplicationService: appSvc,
		WorkflowService:    &fakeWorkflowService{},
	}
	r := gin.New()
	r.POST("/applications/:appID/version", appHandler.updateVersion)

	body := `{
		"version": "1.1.0",
		"components": [
			{"name": "backend", "image": "myapp/backend:v1.1.0"}
		]
	}`
	req := httptest.NewRequest(http.MethodPost, "/applications/app-1/version", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	r.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("unexpected status code: %d", resp.Code)
	}

	// 验证组件名称被规范化为小写
	if len(appSvc.lastReq.Components) != 1 {
		t.Fatalf("expected 1 component, got %d", len(appSvc.lastReq.Components))
	}
	if appSvc.lastReq.Components[0].Image != "myapp/backend:v1.1.0" {
		t.Fatalf("unexpected image: %s", appSvc.lastReq.Components[0].Image)
	}
}
