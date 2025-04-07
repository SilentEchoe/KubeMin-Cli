package model

import "KubeMin-Cli/pkg/apiserver/config"

type JobInfo struct {
	ID            string `json:"id"`
	Type          string `json:"type"`
	WorkflowName  string `json:"workflow_name"`
	ProductName   string `json:"product_name"`
	Status        string `bson:"status" json:"status"`
	StartTime     int64  `bson:"start_time" json:"start_time"`
	EndTime       int64  `bson:"end_time" json:"end_time"`
	ServiceType   string `json:"service_type"`
	ServiceName   string `json:"service_name"`
	ServiceModule string `json:"service_module"`
	Production    bool   `json:"production"` // 是否生产
	TargetEnv     string `json:"target_env"` //目标环境
}

// JobTask 是最小的执行单位
type JobTask struct {
	Name       string `json:"name"`
	Namespace  string `json:"namespace"`
	WorkflowId string `json:"workflow_id"`
	ProjectId  string `json:"project_id"`
	AppId      string `json:"app_id"`
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
	return j.ID
}

func (j *JobInfo) TableName() string {
	return tableNamePrefix + "job"
}

func (j *JobInfo) ShortTableName() string {
	return "jobinfo"
}

func (j *JobInfo) Index() map[string]interface{} {
	index := make(map[string]interface{})
	if j.ID != "" {
		index["id"] = j.ID
	}
	return index
}
