package workflow

import "KubeMin-Cli/pkg/apiserver/domain/model"

type BuildJob struct {
	job      *model.Job
	workflow *model.Workflow
}

func (b BuildJob) Instantiate() error {
	//TODO implement me
	panic("implement me")
}
