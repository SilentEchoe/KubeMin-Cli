package job

import (
	"KubeMin-Cli/pkg/apiserver/utils"
	"context"
	"fmt"
	"github.com/fatih/color"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	"KubeMin-Cli/pkg/apiserver/config"
	"KubeMin-Cli/pkg/apiserver/domain/model"
	"KubeMin-Cli/pkg/apiserver/infrastructure/datastore"
	traitsPlu "KubeMin-Cli/pkg/apiserver/workflow/traits"
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
	//after the deployment is completed, synchronize the status.
	c.wait(ctx)
}

func (c *DeployStatefulSetJobCtl) run(ctx context.Context) error {
	if c.client == nil {
		return fmt.Errorf("client is nil")
	}

	var statefulSet *appsv1.StatefulSet
	if d, ok := c.job.JobInfo.(*appsv1.StatefulSet); ok {
		statefulSet = d
	} else {
		return fmt.Errorf("deploy Job Job.Info Conversion type failure")
	}

	result, err := c.client.AppsV1().StatefulSets(statefulSet.Namespace).Create(ctx, statefulSet, metav1.CreateOptions{})
	if err != nil {
		klog.Errorf("failed to create statefulSet %q namespace: %q : %v", statefulSet.Name, statefulSet.Namespace, err)
		return err
	}
	klog.Infof("JobTask Deploy Successfully %q.\n", result.GetObjectMeta().GetName())

	c.job.Status = config.StatusCompleted
	c.ack()
	return nil
}

func (c *DeployStatefulSetJobCtl) wait(ctx context.Context) {
	timeout := time.After(time.Duration(c.timeout()) * time.Second)

	for {
		select {
		case <-timeout:
			klog.Infof(fmt.Sprintf("%s", c.job.Name))
			newResources, err := getStatefulSetStatus(c.client, c.job.Namespace, c.job.Name)
			if err != nil || newResources == nil {
				msg := fmt.Sprintf("get resource owner info error: %v", err)
				klog.Errorf(msg)
				c.job.Status = config.StatusFailed
			}
		default:
			time.Sleep(2 * time.Second)
			newResources, err := getStatefulSetStatus(c.client, c.job.Namespace, c.job.Name)
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

func (c *DeployStatefulSetJobCtl) timeout() int64 {
	if c.job.Timeout == 0 {
		c.job.Timeout = config.DeployTimeout
	}
	return c.job.Timeout
}

func GenerateStoreService(component *model.ApplicationComponent) interface{} {
	// 如果命名空间为空，则使用默认的命名空间
	if component.Namespace == "" {
		component.Namespace = config.DefaultNamespace
	}
	serviceName := component.Name

	properties := ParseProperties(component.Properties)

	// 构建标签
	labels := buildLabels(component, &properties)
	for k, v := range properties.Labels {
		labels[k] = v
	}

	// 构建需要开放的端口
	var ContainerPort []corev1.ContainerPort
	for _, v := range properties.Ports {
		ContainerPort = append(ContainerPort, corev1.ContainerPort{
			ContainerPort: v.Port,
		})
	}

	// 构建环境变量
	var envs []corev1.EnvVar
	for k, v := range properties.Env {
		envs = append(envs, corev1.EnvVar{Name: k, Value: v})
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
					Containers: []corev1.Container{
						{
							Name:            serviceName,
							Image:           component.Image,
							Ports:           ContainerPort,
							Env:             envs,
							ImagePullPolicy: corev1.PullAlways,
						},
					},
					TerminationGracePeriodSeconds: utils.ParseInt64(30),
				},
			},
		},
	}

	_, err := traitsPlu.ApplyTraits(component, statefulSet)
	if err != nil {
		klog.Errorf("StatefulSet Info %s Traits Error:%s", color.WhiteString(component.Namespace+"/"+component.Name), err)
		return nil
	}
	return statefulSet
}

func buildLabels(c *model.ApplicationComponent, p *model.Properties) map[string]string {
	labels := map[string]string{
		config.LabelCli:   fmt.Sprintf("%s-%s", c.AppId, c.Name),
		config.LabelAppId: c.AppId,
	}
	for k, v := range p.Labels {
		labels[k] = v
	}
	return labels
}

func getStatefulSetStatus(kubeClient *kubernetes.Clientset, namespace string, name string) (deployInfo *model.JobDeployInfo, err error) {
	statefulSet, err := kubeClient.AppsV1().StatefulSets(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			klog.Infof("StatefulSet deploy is nil")
			klog.Infoln(fmt.Sprintf("StatefulSet Name :%s, Namespace: %s", name, namespace))
			return nil, nil
		}
		return nil, err
	}
	klog.Infof(fmt.Sprintf("newResources:%s, Replicas:%d ,ReadyReplicas:%d ", statefulSet.Name, statefulSet.Spec.Replicas, statefulSet.Status.ReadyReplicas))
	isOk := false
	if *statefulSet.Spec.Replicas == statefulSet.Status.ReadyReplicas {
		isOk = true
	}
	return &model.JobDeployInfo{
		Name:          statefulSet.Name,
		Replicas:      *statefulSet.Spec.Replicas,
		ReadyReplicas: statefulSet.Status.ReadyReplicas,
		Ready:         isOk,
	}, nil
}
