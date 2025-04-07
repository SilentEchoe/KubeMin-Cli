package job

import (
	"KubeMin-Cli/pkg/apiserver/config"
	"KubeMin-Cli/pkg/apiserver/domain/model"
	"context"
	"sync"
)

type JobCtl interface {
	Run(ctx context.Context)
	Clean(ctx context.Context)
	SaveInfo(ctx context.Context) error
}

func initJobCtl(job *model.JobTask, ack func()) JobCtl {
	var jobCtl JobCtl
	switch job.JobType {
	case string(config.JobDeploy):
		jobCtl = NewDeployJobCtl(job, ack)
	}
	return jobCtl
}

type Pool struct {
	Jobs        []*model.JobTask
	concurrency int
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
		runJob(p.ctx, job, p.ack)
		p.wg.Done()
	}
}

func RunJobs(ctx context.Context, jobs []*model.JobTask, workflow *model.WorkflowQueue, concurrency int, ack func()) {
	if concurrency == 1 {
		for _, job := range jobs {
			runJob(ctx, job, ack)
			if jobStatusFailed(job.Status) {
				return
			}
		}
		return
	}
	jobPool := NewPool(ctx, jobs, concurrency, ack)
	jobPool.Run()
}

func jobStatusFailed(status config.Status) bool {
	if status == config.StatusCancelled || status == config.StatusFailed || status == config.StatusTimeout || status == config.StatusReject {
		return true
	}
	return false
}

// NewPool initializes a new pool with the given tasks and
// at the given concurrency.
func NewPool(ctx context.Context, jobs []*model.JobTask, concurrency int, ack func()) *Pool {
	return &Pool{
		Jobs:        jobs,
		concurrency: concurrency,
		jobsChan:    make(chan *model.JobTask),
		ack:         ack,
		ctx:         ctx,
	}
}
