package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"kubemin-cli/pkg/apiserver/config"
	"kubemin-cli/pkg/apiserver/domain/model"
	"kubemin-cli/pkg/apiserver/domain/spec"
	apisv1 "kubemin-cli/pkg/apiserver/interfaces/api/dto/v1"
	"kubemin-cli/pkg/apiserver/utils/bcode"
)

func TestCreateApplicationsFromTemplateRequiresEnable(t *testing.T) {
	store := newInMemoryAppStore()
	templateApp := &model.Applications{ID: "tmpl-1", Name: "tmpl", TmpEnable: false}
	store.apps[templateApp.ID] = templateApp
	store.components["tmpl-comp"] = &model.ApplicationComponent{
		Name:          "tmpl-comp",
		AppID:         templateApp.ID,
		Replicas:      1,
		ComponentType: config.StoreJob,
	}

	svc := newMockServiceWithStore(store)
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
	templateApp := &model.Applications{ID: "tmpl-2", Name: "mysql", TmpEnable: true}
	store.apps[templateApp.ID] = templateApp

	templateTraits := apisv1.Traits{
		Storage: []spec.StorageTraitSpec{{
			Name:       "mysql",
			ClaimName:  "mysql",
			SourceName: "tem-mysql-config",
			TmpCreate:  true,
			Size:       "1Gi",
			Type:       config.StorageTypePersistent,
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
		Envs: []spec.SimplifiedEnvSpec{{
			Name: "MYSQL_ROOT_PASSWORD",
			ValueFrom: spec.ValueSource{
				Secret: &spec.SecretSelectorSpec{
					Name: "tem-mysql-secret",
					Key:  "MYSQL_ROOT_PASSWORD",
				},
			},
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

	templateSecretProps := apisv1.Properties{Secret: map[string]string{"MYSQL_ROOT_PASSWORD": "orig"}}
	secretPropsJSON, err := model.NewJSONStructByStruct(templateSecretProps)
	require.NoError(t, err)
	store.components["mysql-secret"] = &model.ApplicationComponent{
		Name:          "mysql-secret",
		AppID:         templateApp.ID,
		Namespace:     config.DefaultNamespace,
		ComponentType: config.SecretJob,
		Properties:    secretPropsJSON,
	}

	templateConfigProps := apisv1.Properties{
		Conf: map[string]string{"master.cnf": "dummy", "slave.cnf": "dummy"},
	}
	configPropsJSON, err := model.NewJSONStructByStruct(templateConfigProps)
	require.NoError(t, err)
	store.components["tem-mysql-config"] = &model.ApplicationComponent{
		Name:          "tem-mysql-config",
		AppID:         templateApp.ID,
		Namespace:     config.DefaultNamespace,
		ComponentType: config.ConfJob,
		Properties:    configPropsJSON,
	}

	svc := newMockServiceWithStore(store)
	tmpEnable := true
	req := apisv1.CreateApplicationsRequest{
		Name:  "cloned-app",
		Alias: "cloned-app",
		Component: []apisv1.CreateComponentRequest{
			{
				Name:          "new-mysql",
				ComponentType: config.StoreJob,
				Properties: apisv1.Properties{
					Env: map[string]string{"a": "override", "NEW": "env"},
				},
				Template: &apisv1.TemplateRef{ID: templateApp.ID, Target: "mysql"},
			},
			{
				Name:          "new-mysql-secret",
				ComponentType: config.SecretJob,
				Properties: apisv1.Properties{
					Secret: map[string]string{"MYSQL_ROOT_PASSWORD": "override-secret"},
				},
				Template: &apisv1.TemplateRef{ID: templateApp.ID, Target: "tem-mysql-secret"},
			},
		},
		TmpEnable: &tmpEnable,
	}

	resp, err := svc.CreateApplications(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.True(t, resp.TmpEnable)

	var createdStore *model.ApplicationComponent
	for _, comp := range store.components {
		if comp.AppID == resp.ID && comp.ComponentType == config.StoreJob {
			createdStore = comp
			break
		}
	}
	require.NotNil(t, createdStore)
	require.Equal(t, "new-mysql", createdStore.Name)

	var clonedTraits apisv1.Traits
	require.NoError(t, json.Unmarshal([]byte(createdStore.Traits.JSON()), &clonedTraits))
	require.Len(t, clonedTraits.Storage, 1)
	require.Equal(t, "new-mysql", clonedTraits.Storage[0].Name)
	require.Equal(t, "new-mysql", clonedTraits.Storage[0].ClaimName)
	require.Equal(t, "cloned-app-config", clonedTraits.Storage[0].SourceName)

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

	// env override 生效
	var clonedProps apisv1.Properties
	require.NoError(t, json.Unmarshal([]byte(createdStore.Properties.JSON()), &clonedProps))
	require.Equal(t, "override", clonedProps.Env["a"])
	require.Equal(t, "env", clonedProps.Env["NEW"])
	require.Len(t, clonedTraits.Envs, 1)
	require.Equal(t, "new-mysql-secret", clonedTraits.Envs[0].ValueFrom.Secret.Name)

	// secret 组件被克隆一次，并允许覆盖 secret 值
	var createdSecret *model.ApplicationComponent
	for _, comp := range store.components {
		if comp.AppID == resp.ID && comp.ComponentType == config.SecretJob {
			createdSecret = comp
			break
		}
	}
	require.NotNil(t, createdSecret)
	require.Equal(t, "new-mysql-secret", createdSecret.Name)
	var secretProps apisv1.Properties
	require.NoError(t, json.Unmarshal([]byte(createdSecret.Properties.JSON()), &secretProps))
	require.Equal(t, "override-secret", secretProps.Secret["MYSQL_ROOT_PASSWORD"])
}

func TestCreateApplicationsFromTemplateRewritesPersistentStorageNames(t *testing.T) {
	store := newInMemoryAppStore()
	templateApp := &model.Applications{ID: "tmpl-3", Name: "mysql", TmpEnable: true}
	store.apps[templateApp.ID] = templateApp

	templateTraits := apisv1.Traits{
		Storage: []spec.StorageTraitSpec{{
			Name:      "data",
			Type:      config.StorageTypePersistent,
			TmpCreate: true,
			Size:      "1Gi",
			MountPath: "/var/lib/mysql",
		}},
		Sidecar: []spec.SidecarTraitsSpec{
			{
				Name: "backup",
				Traits: apisv1.Traits{
					Storage: []spec.StorageTraitSpec{{
						Name:      "data",
						Type:      config.StorageTypePersistent,
						MountPath: "/var/lib/mysql",
					}},
				},
			},
		},
	}
	traitsJSON, err := model.NewJSONStructByStruct(templateTraits)
	require.NoError(t, err)

	propsJSON, err := model.NewJSONStructByStruct(apisv1.Properties{})
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

	svc := newMockServiceWithStore(store)
	req := apisv1.CreateApplicationsRequest{
		Name: "tenant-a-mysql-app",
		Component: []apisv1.CreateComponentRequest{
			{
				Name:          "tenant-a-mysql",
				ComponentType: config.StoreJob,
				Template:      &apisv1.TemplateRef{ID: templateApp.ID, Target: "mysql"},
			},
		},
	}

	resp, err := svc.CreateApplications(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, resp)

	var createdStore *model.ApplicationComponent
	for _, comp := range store.components {
		if comp.AppID == resp.ID && comp.ComponentType == config.StoreJob {
			createdStore = comp
			break
		}
	}
	require.NotNil(t, createdStore)

	var clonedTraits apisv1.Traits
	require.NoError(t, json.Unmarshal([]byte(createdStore.Traits.JSON()), &clonedTraits))
	require.Len(t, clonedTraits.Storage, 1)
	require.Equal(t, "tenant-a-mysql-app-data", clonedTraits.Storage[0].Name)
	require.Empty(t, clonedTraits.Storage[0].ClaimName)

	require.Len(t, clonedTraits.Sidecar, 1)
	require.Len(t, clonedTraits.Sidecar[0].Traits.Storage, 1)
	require.Equal(t, "tenant-a-mysql-app-data", clonedTraits.Sidecar[0].Traits.Storage[0].Name)
	require.Empty(t, clonedTraits.Sidecar[0].Traits.Storage[0].ClaimName)
}

func TestCreateApplicationsFromTemplateShareKeepsTemplateNames(t *testing.T) {
	store := newInMemoryAppStore()
	templateApp := &model.Applications{ID: "tmpl-share-1", Name: "mysql", TmpEnable: true}
	store.apps[templateApp.ID] = templateApp

	traitsJSON, err := model.NewJSONStructByStruct(apisv1.Traits{})
	require.NoError(t, err)

	store.components["mysql"] = &model.ApplicationComponent{
		Name:          "mysql",
		AppID:         templateApp.ID,
		Namespace:     config.DefaultNamespace,
		Image:         "mysql:latest",
		Replicas:      1,
		ComponentType: config.StoreJob,
		Traits:        traitsJSON,
	}

	svc := newMockServiceWithStore(store)
	req := apisv1.CreateApplicationsRequest{
		Name: "shared-mysql-app",
		Component: []apisv1.CreateComponentRequest{
			{
				Name:          "ignored-name",
				ComponentType: config.StoreJob,
				Traits: apisv1.Traits{
					Share: &spec.ShareTraitSpec{Strategy: "default"},
				},
				Template: &apisv1.TemplateRef{ID: templateApp.ID, Target: "mysql"},
			},
		},
	}

	resp, err := svc.CreateApplications(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, resp)

	var createdStore *model.ApplicationComponent
	for _, comp := range store.components {
		if comp.AppID == resp.ID && comp.ComponentType == config.StoreJob {
			createdStore = comp
			break
		}
	}
	require.NotNil(t, createdStore)
	require.Equal(t, "mysql", createdStore.Name)

	var clonedTraits apisv1.Traits
	require.NoError(t, json.Unmarshal([]byte(createdStore.Traits.JSON()), &clonedTraits))
	require.NotNil(t, clonedTraits.Share)
	require.Equal(t, "default", clonedTraits.Share.Strategy)
}
