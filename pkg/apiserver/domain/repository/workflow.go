package repository

import (
	"context"
	"errors"

	"k8s.io/klog/v2"

	"KubeMin-Cli/pkg/apiserver/config"
	"KubeMin-Cli/pkg/apiserver/domain/model"
	"KubeMin-Cli/pkg/apiserver/infrastructure/datastore"
)

func WorkflowByID(ctx context.Context, store datastore.DataStore, workflowID string) (*model.Workflow, error) {
	var workflow = &model.Workflow{
		ID: workflowID,
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

func DelWorkflow(ctx context.Context, store datastore.DataStore, workflow *model.Workflow) error {
	err := store.Delete(ctx, workflow)
	if err != nil {
		return err
	}
	return nil
}

func DelWorkflowsByAppID(ctx context.Context, store datastore.DataStore, appID string) error {
	workflows, err := FindWorkflowsByAppID(ctx, store, appID)
	if err != nil {
		return err
	}
	for _, w := range workflows {
		if w == nil {
			continue
		}
		if err := store.Delete(ctx, w); err != nil {
			if errors.Is(err, datastore.ErrRecordNotExist) {
				continue
			}
			return err
		}
	}
	return nil
}

func FindWorkflowsByAppID(ctx context.Context, store datastore.DataStore, appID string) ([]*model.Workflow, error) {
	entities, err := store.List(ctx, &model.Workflow{AppID: appID}, &datastore.ListOptions{})
	if err != nil {
		return nil, err
	}
	var workflows []*model.Workflow
	for _, entity := range entities {
		component, ok := entity.(*model.Workflow)
		if !ok {
			klog.Warningf("unexpected component entity type: %T", entity)
			continue
		}
		workflows = append(workflows, component)
	}
	return workflows, nil
}

func CreateComponents(ctx context.Context, store datastore.DataStore, workflow *model.ApplicationComponent) error {
	err := store.Add(ctx, workflow)
	if err != nil {
		return err
	}
	return nil
}

func DelComponentsByAppID(ctx context.Context, store datastore.DataStore, appID string) error {
	components, err := FindComponentsByAppID(ctx, store, appID)
	if err != nil {
		return err
	}
	for _, component := range components {
		if component == nil {
			continue
		}
		if err := store.Delete(ctx, component); err != nil {
			if errors.Is(err, datastore.ErrRecordNotExist) {
				continue
			}
			return err
		}
	}
	return nil
}

func FindComponentsByAppID(ctx context.Context, store datastore.DataStore, appID string) ([]*model.ApplicationComponent, error) {
	entities, err := store.List(ctx, &model.ApplicationComponent{AppID: appID}, &datastore.ListOptions{})
	if err != nil {
		return nil, err
	}
	var components []*model.ApplicationComponent
	for _, entity := range entities {
		component, ok := entity.(*model.ApplicationComponent)
		if !ok {
			klog.Warningf("unexpected component entity type: %T", entity)
			continue
		}
		components = append(components, component)
	}
	return components, nil
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
		SortBy: []datastore.SortOption{{Key: "createTime", Order: datastore.SortOrderAscending}},
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
				"",
			},
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

func TaskByID(ctx context.Context, store datastore.DataStore, taskID string) (*model.WorkflowQueue, error) {
	var task = &model.WorkflowQueue{
		TaskID: taskID,
	}
	err := store.Get(ctx, task)
	if err != nil {
		return nil, err
	}
	return task, nil
}

func UpdateTaskStatus(ctx context.Context, store datastore.DataStore, taskID string, from, to config.Status) (bool, error) {
	task := &model.WorkflowQueue{TaskID: taskID}
	if err := store.Get(ctx, task); err != nil {
		if errors.Is(err, datastore.ErrRecordNotExist) {
			return false, nil
		}
		return false, err
	}
	if from != "" && task.Status != from {
		return false, nil
	}
	task.Status = to
	if err := store.Put(ctx, task); err != nil {
		return false, err
	}
	return true, nil
}
