package job

import (
	"KubeMin-Cli/pkg/apiserver/config"
	"KubeMin-Cli/pkg/apiserver/domain/model"
	"reflect"
	"testing"
)

func TestGenerateSecret(t *testing.T) {
	// Test case 1: Basic secret generation
	t.Run("BasicSecret", func(t *testing.T) {
		component := &model.ApplicationComponent{
			Name:      "my-secret",
			Namespace: "default",
			AppID:     "test-app",
			ID:        1,
		}
		properties := &model.Properties{
			Secret: map[string]string{
				"username": "admin",
				"password": "password123",
			},
		}
		expected := &model.SecretInput{
			Name:      "my-secret",
			Namespace: "default",
			Labels:    map[string]string{config.LabelCli: "test-app-my-secret", config.LabelAppID: "test-app", config.LabelComponentID: "1", config.LabelComponentName: "my-secret"},
			Data: map[string]string{
				"username": "admin",
				"password": "password123",
			},
		}
		actual := GenerateSecret(component, properties)
		if !reflect.DeepEqual(actual, expected) {
			t.Errorf("Expected %v, but got %v", expected, actual)
		}
	})

	// Test case 2: Secret generation from URL
	t.Run("SecretFromURL", func(t *testing.T) {
		component := &model.ApplicationComponent{
			Name:      "my-secret-from-url",
			Namespace: "kube-system",
			AppID:     "test-app",
			ID:        2,
		}
		properties := &model.Properties{
			Secret: map[string]string{},
			Conf: map[string]string{
				"config.url":      "http://example.com/config",
				"config.fileName": "my-config-file",
			},
		}
		expected := &model.SecretInput{
			Name:      "my-secret-from-url",
			Namespace: "kube-system",
			URL:       "http://example.com/config",
			FileName:  "my-config-file",
			Labels:    map[string]string{config.LabelCli: "test-app-my-secret-from-url", config.LabelAppID: "test-app", config.LabelComponentID: "2", config.LabelComponentName: "my-secret-from-url"},
		}
		actual := GenerateSecret(component, properties)
		if !reflect.DeepEqual(actual, expected) {
			t.Errorf("Expected %v, but got %v", expected, actual)
		}
	})

	// Test case 3: Nil properties
	t.Run("NilProperties", func(t *testing.T) {
		component := &model.ApplicationComponent{
			Name:      "nil-props-secret",
			Namespace: "default",
			AppID:     "test-app",
			ID:        3,
		}
		expected := &model.SecretInput{
			Name:      "nil-props-secret",
			Namespace: "default",
			Labels:    map[string]string{config.LabelCli: "test-app-nil-props-secret", config.LabelAppID: "test-app", config.LabelComponentID: "3", config.LabelComponentName: "nil-props-secret"},
			Data:      nil,
		}
		actual := GenerateSecret(component, nil)
		if !reflect.DeepEqual(actual, expected) {
			t.Errorf("Expected %v, but got %v", expected, actual)
		}
	})

	// Test case 4: Empty secret data
	t.Run("EmptySecretData", func(t *testing.T) {
		component := &model.ApplicationComponent{
			Name:      "empty-secret",
			Namespace: "default",
			AppID:     "test-app",
			ID:        4,
		}
		properties := &model.Properties{
			Secret: map[string]string{},
		}
		expected := &model.SecretInput{
			Name:      "empty-secret",
			Namespace: "default",
			Labels:    map[string]string{config.LabelCli: "test-app-empty-secret", config.LabelAppID: "test-app", config.LabelComponentID: "4", config.LabelComponentName: "empty-secret"},
			Data:      map[string]string{},
		}
		actual := GenerateSecret(component, properties)
		if !reflect.DeepEqual(actual, expected) {
			t.Errorf("Expected %v, but got %v", expected, actual)
		}
	})

	// Test case 5: No namespace provided
	t.Run("NoNamespace", func(t *testing.T) {
		component := &model.ApplicationComponent{
			Name:  "no-namespace-secret",
			AppID: "test-app",
			ID:    5,
		}
		properties := &model.Properties{
			Secret: map[string]string{"key": "value"},
		}
		expected := &model.SecretInput{
			Name:      "no-namespace-secret",
			Namespace: config.DefaultNamespace,
			Labels:    map[string]string{config.LabelCli: "test-app-no-namespace-secret", config.LabelAppID: "test-app", config.LabelComponentID: "5", config.LabelComponentName: "no-namespace-secret"},
			Data:      map[string]string{"key": "value"},
		}
		actual := GenerateSecret(component, properties)
		if !reflect.DeepEqual(actual, expected) {
			t.Errorf("Expected %v, but got %v", expected, actual)
		}
	})
}
