package job

import (
	"KubeMin-Cli/pkg/apiserver/config"
	"KubeMin-Cli/pkg/apiserver/domain/model"
	"KubeMin-Cli/pkg/apiserver/infrastructure/datastore"
	"context"
	"fmt"
	v1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	"sync"
)

type DeployServiceJobCtl struct {
	namespace string
	job       *model.JobTask
	client    *kubernetes.Clientset
	store     datastore.DataStore
	ack       func()
}

func NewDeployServiceJobCtl(job *model.JobTask, client *kubernetes.Clientset, store datastore.DataStore, ack func()) *DeployJobCtl {
	return &DeployJobCtl{
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
		return
	}
	//这里是部署完毕后，将状态进行同步
	c.wait(ctx)
}

func (c *DeployServiceJobCtl) run(ctx context.Context) error {
	if c.client == nil {
		panic("client is nil")
	}

	var service *v1.Service
	if s, ok := c.job.JobInfo.(*v1.Service); ok {
		service = s
	} else {
		return fmt.Errorf("deploy Job Service Job.Info Conversion type failure")
	}

	isService, err := c.client.CoreV1().Services(c.namespace).Get(ctx, service.Name, metav1.GetOptions{})
	isAlreadyExists := false
	if isService != nil {
		isAlreadyExists = true
	}
	// 如果不存在,并且这个错误并不是没有找到对应的组件，那么证明查询有错误
	if !isAlreadyExists && k8serrors.IsNotFound(err) {
		klog.Errorf(err.Error())
		return err
	}

	if !isAlreadyExists {
		result, err := c.client.CoreV1().Services(c.namespace).Create(ctx, service, metav1.CreateOptions{})
		if err != nil {
			klog.Errorf(err.Error())
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

}
