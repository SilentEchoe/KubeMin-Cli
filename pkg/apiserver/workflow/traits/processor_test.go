package traits

import (
	"KubeMin-Cli/pkg/apiserver/domain/model"
	"fmt"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"
	"testing"
)

func TestApplyTraits_InitTrait_WithNestedTraits(t *testing.T) {
	// 1. Define the input component with two init containers sharing a volume.
	traitsStruct := &model.Traits{
		Init: []model.InitTrait{
			{
				Name: "init-mysql",
				Properties: model.Properties{
					Image:   "kubectl:1.28.5",
					Command: []string{"bash", "-c", ""},
					Env:     map[string]string{"MYSQL_DATABASE": "test"},
				},
				Traits: []model.Traits{
					{
						Storage: []model.StorageTrait{
							{
								Name:      "conf",
								Type:      "config",
								MountPath: "/mnt/conf.d",
							},
							{
								Name:      "config-map",
								Type:      "config",
								MountPath: "/mnt/config-map",
							},
							{
								Name:      "init-scripts",
								Type:      "config",
								MountPath: "/docker-entrypoint-initdb.d",
							},
						},
					},
				},
			},
			{
				Name: "clone-mysql",
				Properties: model.Properties{
					Image:   "xtrabackup:latest",
					Command: []string{"bash", "-c"},
				},
				Traits: []model.Traits{
					{
						Storage: []model.StorageTrait{
							{ //使用稳定存储进行挂载
								Name:      "data",
								Type:      "persistent",
								MountPath: "/var/lib/mysql",
								SubPath:   "mysql",
							},
							{
								Name:      "conf",
								Type:      "config",
								MountPath: "/etc/mysql/conf.d",
							},
						},
					},
				},
			},
		},
	}
	traitsJSON, err := model.NewJSONStructByStruct(traitsStruct)
	require.NoError(t, err)

	component := &model.ApplicationComponent{
		Name:      "test-component",
		Namespace: "test-namespace",
		Traits:    traitsJSON,
	}

	// 2. Define the base workload.
	workload := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: component.Name, Namespace: component.Namespace},
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{Name: component.Name, Image: "my-app:1.0"}},
				},
			},
		},
	}

	// 3. Apply the traits.
	_, err = ApplyTraits(component, workload)
	require.NoError(t, err)

	// 4. Marshal and print for snapshot verification.
	yamlBytes, err := yaml.Marshal(workload.Spec.Template.Spec)
	require.NoError(t, err)
	fmt.Println(string(yamlBytes))
}

func TestApplyTraits_FinalSimplifiedEnvs(t *testing.T) {
	// 1. Define the input component with the final, structured envs spec.
	staticValue := "some_static_value"
	fieldPath := "metadata.namespace"
	traitsStruct := &model.Traits{
		Envs: []model.SimplifiedEnvSpec{
			{
				Name: "STATIC_VAR",
				ValueFrom: model.ValueSource{Static: &staticValue},
			},
			{
				Name: "PASSWORD_FROM_SECRET",
				ValueFrom: model.ValueSource{Secret: &model.SecretSelectorSpec{
					Name: "secret/with/slashes",
					Key:  "password",
				}},
			},
			{
				Name: "API_KEY_FROM_CONFIG",
				ValueFrom: model.ValueSource{Config: &model.ConfigMapSelectorSpec{
					Name: "config/with/slashes",
					Key:  "api-key",
				}},
			},
			{
				Name: "MY_POD_NAMESPACE",
				ValueFrom: model.ValueSource{Field: &fieldPath},
			},
		},
		EnvFrom: []model.EnvFromSourceSpec{
			{
				Type:       "config",
				SourceName: "another-configmap",
			},
		},
	}
	traitsJSON, err := model.NewJSONStructByStruct(traitsStruct)
	require.NoError(t, err)

	component := &model.ApplicationComponent{
		Name:      "test-component",
		Namespace: "test-namespace",
		Traits:    traitsJSON,
	}

	// 2. Define the base workload.
	workload := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: component.Name, Namespace: component.Namespace},
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{Name: component.Name, Image: "my-app:1.0"}},
				},
			},
		},
	}

	// 3. Apply the traits.
	_, err = ApplyTraits(component, workload)
	require.NoError(t, err)

	// 4. Assertions
	mainContainer := workload.Spec.Template.Spec.Containers[0]

	// 4.1 Verify EnvFrom sources
	require.Len(t, mainContainer.EnvFrom, 1, "Expected one EnvFrom source")
	require.Equal(t, "another-configmap", mainContainer.EnvFrom[0].ConfigMapRef.Name)

	// 4.2 Verify Envs (translated from SimplifiedEnvSpec)
	require.Len(t, mainContainer.Env, 4, "Expected four environment variables")

	// Static Var
	require.Equal(t, "STATIC_VAR", mainContainer.Env[0].Name)
	require.Equal(t, "some_static_value", mainContainer.Env[0].Value)

	// SecretKeyRef
	require.Equal(t, "PASSWORD_FROM_SECRET", mainContainer.Env[1].Name)
	require.NotNil(t, mainContainer.Env[1].ValueFrom.SecretKeyRef)
	require.Equal(t, "secret/with/slashes", mainContainer.Env[1].ValueFrom.SecretKeyRef.Name)
	require.Equal(t, "password", mainContainer.Env[1].ValueFrom.SecretKeyRef.Key)

	// ConfigMapKeyRef
	require.Equal(t, "API_KEY_FROM_CONFIG", mainContainer.Env[2].Name)
	require.NotNil(t, mainContainer.Env[2].ValueFrom.ConfigMapKeyRef)
	require.Equal(t, "config/with/slashes", mainContainer.Env[2].ValueFrom.ConfigMapKeyRef.Name)
	require.Equal(t, "api-key", mainContainer.Env[2].ValueFrom.ConfigMapKeyRef.Key)

	// FieldRef
	require.Equal(t, "MY_POD_NAMESPACE", mainContainer.Env[3].Name)
	require.NotNil(t, mainContainer.Env[3].ValueFrom.FieldRef)
	require.Equal(t, "metadata.namespace", mainContainer.Env[3].ValueFrom.FieldRef.FieldPath)

	// 5. Marshal and print for snapshot verification.
	yamlBytes, err := yaml.Marshal(workload.Spec.Template.Spec.Containers)
	require.NoError(t, err)
	fmt.Println(string(yamlBytes))
}