package job

import (
	"testing"

	"github.com/stretchr/testify/require"

	"KubeMin-Cli/pkg/apiserver/config"
	"KubeMin-Cli/pkg/apiserver/domain/model"
)

func TestGenerateServiceSelectorNonBundle(t *testing.T) {
	component := &model.ApplicationComponent{
		Name:      "api",
		Namespace: "default",
		AppID:     "app-1",
		ID:        7,
	}
	properties := &model.Properties{
		Ports: []model.Ports{{Port: 8080}},
	}

	svc := GenerateService(component, properties)
	require.NotNil(t, svc)
	require.NotNil(t, svc.Spec)
	require.Equal(t, map[string]string{
		config.LabelAppID:         "app-1",
		config.LabelComponentName: "api",
	}, svc.Spec.Selector)
}
