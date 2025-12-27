package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"kubemin-cli/pkg/apiserver/domain/model"
	"kubemin-cli/pkg/apiserver/utils/bcode"
)

func TestListApplicationComponentsReturnsSorted(t *testing.T) {
	store := newInMemoryAppStore()
	store.apps["app-1"] = &model.Applications{ID: "app-1", Name: "demo"}
	store.components["old"] = &model.ApplicationComponent{
		Name:  "old",
		AppID: "app-1",
		BaseModel: model.BaseModel{
			UpdateTime: time.Unix(10, 0),
		},
	}
	store.components["new"] = &model.ApplicationComponent{
		Name:  "new",
		AppID: "app-1",
		BaseModel: model.BaseModel{
			UpdateTime: time.Unix(20, 0),
		},
	}
	store.components["other"] = &model.ApplicationComponent{
		Name:  "other",
		AppID: "other-app",
		BaseModel: model.BaseModel{
			UpdateTime: time.Unix(30, 0),
		},
	}

	svc := newMockServiceWithStore(store)
	components, err := svc.ListApplicationComponents(context.Background(), "app-1")
	require.NoError(t, err)
	require.Len(t, components, 2)
	require.Equal(t, "new", components[0].Name)
	require.Equal(t, "old", components[1].Name)
}

func TestListApplicationComponentsMissingApp(t *testing.T) {
	store := newInMemoryAppStore()
	svc := newMockServiceWithStore(store)

	_, err := svc.ListApplicationComponents(context.Background(), "missing")
	require.ErrorIs(t, err, bcode.ErrApplicationNotExist)
}
