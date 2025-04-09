package job

import (
	"KubeMin-Cli/pkg/apiserver/config"
	"KubeMin-Cli/pkg/apiserver/domain/model"
	"KubeMin-Cli/pkg/apiserver/infrastructure/datastore"
	"context"
	"fmt"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	"sync"
	"time"
)

type JobCtl interface {
	Run(ctx context.Context)
	Clean(ctx context.Context)
	SaveInfo(ctx context.Context) error
}

func initJobCtl(job *model.JobTask, client *kubernetes.Clientset, store datastore.DataStore, ack func()) JobCtl {
	if store == nil {
		klog.Errorf("initJobCtl store is nil")
		return nil
	}

	var jobCtl JobCtl
	switch job.JobType {
	case string(config.JobDeploy):
		jobCtl = NewDeployJobCtl(job, client, store, ack)
	case string(config.JobDeployService):
		jobCtl = NewDeployServiceJobCtl(job, client, ack)
	}
	return jobCtl
}

func RunJobs(ctx context.Context, jobs []*model.JobTask, concurrency int, client *kubernetes.Clientset, store datastore.DataStore, ack func()) {
	if concurrency == 1 {
		for _, job := range jobs {
			runJob(ctx, job, client, store, ack)
			if jobStatusFailed(job.Status) {
				return
			}
		}
		return
	}
	jobPool := NewPool(ctx, jobs, concurrency, client, ack)
	jobPool.Run()
}

func runJob(ctx context.Context, job *model.JobTask, client *kubernetes.Clientset, store datastore.DataStore, ack func()) {
	if job.Status == config.StatusPassed || job.Status == config.StatusSkipped {
		return
	}
	job.Status = config.StatusPrepare
	job.StartTime = time.Now().Unix()
	ack()

	if store == nil {
		klog.Errorf(fmt.Sprintf("start job store is nil"))
		return
	}

	klog.Infof(fmt.Sprintf("start job: %s,status: %s", job.JobType, job.Status))
	jobCtl := initJobCtl(job, client, store, ack)
	defer func(jobInfo *JobCtl) {
		if err := recover(); err != nil {
			errMsg := fmt.Sprintf("job: %s panic: %v", job.Name, err)
			klog.Errorf(errMsg)
			job.Status = config.StatusFailed
			job.Error = errMsg
		}
		job.EndTime = time.Now().Unix()
		klog.Infof("finish job: %s,status: %s", job.Name, job.Status)
		ack()
		klog.Infof("updating job info into db...")
		err := jobCtl.SaveInfo(ctx)
		if err != nil {
			klog.Errorf("update job info: %s into db error: %v", err)
		}
	}(&jobCtl)

	// 执行对应的JOb任务
	jobCtl.Run(ctx)

	//如果任务执行失败，则需要根据错误处理的策略进行处理
}

func jobStatusFailed(status config.Status) bool {
	if status == config.StatusCancelled || status == config.StatusFailed || status == config.StatusTimeout || status == config.StatusReject {
		return true
	}
	return false
}

type Pool struct {
	Jobs        []*model.JobTask
	concurrency int
	client      *kubernetes.Clientset
	store       datastore.DataStore
	jobsChan    chan *model.JobTask
	ack         func()
	ctx         context.Context
	wg          sync.WaitGroup
}

func (p *Pool) Run() {
	for i := 0; i < p.concurrency; i++ {
		go p.work()
	}
	p.wg.Add(len(p.Jobs))
	for _, task := range p.Jobs {
		p.jobsChan <- task
	}
	// all workers return
	close(p.jobsChan)
	p.wg.Wait()
}

// The work loop for any single goroutine.
func (p *Pool) work() {
	for job := range p.jobsChan {
		runJob(p.ctx, job, p.client, p.store, p.ack)
		p.wg.Done()
	}
}

// NewPool initializes a new pool with the given tasks and
// at the given concurrency.
func NewPool(ctx context.Context, jobs []*model.JobTask, concurrency int, client *kubernetes.Clientset, ack func()) *Pool {
	return &Pool{
		Jobs:        jobs,
		client:      client,
		concurrency: concurrency,
		jobsChan:    make(chan *model.JobTask),
		ack:         ack,
		ctx:         ctx,
	}
}
