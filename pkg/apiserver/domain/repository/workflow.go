package repository

import (
	"KubeMin-Cli/pkg/apiserver/domain/model"
	"KubeMin-Cli/pkg/apiserver/infrastructure/datastore"
	"context"
)

func WorkflowByName(ctx context.Context, store datastore.DataStore, workflowName string) (*model.Workflow, error) {
	var workflow = &model.Workflow{
		Name: workflowName,
	}
	err := store.Get(ctx, workflow)
	if err != nil {
		return nil, err
	}
	return workflow, nil
}

func WorkflowById(ctx context.Context, store datastore.DataStore, workflowId string) (*model.Workflow, error) {
	var workflow = &model.Workflow{
		ID: workflowId,
	}
	err := store.Get(ctx, workflow)
	if err != nil {
		return nil, err
	}
	return workflow, nil
}

func CreateWorkflow(ctx context.Context, store datastore.DataStore, workflow *model.Workflow) error {
	err := store.Add(ctx, workflow)
	if err != nil {
		return err
	}
	return nil
}

func CreateComponents(ctx context.Context, store datastore.DataStore, workflow *model.ApplicationComponent) error {
	err := store.Add(ctx, workflow)
	if err != nil {
		return err
	}
	return nil
}
