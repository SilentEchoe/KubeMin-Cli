package model

import (
	"KubeMin-Cli/pkg/apiserver/config"
	"time"
)

type WorkflowQueue struct {
	ID                  string        `json:"id,omitempty"`
	TaskID              int64         `json:"task_id"`
	ProjectName         string        `json:"project_name"`
	WorkflowName        string        ` json:"workflow_name"`
	WorkflowDisplayName string        `json:"workflow_display_name"`
	Status              config.Status `json:"status,omitempty"`
	Stages              []*StageTask  `json:"stages"`
	TaskCreator         string        `json:"task_creator,omitempty"`
	TaskRevoker         string        `json:"task_revoker,omitempty"`
	//CreateTime          int64                   `json:"create_time,omitempty"`
	Type config.WorkflowTaskType `json:"type,omitempty"`
	BaseModel
}

func (wq *WorkflowQueue) PrimaryKey() string {
	return wq.ID
}

func (wq *WorkflowQueue) TableName() string {
	return tableNamePrefix + "workflow_queue"
}

func (wq *WorkflowQueue) ShortTableName() string {
	return "workflow_queue"
}

func (wq *WorkflowQueue) Index() map[string]interface{} {
	index := make(map[string]interface{})
	if wq.ID != "" {
		index["id"] = wq.ID
	}

	return index
}

type StageTask struct {
	Name      string        `json:"name"`
	Status    config.Status `json:"status"`
	StartTime int64         `json:"start_time,omitempty"`
	EndTime   int64         `json:"end_time,omitempty"`
	Parallel  bool          `json:"parallel,omitempty"`
	Jobs      []*JobTask    `json:"jobs,omitempty"`
	Error     string        `json:"error"`
}

type JobTask struct {
	Key        string        `json:"key"`
	Name       string        `json:"name"`
	JobType    string        `json:"jobType"`
	JobInfo    interface{}   `json:"jobInfo"`
	Status     config.Status `json:"status"`
	StartTime  time.Time
	EndTime    time.Time
	Error      string
	RetryCount int //重试次数
}
