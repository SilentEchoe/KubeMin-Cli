package model

import "KubeMin-Cli/pkg/apiserver/config"

// Workflow application delivery database model
type Workflow struct {
	BaseModel
	ID          int              `json:"Id" gorm:"primaryKey"`
	Name        string           `json:"name"`
	Alias       string           `json:"alias"` //别名
	Disabled    bool             `json:"disabled"`
	Project     string           `json:"project"`
	Description string           `json:"description"`
	Default     *bool            `json:"default"`
	Stages      []*WorkflowStage `json:"stages"`
	Hash        string           `json:"hash"`
}

type WorkflowStage struct {
	Name     string `json:"name"`
	Parallel bool   `json:"parallel"` //是否并行
	Jobs     []*Job `json:"jobs"`
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
