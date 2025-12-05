package job

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	"KubeMin-Cli/pkg/apiserver/config"
	"KubeMin-Cli/pkg/apiserver/domain/model"
	"KubeMin-Cli/pkg/apiserver/infrastructure/datastore"
)

type DeployConfigMapJobCtl struct {
	namespace string
	job       *model.JobTask
	client    kubernetes.Interface
	store     datastore.DataStore
	ack       func()
}

func NewDeployConfigMapJobCtl(job *model.JobTask, client kubernetes.Interface, store datastore.DataStore, ack func()) *DeployConfigMapJobCtl {
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

func (c *DeployConfigMapJobCtl) Clean(ctx context.Context) {
	if c.client == nil {
		return
	}
	refs := resourcesForCleanup(ctx, config.ResourceConfigMap)
	if len(refs) == 0 {
		return
	}
	cleanupCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	for _, ref := range refs {
		if !ref.Created {
			continue
		}
		ns := ref.Namespace
		if ns == "" {
			ns = c.namespace
		}
		if err := c.client.CoreV1().ConfigMaps(ns).Delete(cleanupCtx, ref.Name, metav1.DeleteOptions{}); err != nil {
			if !k8serrors.IsNotFound(err) {
				klog.Errorf("failed to delete configmap %s/%s during cleanup: %v", ns, ref.Name, err)
			}
		} else {
			klog.Infof("deleted configmap %s/%s after job failure", ns, ref.Name)
		}
	}
}

func (c *DeployConfigMapJobCtl) SaveInfo(ctx context.Context) error {
	jobInfo := model.JobInfo{
		Type:        c.job.JobType,
		WorkflowID:  c.job.WorkflowID,
		ProductID:   c.job.ProjectID,
		AppID:       c.job.AppID,
		TaskID:      c.job.TaskID,
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

func (c *DeployConfigMapJobCtl) Run(ctx context.Context) error {
	logger := klog.FromContext(ctx)
	c.job.Status = config.StatusRunning
	c.job.Error = ""
	c.ack()
	if err := c.run(ctx); err != nil {
		logger.Error(err, "DeployConfigMapJob run error")
		c.job.Status = config.StatusFailed
		c.job.Error = err.Error()
		return err
	}
	c.job.Status = config.StatusCompleted
	c.job.Error = ""
	return nil
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
	logger := klog.FromContext(ctx)
	cli := c.client.CoreV1().ConfigMaps(cm.Namespace)
	// Update if exists, create if not.
	if existing, err := cli.Get(ctx, cm.Name, metav1.GetOptions{}); err == nil {
		// When updating, the ResourceVersion needs to be carried.
		cm.ResourceVersion = existing.ResourceVersion
		if _, err := cli.Update(ctx, cm, metav1.UpdateOptions{}); err != nil {
			return fmt.Errorf("update configmap %q failed: %w", cm.Name, err)
		}
		logger.Info("ConfigMap updated", "namespace", cm.Namespace, "name", cm.Name)
		markResourceObserved(ctx, config.ResourceConfigMap, cm.Namespace, cm.Name)
	} else if k8serrors.IsNotFound(err) {
		if _, err := cli.Create(ctx, cm, metav1.CreateOptions{}); err != nil {
			return fmt.Errorf("create configmap %q failed: %w", cm.Name, err)
		}
		logger.Info("ConfigMap created", "namespace", cm.Namespace, "name", cm.Name)
		MarkResourceCreated(ctx, config.ResourceConfigMap, cm.Namespace, cm.Name)
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

	labels := BuildLabels(component, properties)

	return &model.ConfigMapInput{
		Name:      name,
		Namespace: namespace,
		Labels:    labels,
		Data:      data,
	}
}
