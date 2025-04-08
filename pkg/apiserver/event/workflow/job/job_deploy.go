package job

import (
	"KubeMin-Cli/pkg/apiserver/config"
	"KubeMin-Cli/pkg/apiserver/domain/model"
	"KubeMin-Cli/pkg/apiserver/infrastructure/datastore"
	"context"
	"fmt"
	app "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	"sync"
	"time"
)

type DeployJobCtl struct {
	namespace string
	job       *model.JobTask
	client    *kubernetes.Clientset
	store     datastore.DataStore
	ack       func()
}

func NewDeployJobCtl(job *model.JobTask, client *kubernetes.Clientset, store datastore.DataStore, ack func()) *DeployJobCtl {
	return &DeployJobCtl{
		job:    job,
		client: client,
		store:  store,
		ack:    ack,
	}
}

func (c *DeployJobCtl) Clean(ctx context.Context) {}

// SaveInfo  创建Job的详情信息
func (c *DeployJobCtl) SaveInfo(ctx context.Context) error {
	var jobInfo model.JobInfo
	jobInfo.Type = c.job.JobType
	jobInfo.WorkflowId = c.job.WorkflowId
	jobInfo.ProductId = c.job.ProjectId
	jobInfo.AppId = c.job.AppId
	jobInfo.Status = string(c.job.Status)
	jobInfo.StartTime = c.job.StartTime
	jobInfo.EndTime = c.job.EndTime
	jobInfo.ServiceName = c.job.Name
	jobInfo.Info = c.job.JobInfo.(string)
	err := c.store.Add(ctx, &jobInfo)
	if err != nil {
		return err
	}
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
	timeout := time.After(time.Duration(c.timeout()) * time.Second)
	for {
		select {
		case <-timeout:
			newResources, err := getDeploymentStatus(c.client, c.namespace, c.job.Name)
			if err != nil || !newResources.Ready {
				msg := fmt.Sprintf("get resource owner info error: %v", err)
				klog.Errorf(msg)
				c.job.Status = config.StatusFailed
				return
			}
			return
		default:
			time.Sleep(2 * time.Second)
			newResources, err := getDeploymentStatus(c.client, c.namespace, c.job.Name)
			if err != nil {
				msg := fmt.Sprintf("get resource owner info error: %v", err)
				klog.Errorf(msg)
				c.job.Status = config.StatusFailed
			}
			if newResources.Ready {
				c.job.Status = config.StatusCompleted
				return
			}
		}
	}
}

func GetResourcesPodOwnerUID(kubeClient *kubernetes.Clientset, namespace string, name []string) ([]*model.JobDeployInfo, error) {
	var newResources []*model.JobDeployInfo
	var err error

	for {
		if len(newResources) > 0 || err != nil {
			break
		}
		select {
		case <-timeout:
			newResources, err = getDeploymentByName(kubeClient, namespace, name)
			break
		default:
			time.Sleep(2 * time.Second)
			newResources, err = getDeploymentByName(kubeClient, namespace, name)
			break
		}
	}
	return newResources, nil
}

func getDeploymentStatus(kubeClient *kubernetes.Clientset, namespace string, name string) (deployInfo *model.JobDeployInfo, err error) {
	deploy, err := kubeClient.AppsV1().Deployments(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	isOk := false
	if *deploy.Spec.Replicas == deploy.Status.ReadyReplicas {
		isOk = true
	}
	return &model.JobDeployInfo{
		Name:          deploy.Name,
		Replicas:      *deploy.Spec.Replicas,
		ReadyReplicas: deploy.Status.ReadyReplicas,
		Ready:         isOk}, nil
}

func (c *DeployJobCtl) timeout() int64 {
	if c.job.Timeout == 0 {
		c.job.Timeout = config.DeployTimeout
	}
	return c.job.Timeout
}
