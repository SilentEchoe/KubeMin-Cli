package repository

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"KubeMin-Cli/pkg/apiserver/domain/model"
	"KubeMin-Cli/pkg/apiserver/infrastructure/datastore"
)

type waitingTaskStore struct {
	lastOptions *datastore.ListOptions
}

func (w *waitingTaskStore) Add(context.Context, datastore.Entity) error        { return nil }
func (w *waitingTaskStore) BatchAdd(context.Context, []datastore.Entity) error { return nil }
func (w *waitingTaskStore) Put(context.Context, datastore.Entity) error        { return nil }
func (w *waitingTaskStore) Delete(context.Context, datastore.Entity) error     { return nil }
func (w *waitingTaskStore) DeleteByFilter(context.Context, datastore.Entity, *datastore.FilterOptions) error {
	return nil
}
func (w *waitingTaskStore) Get(context.Context, datastore.Entity) error { return nil }
func (w *waitingTaskStore) List(ctx context.Context, query datastore.Entity, options *datastore.ListOptions) ([]datastore.Entity, error) {
	w.lastOptions = options
	return []datastore.Entity{
		&model.WorkflowQueue{TaskID: "t1"},
		&model.WorkflowQueue{TaskID: "t2"},
	}, nil
}
func (w *waitingTaskStore) Count(context.Context, datastore.Entity, *datastore.FilterOptions) (int64, error) {
	return 0, nil
}
func (w *waitingTaskStore) IsExist(context.Context, datastore.Entity) (bool, error) {
	return false, nil
}
func (w *waitingTaskStore) IsExistByCondition(context.Context, string, map[string]interface{}, interface{}) (bool, error) {
	return false, nil
}

func TestWaitingTasksUsesFIFOSort(t *testing.T) {
	store := &waitingTaskStore{}
	list, err := WaitingTasks(context.Background(), store)
	require.NoError(t, err)
	require.Len(t, list, 2)

	if store.lastOptions == nil || len(store.lastOptions.SortBy) == 0 {
		t.Fatalf("expected sort options to be set")
	}
	require.Equal(t, "createTime", store.lastOptions.SortBy[0].Key)
	require.Equal(t, datastore.SortOrderAscending, store.lastOptions.SortBy[0].Order)
}

var _ datastore.DataStore = (*waitingTaskStore)(nil)
