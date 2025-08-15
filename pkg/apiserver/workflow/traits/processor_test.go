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
				Traits: model.Traits{
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
			{
				Name: "clone-mysql",
				Properties: model.Properties{
					Image:   "xtrabackup:latest",
					Command: []string{"bash", "-c"},
				},
				Traits: model.Traits{
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
