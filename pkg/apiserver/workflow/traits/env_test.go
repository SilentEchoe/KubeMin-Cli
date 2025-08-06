package traits

import (
	"KubeMin-Cli/pkg/apiserver/domain/model"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"
)

// TestEnvProcessor comprehensively tests the env trait processing.
func TestEnvProcessor(t *testing.T) {
	// Register processors needed for the tests.
	// In a real test setup, this might be done in a TestMain or setup function.
	orderedProcessors = []TraitProcessor{} // Clear existing processors for a clean test
	Register(&InitProcessor{})
	Register(&EnvFromProcessor{})
	Register(&StorageProcessor{}) // Storage is often used alongside other traits

	// --- Test Data Setup ---

	// Mock Component with a main container and an init container trait
	mockComponent := &model.ApplicationComponent{
		Name:  "main-app",
		Image: "main-app:v1",
		Traits: toJSONStruct(model.Traits{
			// Top-level env trait for the main container
			EnvFrom: []model.EnvFromSourceSpec{
				{Type: "config", SourceName: "main-app-config"},
				{Type: "secret", SourceName: "main-app-secret"},
			},
			// Init container trait with its own nested env trait
			Init: []model.InitTrait{
				{
					Name: "my-init-container",
					Properties: model.Properties{
						Image: "init:v1",
					},
					Traits: []model.Traits{
						{
							EnvFrom: []model.EnvFromSourceSpec{
								{Type: "config", SourceName: "init-container-config"},
							},
						},
					},
				},
			},
		}),
	}

	// Mock base workload (Deployment)
	mockWorkload := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-deployment",
		},
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "main-app", // Name matches component name
							Image: "main-app:v1",
						},
					},
				},
			},
		},
	}

	// --- Test Cases ---

	t.Run("Correctly applies top-level and nested env traits", func(t *testing.T) {
		// --- Arrange ---
		// Deep copy workload to avoid modification across tests
		workload := mockWorkload.DeepCopy()

		// --- Act ---
		_, err := ApplyTraits(mockComponent, workload)

		// --- Assert ---
		assert.NoError(t, err)

		// 1. Verify the main container
		mainContainer := findContainer(workload.Spec.Template.Spec.Containers, "main-app")
		assert.NotNil(t, mainContainer, "Main container should be found")
		assert.Len(t, mainContainer.EnvFrom, 2, "Main container should have 2 envFrom sources")
		assert.Equal(t, "main-app-config", mainContainer.EnvFrom[0].ConfigMapRef.Name)
		assert.Equal(t, "main-app-secret", mainContainer.EnvFrom[1].SecretRef.Name)

		// 2. Verify the init container
		assert.Len(t, workload.Spec.Template.Spec.InitContainers, 1, "Should be one init container")
		initContainer := findContainer(workload.Spec.Template.Spec.InitContainers, "my-init-container")
		assert.NotNil(t, initContainer, "Init container should be found")
		assert.Len(t, initContainer.EnvFrom, 1, "Init container should have 1 envFrom source")
		assert.Equal(t, "init-container-config", initContainer.EnvFrom[0].ConfigMapRef.Name)

		// 3. Marshal and print for visual verification
		yamlBytes, err := yaml.Marshal(workload.Spec.Template.Spec)
		require.NoError(t, err)
		fmt.Println("--- YAML output for TestEnvProcessor ---")
		fmt.Println(string(yamlBytes))
	})

	t.Run("Handles empty traits gracefully", func(t *testing.T) {
		// --- Arrange ---
		workload := mockWorkload.DeepCopy()
		componentWithEmptyTraits := &model.ApplicationComponent{
			Name:   "main-app",
			Image:  "main-app:v1",
			Traits: toJSONStruct(model.Traits{}),
		}

		// --- Act ---
		_, err := ApplyTraits(componentWithEmptyTraits, workload)

		// --- Assert ---
		assert.NoError(t, err)
		mainContainer := findContainer(workload.Spec.Template.Spec.Containers, "main-app")
		assert.Len(t, mainContainer.EnvFrom, 0, "Main container should have no envFrom sources")
		assert.Len(t, workload.Spec.Template.Spec.InitContainers, 0, "Should be no init containers")
	})
}

// --- Helper Functions ---

// toJSONStruct converts a struct to a model.JSONStruct for easy test setup.
func toJSONStruct(v interface{}) *model.JSONStruct {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	var data model.JSONStruct
	if err := json.Unmarshal(b, &data); err != nil {
		panic(err)
	}
	return &data
}

// findContainer is a helper to find a container by name in a slice of containers.
func findContainer(containers []corev1.Container, name string) *corev1.Container {
	for i, c := range containers {
		if c.Name == name {
			return &containers[i]
		}
	}
	return nil
}
