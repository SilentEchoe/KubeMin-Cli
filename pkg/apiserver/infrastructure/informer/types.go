package informer

import (
	"sync"
	"time"

	appsv1 "k8s.io/api/apps/v1"

	"KubeMin-Cli/pkg/apiserver/config"
)

// ResourceType 资源类型
type ResourceType string

const (
	ResourceTypeDeployment  ResourceType = "Deployment"
	ResourceTypeStatefulSet ResourceType = "StatefulSet"
	ResourceTypePod         ResourceType = "Pod"
)

// WaitEntry 等待条目
type WaitEntry struct {
	Key          string        // namespace/name
	ResourceType ResourceType  // 资源类型
	ReadyChan    chan struct{} // 关闭表示资源就绪
	ErrorChan    chan error    // 错误通道
	CreatedAt    time.Time     // 创建时间
	mu           sync.Mutex    // 保护 closed 字段
	closed       bool          // 是否已关闭
}

// Close 安全关闭 WaitEntry
func (e *WaitEntry) Close() {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.closed {
		return
	}
	e.closed = true
	close(e.ReadyChan)
}

// SendError 发送错误并关闭
func (e *WaitEntry) SendError(err error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.closed {
		return
	}
	e.closed = true
	select {
	case e.ErrorChan <- err:
	default:
	}
	close(e.ErrorChan)
}

// IsClosed 检查是否已关闭
func (e *WaitEntry) IsClosed() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.closed
}

// DeploymentStatus 从 Deployment 提取的状态
type DeploymentStatus struct {
	Name            string
	Namespace       string
	Labels          map[string]string // 资源标签，用于提取 appID/componentID
	Replicas        int32
	ReadyReplicas   int32
	UpdatedReplicas int32
	Ready           bool
}

// StatefulSetStatus 从 StatefulSet 提取的状态
type StatefulSetStatus struct {
	Name          string
	Namespace     string
	Labels        map[string]string // 资源标签，用于提取 appID/componentID
	Replicas      int32
	ReadyReplicas int32
	Ready         bool
}

// ComponentStatusUpdate 组件状态更新信息（传递给数据库同步）
type ComponentStatusUpdate struct {
	AppID         string                 // 应用 ID
	ComponentID   int                    // 组件 ID
	ComponentName string                 // 组件名称
	Status        config.ComponentStatus // 运行状态
	ReadyReplicas int32                  // 就绪副本数
	Replicas      int32                  // 期望副本数
}

// StatusSyncFunc 状态同步回调函数类型
type StatusSyncFunc func(update *ComponentStatusUpdate)

// ExtractDeploymentStatus 从 Deployment 提取状态
func ExtractDeploymentStatus(deploy *appsv1.Deployment) *DeploymentStatus {
	if deploy == nil {
		return nil
	}
	var replicas int32
	if deploy.Spec.Replicas != nil {
		replicas = *deploy.Spec.Replicas
	}
	return &DeploymentStatus{
		Name:            deploy.Name,
		Namespace:       deploy.Namespace,
		Labels:          deploy.Labels,
		Replicas:        replicas,
		ReadyReplicas:   deploy.Status.ReadyReplicas,
		UpdatedReplicas: deploy.Status.UpdatedReplicas,
		Ready:           replicas > 0 && replicas == deploy.Status.ReadyReplicas,
	}
}

// ExtractStatefulSetStatus 从 StatefulSet 提取状态
func ExtractStatefulSetStatus(sts *appsv1.StatefulSet) *StatefulSetStatus {
	if sts == nil {
		return nil
	}
	var replicas int32
	if sts.Spec.Replicas != nil {
		replicas = *sts.Spec.Replicas
	}
	return &StatefulSetStatus{
		Name:          sts.Name,
		Namespace:     sts.Namespace,
		Labels:        sts.Labels,
		Replicas:      replicas,
		ReadyReplicas: sts.Status.ReadyReplicas,
		Ready:         replicas > 0 && replicas == sts.Status.ReadyReplicas,
	}
}

// WaitError 等待错误（携带状态）
type WaitError struct {
	Status config.Status
	Err    error
}

func (e *WaitError) Error() string { return e.Err.Error() }
func (e *WaitError) Unwrap() error { return e.Err }

// NewWaitError 创建带状态的等待错误
func NewWaitError(status config.Status, err error) error {
	if err == nil {
		return nil
	}
	return &WaitError{Status: status, Err: err}
}

// ExtractWaitError 提取 WaitError
func ExtractWaitError(err error) (*WaitError, bool) {
	if err == nil {
		return nil, false
	}
	if we, ok := err.(*WaitError); ok {
		return we, true
	}
	return nil, false
}

