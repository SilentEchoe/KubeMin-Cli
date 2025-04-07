package job

import (
	"KubeMin-Cli/pkg/apiserver/config"
	"KubeMin-Cli/pkg/apiserver/domain/model"
	"context"
	"fmt"
	app "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	"runtime/debug"
	"sync"
	"time"
)

type DeployJobCtl struct {
	namespace string
	job       *model.JobTask
	client    *kubernetes.Clientset
	ack       func()
}

func NewDeployJobCtl(job *model.JobTask, client *kubernetes.Clientset, ack func()) *DeployJobCtl {
	return &DeployJobCtl{
		job:    job,
		client: client,
		ack:    ack,
	}
}

func runJob(ctx context.Context, job *model.JobTask, client *kubernetes.Clientset, ack func()) {
	// 如果Job的状态为暂停或者跳过，则直接返回
	if job.Status == config.StatusPassed || job.Status == config.StatusSkipped {
		return
	}
	job.Status = config.StatusPrepare
	job.StartTime = time.Now().Unix()
	ack()

	klog.Infof(fmt.Sprintf("start job: %s,status: %s", job.JobType, job.Status))
	jobCtl := initJobCtl(job, client, ack)
	defer func(jobInfo *JobCtl) {
		if err := recover(); err != nil {
			errMsg := fmt.Sprintf("job: %s panic: %v", job.Name, err)
			klog.Errorf(errMsg)
			debug.PrintStack()
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
}

func (c *DeployJobCtl) Clean(ctx context.Context) {}

// SaveInfo  创建Job的详情信息
func (c *DeployJobCtl) SaveInfo(ctx context.Context) error {
	return nil
}

func (c *DeployJobCtl) Run(ctx context.Context) {
	c.job.Status = config.StatusRunning
	c.ack() // 通知工作流开始运行
	if err := c.run(ctx); err != nil {
		return
	}
	//这里是部署完毕后，将状态进行同步
	c.wait(ctx)
}

func (c *DeployJobCtl) run(ctx context.Context) error {
	if c.client == nil {
		panic("client is nil")
	}

	var deploy *app.Deployment
	if d, ok := c.job.JobInfo.(*app.Deployment); ok {
		deploy = d
	} else {
		return fmt.Errorf("deploy Job Job.Info Conversion type failure")
	}

	result, err := c.client.AppsV1().Deployments("default").Create(ctx, deploy, metav1.CreateOptions{})
	if err != nil {
		klog.Errorf(err.Error())
		return err
	}
	klog.Infof("JobTask Deploy Successfully %q.\n", result.GetObjectMeta().GetName())

	// 将这个任务标记为已完成
	c.job.Status = config.StatusCompleted
	c.ack()

	// TODO 这里可能需要记录这个Job

	return nil
}

func (c *DeployJobCtl) updateServiceModuleImages(ctx context.Context) error {
	wg := sync.WaitGroup{}
	wg.Wait()
	return nil
}

func (c *DeployJobCtl) wait(ctx context.Context) {
	//timeout := time.After(60 * time.Second)

	// TODO 从k8s元数据中获取PodOwnerUID
	//resources, err := GetResourcesPodOwnerUID(c.kubeClient, c.namespace, c.jobTaskSpec.ServiceAndImages, c.jobTaskSpec.DeployContents, c.jobTaskSpec.ReplaceResources)
	//if err != nil {
	//	msg := fmt.Sprintf("get resource owner info error: %v", err)
	//	logError(c.job, msg, c.logger)
	//	return
	//}
	//c.jobTaskSpec.ReplaceResources = resources
	// 判断状态
	//status, err := CheckDeployStatus(ctx, c.kubeClient, c.namespace, c.jobTaskSpec, timeout, c.logger)
	//if err != nil {
	//	logError(c.job, err.Error(), c.logger)
	//	return
	//}
	//c.job.Status = status
}
