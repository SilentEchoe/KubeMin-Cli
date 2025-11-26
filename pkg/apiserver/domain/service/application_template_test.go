package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"KubeMin-Cli/pkg/apiserver/config"
	"KubeMin-Cli/pkg/apiserver/domain/model"
	"KubeMin-Cli/pkg/apiserver/domain/spec"
	apisv1 "KubeMin-Cli/pkg/apiserver/interfaces/api/dto/v1"
	"KubeMin-Cli/pkg/apiserver/utils/bcode"
)

func TestCreateApplicationsFromTemplateRequiresEnable(t *testing.T) {
	store := newInMemoryAppStore()
	templateApp := &model.Applications{ID: "tmpl-1", Name: "tmpl", TmpEnble: false}
	store.apps[templateApp.ID] = templateApp
	store.components["tmpl-comp"] = &model.ApplicationComponent{
		Name:          "tmpl-comp",
		AppID:         templateApp.ID,
		Replicas:      1,
		ComponentType: config.StoreJob,
	}

	svc := &applicationsServiceImpl{Store: store}
	req := apisv1.CreateApplicationsRequest{
		Name: "new-app",
		Component: []apisv1.CreateComponentRequest{{
			Name:          "new-comp",
			ComponentType: config.StoreJob,
			Template:      &apisv1.TemplateRef{ID: templateApp.ID},
		}},
	}

	_, err := svc.CreateApplications(context.Background(), req)
	require.ErrorIs(t, err, bcode.ErrTemplateNotEnabled)
}

func TestCreateApplicationsFromTemplateClonesTraitsAndNames(t *testing.T) {
	store := newInMemoryAppStore()
	templateApp := &model.Applications{ID: "tmpl-2", Name: "mysql", TmpEnble: true}
	store.apps[templateApp.ID] = templateApp

	templateTraits := apisv1.Traits{
		Storage: []spec.StorageTraitSpec{{
			Name:      "mysql",
			ClaimName: "mysql",
			Create:    true,
			Size:      "1Gi",
			Type:      config.StorageTypePersistent,
		}},
		Ingress: []spec.IngressTraitsSpec{{
			Name: "mysql",
			Routes: []spec.IngressRoutes{{
				Backend: spec.IngressRoute{ServiceName: "mysql"},
			}},
		}},
		RBAC: []spec.RBACPolicySpec{{
			ServiceAccount: "mysql",
			RoleName:       "mysql",
			BindingName:    "mysql",
		}},
	}
	traitsJSON, err := model.NewJSONStructByStruct(templateTraits)
	require.NoError(t, err)

	templateProps := apisv1.Properties{Env: map[string]string{"a": "b"}}
	propsJSON, err := model.NewJSONStructByStruct(templateProps)
	require.NoError(t, err)

	store.components["mysql"] = &model.ApplicationComponent{
		Name:          "mysql",
		AppID:         templateApp.ID,
		Namespace:     config.DefaultNamespace,
		Image:         "mysql:latest",
		Replicas:      1,
		ComponentType: config.StoreJob,
		Properties:    propsJSON,
		Traits:        traitsJSON,
	}

	svc := &applicationsServiceImpl{Store: store}
	tmpEnable := true
	req := apisv1.CreateApplicationsRequest{
		Name:  "cloned-app",
		Alias: "cloned-app",
		Component: []apisv1.CreateComponentRequest{{
			Name:          "new-mysql",
			ComponentType: config.StoreJob,
			Template:      &apisv1.TemplateRef{ID: templateApp.ID},
		}},
		TmpEnble: &tmpEnable,
	}

	resp, err := svc.CreateApplications(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.True(t, resp.TmpEnble)

	var createdComponent *model.ApplicationComponent
	for _, comp := range store.components {
		if comp.AppID == resp.ID {
			createdComponent = comp
			break
		}
	}
	require.NotNil(t, createdComponent)
	require.Equal(t, "new-mysql", createdComponent.Name)

	var clonedTraits apisv1.Traits
	require.NoError(t, json.Unmarshal([]byte(createdComponent.Traits.JSON()), &clonedTraits))
	require.Len(t, clonedTraits.Storage, 1)
	require.Equal(t, "new-mysql", clonedTraits.Storage[0].Name)
	require.Equal(t, "new-mysql", clonedTraits.Storage[0].ClaimName)

	require.Len(t, clonedTraits.Ingress, 1)
	require.Equal(t, "new-mysql", clonedTraits.Ingress[0].Name)
	require.Len(t, clonedTraits.Ingress[0].Routes, 1)
	require.Equal(t, "new-mysql", clonedTraits.Ingress[0].Routes[0].Backend.ServiceName)

	require.Len(t, clonedTraits.RBAC, 1)
	// RBAC 资源名称保持模板值，不强制重写
	require.Equal(t, "mysql", clonedTraits.RBAC[0].ServiceAccount)
	require.Equal(t, "mysql", clonedTraits.RBAC[0].RoleName)
	require.Equal(t, "mysql", clonedTraits.RBAC[0].BindingName)
	require.Equal(t, config.DefaultNamespace, clonedTraits.RBAC[0].Namespace)
}
