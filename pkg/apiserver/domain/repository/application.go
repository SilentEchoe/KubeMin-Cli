package repository

import (
	"KubeMin-Cli/pkg/apiserver/domain/model"
	"KubeMin-Cli/pkg/apiserver/infrastructure/datastore"
	"context"
)

// ListApplications query the application policies
func ListApplications(ctx context.Context, store datastore.DataStore) (list []*model.Applications, err error) {
	var app = model.Applications{}

	apps, err := store.List(ctx, &app, &datastore.ListOptions{})
	if err != nil {
		return nil, err
	}
	for _, app := range apps {
		ap := app.(*model.Applications)
		list = append(list, ap)
	}
	return
}
