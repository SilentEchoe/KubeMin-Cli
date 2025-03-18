package workflow

import "KubeMin-Cli/pkg/apiserver/domain/model"

type DeployJob struct {
	job      *model.Job
	workflow *model.Workflow
	spec     *model.DeployJobSpec
}

// Instantiate 实例化:将Job的Spec转换为Yaml
func (d *DeployJob) Instantiate() error {
	//TODO implement me
	panic("implement me")
}
