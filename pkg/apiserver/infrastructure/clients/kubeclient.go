package clients

import (
	apiConfig "KubeMin-Cli/pkg/apiserver/config"
	"flag"
	"fmt"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	"path/filepath"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

/*
这里没有遵从 kubevela的做法,而是直接使用k8s的Clients
*/

// TODO 这里用的都是k8s内置的基础资源，所以先使用Clientset
var kubeClient *kubernetes.Clientset
var kubeConfig *rest.Config

// SetKubeClient for test
func SetKubeClient(c *kubernetes.Clientset) {
	kubeClient = c
}

func setKubeConfig(conf *rest.Config) (err error) {
	if conf == nil {
		conf, err = config.GetConfig()
		if err != nil {
			return err
		}
	}
	kubeConfig = conf
	//kubeConfig.Wrap(auth.NewImpersonatingRoundTripper)
	return nil
}

// SetKubeConfig generate the kube config from the config of apiserver
func SetKubeConfig(c apiConfig.Config) error {
	conf, err := config.GetConfig()
	if err != nil {
		return err
	}
	kubeConfig = conf
	kubeConfig.Burst = c.KubeBurst
	kubeConfig.QPS = float32(c.KubeQPS)
	return setKubeConfig(kubeConfig)
}

// GetKubeClient create and return kube runtime client
func GetKubeClient() (*kubernetes.Clientset, error) {
	if kubeClient != nil {
		return kubeClient, nil
	}
	if kubeConfig != nil {
		client, err := kubernetes.NewForConfig(kubeConfig)
		if err != nil {
			return nil, err
		}
		SetKubeClient(client)
		return client, nil
	}

	var loadKubeClient *string
	if home := homedir.HomeDir(); home != "" {
		// 如果输入了kubeconfig参数，该参数的值就是kubeconfig文件的绝对路径，
		// 如果没有输入kubeconfig参数，就用默认路径~/.kube/config
		loadKubeClient = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	}

	if loadKubeClient == nil {
		return nil, fmt.Errorf("please call SetKubeConfig first")
	}
	loadConf, err := clientcmd.BuildConfigFromFlags("", *loadKubeClient)
	if err != nil {
		return nil, err
	}
	loadClient, err := kubernetes.NewForConfig(loadConf)
	if err != nil {
		return nil, err
	}
	SetKubeClient(loadClient)
	return loadClient, nil
}

// GetKubeConfig create/get kube runtime config
func GetKubeConfig() (*rest.Config, error) {
	if kubeConfig == nil {
		return nil, fmt.Errorf("please call SetKubeConfig first")
	}
	return kubeConfig, nil
}
