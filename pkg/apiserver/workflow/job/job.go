package job

import (
	"KubeMin-Cli/pkg/apiserver/domain/model"
)

type JobCtl interface {
	Instantiate() error
	ToJobs(taskID int64) ([]*model.JobTask, error)
}
