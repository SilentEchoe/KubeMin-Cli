package clients

import (
	"flag"
	"path/filepath"

	"k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"

	"KubeMin-Cli/pkg/apiserver/config"
)

var kubeClient *kubernetes.Clientset

func NewClient(config config.Config) error {
	var localKubeConfig *string
	var kubeConfig *restclient.Config
	if config.KubeConfig != "" && !config.LocalCluster {
		localKubeConfig = &config.KubeConfig
	}

	if config.LocalCluster {
		if home := homedir.HomeDir(); home != "" {
			localKubeConfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
		} else {
			localKubeConfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
		}
		_, err := clientcmd.BuildConfigFromFlags("", *localKubeConfig)
		if err != nil {
			return err
		}
	}
	clientset, err := kubernetes.NewForConfig(kubeConfig)
	if err != nil {
		return err
	}

	kubeClient = clientset
	return nil
}

// SetKubeConfig generate the kube config from the config of apiserver
func SetKubeConfig() error {
	return nil
}
