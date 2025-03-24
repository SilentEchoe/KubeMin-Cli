package model

import (
	"KubeMin-Cli/pkg/apiserver/config"
	"time"
)

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
