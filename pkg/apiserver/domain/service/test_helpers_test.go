package service

import (
	"context"

	"KubeMin-Cli/pkg/apiserver/config"
	"KubeMin-Cli/pkg/apiserver/domain/model"
	"KubeMin-Cli/pkg/apiserver/domain/repository"
	"KubeMin-Cli/pkg/apiserver/infrastructure/datastore"
)

// mockAppRepo wraps inMemoryAppStore to implement repository.ApplicationRepository
type mockAppRepo struct {
	store *inMemoryAppStore
}

func (m *mockAppRepo) FindByID(ctx context.Context, id string) (*model.Applications, error) {
	app := &model.Applications{ID: id}
	if err := m.store.Get(ctx, app); err != nil {
		return nil, err
	}
	return app, nil
}

func (m *mockAppRepo) FindByName(ctx context.Context, name string) (*model.Applications, error) {
	app := &model.Applications{Name: name}
	if err := m.store.Get(ctx, app); err != nil {
		return nil, err
	}
	return app, nil
}

func (m *mockAppRepo) Create(ctx context.Context, app *model.Applications) error {
	return m.store.Add(ctx, app)
}

func (m *mockAppRepo) Update(ctx context.Context, app *model.Applications) error {
	return m.store.Put(ctx, app)
}

func (m *mockAppRepo) Delete(ctx context.Context, app *model.Applications) error {
	return m.store.Delete(ctx, app)
}

func (m *mockAppRepo) List(ctx context.Context, options datastore.ListOptions) ([]*model.Applications, error) {
	var result []*model.Applications
	for _, app := range m.store.apps {
		result = append(result, app)
	}
	return result, nil
}

var _ repository.ApplicationRepository = (*mockAppRepo)(nil)

// mockWorkflowRepo wraps inMemoryAppStore to implement repository.WorkflowRepository
type mockWorkflowRepo struct {
	store *inMemoryAppStore
}

func (m *mockWorkflowRepo) FindByID(ctx context.Context, workflowID string) (*model.Workflow, error) {
	wf := &model.Workflow{ID: workflowID}
	if err := m.store.Get(ctx, wf); err != nil {
		return nil, err
	}
	return wf, nil
}

func (m *mockWorkflowRepo) Create(ctx context.Context, workflow *model.Workflow) error {
	return m.store.Add(ctx, workflow)
}

func (m *mockWorkflowRepo) Update(ctx context.Context, workflow *model.Workflow) error {
	return m.store.Put(ctx, workflow)
}

func (m *mockWorkflowRepo) Delete(ctx context.Context, workflow *model.Workflow) error {
	return m.store.Delete(ctx, workflow)
}

func (m *mockWorkflowRepo) DeleteByAppID(ctx context.Context, appID string) error {
	for id, wf := range m.store.workflows {
		if wf.AppID == appID {
			delete(m.store.workflows, id)
		}
	}
	return nil
}

func (m *mockWorkflowRepo) FindByAppID(ctx context.Context, appID string) ([]*model.Workflow, error) {
	var result []*model.Workflow
	for _, wf := range m.store.workflows {
		if wf.AppID == appID {
			result = append(result, wf)
		}
	}
	return result, nil
}

var _ repository.WorkflowRepository = (*mockWorkflowRepo)(nil)

// mockComponentRepo wraps inMemoryAppStore to implement repository.ComponentRepository
type mockComponentRepo struct {
	store *inMemoryAppStore
}

func (m *mockComponentRepo) Create(ctx context.Context, component *model.ApplicationComponent) error {
	return m.store.Add(ctx, component)
}

func (m *mockComponentRepo) BatchAdd(ctx context.Context, components []*model.ApplicationComponent) error {
	for _, comp := range components {
		if err := m.store.Add(ctx, comp); err != nil {
			return err
		}
	}
	return nil
}

func (m *mockComponentRepo) DeleteByAppID(ctx context.Context, appID string) error {
	for name, comp := range m.store.components {
		if comp.AppID == appID {
			delete(m.store.components, name)
		}
	}
	return nil
}

func (m *mockComponentRepo) FindByAppID(ctx context.Context, appID string) ([]*model.ApplicationComponent, error) {
	var result []*model.ApplicationComponent
	for _, comp := range m.store.components {
		if comp.AppID == appID {
			result = append(result, comp)
		}
	}
	return result, nil
}

func (m *mockComponentRepo) Update(ctx context.Context, component *model.ApplicationComponent) error {
	return m.store.Put(ctx, component)
}

func (m *mockComponentRepo) Delete(ctx context.Context, component *model.ApplicationComponent) error {
	return m.store.Delete(ctx, component)
}

func (m *mockComponentRepo) FindByName(ctx context.Context, appID, name string) (*model.ApplicationComponent, error) {
	for _, comp := range m.store.components {
		if comp.AppID == appID && comp.Name == name {
			return comp, nil
		}
	}
	return nil, datastore.ErrRecordNotExist
}

var _ repository.ComponentRepository = (*mockComponentRepo)(nil)

// mockCleanupAppRepo wraps cleanupStore for cleanup tests
type mockCleanupAppRepo struct {
	store *cleanupStore
}

func (m *mockCleanupAppRepo) FindByID(ctx context.Context, id string) (*model.Applications, error) {
	if app, ok := m.store.applications[id]; ok {
		return app, nil
	}
	return nil, datastore.ErrRecordNotExist
}

func (m *mockCleanupAppRepo) FindByName(ctx context.Context, name string) (*model.Applications, error) {
	return nil, datastore.ErrRecordNotExist
}

func (m *mockCleanupAppRepo) Create(ctx context.Context, app *model.Applications) error {
	return nil
}

func (m *mockCleanupAppRepo) Update(ctx context.Context, app *model.Applications) error {
	return nil
}

func (m *mockCleanupAppRepo) Delete(ctx context.Context, app *model.Applications) error {
	return nil
}

func (m *mockCleanupAppRepo) List(ctx context.Context, options datastore.ListOptions) ([]*model.Applications, error) {
	return nil, nil
}

var _ repository.ApplicationRepository = (*mockCleanupAppRepo)(nil)

// mockCleanupComponentRepo wraps cleanupStore for cleanup tests
type mockCleanupComponentRepo struct {
	store *cleanupStore
}

func (m *mockCleanupComponentRepo) Create(ctx context.Context, component *model.ApplicationComponent) error {
	return nil
}

func (m *mockCleanupComponentRepo) BatchAdd(ctx context.Context, components []*model.ApplicationComponent) error {
	return nil
}

func (m *mockCleanupComponentRepo) DeleteByAppID(ctx context.Context, appID string) error {
	return nil
}

func (m *mockCleanupComponentRepo) FindByAppID(ctx context.Context, appID string) ([]*model.ApplicationComponent, error) {
	return m.store.components, nil
}

func (m *mockCleanupComponentRepo) Update(ctx context.Context, component *model.ApplicationComponent) error {
	return nil
}

func (m *mockCleanupComponentRepo) Delete(ctx context.Context, component *model.ApplicationComponent) error {
	return nil
}

func (m *mockCleanupComponentRepo) FindByName(ctx context.Context, appID, name string) (*model.ApplicationComponent, error) {
	return nil, datastore.ErrRecordNotExist
}

var _ repository.ComponentRepository = (*mockCleanupComponentRepo)(nil)

// newMockServiceWithStore creates an applicationsServiceImpl with mock repos from the store
func newMockServiceWithStore(store *inMemoryAppStore) *applicationsServiceImpl {
	return &applicationsServiceImpl{
		Store:             store,
		AppRepo:           &mockAppRepo{store: store},
		WorkflowRepo:      &mockWorkflowRepo{store: store},
		ComponentRepo:     &mockComponentRepo{store: store},
		WorkflowQueueRepo: &mockWorkflowQueueRepo{},
	}
}

// mockWorkflowQueueRepo implements repository.WorkflowQueueRepository for tests
type mockWorkflowQueueRepo struct{}

func (m *mockWorkflowQueueRepo) Create(ctx context.Context, queue *model.WorkflowQueue) error {
	return nil
}

func (m *mockWorkflowQueueRepo) Update(ctx context.Context, task *model.WorkflowQueue) error {
	return nil
}

func (m *mockWorkflowQueueRepo) FindByID(ctx context.Context, taskID string) (*model.WorkflowQueue, error) {
	return nil, datastore.ErrRecordNotExist
}

func (m *mockWorkflowQueueRepo) FindWaiting(ctx context.Context) ([]*model.WorkflowQueue, error) {
	return nil, nil
}

func (m *mockWorkflowQueueRepo) FindRunning(ctx context.Context) ([]*model.WorkflowQueue, error) {
	return nil, nil
}

func (m *mockWorkflowQueueRepo) UpdateStatus(ctx context.Context, taskID string, from, to config.Status) (bool, error) {
	return false, nil
}

var _ repository.WorkflowQueueRepository = (*mockWorkflowQueueRepo)(nil)
