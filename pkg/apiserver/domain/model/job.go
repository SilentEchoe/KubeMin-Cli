package model

import (
	"strconv"

	"kubemin-cli/pkg/apiserver/config"
)

func init() {
	RegisterModel(&JobInfo{})
}

type JobInfo struct {
	ID          int    `json:"id" gorm:"primaryKey"`
	Type        string `json:"type"`
	WorkflowID  string `json:"workflow_id"`
	ProductID   string `json:"product_id"`
	AppID       string `json:"app_id"`
	TaskID      string `gorm:"column:taskid" json:"task_id"`
	Status      string `bson:"status" json:"status"`
	StartTime   int64  `bson:"start_time" json:"start_time"`
	EndTime     int64  `bson:"end_time" json:"end_time"`
	ServiceType string `json:"service_type"`
	ServiceName string `json:"service_name"`
	Error       string `json:"error"`
	Production  bool   `json:"production"` // 是否生产
	TargetEnv   string `json:"target_env"` //目标环境
	BaseModel
}

// JobTask 是最小的执行单位
type JobTask struct {
	Name       string `json:"name"`
	Namespace  string `json:"namespace"`
	WorkflowID string `json:"workflow_id"`
	ProjectID  string `json:"project_id"`
	AppID      string `json:"app_id"`
	TaskID     string
	JobInfo    interface{}
	JobType    string
	Status     config.Status
	StartTime  int64
	EndTime    int64
	Error      string
	Timeout    int64
	RetryCount int //重试次数
}

func (j *JobInfo) PrimaryKey() string {
	return strconv.FormatInt(int64(j.ID), 10)
}

func (j *JobInfo) TableName() string {
	return tableNamePrefix + "job"
}

func (j *JobInfo) ShortTableName() string {
	return "job_info"
}

func (j *JobInfo) Index() map[string]interface{} {
	index := make(map[string]interface{})
	if j.TaskID != "" {
		index["taskid"] = j.TaskID
	}
	if j.WorkflowID != "" {
		index["workflow_id"] = j.WorkflowID
	}
	return index
}

type JobDeployInfo struct {
	Name          string
	Ready         bool
	Replicas      int32 //期望副本数量
	ReadyReplicas int32 //就绪副本数量
}
