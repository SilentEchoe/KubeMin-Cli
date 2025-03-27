package model

import "KubeMin-Cli/pkg/apiserver/config"

func init() {
	RegisterModel(&Workflow{})
}

// Workflow application delivery database model
type Workflow struct {
	ID          string        `json:"id" gorm:"primaryKey"`
	Name        string        `json:"name" `
	Alias       string        `json:"alias"`    //别名
	Disabled    bool          `json:"disabled"` //是否关闭，创建时默认为false
	Project     string        `json:"project"`
	AppID       string        `json:"appID"`
	UserID      string        `json:"userID"`
	Description string        `json:"description"`
	Status      config.Status `json:"status"`
	Steps       *JSONStruct   `json:"steps,omitempty" gorm:"serializer:json"`
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
