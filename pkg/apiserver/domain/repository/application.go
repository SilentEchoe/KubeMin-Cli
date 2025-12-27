package repository

import (
	"context"
	"errors"

	"kubemin-cli/pkg/apiserver/domain/model"
	"kubemin-cli/pkg/apiserver/infrastructure/datastore"
)

// ApplicationRepository defines the interface for application data operations.
type ApplicationRepository interface {
	FindByID(ctx context.Context, id string) (*model.Applications, error)
	FindByName(ctx context.Context, name string) (*model.Applications, error)
	Create(ctx context.Context, app *model.Applications) error
	Update(ctx context.Context, app *model.Applications) error
	Delete(ctx context.Context, app *model.Applications) error
	List(ctx context.Context, options datastore.ListOptions) ([]*model.Applications, error)
}

type applicationRepository struct {
	Store datastore.DataStore `inject:"datastore"`
}

// NewApplicationRepository creates a new ApplicationRepository.
// Dependencies are injected via struct tags.
func NewApplicationRepository() ApplicationRepository {
	return &applicationRepository{}
}

func (r *applicationRepository) FindByID(ctx context.Context, id string) (*model.Applications, error) {
	return ApplicationByID(ctx, r.Store, id)
}

func (r *applicationRepository) FindByName(ctx context.Context, name string) (*model.Applications, error) {
	app := model.Applications{Name: name}
	if err := r.Store.Get(ctx, &app); err != nil {
		return nil, err
	}
	return &app, nil
}

func (r *applicationRepository) Create(ctx context.Context, app *model.Applications) error {
	return CreateApplications(ctx, r.Store, app)
}

func (r *applicationRepository) Update(ctx context.Context, app *model.Applications) error {
	return r.Store.Put(ctx, app)
}

func (r *applicationRepository) Delete(ctx context.Context, app *model.Applications) error {
	return r.Store.Delete(ctx, app)
}

func (r *applicationRepository) List(ctx context.Context, options datastore.ListOptions) ([]*model.Applications, error) {
	return ListApplications(ctx, r.Store, options)
}

// ---- Original Functions (kept for backward compatibility) ----

func ApplicationByID(ctx context.Context, store datastore.DataStore, id string) (*model.Applications, error) {
	app := model.Applications{
		ID: id,
	}
	err := store.Get(ctx, &app)
	if err != nil {
		return nil, err
	}
	return &app, nil
}

func CreateApplications(ctx context.Context, store datastore.DataStore, app *model.Applications) error {
	if err := store.Add(ctx, app); err != nil {
		if errors.Is(err, datastore.ErrRecordExist) {
			return store.Put(ctx, app)
		}
		return err
	}
	return nil
}

// ListApplications query the application policies
func ListApplications(ctx context.Context, store datastore.DataStore, listOptions datastore.ListOptions) (list []*model.Applications, err error) {
	var app model.Applications
	entities, err := store.List(ctx, &app, &listOptions)
	if err != nil {
		return nil, err
	}
	for _, entity := range entities {
		appModel, ok := entity.(*model.Applications)
		if !ok {
			continue
		}
		list = append(list, appModel)
	}

	return list, nil
}
