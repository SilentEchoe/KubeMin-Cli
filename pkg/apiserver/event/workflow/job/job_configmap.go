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
		klog.Errorf("DeployConfigMapJobCtl: job is nil")
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
	c.wait(ctx)
}

func (c *DeployConfigMapJobCtl) run(ctx context.Context) error {
	if c.client == nil {
		return fmt.Errorf("client is nil")
	}

	// Compatible with two types of input parameters：ConfigMapInput、corev1.ConfigMap
	var cm *corev1.ConfigMap
	switch v := c.job.JobInfo.(type) {
	case *model.ConfigMapInput:
		conf, err := v.GenerateConf()
		if err != nil {
			return fmt.Errorf("invalid ConfigMap spec: %w", err)
		}
		cm = &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      conf.Name,
				Namespace: conf.Namespace,
				Labels:    conf.Labels,
			},
			Data: conf.Data,
		}
	case *corev1.ConfigMap:
		return c.deployExistingConfigMap(ctx, v)
	default:
		return fmt.Errorf("unsupported configmap jobInfo type: %T", c.job.JobInfo)
	}

	if cm.Namespace == "" {
		cm.Namespace = c.job.Namespace
	}
	return c.deployConfigMap(ctx, cm)
}

func (c *DeployConfigMapJobCtl) deployExistingConfigMap(ctx context.Context, cm *corev1.ConfigMap) error {
	if cm.Namespace == "" {
		cm.Namespace = c.job.Namespace
	}
	return c.deployConfigMap(ctx, cm)
}

func (c *DeployConfigMapJobCtl) deployConfigMap(ctx context.Context, cm *corev1.ConfigMap) error {
	cli := c.client.CoreV1().ConfigMaps(cm.Namespace)
	// Update if exists, create if not.
	if existing, err := cli.Get(ctx, cm.Name, metav1.GetOptions{}); err == nil {
		// When updating, the ResourceVersion needs to be carried.
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

func (c *DeployConfigMapJobCtl) wait(ctx context.Context) {}

// GenerateConfigMap Generate a simplified ConfigMap input based on components and attributes.
// First, read the external file URL from Conf["config.url"]; otherwise, directly use the content in Conf as the content of ConfigMap.
func GenerateConfigMap(component *model.ApplicationComponent, properties *model.Properties) interface{} {
	name := component.Name
	namespace := component.Namespace
	if namespace == "" {
		namespace = config.DefaultNamespace
	}

	// 优先 URL
	if properties != nil && properties.Conf != nil {
		if url, ok := properties.Conf["config.url"]; ok && url != "" {
			fileName := "config"
			if fn, ok := properties.Conf["config.fileName"]; ok && fn != "" {
				fileName = fn
			}
			return &model.ConfigMapInput{
				Name:      name,
				Namespace: namespace,
				URL:       url,
				FileName:  fileName,
				Labels:    properties.Labels,
			}
		}
	}

	data := make(map[string]string)
	if properties == nil || properties.Conf == nil {
		data = nil
	} else {
		data = properties.Conf
	}

	return &model.ConfigMapInput{
		Name:      name,
		Namespace: namespace,
		Labels:    nil,
		Data:      data,
	}
}
