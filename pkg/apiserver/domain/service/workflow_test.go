package service

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"kubemin-cli/pkg/apiserver/infrastructure/datastore"
	apis "kubemin-cli/pkg/apiserver/interfaces/api/dto/v1"
)

type failingDataStore struct {
	err error
}

func (f *failingDataStore) Add(context.Context, datastore.Entity) error        { return f.err }
func (f *failingDataStore) BatchAdd(context.Context, []datastore.Entity) error { return f.err }
func (f *failingDataStore) Put(context.Context, datastore.Entity) error        { return f.err }
func (f *failingDataStore) Delete(context.Context, datastore.Entity) error     { return f.err }
func (f *failingDataStore) DeleteByFilter(context.Context, datastore.Entity, *datastore.FilterOptions) error {
	return f.err
}
func (f *failingDataStore) Get(context.Context, datastore.Entity) error { return f.err }
func (f *failingDataStore) List(context.Context, datastore.Entity, *datastore.ListOptions) ([]datastore.Entity, error) {
	return nil, f.err
}
func (f *failingDataStore) Count(context.Context, datastore.Entity, *datastore.FilterOptions) (int64, error) {
	return 0, f.err
}
func (f *failingDataStore) IsExist(context.Context, datastore.Entity) (bool, error) {
	return false, f.err
}
func (f *failingDataStore) IsExistByCondition(context.Context, string, map[string]interface{}, interface{}) (bool, error) {
	return false, f.err
}

func (f *failingDataStore) CompareAndSwap(context.Context, datastore.Entity, string, interface{}, map[string]interface{}) (bool, error) {
	return false, f.err
}

func TestCreateWorkflowTaskPropagatesStoreError(t *testing.T) {
	storeErr := errors.New("datastore unavailable")
	svc := &workflowServiceImpl{Store: &failingDataStore{err: storeErr}}
	req := apis.CreateWorkflowRequest{Name: "demo-workflow", Project: "proj"}

	_, err := svc.CreateWorkflowTask(context.Background(), req)
	require.Error(t, err)
	require.ErrorIs(t, err, storeErr)
}

// compile-time check that failingDataStore satisfies the interface
var _ datastore.DataStore = (*failingDataStore)(nil)
var _ WorkflowService = (*workflowServiceImpl)(nil)
