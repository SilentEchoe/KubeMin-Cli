package traits

import (
	"kubemin-cli/pkg/apiserver/domain/model"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"
)

func TestApplyTraits_SidecarTrait_WithNestedTraits(t *testing.T) {
	// 1. Define the input component with a sidecar that has its own storage.
	traitsStruct := &model.Traits{
		Sidecar: []model.SidecarSpec{
			{
				Name:  "my-sidecar",
				Image: "sidecar:v1",
				Env:   map[string]string{"SIDECAR_VAR": "value1"},
				Traits: model.Traits{
					Storage: []model.StorageTrait{
						{
							Name:      "sidecar-data",
							Type:      "ephemeral",
							MountPath: "/data",
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
	// Reset processors for a clean test to avoid double registration panics.
	registeredTraitProcessors = []TraitProcessor{}
	Register(&SidecarProcessor{})
	Register(&StorageProcessor{}) // Register dependency trait

	_, err = ApplyTraits(component, workload)
	require.NoError(t, err)

	// 4. Marshal and print for snapshot verification.
	yamlBytes, err := yaml.Marshal(workload.Spec.Template.Spec)
	require.NoError(t, err)
	fmt.Println(string(yamlBytes))

	// 5. Assertions
	require.Len(t, workload.Spec.Template.Spec.Containers, 2, "Expected main container and one sidecar")

	sidecarContainer := findContainer(workload.Spec.Template.Spec.Containers, "my-sidecar")
	require.NotNil(t, sidecarContainer, "Sidecar container should be found")
	require.Equal(t, "sidecar:v1", sidecarContainer.Image)

	require.Len(t, sidecarContainer.VolumeMounts, 1, "Sidecar should have one volume mount")
	require.Equal(t, "sidecar-data", sidecarContainer.VolumeMounts[0].Name)
	require.Equal(t, "/data", sidecarContainer.VolumeMounts[0].MountPath)

	require.Len(t, workload.Spec.Template.Spec.Volumes, 1, "Pod should have one volume for the sidecar")
	require.Equal(t, "sidecar-data", workload.Spec.Template.Spec.Volumes[0].Name)
	require.NotNil(t, workload.Spec.Template.Spec.Volumes[0].EmptyDir, "Volume should be an EmptyDir")
}
