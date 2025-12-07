package repository

import (
	"context"
	"errors"

	"k8s.io/klog/v2"

	"KubeMin-Cli/pkg/apiserver/config"
	"KubeMin-Cli/pkg/apiserver/domain/model"
	"KubeMin-Cli/pkg/apiserver/infrastructure/datastore"
)

// ---- Workflow Repository Interface ----

// WorkflowRepository defines the interface for workflow data operations.
type WorkflowRepository interface {
	FindByID(ctx context.Context, workflowID string) (*model.Workflow, error)
	Create(ctx context.Context, workflow *model.Workflow) error
	Update(ctx context.Context, workflow *model.Workflow) error
	Delete(ctx context.Context, workflow *model.Workflow) error
	DeleteByAppID(ctx context.Context, appID string) error
	FindByAppID(ctx context.Context, appID string) ([]*model.Workflow, error)
}

type workflowRepository struct {
	Store datastore.DataStore `inject:"datastore"`
}

// NewWorkflowRepository creates a new WorkflowRepository.
// Dependencies are injected via struct tags.
func NewWorkflowRepository() WorkflowRepository {
	return &workflowRepository{}
}

func (r *workflowRepository) FindByID(ctx context.Context, workflowID string) (*model.Workflow, error) {
	return WorkflowByID(ctx, r.Store, workflowID)
}

func (r *workflowRepository) Create(ctx context.Context, workflow *model.Workflow) error {
	return CreateWorkflow(ctx, r.Store, workflow)
}

func (r *workflowRepository) Update(ctx context.Context, workflow *model.Workflow) error {
	return r.Store.Put(ctx, workflow)
}

func (r *workflowRepository) Delete(ctx context.Context, workflow *model.Workflow) error {
	return DelWorkflow(ctx, r.Store, workflow)
}

func (r *workflowRepository) DeleteByAppID(ctx context.Context, appID string) error {
	return DelWorkflowsByAppID(ctx, r.Store, appID)
}

func (r *workflowRepository) FindByAppID(ctx context.Context, appID string) ([]*model.Workflow, error) {
	return FindWorkflowsByAppID(ctx, r.Store, appID)
}

// ---- Component Repository Interface ----

// ComponentRepository defines the interface for component data operations.
type ComponentRepository interface {
	Create(ctx context.Context, component *model.ApplicationComponent) error
	Update(ctx context.Context, component *model.ApplicationComponent) error
	Delete(ctx context.Context, component *model.ApplicationComponent) error
	BatchAdd(ctx context.Context, components []*model.ApplicationComponent) error
	DeleteByAppID(ctx context.Context, appID string) error
	FindByAppID(ctx context.Context, appID string) ([]*model.ApplicationComponent, error)
	FindByName(ctx context.Context, appID, name string) (*model.ApplicationComponent, error)
}

type componentRepository struct {
	Store datastore.DataStore `inject:"datastore"`
}

// NewComponentRepository creates a new ComponentRepository.
// Dependencies are injected via struct tags.
func NewComponentRepository() ComponentRepository {
	return &componentRepository{}
}

func (r *componentRepository) Create(ctx context.Context, component *model.ApplicationComponent) error {
	return CreateComponents(ctx, r.Store, component)
}

func (r *componentRepository) Update(ctx context.Context, component *model.ApplicationComponent) error {
	return r.Store.Put(ctx, component)
}

func (r *componentRepository) Delete(ctx context.Context, component *model.ApplicationComponent) error {
	return r.Store.Delete(ctx, component)
}

func (r *componentRepository) BatchAdd(ctx context.Context, components []*model.ApplicationComponent) error {
	entities := make([]datastore.Entity, len(components))
	for i, comp := range components {
		entities[i] = comp
	}
	return r.Store.BatchAdd(ctx, entities)
}

func (r *componentRepository) DeleteByAppID(ctx context.Context, appID string) error {
	return DelComponentsByAppID(ctx, r.Store, appID)
}

func (r *componentRepository) FindByAppID(ctx context.Context, appID string) ([]*model.ApplicationComponent, error) {
	return FindComponentsByAppID(ctx, r.Store, appID)
}

func (r *componentRepository) FindByName(ctx context.Context, appID, name string) (*model.ApplicationComponent, error) {
	components, err := r.FindByAppID(ctx, appID)
	if err != nil {
		return nil, err
	}
	for _, comp := range components {
		if comp != nil && comp.Name == name {
			return comp, nil
		}
	}
	return nil, datastore.ErrRecordNotExist
}

// ---- Workflow Queue Repository Interface ----

// WorkflowQueueRepository defines the interface for workflow queue operations.
type WorkflowQueueRepository interface {
	Create(ctx context.Context, queue *model.WorkflowQueue) error
	Update(ctx context.Context, task *model.WorkflowQueue) error
	FindByID(ctx context.Context, taskID string) (*model.WorkflowQueue, error)
	FindWaiting(ctx context.Context) ([]*model.WorkflowQueue, error)
	FindRunning(ctx context.Context) ([]*model.WorkflowQueue, error)
	UpdateStatus(ctx context.Context, taskID string, from, to config.Status) (bool, error)
}

type workflowQueueRepository struct {
	Store datastore.DataStore `inject:"datastore"`
}

// NewWorkflowQueueRepository creates a new WorkflowQueueRepository.
// Dependencies are injected via struct tags.
func NewWorkflowQueueRepository() WorkflowQueueRepository {
	return &workflowQueueRepository{}
}

func (r *workflowQueueRepository) Create(ctx context.Context, queue *model.WorkflowQueue) error {
	return CreateWorkflowQueue(ctx, r.Store, queue)
}

func (r *workflowQueueRepository) Update(ctx context.Context, task *model.WorkflowQueue) error {
	return UpdateTask(ctx, r.Store, task)
}

func (r *workflowQueueRepository) FindByID(ctx context.Context, taskID string) (*model.WorkflowQueue, error) {
	return TaskByID(ctx, r.Store, taskID)
}

func (r *workflowQueueRepository) FindWaiting(ctx context.Context) ([]*model.WorkflowQueue, error) {
	return WaitingTasks(ctx, r.Store)
}

func (r *workflowQueueRepository) FindRunning(ctx context.Context) ([]*model.WorkflowQueue, error) {
	return TaskRunning(ctx, r.Store)
}

func (r *workflowQueueRepository) UpdateStatus(ctx context.Context, taskID string, from, to config.Status) (bool, error) {
	return UpdateTaskStatus(ctx, r.Store, taskID, from, to)
}

// ---- Original Functions (kept for backward compatibility) ----

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
	for _, entity := range queues {
		wq, ok := entity.(*model.WorkflowQueue)
		if !ok {
			klog.Warningf("unexpected workflow queue entity type: %T", entity)
			continue
		}
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
	for _, entity := range tasks {
		task, ok := entity.(*model.WorkflowQueue)
		if !ok {
			klog.Warningf("unexpected workflow queue entity type: %T", entity)
			continue
		}
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
