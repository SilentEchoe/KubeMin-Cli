package job

import (
	"KubeMin-Cli/pkg/apiserver/config"
	"KubeMin-Cli/pkg/apiserver/domain/model"
	"context"
	"fmt"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	"runtime/debug"
	"sync"
	"time"
)

type DeployJobCtl struct {
	job       *model.JobTask
	namespace string
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
	//var (
	//	err error
	//)
	// TODO 从数据库中获取环境

	// TODO Step.1 创建一个ControllerRuntimeClient
	//c.kubeClient, err = clientmanager.NewKubeClientManager().GetControllerRuntimeClient(c.jobTaskSpec.ClusterID)

	if c.client == nil {
		panic("client is nil")
	}

	pods, err := c.client.CoreV1().Pods("").List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		panic(err.Error())
	}
	for _, pod := range pods.Items {
		fmt.Printf("- %s (Status: %s)\n", pod.Name, pod.Status.Phase)
	}

	// TODO Step.2 获取KubeClient

	// TODO Step.3  创建一个informer

	// TODO Step.4 创建istio客户端连接

	// TODO Step.5 根据Job的类型生成需要部署或更新的元数据

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
