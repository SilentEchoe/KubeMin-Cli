package job

import (
	"KubeMin-Cli/pkg/apiserver/config"
	"KubeMin-Cli/pkg/apiserver/domain/model"
	"KubeMin-Cli/pkg/apiserver/infrastructure/datastore"
	"KubeMin-Cli/pkg/apiserver/workflow/traits"
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	app "k8s.io/api/apps/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	"k8s.io/utils/pointer"
)

type DeployJobCtl struct {
	namespace string
	job       *model.JobTask
	client    *kubernetes.Clientset
	store     datastore.DataStore
	ack       func()
}

func NewDeployJobCtl(job *model.JobTask, client *kubernetes.Clientset, store datastore.DataStore, ack func()) *DeployJobCtl {
	if client == nil || store == nil {
		return nil
	}
	return &DeployJobCtl{
		job:    job,
		client: client,
		store:  store,
		ack:    ack,
	}
}

func (c *DeployJobCtl) Clean(ctx context.Context) {}

// SaveInfo  创建Job的详情信息
func (c *DeployJobCtl) SaveInfo(ctx context.Context) error {
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

func (c *DeployJobCtl) Run(ctx context.Context) {
	c.job.Status = config.StatusRunning
	c.ack() // 通知工作流开始运行
	if err := c.run(ctx); err != nil {
		return
	}
	//这里是部署完毕后，将状态进行同步
	c.wait(ctx)
}

func (c *DeployJobCtl) run(ctx context.Context) error {
	if c.client == nil {
		return fmt.Errorf("client is nil")
	}
	var deploy *app.Deployment
	if d, ok := c.job.JobInfo.(*app.Deployment); ok {
		deploy = d
	} else {
		return fmt.Errorf("deploy Job Job.Info Conversion type failure")
	}

	deployLast, isAlreadyExists, err := c.deploymentExists(ctx, deploy.Name, deploy.Namespace)
	if err != nil {
		return fmt.Errorf("failed to check deployment existence: %w", err)
	}

	if isAlreadyExists {
		//如果存在先进行对比，然后
		if isDeploymentChanged(deployLast, deploy) {
			deploy.ResourceVersion = deployLast.ResourceVersion // 必须设置才能更新
			deploy.Spec.Selector = deployLast.Spec.Selector
			deploy.Spec.Template.Labels = deployLast.Spec.Template.Labels
			// TODO 这里应该通过策略实现多种，比如强制更新，软更新(apply) 或者Path,暂时只实现了Path
			updated, err := c.ApplyDeployment(ctx, deploy)
			if err != nil {
				klog.Errorf("failed to update deployment %q: %v", deploy.Name, err)
				return err
			}
			klog.Infof("Deployment %q updated successfully.", updated.Name)
		} else {
			klog.Infof("Deployment %q is up-to-date, skip apply.", deploy.Name)
		}
	} else {
		result, err := c.client.AppsV1().Deployments(deploy.Namespace).Create(ctx, deploy, metav1.CreateOptions{})
		if err != nil {
			klog.Errorf("failed to create deployment %q namespace: %q : %v", deploy.Name, deploy.Namespace, err)
			return err
		}
		klog.Infof("JobTask Deploy Successfully %q.\n", result.GetObjectMeta().GetName())
	}

	c.job.Status = config.StatusCompleted
	c.ack()
	return nil
}

func (c *DeployJobCtl) updateServiceModuleImages(ctx context.Context) error {
	wg := sync.WaitGroup{}
	wg.Wait()
	return nil
}

func (c *DeployJobCtl) wait(ctx context.Context) {
	timeout := time.After(time.Duration(c.timeout()) * time.Second)
	for {
		select {
		case <-timeout:
			klog.Infof(fmt.Sprintf("%s", c.job.Name))
			newResources, err := getDeploymentStatus(c.client, c.job.Namespace, c.job.Name)
			if err != nil || newResources == nil {
				msg := fmt.Sprintf("get resource owner info error: %v", err)
				klog.Errorf(msg)
				c.job.Status = config.StatusFailed
			}
		default:
			time.Sleep(2 * time.Second)
			newResources, err := getDeploymentStatus(c.client, c.job.Namespace, c.job.Name)
			if err != nil {
				msg := fmt.Sprintf("get resource owner info error: %v", err)
				klog.Errorf(msg)
				c.job.Status = config.StatusFailed
				return
			}
			if newResources != nil {
				klog.Infof(fmt.Sprintf("newResources:%s, Replicas:%d ,ReadyReplicas:%d ", newResources.Name, newResources.Replicas, newResources.ReadyReplicas))
				if newResources.Ready {
					c.job.Status = config.StatusCompleted
					return
				}
			}
		}
	}
}

func getDeploymentStatus(kubeClient *kubernetes.Clientset, namespace string, name string) (deployInfo *model.JobDeployInfo, err error) {
	klog.Infof("%s-%s", namespace, name)
	deploy, err := kubeClient.AppsV1().Deployments(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			// Deployment 不存在，处理这种情况
			klog.Infof("deploy is nil")
			return nil, nil
		}
		return nil, err
	}
	klog.Infof(fmt.Sprintf("newResources:%s, Replicas:%d ,ReadyReplicas:%d ", deploy.Name, deploy.Spec.Replicas, deploy.Status.ReadyReplicas))
	isOk := false
	if *deploy.Spec.Replicas == deploy.Status.ReadyReplicas {
		isOk = true
	}
	return &model.JobDeployInfo{
		Name:          deploy.Name,
		Replicas:      *deploy.Spec.Replicas,
		ReadyReplicas: deploy.Status.ReadyReplicas,
		Ready:         isOk}, nil
}

func (c *DeployJobCtl) timeout() int64 {
	if c.job.Timeout == 0 {
		c.job.Timeout = config.DeployTimeout
	}
	return c.job.Timeout
}

func (c *DeployJobCtl) deploymentExists(ctx context.Context, name, namespaces string) (*appsv1.Deployment, bool, error) {
	oldDeploy, err := c.client.AppsV1().Deployments(namespaces).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return nil, false, nil
		}
		return nil, false, err
	}
	return oldDeploy, true, nil
}

func GenerateWebService(component *model.ApplicationComponent, properties *model.Properties) interface{} {
	serviceName := component.Name
	labels := make(map[string]string)
	labels[config.LabelCli] = fmt.Sprintf("%s-%s", component.AppId, component.Name)
	labels[config.LabelAppId] = component.AppId

	var ContainerPort []corev1.ContainerPort
	for _, v := range properties.Ports {
		ContainerPort = append(ContainerPort, corev1.ContainerPort{
			ContainerPort: v.Port,
		})
	}

	var envs []corev1.EnvVar
	for k, v := range properties.Env {
		envs = append(envs, corev1.EnvVar{Name: k, Value: v})
	}

	for k, v := range properties.Labels {
		labels[k] = v
	}

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceName,
			Namespace: "default",
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &component.Replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  serviceName,
							Image: properties.Image,
							Ports: ContainerPort,
							Env:   envs,
						},
					},
				},
			},
		},
	}

	// Apply all registered traits to the deployment.
	// If applying traits fails, we return nil to prevent deploying a misconfigured workload.
	if err := traits.ApplyTraits(component, deployment); err != nil {
		klog.Errorf("Failed to apply traits to component %s: %v. Aborting resource generation.", component.Name, err)
		return nil
	}

	return deployment
}

func (c *DeployJobCtl) ApplyDeployment(ctx context.Context, deploy *appsv1.Deployment) (*appsv1.Deployment, error) {
	deploy.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "apps",
		Version: "v1",
		Kind:    "Deployment",
	})

	cleanObjectMeta(&deploy.ObjectMeta) // 清理会引发冲突的字段

	patchBytes, err := json.Marshal(deploy)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal deployment: %w", err)
	}

	result, err := c.client.AppsV1().Deployments(deploy.Namespace).Patch(ctx,
		deploy.Name,
		types.ApplyPatchType,
		patchBytes,
		metav1.PatchOptions{
			FieldManager: "kubemin-cli",      // 必须有：用于字段归属跟踪
			Force:        pointer.Bool(true), // 避免字段冲突阻塞更新
		})

	if err != nil {
		return nil, fmt.Errorf("apply deployment failed: %w", err)
	}
	return result, nil
}

func isDeploymentChanged(current, desired *appsv1.Deployment) bool {
	c1 := current.Spec.Template.Spec.Containers
	c2 := desired.Spec.Template.Spec.Containers

	if len(c1) != len(c2) {
		return true
	}

	for i := range c1 {
		if c1[i].Image != c2[i].Image {
			return true
		}

		if !compareContainerPorts(c1[i].Ports, c2[i].Ports) {
			return true
		}

		if !compareEnvVars(c1[i].Env, c2[i].Env) {
			return true
		}

		if !compareResources(c1[i].Resources, c2[i].Resources) {
			return true
		}

		if !compareVolumeMounts(c1[i].VolumeMounts, c2[i].VolumeMounts) {
			return true
		}
	}

	if !compareVolumes(current.Spec.Template.Spec.Volumes, desired.Spec.Template.Spec.Volumes) {
		return true
	}

	return false
}

func compareContainerPorts(a, b []corev1.ContainerPort) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].ContainerPort != b[i].ContainerPort {
			return false
		}
	}
	return true
}

func compareEnvVars(a, b []corev1.EnvVar) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Name != b[i].Name || a[i].Value != b[i].Value {
			return false
		}
	}
	return true
}

func compareResources(a, b corev1.ResourceRequirements) bool {
	return a.Requests.Cpu().Cmp(*b.Requests.Cpu()) == 0 &&
		a.Requests.Memory().Cmp(*b.Requests.Memory()) == 0 &&
		a.Limits.Cpu().Cmp(*b.Limits.Cpu()) == 0 &&
		a.Limits.Memory().Cmp(*b.Limits.Memory()) == 0
}

func compareVolumeMounts(a, b []corev1.VolumeMount) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].MountPath != b[i].MountPath || a[i].Name != b[i].Name {
			return false
		}
	}
	return true
}

func compareVolumes(a, b []corev1.Volume) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Name != b[i].Name {
			return false
		}
		// 这里只对常见的 Volume 类型进行对比（如 EmptyDir、ConfigMap、Secret、PVC）
		if a[i].VolumeSource.ConfigMap != nil || b[i].VolumeSource.ConfigMap != nil {
			if a[i].VolumeSource.ConfigMap == nil || b[i].VolumeSource.ConfigMap == nil ||
				a[i].VolumeSource.ConfigMap.Name != b[i].VolumeSource.ConfigMap.Name {
				return false
			}
		}
		// 可扩展支持 PVC、Secret、HostPath 等
	}
	return true
}

func cleanObjectMeta(meta *metav1.ObjectMeta) {
	meta.ResourceVersion = ""
	meta.UID = ""
	meta.CreationTimestamp = metav1.Time{}
	meta.ManagedFields = nil
}
