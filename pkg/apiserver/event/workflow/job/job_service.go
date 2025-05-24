package job

import (
	"KubeMin-Cli/pkg/apiserver/config"
	"KubeMin-Cli/pkg/apiserver/domain/model"
	"KubeMin-Cli/pkg/apiserver/infrastructure/datastore"
	"context"
	"fmt"
	"sync"
	"time"

	v1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

type DeployServiceJobCtl struct {
	namespace string
	job       *model.JobTask
	client    *kubernetes.Clientset
	store     datastore.DataStore
	ack       func()
}

func NewDeployServiceJobCtl(job *model.JobTask, client *kubernetes.Clientset, store datastore.DataStore, ack func()) *DeployServiceJobCtl {
	return &DeployServiceJobCtl{
		job:    job,
		client: client,
		store:  store,
		ack:    ack,
	}
}

func (c *DeployServiceJobCtl) Clean(ctx context.Context) {}

// SaveInfo  创建Job的详情信息
func (c *DeployServiceJobCtl) SaveInfo(ctx context.Context) error {
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

func (c *DeployServiceJobCtl) Run(ctx context.Context) {
	c.job.Status = config.StatusRunning
	c.ack() // 通知工作流开始运行
	if err := c.run(ctx); err != nil {
		klog.Errorf("DeployServiceJob run error: %v", err)
		c.job.Status = config.StatusFailed
		c.ack()
		return
	}
	//这里是部署完毕后，将状态进行同步
	c.wait(ctx)
}

func (c *DeployServiceJobCtl) run(ctx context.Context) error {
	if c.client == nil {
		return fmt.Errorf("client is nil")
	}

	if c.job == nil || c.job.JobInfo == nil {
		return fmt.Errorf("job or job.JobInfo is nil")
	}

	service, ok := c.job.JobInfo.(*v1.Service)
	if !ok {
		return fmt.Errorf("job.JobInfo is not a *v1.Service (actual type: %T)", c.job.JobInfo)
	}

	isAlreadyExists, err := getServiceStatus(c.client, c.job.Namespace, c.job.Name)
	if err != nil {
		return fmt.Errorf("failed to check deployment existence: %w", err)
	}

	if !isAlreadyExists {
		result, err := c.client.CoreV1().Services(c.job.Namespace).Create(context.TODO(), service, metav1.CreateOptions{})
		if err != nil {
			klog.Errorf("failed to create service %q namespace: %q : %v", service.Name, service.Namespace, err)
			return err
		}
		klog.Infof("JobTask Deploy Service Successfully %q.\n", result.GetObjectMeta().GetName())
	}

	c.job.Status = config.StatusCompleted
	c.ack()
	return nil
}

func (c *DeployServiceJobCtl) updateServiceModuleImages(ctx context.Context) error {
	wg := sync.WaitGroup{}
	wg.Wait()
	return nil
}

func (c *DeployServiceJobCtl) timeout() int {
	if c.job.Timeout == 0 {
		c.job.Timeout = 60 * 10
	}
	return int(c.job.Timeout)
}

func (c *DeployServiceJobCtl) wait(ctx context.Context) {
	timeout := time.After(time.Duration(c.timeout()) * time.Second)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			klog.Warning("timed out waiting for service: %s", c.job.Name)
			c.job.Status = config.StatusFailed
			return
		case <-ticker.C:
			isExist, err := getServiceStatus(c.client, c.job.Namespace, c.job.Name)
			if err != nil {
				c.job.Status = config.StatusFailed
				return
			}
			if isExist {
				c.job.Status = config.StatusCompleted
				return
			}
		}
	}
}

func getServiceStatus(kubeClient *kubernetes.Clientset, namespace string, name string) (bool, error) {
	klog.Infof("Checking service: %s/%s", namespace, name)

	_, err := kubeClient.CoreV1().Services(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		if k8serrors.IsNotFound(err) {
			klog.Infof("service not found: %s/%s", namespace, name)
			return false, nil
		}
		klog.Error("check service error:%s", err)
		return false, err
	}

	return true, nil
}
