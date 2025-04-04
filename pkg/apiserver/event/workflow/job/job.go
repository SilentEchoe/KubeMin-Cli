package job

import (
	"KubeMin-Cli/pkg/apiserver/config"
	"context"
)

type JobCtl interface {
	Run(ctx context.Context)
	Clean(ctx context.Context)
	SaveInfo(ctx context.Context) error
}

// JobTask 是最小的执行单位
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

func initJobCtl(job *JobTask, ack func()) JobCtl {
	var jobCtl JobCtl
	switch job.JobType {
	case string(config.JobDeploy):
		jobCtl = NewDeployJobCtl(job, ack)
	}
	return jobCtl
}
