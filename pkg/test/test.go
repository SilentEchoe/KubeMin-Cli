package main

import (
	"context"
	"flag"
	"fmt"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	"path/filepath"
)

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

	// 获取 Deployment 对象
	deploy, err := clientset.AppsV1().Deployments(namespace).Get(
		context.TODO(),
		deploymentName,
		metav1.GetOptions{},
	)
	if err != nil {
		panic(fmt.Errorf("failed to get Deployment: %v", err))
	}

	fmt.Printf("Deployment: %s\n", deploymentName)
	fmt.Printf("Desired Replicas: %d\n", *deploy.Spec.Replicas)
	fmt.Printf("Current Replicas: %d\n", deploy.Status.Replicas)
	fmt.Printf("Available Replicas: %d\n", deploy.Status.AvailableReplicas)
	fmt.Printf("Ready Replicas: %d\n", deploy.Status.ReadyReplicas)
}
