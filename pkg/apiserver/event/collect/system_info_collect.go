package collect

import (
	"context"
	"github.com/docker/docker/libnetwork/datastore"
	"github.com/robfig/cron/v3"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// InfoCalculateCronJob is the cronJob to calculate the system info store in db
type InfoCalculateCronJob struct {
	KubeClient client.Client       `inject:"kubeClient"`
	Store      datastore.DataStore `inject:"datastore"`
	cron       *cron.Cron
}

func (i InfoCalculateCronJob) Start(ctx context.Context, errChan chan error) {
	//TODO implement me
	panic("implement me")
}
