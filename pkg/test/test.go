package main

import (
	"KubeMin-Cli/pkg/apiserver/domain/model"
	"context"
	"flag"
	"fmt"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	"k8s.io/klog/v2"
	"path/filepath"
	"time"
)

type JobDeployInfo struct {
	Name          string
	Ready         bool
	Replicas      int32 //期望副本数量
	ReadyReplicas int32 //就绪副本数量
}

func main() {
	var loadKubeClient *string
	if home := homedir.HomeDir(); home != "" {
		// 如果输入了kubeconfig参数，该参数的值就是kubeconfig文件的绝对路径，
		// 如果没有输入kubeconfig参数，就用默认路径~/.kube/config
		loadKubeClient = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	}

	if loadKubeClient == nil {
		fmt.Errorf("please call SetKubeConfig first")
		return
	}
	loadConf, err := clientcmd.BuildConfigFromFlags("", *loadKubeClient)
	if err != nil {
		panic(err)
	}

	clientset, err := kubernetes.NewForConfig(loadConf)

	deploymentName := "first-app-front"
	namespace := "default"

	timeout := time.After(20 * time.Second)

	for {
		select {
		case <-timeout:
			newResources, err := getDeploymentStatus(clientset, namespace, deploymentName)
			if err != nil || newResources == nil {
				msg := fmt.Sprintf("get resource owner info error: %v", err)
				klog.Errorf(msg)
				return
			}
		default:
			time.Sleep(2 * time.Second)
			newResources, err := getDeploymentStatus(clientset, namespace, deploymentName)
			if err != nil {
				msg := fmt.Sprintf("get resource owner info error: %v", err)
				klog.Errorf(msg)
				return
			}
			if newResources != nil {
				if newResources.Ready {
					klog.Infof("newResources is ready")
					return
				}
			}
		}
	}
}

func getDeploymentStatus(kubeClient *kubernetes.Clientset, namespace string, name string) (deployInfo *model.JobDeployInfo, err error) {
	deploy, err := kubeClient.AppsV1().Deployments(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			// Deployment 不存在，处理这种情况
			return nil, nil
		}
		return nil, err
	}
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
