package job

import (
	"KubeMin-Cli/pkg/apiserver/config"
	"KubeMin-Cli/pkg/apiserver/domain/model"
	"KubeMin-Cli/pkg/apiserver/infrastructure/datastore"
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

type DeployConfigMapJobCtl struct {
	namespace string
	job       *model.JobTask
	client    *kubernetes.Clientset
	store     datastore.DataStore
	ack       func()
}

func NewDeployConfigMapJobCtl(job *model.JobTask, client *kubernetes.Clientset, store datastore.DataStore, ack func()) *DeployConfigMapJobCtl {
	if job == nil {
		klog.Errorf("DeployStatefulSetJobCtl: job is nil")
		return nil
	}
	return &DeployConfigMapJobCtl{
		namespace: job.Namespace,
		job:       job,
		client:    client,
		store:     store,
		ack:       ack,
	}
}

func (c *DeployConfigMapJobCtl) Clean(ctx context.Context) {}

func (c *DeployConfigMapJobCtl) SaveInfo(ctx context.Context) error {
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

func (c *DeployConfigMapJobCtl) Run(ctx context.Context) {
	c.job.Status = config.StatusRunning
	c.ack() // 通知工作流开始运行
	if err := c.run(ctx); err != nil {
		klog.Errorf("DeployConfigMapJob run error: %v", err)
		c.job.Status = config.StatusFailed
		c.ack()
		return
	}
	//after the deployment is completed, synchronize the status.
	c.wait(ctx)
}

func (c *DeployConfigMapJobCtl) run(ctx context.Context) error {
	if c.client == nil {
		return fmt.Errorf("client is nil")
	}

	// 从 JobInfo 中获取 ConfigMap 对象
	cm, ok := c.job.JobInfo.(*corev1.ConfigMap)
	if !ok || cm == nil {
		return fmt.Errorf("job info is not *corev1.ConfigMap")
	}

	// 如果未设置命名空间，使用 job 的命名空间
	if cm.Namespace == "" {
		cm.Namespace = c.job.Namespace
	}

	cli := c.client.CoreV1().ConfigMaps(cm.Namespace)

	// 存在则更新，不存在则创建
	if existing, err := cli.Get(ctx, cm.Name, metav1.GetOptions{}); err == nil {
		// 更新时需要携带 ResourceVersion
		cm.ResourceVersion = existing.ResourceVersion
		if _, err := cli.Update(ctx, cm, metav1.UpdateOptions{}); err != nil {
			return fmt.Errorf("update configmap %q failed: %w", cm.Name, err)
		}
		klog.Infof("configmap %s/%s updated", cm.Namespace, cm.Name)
	} else if k8serrors.IsNotFound(err) {
		if _, err := cli.Create(ctx, cm, metav1.CreateOptions{}); err != nil {
			return fmt.Errorf("create configmap %q failed: %w", cm.Name, err)
		}
		klog.Infof("configmap %s/%s created", cm.Namespace, cm.Name)
	} else if err != nil {
		return fmt.Errorf("get configmap %q failed: %w", cm.Name, err)
	}

	c.job.Status = config.StatusCompleted
	c.ack()
	return nil
}

// ConfigMap 无需就绪等待，这里留空以对齐 JobCtl 接口
func (c *DeployConfigMapJobCtl) wait(ctx context.Context) {}
