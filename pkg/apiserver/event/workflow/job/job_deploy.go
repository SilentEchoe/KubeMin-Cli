package job

import (
	"KubeMin-Cli/pkg/apiserver/config"
	"KubeMin-Cli/pkg/apiserver/domain/model"
	"KubeMin-Cli/pkg/apiserver/infrastructure/datastore"
	"context"
	"fmt"
	"sync"
	"time"

	app "k8s.io/api/apps/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
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

	isAlreadyExists, err := c.deploymentExists(ctx, deploy.Name, deploy.Namespace)
	if err != nil {
		return fmt.Errorf("failed to check deployment existence: %w", err)
	}

	if !isAlreadyExists {
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

func (c *DeployJobCtl) deploymentExists(ctx context.Context, name, namespaces string) (bool, error) {
	_, err := c.client.AppsV1().Deployments(namespaces).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func GenerateWebService(component *model.ApplicationComponent, properties *model.Properties) interface{} {
	serviceName := component.Name
	labels := make(map[string]string)
	labels["kube-min-cli"] = fmt.Sprintf("%s-%s", component.AppId, component.Name)
	labels["kube-min-cli-appId"] = component.AppId

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

	return deployment
}
