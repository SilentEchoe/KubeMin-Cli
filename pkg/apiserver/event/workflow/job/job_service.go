package job

import (
	"KubeMin-Cli/pkg/apiserver/config"
	"KubeMin-Cli/pkg/apiserver/domain/model"
	"KubeMin-Cli/pkg/apiserver/infrastructure/datastore"
	"KubeMin-Cli/pkg/apiserver/utils"
	"context"
	"fmt"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	applyv1 "k8s.io/client-go/applyconfigurations/core/v1"
	_ "k8s.io/client-go/applyconfigurations/meta/v1"

	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

type DeployServiceJobCtl struct {
	namespace string
	job       *model.JobTask
	client    *kubernetes.Clientset
	store     datastore.DataStore
	ack       func()
}

func NewDeployServiceJobCtl(job *model.JobTask, client *kubernetes.Clientset, store datastore.DataStore, ack func()) *DeployServiceJobCtl {
	if job == nil {
		klog.Errorf("NewDeployServiceJobCtl: job is nil")
		return nil
	}
	return &DeployServiceJobCtl{
		namespace: job.Namespace,
		job:       job,
		client:    client,
		store:     store,
		ack:       ack,
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
		klog.Errorf("DeployServiceJob run error: %v", err)
		c.job.Status = config.StatusFailed
		c.ack()
		return
	}
	//这里是部署完毕后，将状态进行同步
	c.wait(ctx)
}

func (c *DeployServiceJobCtl) run(ctx context.Context) error {
	if c.client == nil {
		return fmt.Errorf("client is nil")
	}

	service, ok := c.job.JobInfo.(*applyv1.ServiceApplyConfiguration)
	if !ok {
		return fmt.Errorf("job.JobInfo is not a *applyv1.ServiceApplyConfiguration (actual type: %T)", c.job.JobInfo)
	}

	// 必要字段检查
	if service.Name == nil || service.Namespace == nil {
		return fmt.Errorf("service name or namespace is nil")
	}

	// 直接使用 ApplyService 处理创建或更新
	updated, err := c.ApplyService(ctx, service)
	if err != nil {
		klog.Errorf("failed to apply service %q: %v", *service.Name, err)
		return fmt.Errorf("apply service failed: %w", err)
	}
	klog.Infof("Service %q applied successfully.", updated.Name)

	// 任务完成
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
	timeout := time.After(time.Duration(c.timeout()) * time.Second)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			klog.Warning("timed out waiting for service: %s", c.job.Name)
			c.job.Status = config.StatusFailed
			return
		case <-ticker.C:
			isExist, err := getServiceStatus(c.client, c.job.Namespace, c.job.Name)
			if err != nil {
				klog.Errorf("error checking service status: %v", err)
				c.job.Status = config.StatusFailed
				return
			}
			if isExist {
				c.job.Status = config.StatusCompleted
				return
			}
		}
	}
}

func getServiceStatus(kubeClient *kubernetes.Clientset, namespace string, name string) (bool, error) {
	klog.Infof("Checking service: %s/%s", namespace, name)

	_, err := kubeClient.CoreV1().Services(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		if k8serrors.IsNotFound(err) {
			klog.Infof("service not found: %s/%s", namespace, name)
			return false, nil
		}
		klog.Error("check service error:%s", err)
		return false, err
	}

	return true, nil
}

func GenerateService(name, namespace string, labels map[string]string, ports []model.Ports) *applyv1.ServiceApplyConfiguration {
	var servicePorts []*applyv1.ServicePortApplyConfiguration
	base := utils.ToRFC1123Name(name)

	for _, p := range ports {
		// 确保每个端口都有一个有效的名称
		portName := fmt.Sprintf("%s-%d", base, p.Port)
		if len(portName) > 15 {
			// 如果名称太长，使用更短的格式
			portName = fmt.Sprintf("p-%d", p.Port)
		}

		port := applyv1.ServicePort().
			WithName(portName).
			WithPort(p.Port).
			WithTargetPort(intstr.FromInt32(p.Port)).
			WithProtocol(corev1.ProtocolTCP)
		servicePorts = append(servicePorts, port)
	}

	if len(labels) == 0 {
		labels = map[string]string{"app": base}
	}

	svc := applyv1.Service(name, namespace).
		WithLabels(labels).
		WithSpec(applyv1.ServiceSpec().
			WithSelector(labels).
			WithPorts(servicePorts...).
			WithType(corev1.ServiceTypeClusterIP)).
		WithKind("Service").
		WithAPIVersion("v1").
		WithName(name).
		WithNamespace(namespace)

	//// 打印完整的 service 配置以便调试
	//raw, _ := json.MarshalIndent(svc, "", "  ")
	//klog.Infof("Generated service configuration:\n%s", string(raw))

	return svc
}

// isServiceChanged 判断两个 Service 是否存在需要更新的差异
func isServiceChanged(current *corev1.Service, desired *applyv1.ServiceApplyConfiguration) bool {
	if current == nil || desired == nil {
		return true
	}

	// 比较类型（ClusterIP, NodePort, LoadBalancer 等）
	if desired.Spec != nil && desired.Spec.Type != nil && *desired.Spec.Type != current.Spec.Type {
		return true
	}

	// 比较 selector
	if desired.Spec != nil && desired.Spec.Selector != nil {
		if !compareStringMap(current.Spec.Selector, desired.Spec.Selector) {
			return true
		}
	}

	// 比较 ports
	if desired.Spec != nil && desired.Spec.Ports != nil {
		if len(current.Spec.Ports) != len(desired.Spec.Ports) {
			return true
		}
		for i := range current.Spec.Ports {
			cp := current.Spec.Ports[i]
			dp := desired.Spec.Ports[i]

			if dp.Port != nil && *dp.Port != cp.Port {
				return true
			}
			if dp.TargetPort != nil && dp.TargetPort.String() != cp.TargetPort.String() {
				return true
			}
			if dp.Protocol != nil && *dp.Protocol != cp.Protocol {
				return true
			}
			if dp.Name != nil && *dp.Name != cp.Name {
				return true
			}
			if dp.NodePort != nil && *dp.NodePort != cp.NodePort {
				return true
			}
		}
	}

	// 比较 labels
	if desired.Labels != nil {
		if !compareStringMap(current.Labels, desired.Labels) {
			return true
		}
	}

	// 比较 annotations
	if desired.Annotations != nil {
		if !compareStringMap(current.Annotations, desired.Annotations) {
			return true
		}
	}

	return false
}

// compareStringMap 比较两个 map[string]string 是否相同
func compareStringMap(a, b map[string]string) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

func (c *DeployServiceJobCtl) ApplyService(ctx context.Context, svc *applyv1.ServiceApplyConfiguration) (*corev1.Service, error) {
	// 转换为 corev1.Service
	coreService := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:        *svc.Name,
			Namespace:   *svc.Namespace,
			Labels:      svc.Labels,
			Annotations: svc.Annotations,
		},
		Spec: corev1.ServiceSpec{
			Type:     *svc.Spec.Type,
			Selector: svc.Spec.Selector,
			Ports:    make([]corev1.ServicePort, len(svc.Spec.Ports)),
		},
	}

	// 转换端口
	for i, port := range svc.Spec.Ports {
		portName := fmt.Sprintf("port-%d", i)
		if port.Name != nil {
			portName = *port.Name
		}

		coreService.Spec.Ports[i] = corev1.ServicePort{
			Name:       portName,
			Port:       *port.Port,
			TargetPort: *port.TargetPort,
			Protocol:   *port.Protocol,
		}
	}

	// 检查 service 是否存在
	_, err := c.client.CoreV1().Services(coreService.Namespace).Get(ctx, coreService.Name, metav1.GetOptions{})
	if err != nil {
		if k8serrors.IsNotFound(err) {
			// 如果不存在，则创建
			appliedSvc, err := c.client.CoreV1().Services(coreService.Namespace).Create(ctx, coreService, metav1.CreateOptions{})
			if err != nil {
				klog.Errorf("Create failed: %v", err)
				return nil, fmt.Errorf("create service failed: %w", err)
			}
			klog.InfoS("Service created", "namespace", appliedSvc.Namespace, "name", appliedSvc.Name)
			return appliedSvc, nil
		}
		return nil, fmt.Errorf("failed to check service existence: %w", err)
	}

	// 如果存在，则更新
	appliedSvc, err := c.client.CoreV1().Services(coreService.Namespace).Update(ctx, coreService, metav1.UpdateOptions{})
	if err != nil {
		klog.Errorf("Update failed: %v", err)
		return nil, fmt.Errorf("update service failed: %w", err)
	}

	klog.Infof("Service updated: %s/%s", appliedSvc.Namespace, appliedSvc.Name)
	return appliedSvc, nil
}

func convertApplyToCoreV1Service(applySvc *applyv1.ServiceApplyConfiguration) (*corev1.Service, error) {
	if applySvc.Name == nil || applySvc.Namespace == nil || applySvc.Spec == nil {
		return nil, fmt.Errorf("missing required fields in apply service")
	}

	core := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:        *applySvc.Name,
			Namespace:   *applySvc.Namespace,
			Labels:      applySvc.Labels,
			Annotations: applySvc.Annotations,
		},
		Spec: corev1.ServiceSpec{
			Selector: applySvc.Spec.Selector,
			Type:     corev1.ServiceTypeClusterIP, // 默认值
		},
	}

	if applySvc.Spec.Type != nil {
		core.Spec.Type = *applySvc.Spec.Type
	}

	for i, p := range applySvc.Spec.Ports {
		if p.Port == nil || p.TargetPort == nil || p.Protocol == nil || p.Name == nil {
			return nil, fmt.Errorf("service port[%d] has missing fields", i)
		}
		core.Spec.Ports = append(core.Spec.Ports, corev1.ServicePort{
			Name:       *p.Name,
			Port:       *p.Port,
			TargetPort: *p.TargetPort,
			Protocol:   *p.Protocol,
		})
	}

	return core, nil
}
