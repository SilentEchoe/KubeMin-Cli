package informer

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/klog/v2"

	"KubeMin-Cli/pkg/apiserver/config"
)

// ResourceReadyWaiter 资源就绪等待器 - 基于 Informer 事件驱动
type ResourceReadyWaiter struct {
	// waiters 存储等待中的资源
	// key 格式: "resourceType/namespace/name"
	waiters sync.Map
	// statusSyncFunc 状态同步回调（更新数据库）
	statusSyncFunc StatusSyncFunc
}

// NewResourceReadyWaiter 创建等待器
func NewResourceReadyWaiter() *ResourceReadyWaiter {
	return &ResourceReadyWaiter{}
}

// SetStatusSyncFunc 设置状态同步回调函数
func (w *ResourceReadyWaiter) SetStatusSyncFunc(fn StatusSyncFunc) {
	w.statusSyncFunc = fn
}

// buildKey 构建唯一键
func buildKey(resourceType ResourceType, namespace, name string) string {
	return fmt.Sprintf("%s/%s/%s", resourceType, namespace, name)
}

// WaitForDeploymentReady 等待 Deployment 就绪
func (w *ResourceReadyWaiter) WaitForDeploymentReady(ctx context.Context, namespace, name string, timeout time.Duration) error {
	return w.waitForResource(ctx, ResourceTypeDeployment, namespace, name, timeout)
}

// WaitForStatefulSetReady 等待 StatefulSet 就绪
func (w *ResourceReadyWaiter) WaitForStatefulSetReady(ctx context.Context, namespace, name string, timeout time.Duration) error {
	return w.waitForResource(ctx, ResourceTypeStatefulSet, namespace, name, timeout)
}

// waitForResource 通用等待逻辑
func (w *ResourceReadyWaiter) waitForResource(ctx context.Context, resourceType ResourceType, namespace, name string, timeout time.Duration) error {
	key := buildKey(resourceType, namespace, name)

	entry := &WaitEntry{
		Key:          key,
		ResourceType: resourceType,
		ReadyChan:    make(chan struct{}),
		ErrorChan:    make(chan error, 1),
		CreatedAt:    time.Now(),
	}

	// 注册等待
	w.waiters.Store(key, entry)
	defer w.waiters.Delete(key)

	klog.V(4).Infof("Waiting for %s %s/%s to be ready (timeout: %v)", resourceType, namespace, name, timeout)

	select {
	case <-entry.ReadyChan:
		klog.V(4).Infof("%s %s/%s is ready", resourceType, namespace, name)
		return nil
	case err := <-entry.ErrorChan:
		klog.V(4).Infof("%s %s/%s wait error: %v", resourceType, namespace, name, err)
		return err
	case <-ctx.Done():
		return NewWaitError(config.StatusCancelled, fmt.Errorf("%s %s/%s cancelled: %w", resourceType, namespace, name, ctx.Err()))
	case <-time.After(timeout):
		return NewWaitError(config.StatusTimeout, fmt.Errorf("%s %s/%s timeout after %v", resourceType, namespace, name, timeout))
	}
}

// OnDeploymentAdd 处理 Deployment 创建事件 - 由 Informer 调用
func (w *ResourceReadyWaiter) OnDeploymentAdd(deploy *appsv1.Deployment) {
	w.OnDeploymentUpdate(nil, deploy)
}

// OnDeploymentUpdate 处理 Deployment 更新事件 - 由 Informer 调用
func (w *ResourceReadyWaiter) OnDeploymentUpdate(oldDeploy, newDeploy *appsv1.Deployment) {
	if newDeploy == nil {
		return
	}

	status := ExtractDeploymentStatus(newDeploy)

	klog.V(4).Infof("Deployment %s/%s update: replicas=%d, ready=%d, isReady=%v",
		newDeploy.Namespace, newDeploy.Name, status.Replicas, status.ReadyReplicas, status.Ready)

	// 1. 同步状态到数据库
	w.syncStatusToDB(status.Labels, status.Replicas, status.ReadyReplicas, status.Ready)

	// 2. 通知等待者
	key := buildKey(ResourceTypeDeployment, newDeploy.Namespace, newDeploy.Name)
	entryVal, ok := w.waiters.Load(key)
	if !ok {
		return
	}

	entry := entryVal.(*WaitEntry)
	if entry.IsClosed() {
		return
	}

	if status.Ready {
		entry.Close()
	}
}

// OnDeploymentDelete 处理 Deployment 删除事件
func (w *ResourceReadyWaiter) OnDeploymentDelete(deploy *appsv1.Deployment) {
	if deploy == nil {
		return
	}

	klog.V(4).Infof("Deployment %s/%s deleted", deploy.Namespace, deploy.Name)

	// 1. 同步删除状态到数据库（ready=0, replicas=0 表示已删除）
	w.syncStatusToDB(deploy.Labels, 0, 0, false)

	// 2. 通知等待者（如果有）
	key := buildKey(ResourceTypeDeployment, deploy.Namespace, deploy.Name)
	entryVal, ok := w.waiters.Load(key)
	if !ok {
		return
	}

	entry := entryVal.(*WaitEntry)
	entry.SendError(NewWaitError(config.StatusFailed, fmt.Errorf("deployment %s/%s was deleted", deploy.Namespace, deploy.Name)))
}

// OnStatefulSetAdd 处理 StatefulSet 创建事件 - 由 Informer 调用
func (w *ResourceReadyWaiter) OnStatefulSetAdd(sts *appsv1.StatefulSet) {
	w.OnStatefulSetUpdate(nil, sts)
}

// OnStatefulSetUpdate 处理 StatefulSet 更新事件 - 由 Informer 调用
func (w *ResourceReadyWaiter) OnStatefulSetUpdate(oldSts, newSts *appsv1.StatefulSet) {
	if newSts == nil {
		return
	}

	status := ExtractStatefulSetStatus(newSts)

	klog.V(4).Infof("StatefulSet %s/%s update: replicas=%d, ready=%d, isReady=%v",
		newSts.Namespace, newSts.Name, status.Replicas, status.ReadyReplicas, status.Ready)

	// 1. 同步状态到数据库
	w.syncStatusToDB(status.Labels, status.Replicas, status.ReadyReplicas, status.Ready)

	// 2. 通知等待者
	key := buildKey(ResourceTypeStatefulSet, newSts.Namespace, newSts.Name)
	entryVal, ok := w.waiters.Load(key)
	if !ok {
		return
	}

	entry := entryVal.(*WaitEntry)
	if entry.IsClosed() {
		return
	}

	if status.Ready {
		entry.Close()
	}
}

// OnStatefulSetDelete 处理 StatefulSet 删除事件
func (w *ResourceReadyWaiter) OnStatefulSetDelete(sts *appsv1.StatefulSet) {
	if sts == nil {
		return
	}

	klog.V(4).Infof("StatefulSet %s/%s deleted", sts.Namespace, sts.Name)

	// 1. 同步删除状态到数据库
	w.syncStatusToDB(sts.Labels, 0, 0, false)

	// 2. 通知等待者（如果有）
	key := buildKey(ResourceTypeStatefulSet, sts.Namespace, sts.Name)
	entryVal, ok := w.waiters.Load(key)
	if !ok {
		return
	}

	entry := entryVal.(*WaitEntry)
	entry.SendError(NewWaitError(config.StatusFailed, fmt.Errorf("statefulset %s/%s was deleted", sts.Namespace, sts.Name)))
}

// GetPendingCount 获取等待中的资源数量（用于监控）
func (w *ResourceReadyWaiter) GetPendingCount() int {
	count := 0
	w.waiters.Range(func(_, _ interface{}) bool {
		count++
		return true
	})
	return count
}

// GetPendingKeys 获取所有等待中的资源键（用于调试）
func (w *ResourceReadyWaiter) GetPendingKeys() []string {
	var keys []string
	w.waiters.Range(func(key, _ interface{}) bool {
		keys = append(keys, key.(string))
		return true
	})
	return keys
}

// syncStatusToDB 同步组件状态到数据库
func (w *ResourceReadyWaiter) syncStatusToDB(labels map[string]string, replicas, readyReplicas int32, ready bool) {
	if w.statusSyncFunc == nil {
		return
	}

	// 从 labels 提取组件标识
	appID := labels[config.LabelAppID]
	componentName := labels[config.LabelComponentName]
	componentIDStr := labels[config.LabelComponentID]

	if appID == "" || componentName == "" {
		return // 不是我们管理的资源
	}

	componentID, _ := strconv.Atoi(componentIDStr)

	// 计算状态
	var status config.ComponentStatus
	if ready {
		status = config.ComponentStatusRunning
	} else if readyReplicas > 0 {
		status = config.ComponentStatusPending
	} else if replicas > 0 {
		status = config.ComponentStatusPending
	} else {
		// replicas=0 表示资源被删除或缩容为0
		status = config.ComponentStatusFailed
	}

	update := &ComponentStatusUpdate{
		AppID:         appID,
		ComponentID:   componentID,
		ComponentName: componentName,
		Status:        status,
		ReadyReplicas: readyReplicas,
		Replicas:      replicas,
	}

	// 异步调用回调，避免阻塞 Informer
	go w.statusSyncFunc(update)
}
