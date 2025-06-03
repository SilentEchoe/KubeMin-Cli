package job

import (
	"KubeMin-Cli/pkg/apiserver/config"
	"KubeMin-Cli/pkg/apiserver/domain/model"
	"KubeMin-Cli/pkg/apiserver/infrastructure/datastore"
	"KubeMin-Cli/pkg/apiserver/utils"
	k "KubeMin-Cli/pkg/apiserver/utils/kube"
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	applyv1 "k8s.io/client-go/applyconfigurations/core/v1"
	_ "k8s.io/client-go/applyconfigurations/meta/v1"

	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	"k8s.io/utils/pointer"
)

type DeployServiceJobCtl struct {
	namespace string
	job       *model.JobTask
	client    *kubernetes.Clientset
	store     datastore.DataStore
	ack       func()
}

func NewDeployServiceJobCtl(job *model.JobTask, client *kubernetes.Clientset, store datastore.DataStore, ack func()) *DeployServiceJobCtl {
	return &DeployServiceJobCtl{
		job:    job,
		client: client,
		store:  store,
		ack:    ack,
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

	if c.job == nil || c.job.JobInfo == nil {
		return fmt.Errorf("job or job.JobInfo is nil")
	}

	service, ok := c.job.JobInfo.(*v1.Service)
	if !ok {
		return fmt.Errorf("job.JobInfo is not a *v1.Service (actual type: %T)", c.job.JobInfo)
	}

	serviceLast, isAlreadyExists, err := k.ServiceExists(ctx, c.client, service.Name, service.Namespace)
	if err != nil {
		return fmt.Errorf("failed to check deployment existence: %w", err)
	}

	if isAlreadyExists {
		if isServiceChanged(serviceLast, service) {
			updated, err := c.ApplyService(ctx, service)
			if err != nil {
				klog.Errorf("failed to update service %q: %v", service.Name, err)
				return err
			}
			klog.Infof("Service %q updated successfully.", updated.Name)
		} else {
			klog.Infof("Service %q is up-to-date, skip apply.", service.Name)
		}
	} else {
		result, err := c.client.CoreV1().Services(c.job.Namespace).Create(context.TODO(), service, metav1.CreateOptions{})
		if err != nil {
			klog.Errorf("failed to create service %q namespace: %q : %v", service.Name, service.Namespace, err)
			return err
		}
		klog.Infof("JobTask Deploy Service Successfully %q.\n", result.GetObjectMeta().GetName())
	}

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
		portName := fmt.Sprintf("%s-%s", base, utils.RandRFC1123Suffix(6))
		servicePorts = append(servicePorts, applyv1.ServicePort().
			WithName(portName).
			WithPort(p.Port).
			WithTargetPort(intstr.FromInt(int(p.Port))).
			WithProtocol(corev1.ProtocolTCP))
	}

	if len(labels) == 0 {
		labels = map[string]string{"app": base}
	}

	return applyv1.Service(name, namespace).
		WithLabels(labels).
		WithSpec(applyv1.ServiceSpec().
			WithSelector(labels).
			WithPorts(servicePorts...).
			WithType(corev1.ServiceTypeClusterIP)).
		WithKind("Service").
		WithAPIVersion("v1").
		WithName(name).
		WithNamespace(namespace).
		WithLabels(labels)
}

//func GenerateService(name, namespace string, labels map[string]string, ports []model.Ports) *corev1.Service {
//	var servicePorts []corev1.ServicePort
//	baseName := utils.ToRFC1123Name(name)
//
//	for _, p := range ports {
//		portName := fmt.Sprintf("%s-%s", baseName, utils.RandRFC1123Suffix(6)) // RFC1123 兼容
//		servicePorts = append(servicePorts, corev1.ServicePort{
//			Name:       portName,
//			Port:       p.Port,
//			TargetPort: intstr.FromInt32(p.Port),
//			Protocol:   corev1.ProtocolTCP,
//		})
//	}
//
//	// labels 必须非空，否则 selector 无法生效
//	if len(labels) == 0 {
//		labels = map[string]string{"app": baseName}
//	}
//
//	return &corev1.Service{
//		ObjectMeta: metav1.ObjectMeta{
//			Name:      name,
//			Namespace: namespace,
//			Labels:    labels,
//		},
//		Spec: corev1.ServiceSpec{
//			Selector: labels,
//			Ports:    servicePorts,
//			Type:     corev1.ServiceTypeClusterIP,
//		},
//	}
//}

// isServiceChanged 判断两个 Service 是否存在需要更新的差异
func isServiceChanged(current, desired *corev1.Service) bool {
	// 比较类型（ClusterIP, NodePort, LoadBalancer 等）
	if current.Spec.Type != desired.Spec.Type {
		return true
	}

	// 比较 selector
	if !compareStringMap(current.Spec.Selector, desired.Spec.Selector) {
		return true
	}

	// 比较 ports
	if len(current.Spec.Ports) != len(desired.Spec.Ports) {
		return true
	}
	for i := range current.Spec.Ports {
		cp := current.Spec.Ports[i]
		dp := desired.Spec.Ports[i]

		if cp.Port != dp.Port || cp.TargetPort.String() != dp.TargetPort.String() || cp.Protocol != dp.Protocol || cp.Name != dp.Name || cp.NodePort != dp.NodePort {
			return true
		}
	}

	// 比较 annotations（常用于 LB 的配置、external-dns 等）
	if !compareStringMap(current.Annotations, desired.Annotations) {
		return true
	}

	return false
}

// compareStringMap 比较两个 map[string]string 是否相同
func compareStringMap(a, b map[string]string) bool {
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

func (c *DeployServiceJobCtl) ApplyService(ctx context.Context, svc *corev1.Service) (*corev1.Service, error) {
	// 设置资源 GVK
	svc.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "",
		Version: "v1",
		Kind:    "Service",
	})

	// 清理可能冲突的字段
	cleanObjectMeta(&svc.ObjectMeta)

	// 打印 JSON 以便调试
	raw, err := json.MarshalIndent(svc, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal service before apply failed: %w", err)
	}
	fmt.Println("Final Service spec to apply:\n", string(raw))

	// 实际发起 Apply 请求
	appliedSvc, err := c.client.CoreV1().Services(svc.Namespace).Patch(ctx,
		svc.Name,
		types.ApplyPatchType,
		raw,
		metav1.PatchOptions{
			FieldManager: "kubemin-cli",
			Force:        pointer.Bool(true),
		},
	)
	if err != nil {
		fmt.Printf("Patch failed: %v\n", err)
		return nil, fmt.Errorf("apply service failed: %w", err)
	}

	fmt.Printf("Service applied: %s/%s\n", appliedSvc.Namespace, appliedSvc.Name)
	return appliedSvc, nil
}
