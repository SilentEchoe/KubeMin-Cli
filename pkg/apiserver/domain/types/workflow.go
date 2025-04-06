package types

import "KubeMin-Cli/pkg/apiserver/config"

// WorkflowQueue represents a workflow queue
type WorkflowQueue struct {
	ID           string                  `json:"id" gorm:"primaryKey"`
	Name         string                  `json:"name" `
	Alias        string                  `json:"alias"`    //别名
	Disabled     bool                    `json:"disabled"` //是否关闭，创建时默认为false
	Project      string                  `json:"project"`
	AppID        string                  `gorm:"column:appid" json:"appID"`
	UserID       string                  `json:"userID"`
	Description  string                  `json:"description"`
	WorkflowType config.WorkflowTaskType `gorm:"column:workflow_type" json:"workflow_type"` //工作流类型
	Status       config.Status           `json:"status"`                                    //分为开启和关闭等状态
}

// JobTask represents a job task
type JobTask struct {
	Name        string `json:"name"`
	Namespace   string `json:"namespace"`
	WorkflowKey string `json:"workflowKey"`
	ProjectKey  string `json:"projectKey"`
	APPKey      string `json:"APPKey"`
	JobInfo     interface{}
	JobType     string
	Status      config.Status
	StartTime   int64
	EndTime     int64
	Error       string
	Timeout     int64
	RetryCount  int //重试次数
}
