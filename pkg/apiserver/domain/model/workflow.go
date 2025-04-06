package model

import "KubeMin-Cli/pkg/apiserver/config"

func init() {
	RegisterModel(&Workflow{})
}

// Workflow application delivery database model
type Workflow struct {
	ID           string                  `json:"id" gorm:"primaryKey"`
	Name         string                  `json:"name" `
	Alias        string                  `json:"alias"`    //别名
	Disabled     bool                    `json:"disabled"` //是否关闭，创建时默认为false
	ProjectId    string                  `json:"project_id"`
	AppID        string                  `gorm:"column:appid" json:"app_id"`
	UserID       string                  `json:"user_id"`
	Description  string                  `json:"description"`
	WorkflowType config.WorkflowTaskType `gorm:"column:workflow_type" json:"workflow_type"` //工作流类型
	Status       config.Status           `json:"status"`                                    //分为开启和关闭等状态
	Steps        *JSONStruct             `json:"steps,omitempty" gorm:"serializer:json"`
	BaseModel
}

type WorkflowSteps struct {
	Steps []*WorkflowStep `json:"steps"`
}

type WorkflowStep struct {
	Name         string         `json:"name"`
	WorkflowType config.JobType `json:"workflowType"`
	Properties   []Policies     `json:"properties"`
}

type Policies struct {
	Policies []string `json:"policies"`
}

func (w *Workflow) PrimaryKey() string {
	return w.ID
}

func (w *Workflow) TableName() string {
	return tableNamePrefix + "workflow"
}

func (w *Workflow) ShortTableName() string {
	return "workflow"
}

func (w *Workflow) Index() map[string]interface{} {
	index := make(map[string]interface{})
	if w.ID != "" {
		index["id"] = w.ID
	}
	if w.Name != "" {
		index["name"] = w.Name
	}
	return index
}
