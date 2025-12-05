package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"KubeMin-Cli/pkg/apiserver/config"
	"KubeMin-Cli/pkg/apiserver/domain/model"
	"KubeMin-Cli/pkg/apiserver/infrastructure/datastore"
)

func TestGetTaskStatusIncludesAllComponents(t *testing.T) {
	steps := &model.WorkflowSteps{
		Steps: []*model.WorkflowStep{
			{
				Name:       "web",
				Properties: []model.Policies{{Policies: []string{"web"}}},
			},
			{
				Name:       "database",
				Properties: []model.Policies{{Policies: []string{"db"}}},
			},
			{
				SubSteps: []*model.WorkflowSubStep{
					{
						Name:       "cache",
						Properties: []model.Policies{{Policies: []string{"cache"}}},
					},
				},
			},
		},
	}
	stepsStruct, err := model.NewJSONStructByStruct(steps)
	require.NoError(t, err)

	task := &model.WorkflowQueue{
		TaskID:       "task-1",
		WorkflowID:   "wf-1",
		WorkflowName: "deploy",
		AppID:        "app-1",
		Status:       config.StatusRunning,
	}

	store := &statusDataStore{
		task: task,
		workflow: &model.Workflow{
			ID:    "wf-1",
			Name:  "deploy",
			AppID: "app-1",
			Steps: stepsStruct,
		},
		jobs: []*model.JobInfo{
			{
				TaskID:      "task-1",
				ServiceName: "web",
				Status:      string(config.StatusRunning),
			},
			{
				TaskID:      "task-1",
				ServiceName: "db",
				Status:      string(config.StatusFailed),
				Error:       "deploy failed",
			},
		},
	}

	svc := &workflowServiceImpl{Store: store}
	resp, err := svc.GetTaskStatus(context.Background(), "task-1")
	require.NoError(t, err)

	require.Equal(t, "task-1", resp.TaskID)
	require.Equal(t, 3, len(resp.Components))

	byName := map[string]string{}
	for _, c := range resp.Components {
		byName[c.Name] = c.Status
	}

	require.Equal(t, string(config.StatusRunning), byName["web"])
	require.Equal(t, string(config.StatusFailed), byName["db"])
	// cache does not have job info yet; expect default waiting status
	require.Equal(t, string(config.StatusWaiting), byName["cache"])
}

type statusDataStore struct {
	task     *model.WorkflowQueue
	workflow *model.Workflow
	jobs     []*model.JobInfo
}

func (s *statusDataStore) Add(context.Context, datastore.Entity) error        { return nil }
func (s *statusDataStore) BatchAdd(context.Context, []datastore.Entity) error { return nil }
func (s *statusDataStore) Put(context.Context, datastore.Entity) error        { return nil }
func (s *statusDataStore) Delete(context.Context, datastore.Entity) error     { return nil }
func (s *statusDataStore) DeleteByFilter(context.Context, datastore.Entity, *datastore.FilterOptions) error {
	return nil
}

func (s *statusDataStore) Get(_ context.Context, entity datastore.Entity) error {
	switch v := entity.(type) {
	case *model.WorkflowQueue:
		if s.task != nil && v.TaskID == s.task.TaskID {
			*v = *s.task
			return nil
		}
	case *model.Workflow:
		if s.workflow != nil && v.ID == s.workflow.ID {
			*v = *s.workflow
			return nil
		}
	}
	return datastore.ErrRecordNotExist
}

func (s *statusDataStore) List(_ context.Context, query datastore.Entity, _ *datastore.ListOptions) ([]datastore.Entity, error) {
	if jobQuery, ok := query.(*model.JobInfo); ok {
		var out []datastore.Entity
		for _, job := range s.jobs {
			if jobQuery.TaskID != "" && job.TaskID != jobQuery.TaskID {
				continue
			}
			out = append(out, job)
		}
		if len(out) == 0 {
			return nil, datastore.ErrRecordNotExist
		}
		return out, nil
	}
	return nil, datastore.ErrRecordNotExist
}

func (s *statusDataStore) Count(context.Context, datastore.Entity, *datastore.FilterOptions) (int64, error) {
	return 0, nil
}

func (s *statusDataStore) IsExist(context.Context, datastore.Entity) (bool, error) {
	return false, nil
}

func (s *statusDataStore) IsExistByCondition(context.Context, string, map[string]interface{}, interface{}) (bool, error) {
	return false, nil
}

var _ datastore.DataStore = (*statusDataStore)(nil)
