package repository

import (
	"KubeMin-Cli/pkg/apiserver/config"
	"KubeMin-Cli/pkg/apiserver/domain/model"
	"KubeMin-Cli/pkg/apiserver/infrastructure/datastore"
	"context"
	"k8s.io/klog/v2"
)

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

func UpdateTask(ctx context.Context, store datastore.DataStore, task *model.WorkflowQueue) error {
	err := store.Put(ctx, task)
	return err
}

func TaskRunning(ctx context.Context, store datastore.DataStore) (list []*model.WorkflowQueue, err error) {
	tasks, err := store.List(ctx, &model.WorkflowQueue{}, &datastore.ListOptions{FilterOptions: datastore.FilterOptions{In: []datastore.InQueryOption{
		{
			Key: "status",
			Values: []string{
				string(config.StatusCreated),
				string(config.StatusRunning),
				string(config.StatusWaiting),
				string(config.StatusQueued),
				string(config.StatusBlocked),
				string(config.QueueItemPending),
				string(config.StatusPrepare),
				string(config.StatusWaitingApprove),
				""},
		},
	}}})

	if err != nil {
		return nil, err
	}
	for _, v := range tasks {
		task := v.(*model.WorkflowQueue)
		list = append(list, task)
	}
	return
}

func TaskById(ctx context.Context, store datastore.DataStore, workId string) (*model.WorkflowQueue, error) {
	var task = &model.WorkflowQueue{
		ID: workId,
	}
	err := store.Get(ctx, task)
	if err != nil {
		return nil, err
	}
	return task, nil
}
