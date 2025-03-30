package model

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
