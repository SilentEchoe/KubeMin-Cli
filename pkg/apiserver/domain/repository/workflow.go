package repository

import (
	"KubeMin-Cli/pkg/apiserver/config"
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

func CreateWorkflowQueue(ctx context.Context, store datastore.DataStore, queue *model.WorkflowQueue) error {
	err := store.Add(ctx, queue)
	if err != nil {
		return err
	}
	return nil
}

func WaitingTasks(ctx context.Context, store datastore.DataStore) (list []*model.WorkflowQueue, err error) {
	var workflowQueue = &model.WorkflowQueue{
		Status: config.StatusWaiting,
	}
	queues, err := store.List(ctx, workflowQueue, &datastore.ListOptions{
		SortBy: []datastore.SortOption{{Key: "createTime", Order: datastore.SortOrderDescending}},
	})
	if err != nil {
		return nil, err
	}
	for _, policy := range queues {
		wq := policy.(*model.WorkflowQueue)
		list = append(list, wq)
	}
	return
}

func UpdateQueue(ctx context.Context, store datastore.DataStore, queue *model.WorkflowQueue) error {
	err := store.Put(ctx, queue)
	return err
}
