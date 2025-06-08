package job

import (
	"KubeMin-Cli/pkg/apiserver/config"
	"KubeMin-Cli/pkg/apiserver/domain/model"
	"KubeMin-Cli/pkg/apiserver/infrastructure/datastore"
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

type DeployStatefulSetJobCtl struct {
	namespace string
	job       *model.JobTask
	client    *kubernetes.Clientset
	store     datastore.DataStore
	ack       func()
}

func NewDeployStatefulSetJobCtl(job *model.JobTask, client *kubernetes.Clientset, store datastore.DataStore, ack func()) *DeployStatefulSetJobCtl {
	if job == nil {
		klog.Errorf("DeployStatefulSetJobCtl: job is nil")
		return nil
	}
	return &DeployStatefulSetJobCtl{
		namespace: job.Namespace,
		job:       job,
		client:    client,
		store:     store,
		ack:       ack,
	}
}

func (c *DeployStatefulSetJobCtl) Clean(ctx context.Context) {}

// SaveInfo  创建Job的详情信息
func (c *DeployStatefulSetJobCtl) SaveInfo(ctx context.Context) error {
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

func (c *DeployStatefulSetJobCtl) Run(ctx context.Context) {
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

func (c *DeployStatefulSetJobCtl) run(ctx context.Context) error {
	if c.client == nil {
		return fmt.Errorf("client is nil")
	}

	// 任务完成
	c.job.Status = config.StatusCompleted
	c.ack()
	return nil
}

func (c *DeployStatefulSetJobCtl) wait(ctx context.Context) {}

func GenerateStoreService(component *model.ApplicationComponent, properties *model.Properties, traits *model.Traits) interface{} {
	if component.Namespace == "" {
		component.Namespace = config.DefaultNamespace
	}

	serviceName := component.Name
	labels := buildLabels(component, properties)

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

	statefulSet := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceName,
			Namespace: component.Namespace,
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: &component.Replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					//TerminationGracePeriodSeconds: ,
					Containers: []corev1.Container{
						{
							Name:         serviceName,
							Image:        properties.Image,
							Ports:        ContainerPort,
							Env:          envs,
							VolumeMounts: make([]corev1.VolumeMount, 0),
						},
					},
				},
			},
			VolumeClaimTemplates: make([]corev1.PersistentVolumeClaim, 0),
		},
	}
	return statefulSet
}

func buildLabels(c *model.ApplicationComponent, p *model.Properties) map[string]string {
	labels := map[string]string{
		"kube-min-cli":       fmt.Sprintf("%s-%s", c.AppId, c.Name),
		"kube-min-cli-appId": c.AppId,
	}
	for k, v := range p.Labels {
		labels[k] = v
	}
	return labels
}

// BuildPVC 构造一个标准 PVC 模板
func BuildPVC(name string, storageClass string, size string) corev1.PersistentVolumeClaim {
	qty, err := resource.ParseQuantity(size)
	if err != nil {
		panic(fmt.Errorf("invalid storage size %s: %w", size, err))
	}

	return corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteOnce,
			},
			StorageClassName: &storageClass,
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: qty,
				},
			},
		},
	}
}
