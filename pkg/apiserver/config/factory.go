package config

import (
	"context"
	"github.com/oam-dev/kubevela/pkg/utils/apply"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Factory handle the config
type Factory interface {
}

// Dispatcher is a client for apply resources.
type Dispatcher func(context.Context, []*unstructured.Unstructured, []apply.ApplyOption) error

type kubeConfigFactory struct {
	cli      client.Client
	apiApply Dispatcher
}

// NewConfigFactory create a config factory instance
func NewConfigFactory(cli client.Client) Factory {
	return &kubeConfigFactory{cli: cli, apiApply: defaultDispatcher(cli)}
}

func defaultDispatcher(cli client.Client) Dispatcher {
	api := apply.NewAPIApplicator(cli)
	return func(ctx context.Context, manifests []*unstructured.Unstructured, ao []apply.ApplyOption) error {
		for _, m := range manifests {
			if err := api.Apply(ctx, m, ao...); err != nil {
				return err
			}
		}
		return nil
	}
}
