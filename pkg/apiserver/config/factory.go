package config

import (
	"KubeMin-Cli/pkg/apiserver/utils"
	"context"
	"github.com/oam-dev/kubevela/pkg/utils/apply"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/kubernetes"
)

// Factory handle the config
type Factory interface {
}

// Dispatcher is a client for apply resources.
type Dispatcher func(context.Context, []*unstructured.Unstructured, []apply.ApplyOption) error

type kubeConfigFactory struct {
	cli      *kubernetes.Clientset
	apiApply Dispatcher
}

// NewConfigFactory create a config factory instance
func NewConfigFactory(cli *utils.AuthClient) Factory {
	return &kubeConfigFactory{cli: cli.Clientset, apiApply: defaultDispatcher(cli)}
}

func defaultDispatcher(cli *utils.AuthClient) Dispatcher {
	return nil
}
