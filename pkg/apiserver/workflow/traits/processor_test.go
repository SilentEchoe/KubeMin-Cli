package traits

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"

	"KubeMin-Cli/pkg/apiserver/domain/model"
	spec "KubeMin-Cli/pkg/apiserver/domain/spec"
)

func setupProcessors() {
	registeredTraitProcessors = []TraitProcessor{}
	RegisterAllProcessors()
}

func TestApplyTraits_InitTrait_WithNestedTraits(t *testing.T) {
	setupProcessors()
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

func TestApplyTraitsBindsServiceAccount(t *testing.T) {
	setupProcessors()
	automount := true
	traitsStruct := &spec.Traits{
		RBAC: []spec.RBACPolicySpec{
			{
				ServiceAccount:             "pod-labeler-sa",
				ServiceAccountAutomountSAT: &automount,
				Rules: []spec.RBACRuleSpec{
					{
						Verbs:     []string{"get"},
						Resources: []string{"pods"},
					},
				},
			},
		},
	}
	traitsJSON, err := model.NewJSONStructByStruct(traitsStruct)
	require.NoError(t, err)
	raw, err := json.Marshal(traitsJSON)
	require.NoError(t, err)
	var parsed spec.Traits
	require.NoError(t, json.Unmarshal(raw, &parsed))
	require.NotNil(t, parsed.RBAC[0].ServiceAccountAutomountSAT)
	require.True(t, *parsed.RBAC[0].ServiceAccountAutomountSAT)

	component := &model.ApplicationComponent{
		Name:      "worker",
		Namespace: "demo",
		Traits:    traitsJSON,
	}

	workload := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{Name: component.Name, Namespace: component.Namespace},
		Spec: appsv1.StatefulSetSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{Name: component.Name, Image: "example.com/worker:latest"}},
				},
			},
		},
	}

	additional, err := ApplyTraits(component, workload)
	require.NoError(t, err)
	require.NotNil(t, additional)
	require.GreaterOrEqual(t, len(additional), 3)
	sa, ok := additional[0].(*corev1.ServiceAccount)
	require.True(t, ok)
	require.NotNil(t, sa.AutomountServiceAccountToken)
	require.True(t, *sa.AutomountServiceAccountToken)
	t.Logf("podSpec SA=%s automount ptr=%v", workload.Spec.Template.Spec.ServiceAccountName, workload.Spec.Template.Spec.AutomountServiceAccountToken)
	require.Equal(t, "pod-labeler-sa", workload.Spec.Template.Spec.ServiceAccountName)
	require.NotNil(t, workload.Spec.Template.Spec.AutomountServiceAccountToken)
	require.True(t, *workload.Spec.Template.Spec.AutomountServiceAccountToken)
}
