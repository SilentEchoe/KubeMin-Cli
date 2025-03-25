package model

import (
	"KubeMin-Cli/pkg/apiserver/config"
)

func init() {
	RegisterModel(&Workflow{}, &WorkflowComponent{})
}

// Workflow application delivery database model
type Workflow struct {
	BaseModel
	ID          string         `json:"id" gorm:"primaryKey"`
	Name        string         `json:"name" `
	Alias       string         `json:"alias"`    //别名
	Disabled    bool           `json:"disabled"` //是否关闭，创建时默认为true
	Project     string         `json:"project"`
	Description string         `json:"description"`
	Steps       []WorkflowStep `json:"steps,omitempty" gorm:"serializer:json"`
}

type WorkflowStep struct {
	Name     string `json:"name"`
	Parallel bool   `json:"parallel"` //是否并行
	Jobs     []Job  `json:"jobs"`
}

type Job struct {
	Name        string              `json:"name"`
	JobType     config.JobType      `json:"type"`
	Skipped     bool                `json:"skipped"`
	Spec        interface{}         `json:"spec"`
	RunPolicy   config.JobRunPolicy `json:"run_policy"`   //运行策略
	ErrorPolicy *JobErrorPolicy     `json:"error_policy"` //错误策略
}

type JobErrorPolicy struct {
	Policy       config.JobErrorPolicy `json:"policy"`        //Job的错误策略
	MaximumRetry int                   `json:"maximum_retry"` //最大重试次数
}

type DeployJobSpec struct {
	Env string `json:"env"`
}

func (w *Workflow) PrimaryKey() string {
	return w.Name
}

func (w *Workflow) TableName() string {
	return tableNamePrefix + "workflow"
}

func (w *Workflow) ShortTableName() string {
	return "work"
}

func (w *Workflow) Index() map[string]interface{} {
	index := make(map[string]interface{})
	if w.Name != "" {
		index["name"] = w.Name
	}
	return index
}

// WorkflowComponent delivery database model 组件信息
type WorkflowComponent struct {
	ID            int            `json:"id" gorm:"primaryKey"`
	WorkflowId    string         `json:"workflowId"`
	Name          string         `json:"name" `
	ComponentType config.JobType `json:"componentType"`
	Properties    *JSONStruct    `json:"properties,omitempty" gorm:"serializer:json"`
	BaseModel
}

func (w *WorkflowComponent) PrimaryKey() string {
	return w.Name
}

func (w *WorkflowComponent) TableName() string {
	return tableNamePrefix + "workflow_components"
}

func (w *WorkflowComponent) ShortTableName() string {
	return "work_component"
}

func (w *WorkflowComponent) Index() map[string]interface{} {
	index := make(map[string]interface{})
	if w.Name != "" {
		index["name"] = w.Name
	}
	return index
}
