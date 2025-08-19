package traits

import (
	"KubeMin-Cli/pkg/apiserver/domain/model"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestResourcesProcessor(t *testing.T) {
	// Register processors needed for the test
	orderedProcessors = []TraitProcessor{} // Clear existing processors for a clean test
	Register(&ResourcesProcessor{})

	// Test component with resources trait
	component := &model.ApplicationComponent{
		Name:      "test-component",
		Namespace: "test-namespace",
		Traits: toJSONStruct(model.Traits{
			Resources: &model.ResourceSpec{
				CPU:    "500m",
				Memory: "512Mi",
				GPU:    "1",
			},
		}),
	}

	// Base workload
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

	// Apply traits
	_, err := ApplyTraits(component, workload)
	require.NoError(t, err)

	// Verify resources were applied to the main container
	mainContainer := workload.Spec.Template.Spec.Containers[0]
	assert.NotNil(t, mainContainer.Resources.Limits)

	// Check CPU
	cpuQty, exists := mainContainer.Resources.Limits[corev1.ResourceCPU]
	assert.True(t, exists)
	assert.Equal(t, resource.MustParse("500m"), cpuQty)

	// Check Memory
	memQty, exists := mainContainer.Resources.Limits[corev1.ResourceMemory]
	assert.True(t, exists)
	assert.Equal(t, resource.MustParse("512Mi"), memQty)

	// Check GPU
	gpuQty, exists := mainContainer.Resources.Limits[corev1.ResourceName("nvidia.com/gpu")]
	assert.True(t, exists)
	assert.Equal(t, resource.MustParse("1"), gpuQty)
}

func TestResourcesProcessor_WithSidecar(t *testing.T) {
	// Register processors needed for the test
	orderedProcessors = []TraitProcessor{} // Clear existing processors for a clean test
	Register(&ResourcesProcessor{})
	Register(&SidecarProcessor{})

	// Test component with sidecar that has its own resources
	component := &model.ApplicationComponent{
		Name:      "test-component",
		Namespace: "test-namespace",
		Traits: toJSONStruct(model.Traits{
			Sidecar: []model.SidecarSpec{
				{
					Name:  "my-sidecar",
					Image: "sidecar:v1",
					Traits: model.Traits{
						Resources: &model.ResourceSpec{
							CPU:    "200m",
							Memory: "256Mi",
						},
					},
				},
			},
		}),
	}

	// Base workload
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

	// Apply traits
	_, err := ApplyTraits(component, workload)
	require.NoError(t, err)

	// Verify sidecar container was created with resources
	require.Len(t, workload.Spec.Template.Spec.Containers, 2, "Should have main container and sidecar")

	sidecarContainer := workload.Spec.Template.Spec.Containers[1]
	assert.Equal(t, "my-sidecar", sidecarContainer.Name)
	assert.NotNil(t, sidecarContainer.Resources.Limits)

	// Check sidecar CPU
	cpuQty, exists := sidecarContainer.Resources.Limits[corev1.ResourceCPU]
	assert.True(t, exists)
	assert.Equal(t, resource.MustParse("200m"), cpuQty)

	// Check sidecar Memory
	memQty, exists := sidecarContainer.Resources.Limits[corev1.ResourceMemory]
	assert.True(t, exists)
	assert.Equal(t, resource.MustParse("256Mi"), memQty)
}
