package model

import "KubeMin-Cli/pkg/apiserver/config"

// Workflow application delivery database model
type Workflow struct {
	BaseModel
	ID          int              `json:"Id" gorm:"primaryKey"`
	Name        string           `json:"name"`
	Alias       string           `json:"alias"`
	Description string           `json:"description"`
	Default     *bool            `json:"default"`
	Stages      []*WorkflowStage `json:"stages"`
}

type WorkflowStage struct {
	Name string `json:"name"`
	Jobs []*Job `json:"jobs"`
}

type Job struct {
	Name      string              `json:"name"`
	JobType   config.JobType      `json:"type"`
	Spec      interface{}         `json:"spec"`
	RunPolicy config.JobRunPolicy `json:"run_policy"`
}

type DeployJobSpec struct {
	Env string `json:"env"`
}

func (w Workflow) PrimaryKey() string {
	return w.Name
}

func (w Workflow) TableName() string {
	return tableNamePrefix + "workflow"
}

func (w Workflow) ShortTableName() string {
	return "work"
}

func (w Workflow) Index() map[string]interface{} {
	index := make(map[string]interface{})
	if w.Name != "" {
		index["name"] = w.Name
	}
	return index
}
