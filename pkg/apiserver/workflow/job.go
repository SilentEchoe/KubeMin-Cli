package workflow

import (
	"KubeMin-Cli/pkg/apiserver/config"
	"KubeMin-Cli/pkg/apiserver/domain/model"
	"fmt"
)

type JobCtl interface {
	Instantiate() error
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
		return err
	}
	return ctl.Instantiate()
}

func JobSkiped(job *model.Job) bool {
	if job.RunPolicy == config.ForceRun {
		return false
	}
	return true
}

func InitJobCtl(job *model.Job, workflow *model.Workflow) (JobCtl, error) {
	var resp JobCtl
	switch job.JobType {
	case config.DefaultJobBuild:
		resp = &BuildJob{job: job, workflow: workflow}
	case config.DefaultJobDeploy:
		resp = &DeployJob{job: job, workflow: workflow}
	default:
		return resp, fmt.Errorf("job type not found %s", job.JobType)
	}
	return resp, nil
}
