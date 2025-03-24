package job

import (
	"KubeMin-Cli/pkg/apiserver/config"
	"KubeMin-Cli/pkg/apiserver/domain/model"
	"fmt"
)

type JobCtl interface {
	Instantiate() error
	ToJobs(taskID int64) ([]*model.JobTask, error)
}

func InitJobCtl(job *model.Job, workflow *model.Workflow) (JobCtl, error) {
	var resp JobCtl
	switch job.JobType {
	case config.JobDeploy:
		resp = &DeployJob{job: job, workflow: workflow}
	default:
		return resp, fmt.Errorf("job type not found %s", job.JobType)
	}
	return resp, nil
}

func InstantiateWorkflow(workflow *model.Workflow) error {
	for _, stage := range workflow.Stages {
		for _, job := range stage.Jobs {
			if JobSkiped(job) {
				continue
			}

			if err := Instantiate(job, workflow); err != nil {
				return err
			}
		}
	}
	return nil
}

func Instantiate(job *model.Job, workflow *model.Workflow) error {
	ctl, err := InitJobCtl(job, workflow)
	if err != nil {
		return warpJobError(job.Name, err)
	}
	return ctl.Instantiate()
}

func JobSkiped(job *model.Job) bool {
	if job.RunPolicy == config.ForceRun {
		return false
	}
	return job.Skipped
}

func ToJobs(job *model.Job, workflow *model.Workflow, taskID int64) ([]*model.JobTask, error) {
	jobCtl, err := InitJobCtl(job, workflow)
	if err != nil {
		return []*model.JobTask{}, warpJobError(job.Name, err)
	}
	return jobCtl.ToJobs(taskID)
}

func warpJobError(jobName string, err error) error {
	return fmt.Errorf("[job: %s] %v", jobName, err)
}
