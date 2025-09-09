package model

import (
	"KubeMin-Cli/pkg/apiserver/config"
	"KubeMin-Cli/pkg/apiserver/spec"
)

func init() {
	RegisterModel(&Applications{}, &ApplicationComponent{})
}

type Applications struct {
	ID          string            `json:"id" gorm:"primaryKey"`
	Name        string            `json:"name"`        //应用名称
	Namespace   string            `json:"-"`           //命名空间，但是不对外暴露
	Version     string            `json:"version"`     //版本，如果为空则默认为1.0.0
	Alias       string            `json:"alias"`       //别名
	Project     string            `json:"project"`     //项目
	Description string            `json:"description"` //详情
	Icon        string            `json:"icon"`        //图标
	Labels      map[string]string `json:"labels,omitempty" gorm:"serializer:json"`
	BaseModel
}

func (a *Applications) PrimaryKey() string {
	return a.Name
}

func (a *Applications) TableName() string {
	return tableNamePrefix + "applications"
}

func (a *Applications) ShortTableName() string {
	return "app"
}

// Index return custom index
func (a *Applications) Index() map[string]interface{} {
	index := make(map[string]interface{})
	if a.Name != "" {
		index["name"] = a.Name
	}
	if a.Version != "" {
		index["version"] = a.Version
	}
	if a.Project != "" {
		index["project"] = a.Project
	}
	return index
}

// ApplicationComponent delivery database model 组件信息
type ApplicationComponent struct {
	ID            int            `json:"id" gorm:"primaryKey"`
	AppId         string         `json:"appId"`
	Name          string         `json:"name"`
	Namespace     string         `json:"namespace"`
	Image         string         `json:"image"`
	Replicas      int32          `json:"replicas"`
	ComponentType config.JobType `json:"componentType"`
	Properties    *JSONStruct    `json:"properties,omitempty" gorm:"serializer:json"`
	Traits        *JSONStruct    `json:"traits" gorm:"serializer:json"`
	BaseModel
}

func (w *ApplicationComponent) PrimaryKey() string {
	return w.Name
}

func (w *ApplicationComponent) TableName() string {
	return tableNamePrefix + "app_components"
}

func (w *ApplicationComponent) ShortTableName() string {
	return "app_component"
}

func (w *ApplicationComponent) Index() map[string]interface{} {
	index := make(map[string]interface{})
	if w.Name != "" {
		index["name"] = w.Name
	}
	if w.AppId != "" {
		index["appid"] = w.AppId
	}
	return index
}

type Properties = spec.Properties

type Ports = spec.Ports

// Traits 附加特性
type Traits = spec.Traits

// EnvFromSourceSpec corresponds to a single corev1.EnvFromSource.
type EnvFromSourceSpec = spec.EnvFromSourceSpec

// SimplifiedEnvSpec is the user-friendly, simplified way to define environment variables.
type SimplifiedEnvSpec = spec.SimplifiedEnvSpec

// ValueSource defines the source for an environment variable's value.
// Only one of its fields may be set.
// Static 可能根本不需要实现，因为Env就直接实现这种方式
type ValueSource = spec.ValueSource

// SecretSelectorSpec selects a key from a Secret.
type SecretSelectorSpec = spec.SecretSelectorSpec

// ConfigMapSelectorSpec selects a key from a ConfigMap.
type ConfigMapSelectorSpec = spec.ConfigMapSelectorSpec

// InitTrait 初始化容器的特征
type InitTrait = spec.InitTraitSpec

// StorageTrait 描述存储特征
type StorageTrait = spec.StorageTraitSpec

type ConfigMapSpec = spec.ConfigMapSpec

type SecretSpec = spec.SecretTraitsSpec

type SidecarSpec = spec.SidecarTraitsSpec

// ResourceSpec defines CPU/Memory/GPU resources for containers
type ResourceSpec = spec.ResourceTraitsSpec
