package workflow

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/client-go/kubernetes/fake"

	"KubeMin-Cli/pkg/apiserver/config"
	"KubeMin-Cli/pkg/apiserver/domain/model"
	"KubeMin-Cli/pkg/apiserver/infrastructure/datastore"
	msg "KubeMin-Cli/pkg/apiserver/infrastructure/messaging"
	apis "KubeMin-Cli/pkg/apiserver/interfaces/api/dto/v1"
)

type workflowAckTestStore struct {
	workflow *model.Workflow
	task     *model.WorkflowQueue
}

func (s *workflowAckTestStore) Add(context.Context, datastore.Entity) error        { return nil }
func (s *workflowAckTestStore) BatchAdd(context.Context, []datastore.Entity) error { return nil }
func (s *workflowAckTestStore) Put(context.Context, datastore.Entity) error        { return nil }
func (s *workflowAckTestStore) Delete(context.Context, datastore.Entity) error     { return nil }
func (s *workflowAckTestStore) DeleteByFilter(context.Context, datastore.Entity, *datastore.FilterOptions) error {
	return nil
}

func (s *workflowAckTestStore) Get(ctx context.Context, entity datastore.Entity) error {
	switch e := entity.(type) {
	case *model.Workflow:
		if s.workflow == nil {
			return fmt.Errorf("workflow not configured")
		}
		*e = *s.workflow
	case *model.WorkflowQueue:
		if s.task == nil {
			return datastore.ErrRecordNotExist
		}
		*e = *s.task
	default:
	}
	return nil
}

func (s *workflowAckTestStore) List(ctx context.Context, query datastore.Entity, _ *datastore.ListOptions) ([]datastore.Entity, error) {
	if _, ok := query.(*model.ApplicationComponent); ok {
		return []datastore.Entity{}, nil
	}
	return nil, nil
}

func (s *workflowAckTestStore) Count(context.Context, datastore.Entity, *datastore.FilterOptions) (int64, error) {
	return 0, nil
}

func (s *workflowAckTestStore) IsExist(context.Context, datastore.Entity) (bool, error) {
	return false, nil
}

func (s *workflowAckTestStore) IsExistByCondition(context.Context, string, map[string]interface{}, interface{}) (bool, error) {
	return false, nil
}

type stubWorkflowService struct {
	updateOK bool
}

func (s *stubWorkflowService) ListApplicationWorkflow(context.Context, *model.Applications) error {
	return nil
}

func (s *stubWorkflowService) CreateWorkflowTask(context.Context, apis.CreateWorkflowRequest) (*apis.CreateWorkflowResponse, error) {
	return nil, nil
}

func (s *stubWorkflowService) ExecWorkflowTask(context.Context, string) (*apis.ExecWorkflowResponse, error) {
	return nil, nil
}

func (s *stubWorkflowService) ExecWorkflowTaskForApp(context.Context, string, string) (*apis.ExecWorkflowResponse, error) {
	return nil, nil
}

func (s *stubWorkflowService) WaitingTasks(context.Context) ([]*model.WorkflowQueue, error) {
	return nil, nil
}
func (s *stubWorkflowService) UpdateTask(context.Context, *model.WorkflowQueue) bool {
	return s.updateOK
}
func (s *stubWorkflowService) TaskRunning(context.Context) ([]*model.WorkflowQueue, error) {
	return nil, nil
}
func (s *stubWorkflowService) CancelWorkflowTask(context.Context, string, string, string) error {
	return nil
}

func (s *stubWorkflowService) CancelWorkflowTaskForApp(context.Context, string, string, string, string) error {
	return nil
}
func (s *stubWorkflowService) MarkTaskStatus(context.Context, string, config.Status, config.Status) (bool, error) {
	return true, nil
}

func newWorkflowForAckTests(updateOK bool) *Workflow {
	steps, _ := model.NewJSONStructByStruct(&model.WorkflowSteps{})
	store := &workflowAckTestStore{
		workflow: &model.Workflow{
			ID:        "wf-1",
			Namespace: "default",
			Steps:     steps,
		},
		task: &model.WorkflowQueue{
			TaskID:       "task-1",
			WorkflowID:   "wf-1",
			AppID:        "app-1",
			ProjectID:    "proj-1",
			WorkflowName: "demo",
		},
	}

	return &Workflow{
		KubeClient:      fake.NewSimpleClientset(),
		Store:           store,
		WorkflowService: &stubWorkflowService{updateOK: updateOK},
		Queue:           &msg.NoopQueue{},
		Cfg:             &config.Config{},
	}
}

func TestProcessDispatchMessageAckOnSuccess(t *testing.T) {
	w := newWorkflowForAckTests(true)
	payload, err := MarshalTaskDispatch(TaskDispatch{TaskID: "task-1", WorkflowID: "wf-1"})
	require.NoError(t, err)

	ack, taskID := w.processDispatchMessage(context.Background(), msg.Message{ID: "1-0", Payload: payload})
	require.True(t, ack)
	require.Equal(t, "task-1", taskID)
}

func TestProcessDispatchMessageLeavesPendingOnFailure(t *testing.T) {
	w := newWorkflowForAckTests(false)
	payload, err := MarshalTaskDispatch(TaskDispatch{TaskID: "task-1", WorkflowID: "wf-1"})
	require.NoError(t, err)

	ack, taskID := w.processDispatchMessage(context.Background(), msg.Message{ID: "1-0", Payload: payload})
	require.False(t, ack)
	require.Equal(t, "task-1", taskID)
}

func TestProcessDispatchMessageAckOnDecodeError(t *testing.T) {
	w := newWorkflowForAckTests(true)
	ack, taskID := w.processDispatchMessage(context.Background(), msg.Message{ID: "1-0", Payload: []byte("oops")})
	require.True(t, ack)
	require.Equal(t, "", taskID)
}
