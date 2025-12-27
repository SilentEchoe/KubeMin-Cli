package job

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/client-go/kubernetes/fake"

	"kubemin-cli/pkg/apiserver/config"
	"kubemin-cli/pkg/apiserver/domain/model"
	"kubemin-cli/pkg/apiserver/infrastructure/datastore"
)

type jobInfoStore struct {
	addCount  int
	lastAdded datastore.Entity
}

func (s *jobInfoStore) Add(_ context.Context, entity datastore.Entity) error {
	s.addCount++
	s.lastAdded = entity
	return nil
}

func (s *jobInfoStore) BatchAdd(context.Context, []datastore.Entity) error { return nil }
func (s *jobInfoStore) Put(context.Context, datastore.Entity) error        { return nil }
func (s *jobInfoStore) Delete(context.Context, datastore.Entity) error     { return nil }
func (s *jobInfoStore) DeleteByFilter(context.Context, datastore.Entity, *datastore.FilterOptions) error {
	return nil
}
func (s *jobInfoStore) Get(context.Context, datastore.Entity) error { return nil }
func (s *jobInfoStore) List(context.Context, datastore.Entity, *datastore.ListOptions) ([]datastore.Entity, error) {
	return nil, nil
}
func (s *jobInfoStore) Count(context.Context, datastore.Entity, *datastore.FilterOptions) (int64, error) {
	return 0, nil
}
func (s *jobInfoStore) IsExist(context.Context, datastore.Entity) (bool, error) { return false, nil }
func (s *jobInfoStore) IsExistByCondition(context.Context, string, map[string]interface{}, interface{}) (bool, error) {
	return false, nil
}
func (s *jobInfoStore) CompareAndSwap(context.Context, datastore.Entity, string, interface{}, map[string]interface{}) (bool, error) {
	return false, nil
}

var _ datastore.DataStore = (*jobInfoStore)(nil)

func TestRunJob_SkippedWritesJobInfo(t *testing.T) {
	store := &jobInfoStore{}
	jobTask := &model.JobTask{
		Name:       "demo",
		Namespace:  "default",
		WorkflowID: "wf-1",
		ProjectID:  "proj-1",
		AppID:      "app-1",
		TaskID:     "task-1",
		JobType:    string(config.JobDeploy),
		Status:     config.StatusSkipped,
	}

	runJob(context.Background(), jobTask, fake.NewSimpleClientset(), store, func() {})

	require.Equal(t, 1, store.addCount)
	info, ok := store.lastAdded.(*model.JobInfo)
	require.True(t, ok)
	require.Equal(t, string(config.StatusSkipped), info.Status)
	require.NotZero(t, info.StartTime)
	require.NotZero(t, info.EndTime)
}
