package clients

import (
	"flag"
	"fmt"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	"path/filepath"
)

var kubeClient *kubernetes.Clientset
var kubeConfig *string

func SetKubeConfig() (*string, error) {
	if kubeConfig == nil {
		return nil, fmt.Errorf("please call SetKubeConfig first")
	}
	return kubeConfig, nil
}

// KubeConfigLocal Get the local KubeConfig
func KubeConfigLocal() {
	if home := homedir.HomeDir(); home != "" {
		kubeConfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	} else {
		kubeConfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	}
	NewKubeClient()
}

func GetKubeClient() *kubernetes.Clientset {
	if kubeClient == nil {
		return nil
	}
	return kubeClient
}

func NewKubeClient() *kubernetes.Clientset {
	config, err := clientcmd.BuildConfigFromFlags("", *kubeConfig)
	if err != nil {
		panic(err.Error())
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}
	kubeClient = clientset
	return kubeClient
}
