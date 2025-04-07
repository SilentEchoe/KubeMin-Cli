package model

import "KubeMin-Cli/pkg/apiserver/config"

func init() {
	RegisterModel(&Applications{}, &ApplicationComponent{})
}

type Applications struct {
	ID          string            `json:"id" gorm:"primaryKey"`
	Name        string            `json:"name"` //应用名称
	Namespace   string            //命名空间，但是不对外暴露
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
	if a.Project != "" {
		index["project"] = a.Project
	}
	return index
}

// ApplicationComponent delivery database model 组件信息
type ApplicationComponent struct {
	ID            int               `json:"id" gorm:"primaryKey"`
	AppId         string            `json:"appId"`
	Name          string            `json:"name"`
	Replicas      int32             `json:"replicas"`
	ComponentType config.JobType    `json:"componentType"`
	Properties    *JSONStruct       `json:"properties,omitempty" gorm:"serializer:json"` //属性
	Labels        map[string]string `gorm:"-" json:"labels"`
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
	return index
}

type Properties struct {
	Image string  `json:"image"`
	Ports []Ports `json:"ports"`
}

type Ports struct {
	Port   int32 `json:"port"`
	Expose bool  `json:"expose"`
}
