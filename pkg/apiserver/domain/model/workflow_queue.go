package model

import (
	"KubeMin-Cli/pkg/apiserver/config"
	"time"
)

func init() {
	RegisterModel(&WorkflowQueue{})
}

type WorkflowQueue struct {
	ID                  int64                   `json:"id,omitempty"`  //动态ID
	TaskID              string                  `json:"task_id"`       //任务ID，自生成
	ProjectId           string                  `json:"projectId"`     //所属项目
	WorkflowName        string                  `json:"workflow_name"` //工作流名称(唯一)
	AppID               string                  `json:"app_id"`
	WorkflowId          string                  `gorm:"column:workflowId" json:"workflow_id"`
	WorkflowDisplayName string                  `json:"workflow_display_name"`  //工作流显示名称
	Status              config.Status           `json:"status,omitempty"`       //当前状态
	TaskCreator         string                  `json:"task_creator,omitempty"` //任务创建者
	TaskRevoker         string                  `json:"task_revoker,omitempty"` //任务取消者
	Type                config.WorkflowTaskType `json:"type,omitempty"`         //工作流类型
	BaseModel
}

func (wq *WorkflowQueue) PrimaryKey() string {
	return wq.TaskID
}

func (wq *WorkflowQueue) TableName() string {
	return tableNamePrefix + "workflow_queue"
}

func (wq *WorkflowQueue) ShortTableName() string {
	return "workflow_queue"
}

func (wq *WorkflowQueue) Index() map[string]interface{} {
	index := make(map[string]interface{})
	if wq.TaskID != "" {
		index["task_id"] = wq.TaskID
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
