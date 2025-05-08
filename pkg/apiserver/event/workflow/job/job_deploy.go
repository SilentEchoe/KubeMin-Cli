package job

import (
	"KubeMin-Cli/pkg/apiserver/config"
	"KubeMin-Cli/pkg/apiserver/domain/model"
	"KubeMin-Cli/pkg/apiserver/infrastructure/datastore"
	"context"
	"fmt"
	app "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
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
	if client == nil || store == nil {
		return nil
	}
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
	jobInfo := model.JobInfo{
		Type:        c.job.JobType,
		WorkflowId:  c.job.WorkflowId,
		ProductId:   c.job.ProjectId,
		AppId:       c.job.AppId,
		Status:      string(c.job.Status),
		StartTime:   c.job.StartTime,
		EndTime:     c.job.EndTime,
		Error:       c.job.Error,
		ServiceName: c.job.Name,
	}
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

	// TODO 这里是防止重复创建，所以如果创建应该直接跳过，或者修改,之后可以根据策略来判断是否重新部署
	isDeploy, err := c.client.AppsV1().Deployments("default").Get(ctx, deploy.Name, metav1.GetOptions{})

	isAlreadyExists := false
	if isDeploy != nil {
		isAlreadyExists = true
	}

	// 如果不存在,并且这个错误并不是没有找到对应的组件，那么证明查询有错误
	if !isAlreadyExists && k8serrors.IsNotFound(err) {
		klog.Errorf(err.Error())
		return err
	}

	if !isAlreadyExists {
		result, err := c.client.AppsV1().Deployments("default").Create(ctx, deploy, metav1.CreateOptions{})
		if err != nil {
			klog.Errorf(err.Error())
			return err
		}
		klog.Infof("JobTask Deploy Successfully %q.\n", result.GetObjectMeta().GetName())
	}
	c.job.Status = config.StatusCompleted
	c.ack()
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
			klog.Infof(fmt.Sprintf("%s", c.job.Name))
			newResources, err := getDeploymentStatus(c.client, c.job.Namespace, c.job.Name)
			if err != nil || newResources == nil {
				msg := fmt.Sprintf("get resource owner info error: %v", err)
				klog.Errorf(msg)
				c.job.Status = config.StatusFailed
			}
		default:
			time.Sleep(2 * time.Second)
			newResources, err := getDeploymentStatus(c.client, c.job.Namespace, c.job.Name)
			if err != nil {
				msg := fmt.Sprintf("get resource owner info error: %v", err)
				klog.Errorf(msg)
				c.job.Status = config.StatusFailed
				return
			}
			if newResources != nil {
				klog.Infof(fmt.Sprintf("newResources:%s, Replicas:%d ,ReadyReplicas:%d ", newResources.Name, newResources.Replicas, newResources.ReadyReplicas))
				if newResources.Ready {
					c.job.Status = config.StatusCompleted
					return
				}
			}
		}
	}
}

func getDeploymentStatus(kubeClient *kubernetes.Clientset, namespace string, name string) (deployInfo *model.JobDeployInfo, err error) {
	klog.Infof("%s-%s", namespace, name)
	deploy, err := kubeClient.AppsV1().Deployments(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			// Deployment 不存在，处理这种情况
			klog.Infof("deploy is nil")
			return nil, nil
		}
		return nil, err
	}
	klog.Infof(fmt.Sprintf("newResources:%s, Replicas:%d ,ReadyReplicas:%d ", deploy.Name, deploy.Spec.Replicas, deploy.Status.ReadyReplicas))
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
