package repository

import (
	"context"

	"k8s.io/klog/v2"

	"KubeMin-Cli/pkg/apiserver/domain/model"
	"KubeMin-Cli/pkg/apiserver/infrastructure/datastore"
	"KubeMin-Cli/pkg/apiserver/utils/bcode"
)

func IsExist(ctx context.Context, store datastore.DataStore, Id, version string) (bool, error) {
	application := model.Applications{
		ID:      Id,
		Version: version,
	}
	exist, err := store.IsExist(ctx, &application)
	if err != nil {
		klog.Errorf("check application name is exist failure %s", err.Error())
		return false, bcode.ErrApplicationExist
	}
	return exist, nil
}

func ApplicationById(ctx context.Context, store datastore.DataStore, Id string) (*model.Applications, error) {
	app := model.Applications{
		ID: Id,
	}
	err := store.Get(ctx, &app)
	if err != nil {
		return nil, err
	}
	return &app, nil
}

func CreateApplications(ctx context.Context, store datastore.DataStore, app *model.Applications) error {
	if err := store.Add(ctx, app); err != nil {
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
