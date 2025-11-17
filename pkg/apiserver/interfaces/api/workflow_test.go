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
func (noopApplicationsService) DeleteApplication(context.Context, *model.Applications) error {
	return nil
}
func (noopApplicationsService) CleanupApplicationResources(context.Context, string) (*apis.CleanupApplicationResourcesResponse, error) {
	return nil, nil
}
func (noopApplicationsService) UpdateApplicationWorkflow(context.Context, string, apis.UpdateApplicationWorkflowRequest) (*apis.UpdateWorkflowResponse, error) {
	return nil, nil
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
