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

	"kubemin-cli/pkg/apiserver/config"
	"kubemin-cli/pkg/apiserver/domain/model"
	"kubemin-cli/pkg/apiserver/infrastructure/datastore"
)

type DeployPVCJobCtl struct {
	namespace string
	job       *model.JobTask
	client    kubernetes.Interface
	store     datastore.DataStore
	ack       func()
}

func NewDeployPVCJobCtl(job *model.JobTask, client kubernetes.Interface, store datastore.DataStore, ack func()) *DeployPVCJobCtl {
	if job == nil {
		klog.Errorf("NewDeployPVCJobCtl: job is nil")
		return nil
	}
	return &DeployPVCJobCtl{
		namespace: job.Namespace,
		job:       job,
		client:    client,
		store:     store,
		ack:       ack,
	}
}

func (c *DeployPVCJobCtl) Clean(ctx context.Context) {
	if c.client == nil {
		return
	}
	refs := resourcesForCleanup(ctx, config.ResourcePVC)
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
			ns = c.job.Namespace
		}
		if err := c.client.CoreV1().PersistentVolumeClaims(ns).Delete(cleanupCtx, ref.Name, metav1.DeleteOptions{}); err != nil {
			if !k8serrors.IsNotFound(err) {
				klog.Errorf("failed to delete pvc %s/%s during cleanup: %v", ns, ref.Name, err)
			}
		} else {
			klog.Infof("deleted pvc %s/%s after job failure", ns, ref.Name)
		}
	}
}

func (c *DeployPVCJobCtl) SaveInfo(ctx context.Context) error {
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

func (c *DeployPVCJobCtl) Run(ctx context.Context) error {
	logger := klog.FromContext(ctx)
	c.job.Status = config.StatusRunning
	c.job.Error = ""
	c.ack()

	if err := c.run(ctx); err != nil {
		logger.Error(err, "deploy pvc job run error")
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
		logger.Error(err, "deploy pvc wait error")
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

func (c *DeployPVCJobCtl) run(ctx context.Context) error {
	logger := klog.FromContext(ctx)
	if c.client == nil {
		return fmt.Errorf("client is nil")
	}

	var pvc *corev1.PersistentVolumeClaim
	if p, ok := c.job.JobInfo.(*corev1.PersistentVolumeClaim); ok {
		pvc = p
	} else {
		return fmt.Errorf("deploy pvc job info conversion type failure")
	}

	// 检查PVC是否已存在
	shareName, shareStrategy := shareInfoFromLabels(pvc.Labels)
	unlock, skipped, err := resolveSharedResource(ctx, shareName, shareStrategy, config.ResourcePVC, func(ctx context.Context, opts metav1.ListOptions) (int, error) {
		list, err := c.client.CoreV1().PersistentVolumeClaims(pvc.Namespace).List(ctx, opts)
		if err != nil {
			return 0, err
		}
		return len(list.Items), nil
	})
	if err != nil {
		return fmt.Errorf("resolve shared pvc failed: %w", err)
	}
	if unlock != nil {
		defer unlock()
	}
	if skipped {
		if shareStrategy == config.ShareStrategyIgnore {
			logger.Info("pvc marked as shared ignore; skipping", "pvcName", pvc.Name)
		} else {
			logger.Info("pvc already exists and is shared; skipping", "pvcName", pvc.Name)
		}
		c.job.Status = config.StatusSkipped
		c.job.Error = ""
		c.ack()
		return nil
	}

	existingPVC, err := c.client.CoreV1().PersistentVolumeClaims(pvc.Namespace).Get(ctx, pvc.Name, metav1.GetOptions{})
	if err == nil {
		// PVC已存在，检查是否需要更新
		if c.shouldUpdatePVC(existingPVC, pvc) {
			pvc.ResourceVersion = existingPVC.ResourceVersion
			_, err = c.client.CoreV1().PersistentVolumeClaims(pvc.Namespace).Update(ctx, pvc, metav1.UpdateOptions{})
			if err != nil {
				return fmt.Errorf("failed to update PVC %q: %w", pvc.Name, err)
			}
			logger.Info("pvc updated successfully", "pvcName", pvc.Name)
		} else {
			logger.Info("pvc is up-to-date, skipping update", "pvcName", pvc.Name)
		}
		markResourceObserved(ctx, config.ResourcePVC, pvc.Namespace, pvc.Name)
	} else if k8serrors.IsNotFound(err) {
		// PVC不存在，创建新的
		_, err = c.client.CoreV1().PersistentVolumeClaims(pvc.Namespace).Create(ctx, pvc, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("failed to create PVC %q: %w", pvc.Name, err)
		}
		MarkResourceCreated(ctx, config.ResourcePVC, pvc.Namespace, pvc.Name)
		logger.Info("PVC created successfully", "pvcName", pvc.Name)
	} else {
		return fmt.Errorf("failed to check PVC existence: %w", err)
	}

	return nil
}

func (c *DeployPVCJobCtl) shouldUpdatePVC(existing, desired *corev1.PersistentVolumeClaim) bool {
	// 比较关键字段，决定是否需要更新
	if !compareStorageSmart(existing.Spec.Resources.Requests, desired.Spec.Resources.Requests) {
		return true
	}

	if len(existing.Spec.AccessModes) != len(desired.Spec.AccessModes) {
		return true
	}

	for i, mode := range existing.Spec.AccessModes {
		if mode != desired.Spec.AccessModes[i] {
			return true
		}
	}

	return false
}

func (c *DeployPVCJobCtl) wait(ctx context.Context) error {
	logger := klog.FromContext(ctx)
	var pvcName string
	if p, ok := c.job.JobInfo.(*corev1.PersistentVolumeClaim); ok {
		pvcName = p.Name
	} else {
		pvcName = c.job.Name
	}

	timeout := time.After(time.Duration(c.timeout()) * time.Second)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return NewStatusError(config.StatusCancelled, fmt.Errorf("pvc %s cancelled: %w", pvcName, ctx.Err()))
		case <-timeout:
			logger.Info("Timed out waiting for PVC", "pvcName", pvcName)
			return NewStatusError(config.StatusTimeout, fmt.Errorf("wait pvc %s timeout", pvcName))
		case <-ticker.C:
			isReady, err := c.getPVCStatus(ctx)
			if err != nil {
				logger.Error(err, "Error checking PVC status", "pvcName", pvcName)
				return fmt.Errorf("wait pvc %s error: %w", pvcName, err)
			}
			if isReady {
				logger.Info("PVC is ready", "pvcName", pvcName)
				return nil
			}
		}
	}
}

func (c *DeployPVCJobCtl) getPVCStatus(ctx context.Context) (bool, error) {
	var pvcName string
	if p, ok := c.job.JobInfo.(*corev1.PersistentVolumeClaim); ok {
		pvcName = p.Name
	} else {
		return false, fmt.Errorf("failed to get PVC info from job: %s", c.job.Name)
	}

	pvc, err := c.client.CoreV1().PersistentVolumeClaims(c.job.Namespace).Get(ctx, pvcName, metav1.GetOptions{})
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}

	// PVC创建成功且状态为Bound或Pending都认为是就绪
	return pvc.Status.Phase == corev1.ClaimBound || pvc.Status.Phase == corev1.ClaimPending, nil
}

func (c *DeployPVCJobCtl) timeout() int64 {
	if c.job.Timeout == 0 {
		c.job.Timeout = config.DeployTimeout
	}
	return c.job.Timeout
}

// compareStorageSmart 智能比较存储资源，PVC特殊处理
func compareStorageSmart(a, b corev1.ResourceList) bool {
	// 对于PVC存储，零值应该视为使用默认值，而不是"未设置"
	aStorage := getPVCStorageValue(a)
	bStorage := getPVCStorageValue(b)
	return aStorage == bStorage
}

// getPVCStorageValue 获取PVC存储值，零值使用默认值
func getPVCStorageValue(resources corev1.ResourceList) string {
	if val, ok := resources[corev1.ResourceStorage]; ok {
		// 如果值是零，返回默认PVC大小
		if val.IsZero() {
			return "1Gi" // PVC默认大小
		}
		return val.String()
	}
	return "1Gi" // 资源未设置，使用默认大小
}
