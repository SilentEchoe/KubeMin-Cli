package job

import (
	"context"
	"fmt"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	applyv1 "k8s.io/client-go/applyconfigurations/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	"KubeMin-Cli/pkg/apiserver/config"
	"KubeMin-Cli/pkg/apiserver/domain/model"
	"KubeMin-Cli/pkg/apiserver/infrastructure/datastore"
	"KubeMin-Cli/pkg/apiserver/utils"
)

type DeployServiceJobCtl struct {
	namespace string
	job       *model.JobTask
	client    kubernetes.Interface
	store     datastore.DataStore
	ack       func()
}

func NewDeployServiceJobCtl(job *model.JobTask, client kubernetes.Interface, store datastore.DataStore, ack func()) *DeployServiceJobCtl {
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

func (c *DeployServiceJobCtl) Clean(ctx context.Context) {
	if c.client == nil {
		return
	}
	refs := resourcesForCleanup(ctx, config.ResourceService)
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
		if err := c.client.CoreV1().Services(ns).Delete(cleanupCtx, ref.Name, metav1.DeleteOptions{}); err != nil {
			if !k8serrors.IsNotFound(err) {
				klog.Errorf("failed to delete service %s/%s during cleanup: %v", ns, ref.Name, err)
			}
		} else {
			klog.Infof("deleted service %s/%s after job failure", ns, ref.Name)
		}
	}
}

// SaveInfo  创建Job的详情信息
func (c *DeployServiceJobCtl) SaveInfo(ctx context.Context) error {
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

func (c *DeployServiceJobCtl) Run(ctx context.Context) error {
	c.job.Status = config.StatusRunning
	c.job.Error = ""
	c.ack()

	if err := c.run(ctx); err != nil {
		klog.Errorf("DeployServiceJob run error: %v", err)
		c.job.Error = err.Error()
		if statusErr, ok := ExtractStatusError(err); ok {
			c.job.Status = statusErr.Status
		} else {
			c.job.Status = config.StatusFailed
		}
		return err
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
	name := *service.Name
	namespace := *service.Namespace

	if _, err := c.client.CoreV1().Services(namespace).Get(ctx, name, metav1.GetOptions{}); err != nil {
		if k8serrors.IsNotFound(err) {
			MarkResourceCreated(ctx, config.ResourceService, namespace, name)
		} else {
			return fmt.Errorf("check service existence failed: %w", err)
		}
	} else {
		markResourceObserved(ctx, config.ResourceService, namespace, name)
	}

	// 直接使用 ApplyService 处理创建或更新
	updated, err := c.ApplyService(ctx, service)
	if err != nil {
		klog.Errorf("failed to apply service %q: %v", *service.Name, err)
		return fmt.Errorf("apply service failed: %w", err)
	}
	klog.Infof("Service %q applied successfully.", updated.Name)

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

func (c *DeployServiceJobCtl) wait(ctx context.Context) error {
	timeout := time.After(time.Duration(c.timeout()) * time.Second)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	serviceName := buildServiceName(c.job.Name, c.job.AppID)
	for {
		select {
		case <-ctx.Done():
			return NewStatusError(config.StatusCancelled, fmt.Errorf("service %s cancelled: %w", c.job.Name, ctx.Err()))
		case <-timeout:
			klog.Warningf("timed out waiting for service: %s", c.job.Name)
			return NewStatusError(config.StatusTimeout, fmt.Errorf("wait service %s timeout", c.job.Name))
		case <-ticker.C:
			isExist, err := getServiceStatus(c.client, c.job.Namespace, serviceName)
			if err != nil {
				klog.Errorf("error checking service status: %v", err)
				return fmt.Errorf("wait service %s error: %w", c.job.Name, err)
			}
			if isExist {
				return nil
			}
		}
	}
}

func getServiceStatus(kubeClient kubernetes.Interface, namespace string, name string) (bool, error) {
	klog.Infof("Checking service: %s/%s", namespace, name)

	_, err := kubeClient.CoreV1().Services(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		if k8serrors.IsNotFound(err) {
			klog.Errorf("service not found: %s/%s", namespace, name)
			return false, nil
		}
		klog.Errorf("check service error:%s", err)
		return false, err
	}

	return true, nil
}

func GenerateService(component *model.ApplicationComponent, properties *model.Properties) *applyv1.ServiceApplyConfiguration {
	var servicePorts []*applyv1.ServicePortApplyConfiguration
	base := utils.ToRFC1123Name(component.Name)

	for _, p := range properties.Ports {
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

	labels := BuildLabels(component, properties)

	selectorLabel := map[string]string{config.LabelAppID: component.AppID}

	serviceName := buildServiceName(component.Name, component.AppID)
	svc := applyv1.Service(serviceName, component.Namespace).
		WithLabels(labels).
		WithSpec(applyv1.ServiceSpec().
			WithSelector(selectorLabel).
			WithPorts(servicePorts...).
			WithType(corev1.ServiceTypeClusterIP)).
		WithKind("Service").
		WithAPIVersion("v1").
		WithName(serviceName).
		WithNamespace(component.Namespace)

	return svc
}

func (c *DeployServiceJobCtl) ApplyService(ctx context.Context, svc *applyv1.ServiceApplyConfiguration) (*corev1.Service, error) {
	// 处理可能为 nil 的字段
	var serviceType corev1.ServiceType = corev1.ServiceTypeClusterIP // 默认值
	if svc.Spec.Type != nil {
		serviceType = *svc.Spec.Type
	}

	coreService := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:        *svc.Name,
			Namespace:   *svc.Namespace,
			Labels:      svc.Labels,
			Annotations: svc.Annotations,
		},
		Spec: corev1.ServiceSpec{
			Type:     serviceType,
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

		// 处理可能为 nil 的字段
		var targetPort intstr.IntOrString
		if port.TargetPort != nil {
			targetPort = *port.TargetPort
		}

		var protocol corev1.Protocol = corev1.ProtocolTCP // 默认值
		if port.Protocol != nil {
			protocol = *port.Protocol
		}

		coreService.Spec.Ports[i] = corev1.ServicePort{
			Name:       portName,
			Port:       *port.Port,
			TargetPort: targetPort,
			Protocol:   protocol,
		}
	}

	// 检查 service 是否存在并获取现有 service 信息
	existingService, err := c.client.CoreV1().Services(coreService.Namespace).Get(ctx, coreService.Name, metav1.GetOptions{})
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

	// ✅ 复制必要字段 - 修复 Service 更新问题
	if existingService != nil {
		// 复制 ResourceVersion 用于乐观并发控制
		coreService.ResourceVersion = existingService.ResourceVersion

		// 复制 ClusterIP 和 ClusterIPs（不可变字段）
		coreService.Spec.ClusterIP = existingService.Spec.ClusterIP
		coreService.Spec.ClusterIPs = existingService.Spec.ClusterIPs

		// 复制 IPFamilies（如果存在）
		if len(existingService.Spec.IPFamilies) > 0 {
			coreService.Spec.IPFamilies = existingService.Spec.IPFamilies
		}

		// 复制 SessionAffinityConfig（如果存在）
		if existingService.Spec.SessionAffinityConfig != nil {
			coreService.Spec.SessionAffinityConfig = existingService.Spec.SessionAffinityConfig
		}

		// 复制其他可能需要保留的字段
		if existingService.Spec.SessionAffinity != "" {
			coreService.Spec.SessionAffinity = existingService.Spec.SessionAffinity
		}

		// 复制 LoadBalancerIP（如果存在）
		if existingService.Spec.LoadBalancerIP != "" {
			coreService.Spec.LoadBalancerIP = existingService.Spec.LoadBalancerIP
		}

		// 复制 LoadBalancerSourceRanges（如果存在）
		if len(existingService.Spec.LoadBalancerSourceRanges) > 0 {
			coreService.Spec.LoadBalancerSourceRanges = existingService.Spec.LoadBalancerSourceRanges
		}

		// 复制 ExternalName（如果存在）
		if existingService.Spec.ExternalName != "" {
			coreService.Spec.ExternalName = existingService.Spec.ExternalName
		}

		// 复制 ExternalTrafficPolicy（如果存在）
		if existingService.Spec.ExternalTrafficPolicy != "" {
			coreService.Spec.ExternalTrafficPolicy = existingService.Spec.ExternalTrafficPolicy
		}

		// 复制 HealthCheckNodePort（如果存在）
		if existingService.Spec.HealthCheckNodePort != 0 {
			coreService.Spec.HealthCheckNodePort = existingService.Spec.HealthCheckNodePort
		}

		// 复制 PublishNotReadyAddresses（如果存在）
		if existingService.Spec.PublishNotReadyAddresses {
			coreService.Spec.PublishNotReadyAddresses = existingService.Spec.PublishNotReadyAddresses
		}

		// 复制 InternalTrafficPolicy（如果存在）
		if existingService.Spec.InternalTrafficPolicy != nil {
			coreService.Spec.InternalTrafficPolicy = existingService.Spec.InternalTrafficPolicy
		}

		klog.Infof("Copying necessary fields from existing service %s/%s: ResourceVersion=%s, ClusterIP=%s",
			existingService.Namespace, existingService.Name,
			existingService.ResourceVersion, existingService.Spec.ClusterIP)
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
