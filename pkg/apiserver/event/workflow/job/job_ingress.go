package job

import (
	"context"
	"fmt"
	"time"

	"KubeMin-Cli/pkg/apiserver/config"
	"KubeMin-Cli/pkg/apiserver/domain/model"
	"KubeMin-Cli/pkg/apiserver/infrastructure/datastore"

	networkingv1 "k8s.io/api/networking/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

type DeployIngressJobCtl struct {
	namespace string
	job       *model.JobTask
	client    kubernetes.Interface
	store     datastore.DataStore
	ack       func()
}

func NewDeployIngressJobCtl(job *model.JobTask, client kubernetes.Interface, store datastore.DataStore, ack func()) *DeployIngressJobCtl {
	if job == nil {
		klog.Errorf("DeployIngressJobCtl: job is nil")
		return nil
	}
	return &DeployIngressJobCtl{
		namespace: job.Namespace,
		job:       job,
		client:    client,
		store:     store,
		ack:       ack,
	}
}

func (c *DeployIngressJobCtl) Clean(ctx context.Context) {
	if c.client == nil {
		return
	}
	refs := resourcesForCleanup(ctx, config.ResourceIngress)
	if len(refs) == 0 {
		return
	}
	cleanupCtx, cancel := context.WithTimeout(context.Background(), config.DelTimeOut)
	defer cancel()
	for _, ref := range refs {
		if !ref.Created {
			continue
		}
		ns := ref.Namespace
		if ns == "" {
			ns = c.namespace
		}
		if err := c.client.NetworkingV1().Ingresses(ns).Delete(cleanupCtx, ref.Name, metav1.DeleteOptions{}); err != nil {
			if !k8serrors.IsNotFound(err) {
				klog.Errorf("failed to delete ingress %s/%s during cleanup: %v", ns, ref.Name, err)
			}
		} else {
			klog.Infof("deleted ingress %s/%s after job failure", ns, ref.Name)
		}
	}
}

// SaveInfo 创建Job的详情信息
func (c *DeployIngressJobCtl) SaveInfo(ctx context.Context) error {
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
	return c.store.Add(ctx, &jobInfo)
}

func (c *DeployIngressJobCtl) Run(ctx context.Context) error {
	c.job.Status = config.StatusRunning
	c.job.Error = ""
	c.ack()

	if err := c.run(ctx); err != nil {
		klog.Errorf("DeployIngressJob run error: %v", err)
		c.job.Error = err.Error()
		if statusErr, ok := ExtractStatusError(err); ok {
			c.job.Status = statusErr.Status
		} else {
			c.job.Status = config.StatusFailed
		}
		return err
	}

	if c.job.Status == config.StatusSkipped {
		c.job.Error = ""
		return nil
	}

	if err := c.wait(ctx); err != nil {
		c.job.Error = err.Error()
		if statusErr, ok := ExtractStatusError(err); ok {
			c.job.Status = statusErr.Status
		} else {
			c.job.Status = config.StatusFailed
		}
		return err
	}

	c.job.Status = config.StatusCompleted
	c.job.Error = ""
	return nil
}

func (c *DeployIngressJobCtl) run(ctx context.Context) error {
	if c.client == nil {
		return fmt.Errorf("client is nil")
	}

	ingress, ok := c.job.JobInfo.(*networkingv1.Ingress)
	if !ok {
		return fmt.Errorf("deploy job Job.Info conversion type failure (actual: %T)", c.job.JobInfo)
	}
	if ingress == nil {
		return fmt.Errorf("ingress job info is nil")
	}
	if ingress.Namespace == "" {
		ingress.Namespace = c.namespace
	}

	shareName, shareStrategy := shareInfoFromLabels(ingress.Labels)
	unlock, skipped, err := resolveSharedResource(ctx, shareName, shareStrategy, config.ResourceIngress, func(ctx context.Context, opts metav1.ListOptions) (int, error) {
		list, err := c.client.NetworkingV1().Ingresses(ingress.Namespace).List(ctx, opts)
		if err != nil {
			return 0, err
		}
		return len(list.Items), nil
	})
	if err != nil {
		return fmt.Errorf("resolve shared ingress failed: %w", err)
	}
	if unlock != nil {
		defer unlock()
	}
	if skipped {
		if shareStrategy == config.ShareStrategyIgnore {
			klog.Infof("Ingress %s/%s marked as shared ignore; skipping", ingress.Namespace, ingress.Name)
		} else {
			klog.Infof("Ingress %s/%s already exists and is shared; skipping", ingress.Namespace, ingress.Name)
		}
		c.job.Status = config.StatusSkipped
		c.job.Error = ""
		c.ack()
		return nil
	}

	existing, err := c.client.NetworkingV1().Ingresses(ingress.Namespace).Get(ctx, ingress.Name, metav1.GetOptions{})
	switch {
	case err == nil:
		ingress.ResourceVersion = existing.ResourceVersion
		if ingress.Labels == nil && len(existing.Labels) > 0 {
			ingress.Labels = existing.Labels
		}
		if _, err := c.client.NetworkingV1().Ingresses(ingress.Namespace).Update(ctx, ingress, metav1.UpdateOptions{}); err != nil {
			return fmt.Errorf("update ingress %s/%s failed: %w", ingress.Namespace, ingress.Name, err)
		}
		markResourceObserved(ctx, config.ResourceIngress, ingress.Namespace, ingress.Name)
		klog.Infof("Ingress %s/%s updated successfully", ingress.Namespace, ingress.Name)
	case k8serrors.IsNotFound(err):
		result, err := c.client.NetworkingV1().Ingresses(ingress.Namespace).Create(ctx, ingress, metav1.CreateOptions{})
		if err != nil {
			klog.Errorf("failed to create ingress %q namespace: %q: %v", ingress.Name, ingress.Namespace, err)
			return err
		}
		MarkResourceCreated(ctx, config.ResourceIngress, ingress.Namespace, ingress.Name)
		klog.Infof("Ingress %q created successfully", result.GetObjectMeta().GetName())
	default:
		return fmt.Errorf("get ingress %s/%s failed: %w", ingress.Namespace, ingress.Name, err)
	}

	return nil
}

func (c *DeployIngressJobCtl) wait(ctx context.Context) error {
	timeout := time.After(time.Duration(c.timeout()) * time.Second)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	getIngressName := func() string {
		if ingressObj, ok := c.job.JobInfo.(*networkingv1.Ingress); ok && ingressObj != nil && ingressObj.Name != "" {
			return ingressObj.Name
		}
		return c.job.Name
	}

	for {
		select {
		case <-ctx.Done():
			name := getIngressName()
			return NewStatusError(config.StatusCancelled, fmt.Errorf("ingress %s cancelled: %w", name, ctx.Err()))
		case <-timeout:
			name := getIngressName()
			return NewStatusError(config.StatusTimeout, fmt.Errorf("wait ingress %s timeout", name))
		case <-ticker.C:
			ingressName := getIngressName()
			ing, err := c.client.NetworkingV1().Ingresses(c.job.Namespace).Get(ctx, ingressName, metav1.GetOptions{})
			if err != nil {
				if k8serrors.IsNotFound(err) {
					continue
				}
				return fmt.Errorf("wait ingress %s error: %w", ingressName, err)
			}
			if ingressReady(ing) {
				return nil
			}
		}
	}
}

func (c *DeployIngressJobCtl) timeout() int64 {
	if c.job.Timeout == 0 {
		c.job.Timeout = config.DeployTimeout
	}
	return c.job.Timeout
}

func ingressReady(ing *networkingv1.Ingress) bool {
	if ing == nil {
		return false
	}
	return true
}
