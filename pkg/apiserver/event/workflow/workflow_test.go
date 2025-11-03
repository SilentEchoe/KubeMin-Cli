package workflow

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"KubeMin-Cli/pkg/apiserver/config"
	"KubeMin-Cli/pkg/apiserver/domain/model"
	job "KubeMin-Cli/pkg/apiserver/event/workflow/job"
	"KubeMin-Cli/pkg/apiserver/infrastructure/datastore"
	"KubeMin-Cli/pkg/apiserver/utils"
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

	executions := GenerateJobTasks(context.Background(), task, store)
	require.Len(t, executions, 2)

	first := executions[0]
	require.Equal(t, "server", first.Name)
	require.Equal(t, config.WorkflowModeStepByStep, first.Mode)
	require.Len(t, first.Jobs[config.JobPriorityNormal], 2)

	second := executions[1]
	require.Equal(t, "config", second.Name)
	require.Equal(t, config.WorkflowModeStepByStep, second.Mode)
	require.Len(t, second.Jobs[config.JobPriorityHigh], 1)
	cmJob := second.Jobs[config.JobPriorityHigh][0]
	require.Equal(t, job.BuildConfigMapName(configComponent.Name, configComponent.AppID), cmJob.Name)
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

	executions := GenerateJobTasks(context.Background(), task, store)
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
		jobs, err := CreateObjectJobsFromResult([]client.Object{ing}, component, task, nil)
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
		pvc := &corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name: baseName,
			},
		}

		j, err := CreateObjectJobsFromResult([]client.Object{pvc}, component, task, nil)
		require.NoError(t, err)
		require.Len(t, j, 1)

		expected := job.BuildPVCName(baseName, component.AppID)
		require.Equal(t, expected, j[0].Name)

		pvcObj, ok := j[0].JobInfo.(*corev1.PersistentVolumeClaim)
		require.True(t, ok)
		require.Equal(t, expected, pvcObj.Name)
		require.Equal(t, component.Namespace, pvcObj.Namespace)
	})

	t.Run("normalize existing ingress name", func(t *testing.T) {
		ing := &networkingv1.Ingress{}
		baseName := "CustomRoute"
		ing.Name = baseName

		jobs, err := CreateObjectJobsFromResult([]client.Object{ing}, component, task, nil)
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
	buckets := buildJobsForComponent(ctx, component, task)
	jobs := buckets[config.JobPriorityHigh]
	require.Len(t, jobs, 1)

	expectedName := job.BuildSecretName(component.Name, component.AppID)
	require.Equal(t, expectedName, jobs[0].Name)

	secretInput, ok := jobs[0].JobInfo.(*model.SecretInput)
	require.True(t, ok)
	require.Equal(t, expectedName, secretInput.Name)
	require.Equal(t, config.DefaultNamespace, secretInput.Namespace)
}
