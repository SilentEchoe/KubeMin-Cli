package job

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"KubeMin-Cli/pkg/apiserver/config"
	"KubeMin-Cli/pkg/apiserver/domain/model"
	"KubeMin-Cli/pkg/apiserver/infrastructure/datastore"
)

func TestDeployServiceAccountJobCtl_Create(t *testing.T) {
	client := fake.NewSimpleClientset()
	jobTask := &model.JobTask{
		Name:      "pod-labeler-sa",
		Namespace: "ops",
		AppID:     "app-1",
		JobType:   string(config.JobDeployServiceAccount),
		JobInfo: &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name: "pod-labeler-sa",
			},
		},
	}
	ctl := NewDeployServiceAccountJobCtl(jobTask, client, &noopStore{}, func() {})
	ctx := WithCleanupTracker(context.Background())

	if err := ctl.Run(ctx); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	created, err := client.CoreV1().ServiceAccounts("ops").Get(context.Background(), "pod-labeler-sa", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("expected service account to be created: %v", err)
	}
	if created.Labels[config.LabelCli] != "app-1" {
		t.Fatalf("expected label %s=app-1, got %v", config.LabelCli, created.Labels)
	}
}

func TestDeployServiceAccountJobCtl_SkipUnmanaged(t *testing.T) {
	existing := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "shared-sa",
			Namespace: "ops",
			Labels: map[string]string{
				"owner": "platform",
			},
		},
	}
	client := fake.NewSimpleClientset(existing)
	jobTask := &model.JobTask{
		Name:      "shared-sa",
		Namespace: "ops",
		AppID:     "app-1",
		JobType:   string(config.JobDeployServiceAccount),
		JobInfo: &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name: "shared-sa",
			},
		},
	}
	ctl := NewDeployServiceAccountJobCtl(jobTask, client, &noopStore{}, func() {})
	ctx := WithCleanupTracker(context.Background())

	if err := ctl.Run(ctx); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	after, err := client.CoreV1().ServiceAccounts("ops").Get(context.Background(), "shared-sa", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("expected service account to exist: %v", err)
	}
	if _, ok := after.Labels[config.LabelCli]; ok {
		t.Fatalf("expected unmanaged service account to remain unchanged, got labels %v", after.Labels)
	}
}

func TestDeployRoleJobCtl_UpdateManaged(t *testing.T) {
	existing := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pod-labeler-role",
			Namespace: "ops",
			Labels: map[string]string{
				config.LabelCli: "app-1",
			},
		},
		Rules: []rbacv1.PolicyRule{{
			Verbs:     []string{"get"},
			APIGroups: []string{""},
			Resources: []string{"pods"},
		}},
	}
	client := fake.NewSimpleClientset(existing)
	jobTask := &model.JobTask{
		Name:      "pod-labeler-role",
		Namespace: "ops",
		AppID:     "app-1",
		JobType:   string(config.JobDeployRole),
		JobInfo: &rbacv1.Role{
			ObjectMeta: metav1.ObjectMeta{
				Name: "pod-labeler-role",
			},
			Rules: []rbacv1.PolicyRule{{
				Verbs:     []string{"get", "patch"},
				APIGroups: []string{""},
				Resources: []string{"pods"},
			}},
		},
	}
	ctl := NewDeployRoleJobCtl(jobTask, client, &noopStore{}, func() {})
	ctx := WithCleanupTracker(context.Background())

	if err := ctl.Run(ctx); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	updated, err := client.RbacV1().Roles("ops").Get(context.Background(), "pod-labeler-role", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("expected role to exist: %v", err)
	}
	if len(updated.Rules) != 1 || len(updated.Rules[0].Verbs) != 2 {
		t.Fatalf("expected role rules to be updated, got %+v", updated.Rules)
	}
}

// noopStore is a minimal datastore implementation for tests that do not persist.
type noopStore struct{}

func (*noopStore) Add(context.Context, datastore.Entity) error        { return nil }
func (*noopStore) BatchAdd(context.Context, []datastore.Entity) error { return nil }
func (*noopStore) Put(context.Context, datastore.Entity) error        { return nil }
func (*noopStore) Delete(context.Context, datastore.Entity) error     { return nil }
func (*noopStore) DeleteByFilter(context.Context, datastore.Entity, *datastore.FilterOptions) error {
	return nil
}
func (*noopStore) Get(context.Context, datastore.Entity) error { return nil }
func (*noopStore) List(context.Context, datastore.Entity, *datastore.ListOptions) ([]datastore.Entity, error) {
	return nil, nil
}
func (*noopStore) Count(context.Context, datastore.Entity, *datastore.FilterOptions) (int64, error) {
	return 0, nil
}
func (*noopStore) IsExist(context.Context, datastore.Entity) (bool, error) { return false, nil }
func (*noopStore) IsExistByCondition(context.Context, string, map[string]interface{}, interface{}) (bool, error) {
	return false, nil
}
func (*noopStore) CompareAndSwap(context.Context, datastore.Entity, string, interface{}, map[string]interface{}) (bool, error) {
	return true, nil
}
