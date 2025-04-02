package job

import (
	"KubeMin-Cli/pkg/apiserver/config"
	"context"
)

type JobCtl interface {
	Run(ctx context.Context)
	// do some clean stuff when workflow finished, like collect reports or clean up resources.
	Clean(ctx context.Context)
	// SaveInfo is used to update the basic information of the job task to the mongoDB
	SaveInfo(ctx context.Context) error
}

// JobTask 是最小的执行单位
type JobTask struct {
	Name        string `json:"name"`
	WorkflowKey string `json:"workflowKey"`
	ProjectKey  string `json:"projectKey"`
	JobInfo     interface{}
	JobType     string
	Status      config.Status
	StartTime   int64
	EndTime     int64
	Error       string
	Timeout     int64
	Spec        interface{}
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
