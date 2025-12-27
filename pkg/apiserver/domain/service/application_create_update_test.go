package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"kubemin-cli/pkg/apiserver/config"
	"kubemin-cli/pkg/apiserver/domain/model"
	apisv1 "kubemin-cli/pkg/apiserver/interfaces/api/dto/v1"
)

func TestCreateApplications_UpdatePreservesWorkflowID(t *testing.T) {
	store := newInMemoryAppStore()
	app := &model.Applications{
		ID:        "app-1",
		Name:      "demo",
		Namespace: config.DefaultNamespace,
		Project:   "proj-1",
	}
	store.apps[app.ID] = app

	previousSteps, err := model.NewJSONStructByStruct(&model.WorkflowSteps{
		Steps: []*model.WorkflowStep{{
			Name:         "old",
			WorkflowType: config.JobDeploy,
			Mode:         config.WorkflowModeStepByStep,
		}},
	})
	require.NoError(t, err)

	workflow := &model.Workflow{
		ID:           "wf-1",
		Name:         "demo-old",
		Alias:        "demo-workflow",
		Namespace:    app.Namespace,
		AppID:        app.ID,
		WorkflowType: config.WorkflowTaskTypeWorkflow,
		Steps:        previousSteps,
	}
	store.workflows[workflow.ID] = workflow

	svc := newMockServiceWithStore(store)
	req := apisv1.CreateApplicationsRequest{
		ID:        app.ID,
		Name:      "demo",
		Namespace: app.Namespace,
		Version:   "2.0.0",
		Component: []apisv1.CreateComponentRequest{{
			Name:          "c1",
			ComponentType: config.ServerJob,
			Image:         "nginx:latest",
			Replicas:      1,
			Properties:    apisv1.Properties{},
			Traits:        apisv1.Traits{},
		}},
		WorkflowSteps: []apisv1.CreateWorkflowStepRequest{{
			Name:         "step1",
			WorkflowType: config.JobDeploy,
			Components:   []string{"c1"},
			Mode:         string(config.WorkflowModeStepByStep),
		}},
	}

	resp, err := svc.CreateApplications(context.Background(), req)
	require.NoError(t, err)
	require.Equal(t, workflow.ID, resp.WorkflowID)
	require.Len(t, store.workflows, 1)

	updated := store.workflows[workflow.ID]
	require.NotNil(t, updated)
	require.Equal(t, "demo-workflow", updated.Name)

	decoded := decodeWorkflowSteps(t, updated.Steps)
	require.Len(t, decoded.Steps, 1)
	require.Equal(t, "step1", decoded.Steps[0].Name)
}
