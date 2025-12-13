package workflow

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"KubeMin-Cli/pkg/apiserver/config"
	"KubeMin-Cli/pkg/apiserver/domain/model"
	"KubeMin-Cli/pkg/apiserver/infrastructure/datastore"
	"KubeMin-Cli/pkg/apiserver/utils"
	wfNaming "KubeMin-Cli/pkg/apiserver/workflow/naming"
)

type fakeDataStore struct {
	workflow   *model.Workflow
	components []*model.ApplicationComponent
}

func (f *fakeDataStore) Add(context.Context, datastore.Entity) error {
	return fmt.Errorf("not implemented")
}
func (f *fakeDataStore) BatchAdd(context.Context, []datastore.Entity) error {
	return fmt.Errorf("not implemented")
}
func (f *fakeDataStore) Put(context.Context, datastore.Entity) error {
	return fmt.Errorf("not implemented")
}
func (f *fakeDataStore) Delete(context.Context, datastore.Entity) error {
	return fmt.Errorf("not implemented")
}
func (f *fakeDataStore) DeleteByFilter(context.Context, datastore.Entity, *datastore.FilterOptions) error {
	return fmt.Errorf("not implemented")
}

func (f *fakeDataStore) Get(ctx context.Context, entity datastore.Entity) error {
	switch e := entity.(type) {
	case *model.Workflow:
		*e = *f.workflow
		return nil
	default:
		return fmt.Errorf("unsupported entity type %T", entity)
	}
}

func (f *fakeDataStore) List(ctx context.Context, query datastore.Entity, _ *datastore.ListOptions) ([]datastore.Entity, error) {
	switch query.(type) {
	case *model.ApplicationComponent:
		result := make([]datastore.Entity, len(f.components))
		for i, c := range f.components {
			result[i] = c
		}
		return result, nil
	default:
		return nil, fmt.Errorf("unsupported list query %T", query)
	}
}

func (f *fakeDataStore) Count(context.Context, datastore.Entity, *datastore.FilterOptions) (int64, error) {
	return 0, fmt.Errorf("not implemented")
}

func (f *fakeDataStore) IsExist(context.Context, datastore.Entity) (bool, error) {
	return false, fmt.Errorf("not implemented")
}

func (f *fakeDataStore) IsExistByCondition(context.Context, string, map[string]interface{}, interface{}) (bool, error) {
	return false, fmt.Errorf("not implemented")
}

func (f *fakeDataStore) CompareAndSwap(context.Context, datastore.Entity, string, interface{}, map[string]interface{}) (bool, error) {
	return true, nil
}

func TestGenerateJobTasksSequential(t *testing.T) {
	serverProps, err := model.NewJSONStructByStruct(model.Properties{
		Image: "nginx:1.21",
		Ports: []model.Ports{{Port: 80}},
	})
	require.NoError(t, err)

	configProps, err := model.NewJSONStructByStruct(model.Properties{
		Conf: map[string]string{"config": "value"},
	})
	require.NoError(t, err)

	serverComponent := &model.ApplicationComponent{
		Name:          "server",
		AppID:         "app-1",
		Namespace:     "default",
		Image:         "nginx:1.21",
		Replicas:      1,
		ComponentType: config.ServerJob,
		Properties:    serverProps,
	}

	configComponent := &model.ApplicationComponent{
		Name:          "config",
		AppID:         "app-1",
		Namespace:     "default",
		ComponentType: config.ConfJob,
		Properties:    configProps,
	}

	steps := &model.WorkflowSteps{
		Steps: []*model.WorkflowStep{
			{Name: "server"},
			{Name: "config"},
		},
	}
	stepsJSON, err := model.NewJSONStructByStruct(steps)
	require.NoError(t, err)

	workflow := &model.Workflow{
		ID:    "wf-1",
		Steps: stepsJSON,
	}

	store := &fakeDataStore{
		workflow:   workflow,
		components: []*model.ApplicationComponent{serverComponent, configComponent},
	}

	task := &model.WorkflowQueue{
		WorkflowID:   "wf-1",
		AppID:        "app-1",
		ProjectID:    "proj-1",
		WorkflowName: "test-workflow",
	}

	executions := GenerateJobTasks(context.Background(), task, store, int64(config.DefaultJobTaskTimeout))
	require.Len(t, executions, 2)

	first := executions[0]
	require.Equal(t, "server", first.Name)
	require.Equal(t, config.WorkflowModeStepByStep, first.Mode)
	require.Len(t, first.Jobs[config.JobPriorityNormal], 2)

	second := executions[1]
	require.Equal(t, "config", second.Name)
	require.Equal(t, config.WorkflowModeStepByStep, second.Mode)
	require.Len(t, second.Jobs[config.JobPriorityMaxHigh], 1)
	cmJob := second.Jobs[config.JobPriorityMaxHigh][0]
	require.Equal(t, configComponent.Name, cmJob.Name)
	cmInput, ok := cmJob.JobInfo.(*model.ConfigMapInput)
	require.True(t, ok)
	require.Equal(t, cmJob.Name, cmInput.Name)
}

func TestGenerateJobTasksParallel(t *testing.T) {
	frontendProps, err := model.NewJSONStructByStruct(model.Properties{
		Image: "nginx:1.21",
		Ports: []model.Ports{{Port: 8080}},
	})
	require.NoError(t, err)

	backendProps, err := model.NewJSONStructByStruct(model.Properties{
		Image: "nginx:1.21",
		Ports: []model.Ports{{Port: 8081}},
	})
	require.NoError(t, err)

	frontend := &model.ApplicationComponent{
		Name:          "frontend",
		AppID:         "app-1",
		Namespace:     "default",
		Image:         "nginx:1.21",
		Replicas:      1,
		ComponentType: config.ServerJob,
		Properties:    frontendProps,
	}

	backend := &model.ApplicationComponent{
		Name:          "backend",
		AppID:         "app-1",
		Namespace:     "default",
		Image:         "nginx:1.21",
		Replicas:      1,
		ComponentType: config.ServerJob,
		Properties:    backendProps,
	}

	steps := &model.WorkflowSteps{
		Steps: []*model.WorkflowStep{
			{
				Name:       "apply-services",
				Mode:       config.WorkflowModeDAG,
				Properties: []model.Policies{{Policies: []string{"frontend", "backend"}}},
			},
		},
	}
	stepsJSON, err := model.NewJSONStructByStruct(steps)
	require.NoError(t, err)

	workflow := &model.Workflow{
		ID:    "wf-2",
		Steps: stepsJSON,
	}

	store := &fakeDataStore{
		workflow:   workflow,
		components: []*model.ApplicationComponent{frontend, backend},
	}

	task := &model.WorkflowQueue{
		WorkflowID:   "wf-2",
		AppID:        "app-1",
		ProjectID:    "proj-1",
		WorkflowName: "parallel-workflow",
	}

	executions := GenerateJobTasks(context.Background(), task, store, int64(config.DefaultJobTaskTimeout))
	require.Len(t, executions, 1)

	parallel := executions[0]
	require.Equal(t, config.WorkflowModeDAG, parallel.Mode)
	require.Equal(t, "apply-services", parallel.Name)

	jobs := parallel.Jobs[config.JobPriorityNormal]
	require.GreaterOrEqual(t, len(jobs), 2)
	deployCount := 0
	for _, job := range jobs {
		if job.JobType == string(config.JobDeploy) {
			deployCount++
		}
	}
	require.Equal(t, 2, deployCount)
}

func TestCreateObjectJobsFromResultIngressNaming(t *testing.T) {
	component := &model.ApplicationComponent{
		Name:      "Gateway",
		AppID:     "App-1",
		Namespace: "default",
	}
	task := &model.WorkflowQueue{
		WorkflowID: "wf-1",
		ProjectID:  "proj-1",
		AppID:      "App-1",
	}

	t.Run("auto name when ingress missing name", func(t *testing.T) {
		ing := &networkingv1.Ingress{}
		jobs, err := CreateObjectJobsFromResult([]client.Object{ing}, component, task, nil, int64(config.DefaultJobTaskTimeout))
		require.NoError(t, err)
		require.Len(t, jobs, 1)

		expected := fmt.Sprintf("ing-%s-%s", utils.NormalizeLowerStrip(component.Name), utils.NormalizeLowerStrip(component.AppID))
		require.Equal(t, expected, jobs[0].Name)

		ingressObj, ok := jobs[0].JobInfo.(*networkingv1.Ingress)
		require.True(t, ok)
		require.Equal(t, expected, ingressObj.Name)
		require.Equal(t, component.Namespace, ingressObj.Namespace)
	})

	t.Run("normalize pvc name and namespace", func(t *testing.T) {
		baseName := "DataVol"
		canonical := wfNaming.PVCName(baseName, component.AppID)
		pvc := &corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      canonical,
				Namespace: component.Namespace,
			},
		}

		j, err := CreateObjectJobsFromResult([]client.Object{pvc}, component, task, nil, int64(config.DefaultJobTaskTimeout))
		require.NoError(t, err)
		require.Len(t, j, 1)
		require.Equal(t, canonical, j[0].Name)

		pvcObj, ok := j[0].JobInfo.(*corev1.PersistentVolumeClaim)
		require.True(t, ok)
		require.Equal(t, canonical, pvcObj.Name)
		require.Equal(t, component.Namespace, pvcObj.Namespace)
	})

	t.Run("fill namespace when pvc missing it", func(t *testing.T) {
		canonical := wfNaming.PVCName("cache", component.AppID)
		pvc := &corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name: canonical,
			},
		}

		j, err := CreateObjectJobsFromResult([]client.Object{pvc}, component, task, nil, int64(config.DefaultJobTaskTimeout))
		require.NoError(t, err)
		require.Len(t, j, 1)

		pvcObj, ok := j[0].JobInfo.(*corev1.PersistentVolumeClaim)
		require.True(t, ok)
		require.Equal(t, component.Namespace, pvcObj.Namespace)
		require.Equal(t, canonical, j[0].Name)
	})

	t.Run("normalize existing ingress name", func(t *testing.T) {
		ing := &networkingv1.Ingress{}
		baseName := "CustomRoute"
		ing.Name = baseName

		jobs, err := CreateObjectJobsFromResult([]client.Object{ing}, component, task, nil, int64(config.DefaultJobTaskTimeout))
		require.NoError(t, err)
		require.Len(t, jobs, 1)

		expected := fmt.Sprintf("ing-%s-%s", utils.NormalizeLowerStrip(baseName), utils.NormalizeLowerStrip(component.AppID))
		require.Equal(t, expected, jobs[0].Name)

		ingressObj, ok := jobs[0].JobInfo.(*networkingv1.Ingress)
		require.True(t, ok)
		require.Equal(t, expected, ingressObj.Name)
		require.Equal(t, component.Namespace, ingressObj.Namespace)
	})
}

func TestCreateObjectJobsFromResultIgnoresConfigAndSecret(t *testing.T) {
	component := &model.ApplicationComponent{Name: "app", Namespace: "demo", AppID: "aid"}
	task := &model.WorkflowQueue{WorkflowID: "wf", ProjectID: "proj", AppID: "aid"}
	cm := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "app-config"}, Data: map[string]string{"key": "value"}}
	secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "app-secret"}}
	jobs, err := CreateObjectJobsFromResult([]client.Object{cm, secret}, component, task, nil, int64(config.DefaultJobTaskTimeout))
	require.NoError(t, err)
	require.Empty(t, jobs, "configmap/secret should be ignored; dedicated jobs exist elsewhere")
}

func TestCreateObjectJobsFromResultRBAC(t *testing.T) {
	component := &model.ApplicationComponent{
		Name:      "Labeler",
		AppID:     "App-2",
		Namespace: "ops",
	}
	task := &model.WorkflowQueue{
		WorkflowID: "wf-rbac",
		ProjectID:  "proj-rbac",
		AppID:      component.AppID,
	}

	sa := &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: "pod-labeler-sa"}}
	role := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{Name: "pod-labeler-role"},
		Rules: []rbacv1.PolicyRule{{
			Verbs:     []string{"get"},
			APIGroups: []string{""},
			Resources: []string{"pods"},
		}},
	}
	binding := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{Name: "pod-labeler-binding"},
		Subjects: []rbacv1.Subject{{
			Kind:      rbacv1.ServiceAccountKind,
			Name:      sa.Name,
			Namespace: component.Namespace,
		}},
		RoleRef: rbacv1.RoleRef{
			Kind:     "Role",
			APIGroup: rbacv1.GroupName,
			Name:     role.Name,
		},
	}
	clusterRole := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{Name: "pod-labeler-cluster-role"},
		Rules: []rbacv1.PolicyRule{{
			Verbs:     []string{"list"},
			APIGroups: []string{""},
			Resources: []string{"pods"},
		}},
	}
	clusterBinding := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{Name: "pod-labeler-cluster-binding"},
		Subjects: []rbacv1.Subject{{
			Kind:      rbacv1.ServiceAccountKind,
			Name:      sa.Name,
			Namespace: component.Namespace,
		}},
		RoleRef: rbacv1.RoleRef{
			Kind:     "ClusterRole",
			APIGroup: rbacv1.GroupName,
			Name:     clusterRole.Name,
		},
	}

	objs := []client.Object{sa, role, binding, clusterRole, clusterBinding}
	jobs, err := CreateObjectJobsFromResult(objs, component, task, nil, int64(config.DefaultJobTaskTimeout))
	require.NoError(t, err)
	require.Len(t, jobs, 5)

	jobTypes := make(map[string]bool)
	for _, job := range jobs {
		jobTypes[job.JobType] = true
		require.NotNil(t, job.JobInfo)
		switch job.JobType {
		case string(config.JobDeployServiceAccount):
			saObj, ok := job.JobInfo.(*corev1.ServiceAccount)
			require.True(t, ok)
			require.Equal(t, component.Namespace, saObj.Namespace)
		case string(config.JobDeployRole):
			roleObj, ok := job.JobInfo.(*rbacv1.Role)
			require.True(t, ok)
			require.Equal(t, component.Namespace, roleObj.Namespace)
		case string(config.JobDeployRoleBinding):
			bindingObj, ok := job.JobInfo.(*rbacv1.RoleBinding)
			require.True(t, ok)
			require.Equal(t, component.Namespace, bindingObj.Namespace)
		case string(config.JobDeployClusterRole):
			_, ok := job.JobInfo.(*rbacv1.ClusterRole)
			require.True(t, ok)
		case string(config.JobDeployClusterRoleBinding):
			_, ok := job.JobInfo.(*rbacv1.ClusterRoleBinding)
			require.True(t, ok)
		default:
			t.Fatalf("unexpected job type %s", job.JobType)
		}
	}

	require.True(t, jobTypes[string(config.JobDeployServiceAccount)])
	require.True(t, jobTypes[string(config.JobDeployRole)])
	require.True(t, jobTypes[string(config.JobDeployRoleBinding)])
	require.True(t, jobTypes[string(config.JobDeployClusterRole)])
	require.True(t, jobTypes[string(config.JobDeployClusterRoleBinding)])
}

func TestSecretJobNameNormalization(t *testing.T) {
	secretProps, err := model.NewJSONStructByStruct(model.Properties{Secret: map[string]string{"token": "value"}})
	require.NoError(t, err)

	component := &model.ApplicationComponent{
		Name:          "ApiKey",
		AppID:         "App-Env",
		Namespace:     "",
		ComponentType: config.SecretJob,
		Properties:    secretProps,
	}

	task := &model.WorkflowQueue{
		WorkflowID: "wf-secret",
		ProjectID:  "proj-1",
		AppID:      component.AppID,
	}

	ctx := context.Background()
	buckets := buildJobsForComponent(ctx, component, task, int64(config.DefaultJobTaskTimeout))
	jobs := buckets[config.JobPriorityMaxHigh]
	require.Len(t, jobs, 1)

	expectedName := component.Name
	require.Equal(t, expectedName, jobs[0].Name)

	secretInput, ok := jobs[0].JobInfo.(*model.SecretInput)
	require.True(t, ok)
	require.Equal(t, expectedName, secretInput.Name)
	require.Equal(t, config.DefaultNamespace, secretInput.Namespace)
}
