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
	cancelCalled bool
	lastUser     string
	lastReason   string
	lastTaskID   string
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
	f.lastTaskID = taskID
	return nil
}

func (f *fakeWorkflowService) MarkTaskStatus(context.Context, string, config.Status, config.Status) (bool, error) {
	return false, nil
}

func TestCancelWorkflowEndpoint(t *testing.T) {
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

	if resp.Code != http.StatusOK {
		t.Fatalf("unexpected status code: %d", resp.Code)
	}

	var payload apis.CancelWorkflowResponse
	if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if payload.TaskId != "demo-task" {
		t.Fatalf("expected taskId demo-task, got %s", payload.TaskId)
	}
	if payload.Status != string(config.StatusCancelled) {
		t.Fatalf("expected status cancelled, got %s", payload.Status)
	}
	if !svc.cancelCalled {
		t.Fatalf("expected cancel to be called")
	}
	if svc.lastUser != "tester" {
		t.Fatalf("unexpected user: %s", svc.lastUser)
	}
	if svc.lastReason != "manual stop" {
		t.Fatalf("unexpected reason: %s", svc.lastReason)
	}
	if svc.lastTaskID != "demo-task" {
		t.Fatalf("unexpected taskID: %s", svc.lastTaskID)
	}
}
