package service

import (
	"context"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/stretchr/testify/require"

	"KubeMin-Cli/pkg/apiserver/config"
	"KubeMin-Cli/pkg/apiserver/domain/model"
	"KubeMin-Cli/pkg/apiserver/infrastructure/datastore"
	"KubeMin-Cli/pkg/apiserver/workflow/naming"
)

func TestCleanupApplicationResourcesDeletesWorkload(t *testing.T) {
	app := &model.Applications{
		ID:        "app-1",
		Name:      "demo",
		Namespace: "default",
	}
	props, err := model.NewJSONStructByStruct(&model.Properties{
		Ports: []model.Ports{{Port: 8080}},
	})
	require.NoError(t, err)

	component := &model.ApplicationComponent{
		Name:          "web",
		AppID:         app.ID,
		Namespace:     "default",
		ComponentType: config.ServerJob,
		Replicas:      1,
		Image:         "nginx:latest",
		Properties:    props,
	}
	store := &cleanupStore{
		app:        app,
		components: []*model.ApplicationComponent{component},
		applications: map[string]*model.Applications{
			app.ID: app,
		},
	}

	deployName := naming.WebServiceName(component.Name, component.AppID)
	serviceName := naming.ServiceName(component.Name, component.AppID)
	clientset := fake.NewSimpleClientset(
		&appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: deployName, Namespace: "default"}},
		&corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: serviceName, Namespace: "default"}},
	)

	svc := &applicationsServiceImpl{
		KubeClient:    clientset,
		AppRepo:       &mockCleanupAppRepo{store: store},
		ComponentRepo: &mockCleanupComponentRepo{store: store},
	}

	resp, err := svc.CleanupApplicationResources(context.Background(), app.ID)
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Equal(t, app.ID, resp.AppID)
	require.Contains(t, resp.DeletedResources, "Deployment:default/"+deployName)
	require.Contains(t, resp.DeletedResources, "Service:default/"+serviceName)
	require.Empty(t, resp.FailedResources)
}

type cleanupStore struct {
	app          *model.Applications
	components   []*model.ApplicationComponent
	applications map[string]*model.Applications
}

func (c *cleanupStore) Add(context.Context, datastore.Entity) error        { return nil }
func (c *cleanupStore) BatchAdd(context.Context, []datastore.Entity) error { return nil }
func (c *cleanupStore) Put(context.Context, datastore.Entity) error        { return nil }
func (c *cleanupStore) Delete(context.Context, datastore.Entity) error     { return nil }
func (c *cleanupStore) DeleteByFilter(context.Context, datastore.Entity, *datastore.FilterOptions) error {
	return nil
}

func (c *cleanupStore) Get(_ context.Context, entity datastore.Entity) error {
	switch v := entity.(type) {
	case *model.Applications:
		if app, ok := c.applications[v.ID]; ok {
			*v = *app
			return nil
		}
		return datastore.ErrRecordNotExist
	default:
		return datastore.ErrRecordNotExist
	}
}

func (c *cleanupStore) List(_ context.Context, query datastore.Entity, _ *datastore.ListOptions) ([]datastore.Entity, error) {
	switch query.(type) {
	case *model.ApplicationComponent:
		entities := make([]datastore.Entity, len(c.components))
		for i, comp := range c.components {
			entities[i] = comp
		}
		return entities, nil
	default:
		return nil, nil
	}
}

func (c *cleanupStore) Count(context.Context, datastore.Entity, *datastore.FilterOptions) (int64, error) {
	return 0, nil
}

func (c *cleanupStore) IsExist(context.Context, datastore.Entity) (bool, error) {
	return false, nil
}

func (c *cleanupStore) IsExistByCondition(context.Context, string, map[string]interface{}, interface{}) (bool, error) {
	return false, nil
}

var _ datastore.DataStore = (*cleanupStore)(nil)
