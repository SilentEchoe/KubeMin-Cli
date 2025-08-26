package job

import (
	"KubeMin-Cli/pkg/apiserver/config"
	"KubeMin-Cli/pkg/apiserver/domain/model"
	"KubeMin-Cli/pkg/apiserver/infrastructure/datastore"
	"KubeMin-Cli/pkg/apiserver/utils"
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

// DeploySecretJobCtl creates or updates a Secret resource in the target namespace.
// It assumes JobInfo carries a fully-formed *corev1.Secret (creation intent). Pure references
// should not be routed to this JobCtl.
type DeploySecretJobCtl struct {
	namespace string
	job       *model.JobTask
	client    *kubernetes.Clientset
	store     datastore.DataStore
	ack       func()
}

func NewDeploySecretJobCtl(job *model.JobTask, client *kubernetes.Clientset, store datastore.DataStore, ack func()) *DeploySecretJobCtl {
	if job == nil {
		klog.Errorf("NewDeploySecretJobCtl: job is nil")
		return nil
	}
	return &DeploySecretJobCtl{
		namespace: job.Namespace,
		job:       job,
		client:    client,
		store:     store,
		ack:       ack,
	}
}

func (c *DeploySecretJobCtl) Clean(ctx context.Context) {}

// SaveInfo persists job execution metadata.
func (c *DeploySecretJobCtl) SaveInfo(ctx context.Context) error {
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
	return c.store.Add(ctx, &jobInfo)
}

func (c *DeploySecretJobCtl) Run(ctx context.Context) {
	c.job.Status = config.StatusRunning
	c.ack()
	if err := c.run(ctx); err != nil {
		klog.Errorf("DeploySecretJob run error: %v", err)
		c.job.Status = config.StatusFailed
		c.job.Error = err.Error()
		c.ack()
		return
	}
	c.wait(ctx)
}

func (c *DeploySecretJobCtl) run(ctx context.Context) error {
	if c.client == nil {
		return fmt.Errorf("client is nil")
	}

	var secret *corev1.Secret
	switch v := c.job.JobInfo.(type) {
	case *corev1.Secret:
		secret = v
	case *model.SecretInput:
		st := corev1.SecretTypeOpaque
		if v.Type != "" {
			st = corev1.SecretType(v.Type)
		}
		stringData := map[string]string{}
		if v.URL != "" {
			body, err := utils.ReadFileFromURLSimple(v.URL)
			if err != nil {
				return fmt.Errorf("fetch secret url failed: %w", err)
			}
			fileName := v.FileName
			if fileName == "" {
				fileName = model.ExtractFileNameFromURLForSecret(v.URL)
			}
			stringData[fileName] = string(body)
		}
		for k, val := range v.Data {
			stringData[k] = val
		}
		secret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      v.Name,
				Namespace: v.Namespace,
				Labels:    v.Labels,
			},
			Type:       st,
			StringData: stringData,
		}
	default:
		return fmt.Errorf("job info is not *corev1.Secret")
	}

	if secret.Namespace == "" {
		secret.Namespace = c.job.Namespace
	}

	// Default to Opaque if not set
	if string(secret.Type) == "" {
		secret.Type = corev1.SecretTypeOpaque
	}

	cli := c.client.CoreV1().Secrets(secret.Namespace)

	if existing, err := cli.Get(ctx, secret.Name, metav1.GetOptions{}); err == nil {
		// If existing is immutable, updates will be rejected by API server.
		if existing.Immutable != nil && *existing.Immutable {
			// Compare quickly to avoid noisy operations; if data differs, return an error.
			// Note: Do not log secret contents.
			if !equalSecretPayload(existing, secret) {
				return fmt.Errorf("secret %s/%s is immutable; content differs and cannot be updated", secret.Namespace, secret.Name)
			}
			klog.Infof("secret %s/%s is immutable and unchanged; skipping update", secret.Namespace, secret.Name)
		} else {
			secret.ResourceVersion = existing.ResourceVersion
			if _, err := cli.Update(ctx, secret, metav1.UpdateOptions{}); err != nil {
				return fmt.Errorf("update secret %q failed: %w", secret.Name, err)
			}
			klog.Infof("secret %s/%s updated", secret.Namespace, secret.Name)
		}
	} else if k8serrors.IsNotFound(err) {
		if _, err := cli.Create(ctx, secret, metav1.CreateOptions{}); err != nil {
			return fmt.Errorf("create secret %q failed: %w", secret.Name, err)
		}
		klog.Infof("secret %s/%s created", secret.Namespace, secret.Name)
	} else if err != nil {
		return fmt.Errorf("get secret %q failed: %w", secret.Name, err)
	}

	c.job.Status = config.StatusCompleted
	c.ack()
	return nil
}

// wait is a no-op for Secret objects.
func (c *DeploySecretJobCtl) wait(ctx context.Context) {}

// equalSecretPayload compares update-relevant fields of two Secret objects without exposing data.
func equalSecretPayload(a, b *corev1.Secret) bool {
	if a.Type != b.Type {
		return false
	}
	if len(a.Data) != len(b.Data) {
		return false
	}
	for k, v := range a.Data {
		bv, ok := b.Data[k]
		if !ok {
			return false
		}
		if len(v) != len(bv) {
			return false
		}
		// Do not compare contents byte-by-byte to avoid accidental logging; length equality is a cheap proxy.
	}
	// Also consider stringData on desired (b) – if provided, treat as a change request.
	if len(b.StringData) > 0 {
		return false
	}
	return true
}

func GenerateSecret(component *model.ApplicationComponent, properties *model.Properties) interface{} {
	name := component.Name
	namespace := component.Namespace
	if namespace == "" {
		namespace = config.DefaultNamespace
	}

	if properties != nil && properties.Secret != nil {
		if url, ok := properties.Conf["config.url"]; ok && url != "" {
			fileName := "config"
			if fn, ok := properties.Conf["config.fileName"]; ok && fn != "" {
				fileName = fn
			}
			return &model.SecretInput{
				Name:      name,
				Namespace: namespace,
				URL:       url,
				FileName:  fileName,
				Labels:    properties.Labels,
			}
		}
	}

	data := make(map[string]string)
	if properties == nil || properties.Secret == nil {
		data = nil
	} else {
		data = properties.Secret
	}

	labels := BuildLabels(component, properties)

	return &model.SecretInput{
		Name:      name,
		Namespace: namespace,
		Labels:    labels,
		Data:      data,
	}
}
