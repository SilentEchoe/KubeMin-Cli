package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"KubeMin-Cli/pkg/apiserver/config"
	"KubeMin-Cli/pkg/apiserver/domain/model"
	"KubeMin-Cli/pkg/apiserver/infrastructure/datastore"
	apisv1 "KubeMin-Cli/pkg/apiserver/interfaces/api/dto/v1"
	"KubeMin-Cli/pkg/apiserver/utils/bcode"
)

func TestUpdateApplicationWorkflowCreatesWorkflow(t *testing.T) {
	store := newInMemoryAppStore()
	store.apps["app-1"] = &model.Applications{
		ID:      "app-1",
		Name:    "DemoApp",
		Project: "proj-1",
	}
	store.components["config"] = &model.ApplicationComponent{Name: "config", AppID: "app-1"}
	store.components["mysql-primary"] = &model.ApplicationComponent{Name: "mysql-primary", AppID: "app-1"}
	store.components["mysql-replica"] = &model.ApplicationComponent{Name: "mysql-replica", AppID: "app-1"}
	store.components["dashboard"] = &model.ApplicationComponent{Name: "dashboard", AppID: "app-1"}
	svc := &applicationsServiceImpl{Store: store}

	req := apisv1.UpdateApplicationWorkflowRequest{
		Name:  "custom-flow",
		Alias: "primary-flow",
		Workflow: []apisv1.CreateWorkflowStepRequest{
			{Name: "prepare-config", Mode: "StepByStep", Components: []string{"config"}},
			{Name: "databases", Mode: "DAG", Components: []string{"mysql-primary", "mysql-replica"}},
			{Name: "deploy-dashboard", Components: []string{"dashboard"}},
		},
	}

	resp, err := svc.UpdateApplicationWorkflow(context.Background(), "app-1", req)
	require.NoError(t, err)
	require.NotEmpty(t, resp.WorkflowID)

	stored := store.workflows[resp.WorkflowID]
	require.NotNil(t, stored)
	require.Equal(t, "custom-flow", stored.Name)
	require.Equal(t, "primary-flow", stored.Alias)

	steps := decodeWorkflowSteps(t, stored.Steps)
	require.Len(t, steps.Steps, 3)
	require.Equal(t, config.WorkflowModeDAG, steps.Steps[1].Mode)
	require.ElementsMatch(t, []string{"mysql-primary", "mysql-replica"}, steps.Steps[1].Properties[0].Policies)
}

func TestUpdateApplicationWorkflowUpdatesExisting(t *testing.T) {
	store := newInMemoryAppStore()
	store.apps["app-1"] = &model.Applications{
		ID:      "app-1",
		Name:    "DemoApp",
		Project: "proj-1",
	}
	existing := &model.Workflow{
		ID:        "wf-1",
		Name:      "existing",
		AppID:     "app-1",
		ProjectID: "proj-1",
		Alias:     "old",
	}
	store.workflows[existing.ID] = existing
	store.components["dashboard"] = &model.ApplicationComponent{Name: "dashboard", AppID: "app-1"}

	svc := &applicationsServiceImpl{Store: store}

	req := apisv1.UpdateApplicationWorkflowRequest{
		WorkflowID: "wf-1",
		Alias:      "updated-alias",
		Workflow: []apisv1.CreateWorkflowStepRequest{
			{Name: "deploy-dashboard", Mode: "StepByStep", Components: []string{"dashboard"}},
		},
	}

	resp, err := svc.UpdateApplicationWorkflow(context.Background(), "app-1", req)
	require.NoError(t, err)
	require.Equal(t, "wf-1", resp.WorkflowID)

	stored := store.workflows["wf-1"]
	require.Equal(t, "updated-alias", stored.Alias)
	steps := decodeWorkflowSteps(t, stored.Steps)
	require.Len(t, steps.Steps, 1)
	require.Equal(t, "deploy-dashboard", steps.Steps[0].Name)
	require.Equal(t, config.WorkflowModeStepByStep, steps.Steps[0].Mode)
}

func TestUpdateApplicationWorkflowCreatesNewWhenWorkflowIDMissing(t *testing.T) {
	store := newInMemoryAppStore()
	store.apps["app-1"] = &model.Applications{
		ID:      "app-1",
		Name:    "DemoApp",
		Project: "proj-1",
	}
	store.components["nginx"] = &model.ApplicationComponent{Name: "nginx", AppID: "app-1"}

	existing := &model.Workflow{
		ID:        "wf-1",
		Name:      "default-flow",
		AppID:     "app-1",
		ProjectID: "proj-1",
	}
	store.workflows[existing.ID] = existing

	svc := &applicationsServiceImpl{Store: store}
	req := apisv1.UpdateApplicationWorkflowRequest{
		Name: "custom-flow",
		Workflow: []apisv1.CreateWorkflowStepRequest{
			{Name: "deploy-nginx", Components: []string{"nginx"}},
		},
	}

	resp, err := svc.UpdateApplicationWorkflow(context.Background(), "app-1", req)
	require.NoError(t, err)
	require.NotEqual(t, existing.ID, resp.WorkflowID)

	newWorkflow := store.workflows[resp.WorkflowID]
	require.NotNil(t, newWorkflow)
	require.Equal(t, "custom-flow", newWorkflow.Name)
	require.Equal(t, "default-flow", store.workflows["wf-1"].Name)
}

func TestUpdateApplicationWorkflowInheritsMetadata(t *testing.T) {
	store := newInMemoryAppStore()
	store.apps["app-1"] = &model.Applications{
		ID:        "app-1",
		Name:      "DemoApp",
		Namespace: "app-ns",
		Project:   "proj-app",
	}
	store.components["nginx"] = &model.ApplicationComponent{Name: "nginx", AppID: "app-1"}
	store.workflows["wf-legacy"] = &model.Workflow{
		ID:          "wf-legacy",
		Name:        "demoapp-workflow",
		AppID:       "app-1",
		Namespace:   "legacy-ns",
		ProjectID:   "legacy-proj",
		Description: "legacy-desc",
	}

	svc := &applicationsServiceImpl{Store: store}
	req := apisv1.UpdateApplicationWorkflowRequest{
		Name: "another-flow",
		Workflow: []apisv1.CreateWorkflowStepRequest{
			{Name: "deploy-nginx", Components: []string{"nginx"}},
		},
	}

	resp, err := svc.UpdateApplicationWorkflow(context.Background(), "app-1", req)
	require.NoError(t, err)

	created := store.workflows[resp.WorkflowID]
	require.NotNil(t, created)
	require.Equal(t, "legacy-ns", created.Namespace)
	require.Equal(t, "legacy-proj", created.ProjectID)
	require.Equal(t, "legacy-desc", created.Description)
}

func TestUpdateApplicationWorkflowDefaultsMetadataFromApp(t *testing.T) {
	store := newInMemoryAppStore()
	store.apps["app-1"] = &model.Applications{
		ID:        "app-1",
		Name:      "DemoApp",
		Namespace: "app-ns",
		Project:   "proj-app",
	}
	store.components["nginx"] = &model.ApplicationComponent{Name: "nginx", AppID: "app-1"}

	svc := &applicationsServiceImpl{Store: store}
	req := apisv1.UpdateApplicationWorkflowRequest{
		Workflow: []apisv1.CreateWorkflowStepRequest{
			{Name: "deploy-nginx", Components: []string{"nginx"}},
		},
	}

	resp, err := svc.UpdateApplicationWorkflow(context.Background(), "app-1", req)
	require.NoError(t, err)

	created := store.workflows[resp.WorkflowID]
	require.NotNil(t, created)
	require.Equal(t, "app-ns", created.Namespace)
	require.Equal(t, "proj-app", created.ProjectID)
}

func TestUpdateApplicationWorkflowGeneratesUniqueName(t *testing.T) {
	store := newInMemoryAppStore()
	store.apps["app-1"] = &model.Applications{
		ID:      "app-1",
		Name:    "DemoApp",
		Project: "proj-1",
	}
	store.components["nginx"] = &model.ApplicationComponent{Name: "nginx", AppID: "app-1"}
	store.workflows["wf-default"] = &model.Workflow{
		ID:        "wf-default",
		Name:      "demoapp-workflow",
		AppID:     "app-1",
		ProjectID: "proj-1",
	}

	svc := &applicationsServiceImpl{Store: store}
	req := apisv1.UpdateApplicationWorkflowRequest{
		Workflow: []apisv1.CreateWorkflowStepRequest{
			{Name: "deploy-nginx", Components: []string{"nginx"}},
		},
	}

	resp, err := svc.UpdateApplicationWorkflow(context.Background(), "app-1", req)
	require.NoError(t, err)
	require.NotEqual(t, "wf-default", resp.WorkflowID)

	created := store.workflows[resp.WorkflowID]
	require.NotNil(t, created)
	require.Equal(t, "demoapp-workflow-1", created.Name)
}

func TestUpdateApplicationWorkflowMissingComponent(t *testing.T) {
	store := newInMemoryAppStore()
	store.apps["app-1"] = &model.Applications{
		ID:      "app-1",
		Name:    "DemoApp",
		Project: "proj-1",
	}
	store.components["config"] = &model.ApplicationComponent{Name: "config", AppID: "app-1"}

	svc := &applicationsServiceImpl{Store: store}

	req := apisv1.UpdateApplicationWorkflowRequest{
		Name: "bad-flow",
		Workflow: []apisv1.CreateWorkflowStepRequest{
			{Name: "missing", Components: []string{"not-found"}},
		},
	}

	_, err := svc.UpdateApplicationWorkflow(context.Background(), "app-1", req)
	require.Error(t, err)
	require.True(t, errors.Is(err, bcode.ErrWorkflowConfig))
	require.Contains(t, err.Error(), "not found")
}

func TestListApplicationWorkflows(t *testing.T) {
	store := newInMemoryAppStore()
	store.apps["app-1"] = &model.Applications{
		ID:      "app-1",
		Name:    "DemoApp",
		Project: "proj-1",
	}
	store.workflows["wf-old"] = &model.Workflow{
		ID:        "wf-old",
		AppID:     "app-1",
		ProjectID: "proj-1",
		BaseModel: model.BaseModel{
			UpdateTime: time.Unix(1, 0),
		},
	}
	store.workflows["wf-new"] = &model.Workflow{
		ID:        "wf-new",
		AppID:     "app-1",
		ProjectID: "proj-1",
		BaseModel: model.BaseModel{
			UpdateTime: time.Unix(10, 0),
		},
	}

	svc := &applicationsServiceImpl{Store: store}
	list, err := svc.ListApplicationWorkflows(context.Background(), "app-1")
	require.NoError(t, err)
	require.Len(t, list, 2)
	require.Equal(t, "wf-new", list[0].ID)
	require.Equal(t, "wf-old", list[1].ID)
}

func TestListApplicationWorkflowsMissingApp(t *testing.T) {
	store := newInMemoryAppStore()
	svc := &applicationsServiceImpl{Store: store}
	_, err := svc.ListApplicationWorkflows(context.Background(), "missing")
	require.Error(t, err)
	require.True(t, errors.Is(err, bcode.ErrApplicationNotExist))
}

func decodeWorkflowSteps(t *testing.T, js *model.JSONStruct) *model.WorkflowSteps {
	t.Helper()
	var steps model.WorkflowSteps
	if js == nil {
		return &steps
	}
	if err := json.Unmarshal([]byte(js.JSON()), &steps); err != nil {
		t.Fatalf("decode workflow steps: %v", err)
	}
	return &steps
}

type inMemoryAppStore struct {
	apps       map[string]*model.Applications
	workflows  map[string]*model.Workflow
	components map[string]*model.ApplicationComponent
}

func newInMemoryAppStore() *inMemoryAppStore {
	return &inMemoryAppStore{
		apps:       make(map[string]*model.Applications),
		workflows:  make(map[string]*model.Workflow),
		components: make(map[string]*model.ApplicationComponent),
	}
}

func (s *inMemoryAppStore) Add(_ context.Context, entity datastore.Entity) error {
	switch v := entity.(type) {
	case *model.Applications:
		cp := *v
		s.apps[v.ID] = &cp
	case *model.Workflow:
		cp := *v
		s.workflows[v.ID] = &cp
	case *model.ApplicationComponent:
		cp := *v
		s.components[v.Name] = &cp
	}
	return nil
}

func (s *inMemoryAppStore) BatchAdd(context.Context, []datastore.Entity) error { return nil }

func (s *inMemoryAppStore) Put(_ context.Context, entity datastore.Entity) error {
	switch v := entity.(type) {
	case *model.Workflow:
		if existing, ok := s.workflows[v.ID]; ok {
			*existing = *v
		} else {
			cp := *v
			s.workflows[v.ID] = &cp
		}
	case *model.Applications:
		if existing, ok := s.apps[v.ID]; ok {
			*existing = *v
		} else {
			cp := *v
			s.apps[v.ID] = &cp
		}
	case *model.ApplicationComponent:
		if existing, ok := s.components[v.Name]; ok {
			*existing = *v
		} else {
			cp := *v
			s.components[v.Name] = &cp
		}
	}
	return nil
}

func (s *inMemoryAppStore) Delete(context.Context, datastore.Entity) error { return nil }

func (s *inMemoryAppStore) DeleteByFilter(context.Context, datastore.Entity, *datastore.FilterOptions) error {
	return nil
}

func (s *inMemoryAppStore) Get(_ context.Context, entity datastore.Entity) error {
	switch v := entity.(type) {
	case *model.Applications:
		if v.ID != "" {
			if app, ok := s.apps[v.ID]; ok {
				*v = *app
				return nil
			}
		} else if v.Name != "" {
			for _, app := range s.apps {
				if app.Name == v.Name {
					*v = *app
					return nil
				}
			}
		}
		return datastore.ErrRecordNotExist
	case *model.Workflow:
		if wf, ok := s.workflows[v.ID]; ok {
			*v = *wf
			return nil
		}
		return datastore.ErrRecordNotExist
	case *model.ApplicationComponent:
		if v.Name != "" {
			if comp, ok := s.components[v.Name]; ok {
				*v = *comp
				return nil
			}
		}
		for _, comp := range s.components {
			if v.AppID != "" && comp.AppID == v.AppID {
				*v = *comp
				return nil
			}
		}
		return datastore.ErrRecordNotExist
	default:
		return nil
	}
}

func (s *inMemoryAppStore) List(_ context.Context, query datastore.Entity, _ *datastore.ListOptions) ([]datastore.Entity, error) {
	switch q := query.(type) {
	case *model.Workflow:
		var result []datastore.Entity
		for _, wf := range s.workflows {
			if q.AppID != "" && wf.AppID != q.AppID {
				continue
			}
			result = append(result, wf)
		}
		return result, nil
	case *model.ApplicationComponent:
		var result []datastore.Entity
		for _, comp := range s.components {
			if q.AppID != "" && comp.AppID != q.AppID {
				continue
			}
			result = append(result, comp)
		}
		return result, nil
	default:
		return nil, nil
	}
}

func (s *inMemoryAppStore) Count(context.Context, datastore.Entity, *datastore.FilterOptions) (int64, error) {
	return 0, nil
}

func (s *inMemoryAppStore) IsExist(context.Context, datastore.Entity) (bool, error) {
	return false, nil
}

func (s *inMemoryAppStore) IsExistByCondition(context.Context, string, map[string]interface{}, interface{}) (bool, error) {
	return false, nil
}

var _ datastore.DataStore = (*inMemoryAppStore)(nil)
