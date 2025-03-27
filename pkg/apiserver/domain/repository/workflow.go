package repository

import (
	"KubeMin-Cli/pkg/apiserver/domain/model"
	"KubeMin-Cli/pkg/apiserver/infrastructure/datastore"
	"context"
	"k8s.io/klog/v2"
)

func WorkflowByName(ctx context.Context, store datastore.DataStore, workflowName string) (*model.Workflow, error) {
	var workflow = &model.Workflow{
		ID: workflowName,
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
		klog.Error(err)
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
