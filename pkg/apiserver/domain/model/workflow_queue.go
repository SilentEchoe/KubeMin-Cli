package model

import (
	"kubemin-cli/pkg/apiserver/config"
)

func init() {
	RegisterModel(&WorkflowQueue{})
}

type WorkflowQueue struct {
	TaskID              string                  `gorm:"primaryKey;type:varchar(255)" json:"task_id"` //任务ID，自生成
	ProjectID           string                  `json:"project_id"`                                   //所属项目
	WorkflowName        string                  `json:"workflow_name"`                               //工作流名称(唯一)
	AppID               string                  `json:"app_id"`
	WorkflowID          string                  `gorm:"column:workflowId" json:"workflow_id"`
	WorkflowDisplayName string                  `json:"workflow_display_name"`                 //工作流显示名称
	Status              config.Status           `gorm:"column:status" json:"status,omitempty"` //当前状态
	TaskCreator         string                  `json:"task_creator,omitempty"`                //任务创建者
	TaskRevoker         string                  `json:"task_revoker,omitempty"`                //任务取消者
	Type                config.WorkflowTaskType `json:"type,omitempty"`                        //工作流类型
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
	if wq.AppID != "" {
		index["app_id"] = wq.AppID
	}
	if wq.TaskID != "" {
		index["task_id"] = wq.TaskID
	}
	if wq.Status != "" {
		index["status"] = wq.Status
	}
	return index
}
