package informer

import (
	"context"
	"fmt"
	"sync"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
)

// Manager 管理所有 Informer
type Manager struct {
	client        kubernetes.Interface
	factory       informers.SharedInformerFactory
	stopCh        chan struct{}
	waiter        *ResourceReadyWaiter
	mu            sync.RWMutex
	started       bool
	resyncPeriod  time.Duration
	labelSelector string
}

// ManagerOption 配置选项
type ManagerOption func(*Manager)

// WithResyncPeriod 设置重新同步周期
func WithResyncPeriod(d time.Duration) ManagerOption {
	return func(m *Manager) {
		m.resyncPeriod = d
	}
}

// WithLabelSelector 设置标签过滤器（减少内存消耗）
func WithLabelSelector(selector string) ManagerOption {
	return func(m *Manager) {
		m.labelSelector = selector
	}
}

// NewManager 创建 Informer 管理器
func NewManager(client kubernetes.Interface, opts ...ManagerOption) *Manager {
	m := &Manager{
		client:       client,
		stopCh:       make(chan struct{}),
		waiter:       NewResourceReadyWaiter(),
		resyncPeriod: 30 * time.Second,
	}

	for _, opt := range opts {
		opt(m)
	}

	// 创建 InformerFactory
	if m.labelSelector != "" {
		m.factory = informers.NewSharedInformerFactoryWithOptions(
			client,
			m.resyncPeriod,
			informers.WithTweakListOptions(func(opts *metav1.ListOptions) {
				opts.LabelSelector = m.labelSelector
			}),
		)
		klog.V(2).Infof("Informer manager created with label selector: %s", m.labelSelector)
	} else {
		m.factory = informers.NewSharedInformerFactory(client, m.resyncPeriod)
		klog.V(2).Info("Informer manager created without label selector")
	}

	return m
}

// GetWaiter 获取资源等待器
func (m *Manager) GetWaiter() *ResourceReadyWaiter {
	return m.waiter
}

// Start 启动 Informer
func (m *Manager) Start(ctx context.Context) error {
	m.mu.Lock()
	if m.started {
		m.mu.Unlock()
		klog.V(2).Info("Informer manager already started")
		return nil
	}
	m.started = true
	m.mu.Unlock()

	klog.Info("Starting informer manager...")

	// 设置 Deployment Informer
	deployInformer := m.factory.Apps().V1().Deployments().Informer()
	_, err := deployInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			if deploy, ok := obj.(*appsv1.Deployment); ok {
				m.waiter.OnDeploymentAdd(deploy)
			}
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			oldDeploy, ok1 := oldObj.(*appsv1.Deployment)
			newDeploy, ok2 := newObj.(*appsv1.Deployment)
			if ok1 && ok2 {
				m.waiter.OnDeploymentUpdate(oldDeploy, newDeploy)
			}
		},
		DeleteFunc: func(obj interface{}) {
			if deploy, ok := obj.(*appsv1.Deployment); ok {
				m.waiter.OnDeploymentDelete(deploy)
			} else if tombstone, ok := obj.(cache.DeletedFinalStateUnknown); ok {
				if deploy, ok := tombstone.Obj.(*appsv1.Deployment); ok {
					m.waiter.OnDeploymentDelete(deploy)
				}
			}
		},
	})
	if err != nil {
		return fmt.Errorf("failed to add deployment event handler: %w", err)
	}
	klog.V(2).Info("Deployment informer event handler registered")

	// 设置 StatefulSet Informer
	stsInformer := m.factory.Apps().V1().StatefulSets().Informer()
	_, err = stsInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			if sts, ok := obj.(*appsv1.StatefulSet); ok {
				m.waiter.OnStatefulSetAdd(sts)
			}
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			oldSts, ok1 := oldObj.(*appsv1.StatefulSet)
			newSts, ok2 := newObj.(*appsv1.StatefulSet)
			if ok1 && ok2 {
				m.waiter.OnStatefulSetUpdate(oldSts, newSts)
			}
		},
		DeleteFunc: func(obj interface{}) {
			if sts, ok := obj.(*appsv1.StatefulSet); ok {
				m.waiter.OnStatefulSetDelete(sts)
			} else if tombstone, ok := obj.(cache.DeletedFinalStateUnknown); ok {
				if sts, ok := tombstone.Obj.(*appsv1.StatefulSet); ok {
					m.waiter.OnStatefulSetDelete(sts)
				}
			}
		},
	})
	if err != nil {
		return fmt.Errorf("failed to add statefulset event handler: %w", err)
	}
	klog.V(2).Info("StatefulSet informer event handler registered")

	// 启动所有 Informer
	m.factory.Start(m.stopCh)

	// 等待缓存同步
	klog.Info("Waiting for informer caches to sync...")
	synced := m.factory.WaitForCacheSync(m.stopCh)
	for typ, ok := range synced {
		if !ok {
			return fmt.Errorf("failed to sync cache for %v", typ)
		}
		klog.V(2).Infof("Cache synced for %v", typ)
	}
	klog.Info("All informer caches synced successfully")

	// 监听 context 取消
	go func() {
		<-ctx.Done()
		klog.Info("Context cancelled, stopping informer manager...")
		m.Stop()
	}()

	return nil
}

// Stop 停止所有 Informer
func (m *Manager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.started {
		return
	}

	close(m.stopCh)
	m.started = false
	klog.Info("Informer manager stopped")
}

// IsStarted 检查是否已启动
func (m *Manager) IsStarted() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.started
}

// GetStats 获取统计信息（用于监控）
func (m *Manager) GetStats() map[string]interface{} {
	return map[string]interface{}{
		"started":        m.IsStarted(),
		"pendingWaiters": m.waiter.GetPendingCount(),
		"pendingKeys":    m.waiter.GetPendingKeys(),
		"labelSelector":  m.labelSelector,
		"resyncPeriod":   m.resyncPeriod.String(),
	}
}
