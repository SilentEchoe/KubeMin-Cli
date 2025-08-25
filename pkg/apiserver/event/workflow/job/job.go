package job

import (
	"KubeMin-Cli/pkg/apiserver/config"
	"KubeMin-Cli/pkg/apiserver/domain/model"
	"KubeMin-Cli/pkg/apiserver/infrastructure/datastore"
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
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
	if job == nil {
		klog.Errorf("initJobCtl job is nil")
		return nil
	}
	if client == nil {
		klog.Errorf("initJobCtl client is nil")
		return nil
	}

	var jobCtl JobCtl
	switch job.JobType {
	case string(config.JobDeploy):
		jobCtl = NewDeployJobCtl(job, client, store, ack)
	case string(config.JobDeployService):
		jobCtl = NewDeployServiceJobCtl(job, client, store, ack)
	case string(config.JobDeployStore):
		jobCtl = NewDeployStatefulSetJobCtl(job, client, store, ack)
	case string(config.JobDeployConfigMap):
		jobCtl = NewDeployConfigMapJobCtl(job, client, store, ack)
	case string(config.JobDeploySecret):
		jobCtl = NewDeploySecretJobCtl(job, client, store, ack)
	default:
		klog.Errorf("unknown job type: %s", job.JobType)
		return nil
	}
	return jobCtl
}

func RunJobs(ctx context.Context, jobs []*model.JobTask, concurrency int, client *kubernetes.Clientset, store datastore.DataStore, ack func()) {
	if len(jobs) == 0 {
		klog.Info("no jobs to run")
		return
	}

	if concurrency == 1 {
		for _, job := range jobs {
			klog.Info("Job started: ", job.Name, job.JobType)
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
	if job == nil {
		klog.Errorf("runJob received nil job")
		return
	}

	if job.Status == config.StatusPassed || job.Status == config.StatusSkipped {
		return
	}
	job.Status = config.StatusPrepare
	job.StartTime = time.Now().Unix()
	ack()

	if store == nil {
		klog.Errorf("start job store is nil")
		return
	}
	klog.Infof(fmt.Sprintf("start job: %s,status: %s", job.JobType, job.Status))
	jobCtl := initJobCtl(job, client, store, ack)
	if jobCtl == nil {
		errMsg := fmt.Sprintf("failed to initialize job controller for job: %s", job.Name)
		klog.Errorf(errMsg)
		job.Status = config.StatusFailed
		job.Error = errMsg
		job.EndTime = time.Now().Unix()
		ack()
		return
	}

	defer func() {
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
			klog.Errorf("update job info into db error: %v", err)
		}
	}()
	// 执行对应的JOb任务
	jobCtl.Run(ctx)
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

func ParseProperties(properties *model.JSONStruct) model.Properties {
	cProperties, err := json.Marshal(properties)
	if err != nil {
		klog.Errorf("Component.Properties deserialization failure: %s", err)
		return model.Properties{}
	}

	var propertied model.Properties
	err = json.Unmarshal(cProperties, &propertied)
	if err != nil {
		klog.Errorf("WorkflowSteps deserialization failure: %s", err)
		return model.Properties{}
	}
	return propertied
}
