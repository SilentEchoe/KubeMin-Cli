package model

import (
	"KubeMin-Cli/pkg/apiserver/config"
)

func init() {
	RegisterModel(&Applications{}, &ApplicationComponent{})
}

type Applications struct {
	ID          string            `json:"id" gorm:"primaryKey"`
	Name        string            `json:"name"` //应用名称
	Namespace   string            //命名空间，但是不对外暴露
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

func (a Applications) TableName() string {
	return tableNamePrefix + "applications"
}

func (a Applications) ShortTableName() string {
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

type Properties struct {
	Image  string            `json:"image"`
	Ports  []Ports           `json:"ports"`
	Env    map[string]string `json:"env"`
	Labels map[string]string `json:"labels"`
}

type Ports struct {
	Port   int32 `json:"port"`
	Expose bool  `json:"expose"`
}

// Traits 附加特性
type Traits struct {
	Storage []StorageTrait  `json:"storage,omitempty"` //存储特性
	Config  []ConfigMapSpec `json:"config,omitempty"`  //配置文件
	Secret  []SecretSpec    `json:"secret,omitempty"`  //密钥信息
	Sidecar []SidecarSpec   `json:"sidecar,omitempty"` //容器边车
}

type StorageTrait struct {
	Name      string `json:"name,omitempty"`
	Type      string `json:"type"`
	MountPath string `json:"mountPath"`
	Size      string `json:"size"`
}

type ConfigMapSpec struct {
	Data map[string]string `json:"data"`
}

type SecretSpec struct {
	Data map[string]string `json:"data"`
}

type SidecarSpec struct {
	Name    string            `json:"name"`  //如果用户不填写，可以根据组件的名称来自定义
	Image   string            `json:"image"` //必填镜像
	Command []string          `json:"command,omitempty"`
	Args    []string          `json:"args,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
	Traits  Traits            `json:"mounts,omitempty"` //可以附加各种特征，但是边车容器内不能附加边车容器，这点需要进行校验
}
