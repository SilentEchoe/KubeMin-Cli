package collect

import (
	"context"
	"github.com/docker/docker/libnetwork/datastore"
	"github.com/robfig/cron/v3"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"time"
)

// CrontabSpec the cron spec of job running
var CrontabSpec = "0 0 * * *"

// maximum tires is 5, initial duration is 1 minute
var waitBackOff = wait.Backoff{
	Steps:    5,
	Duration: 1 * time.Minute,
	Factor:   5.0,
	Jitter:   0.1,
}

// InfoCalculateCronJob is the cronJob to calculate the system info store in db
// 用于定时任务
type InfoCalculateCronJob struct {
	KubeClient client.Client `inject:"kubeClient"`
	Store      datastore.DataStore
	cron       *cron.Cron
}

func (i InfoCalculateCronJob) Start(ctx context.Context, errChan chan error) {
	i.start(CrontabSpec)
	defer i.cron.Stop()
	<-ctx.Done()
}

func (i *InfoCalculateCronJob) start(cronSpec string) {
	// 这里是一个定时任务的队列
	c := cron.New(cron.WithChain(
		// 这里会Reconver 所有的Cron
		cron.Recover(cron.DefaultLogger),
	))
	//  ignore the entityId and error, the cron spec is defined by hard code, mustn't generate error
	// 忽略entityId和error， cron规范是由硬代码定义的，一定不会产生错误
	_, _ = c.AddFunc(cronSpec, func() {
		// 重试这个任务
		err := retry.OnError(waitBackOff, func(err error) bool { //nolint:revive,unused
			// always retry
			return true
		}, func() error {
			if err := i.run(); err != nil {
				klog.Errorf("Failed to calculate systemInfo, will try again after several minute error %v", err)
				return err
			}
			klog.Info("Successfully to calculate systemInfo")
			return nil
		})
		if err != nil {
			klog.Errorf("After 5 tries the calculating cronJob failed: %v", err)
		}
	})
	i.cron = c
	c.Start()
}

func (i InfoCalculateCronJob) run() error {
	return nil
}
