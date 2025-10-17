package repository

import (
	"context"
	"errors"

	"KubeMin-Cli/pkg/apiserver/domain/model"
	"KubeMin-Cli/pkg/apiserver/infrastructure/datastore"
)

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
