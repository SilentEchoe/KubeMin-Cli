package job

import (
	"KubeMin-Cli/pkg/apiserver/config"
	"KubeMin-Cli/pkg/apiserver/domain/model"
	"reflect"
	"testing"
)

func TestGenerateConfigMap(t *testing.T) {
	// Test case 1: Basic ConfigMap generation
	t.Run("BasicConfigMap", func(t *testing.T) {
		component := &model.ApplicationComponent{
			Name:      "my-configmap",
			Namespace: "default",
			AppID:     "test-app",
			ID:        1,
		}
		properties := &model.Properties{
			Conf: map[string]string{
				"key1": "value1",
				"key2": "value2",
			},
		}
		expected := &model.ConfigMapInput{
			Name:      "my-configmap",
			Namespace: "default",
			Labels:    map[string]string{config.LabelCli: "test-app-my-configmap", config.LabelAppID: "test-app", config.LabelComponentID: "1", config.LabelComponentName: "my-configmap"},
			Data: map[string]string{
				"key1": "value1",
				"key2": "value2",
			},
		}
		actual := GenerateConfigMap(component, properties)
		if !reflect.DeepEqual(actual, expected) {
			t.Errorf("Expected %v, but got %v", expected, actual)
		}
	})

	// Test case 2: ConfigMap generation from URL
	t.Run("ConfigMapFromURL", func(t *testing.T) {
		component := &model.ApplicationComponent{
			Name:      "my-configmap-from-url",
			Namespace: "kube-system",
			AppID:     "test-app",
			ID:        2,
		}
		properties := &model.Properties{
			Conf: map[string]string{
				"config.url":      "http://example.com/config.txt",
				"config.fileName": "my-config-file.txt",
			},
		}
		expected := &model.ConfigMapInput{
			Name:      "my-configmap-from-url",
			Namespace: "kube-system",
			URL:       "http://example.com/config.txt",
			FileName:  "my-config-file.txt",
		}
		actual := GenerateConfigMap(component, properties)
		if !reflect.DeepEqual(actual, expected) {
			t.Errorf("Expected %v, but got %v", expected, actual)
		}
	})

	// Test case 3: Nil properties
	t.Run("NilProperties", func(t *testing.T) {
		component := &model.ApplicationComponent{
			Name:      "nil-props-configmap",
			Namespace: "default",
			AppID:     "test-app",
			ID:        3,
		}
		expected := &model.ConfigMapInput{
			Name:      "nil-props-configmap",
			Namespace: "default",
			Labels:    map[string]string{config.LabelCli: "test-app-nil-props-configmap", config.LabelAppID: "test-app", config.LabelComponentID: "3", config.LabelComponentName: "nil-props-configmap"},
			Data:      nil,
		}
		actual := GenerateConfigMap(component, nil)
		if !reflect.DeepEqual(actual, expected) {
			t.Errorf("Expected %v, but got %v", expected, actual)
		}
	})

	// Test case 4: Empty ConfigMap data
	t.Run("EmptyConfigMapData", func(t *testing.T) {
		component := &model.ApplicationComponent{
			Name:      "empty-configmap",
			Namespace: "default",
			AppID:     "test-app",
			ID:        4,
		}
		properties := &model.Properties{
			Conf: map[string]string{},
		}
		expected := &model.ConfigMapInput{
			Name:      "empty-configmap",
			Namespace: "default",
			Labels:    map[string]string{config.LabelCli: "test-app-empty-configmap", config.LabelAppID: "test-app", config.LabelComponentID: "4", config.LabelComponentName: "empty-configmap"},
			Data:      map[string]string{},
		}
		actual := GenerateConfigMap(component, properties)
		if !reflect.DeepEqual(actual, expected) {
			t.Errorf("Expected %v, but got %v", expected, actual)
		}
	})

	// Test case 5: No namespace provided
	t.Run("NoNamespace", func(t *testing.T) {
		component := &model.ApplicationComponent{
			Name:  "no-namespace-configmap",
			AppID: "test-app",
			ID:    5,
		}
		properties := &model.Properties{
			Conf: map[string]string{"key": "value"},
		}
		expected := &model.ConfigMapInput{
			Name:      "no-namespace-configmap",
			Namespace: config.DefaultNamespace,
			Labels:    map[string]string{config.LabelCli: "test-app-no-namespace-configmap", config.LabelAppID: "test-app", config.LabelComponentID: "5", config.LabelComponentName: "no-namespace-configmap"},
			Data:      map[string]string{"key": "value"},
		}
		actual := GenerateConfigMap(component, properties)
		if !reflect.DeepEqual(actual, expected) {
			t.Errorf("Expected %v, but got %v", expected, actual)
		}
	})
}
