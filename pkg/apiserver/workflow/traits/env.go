package traits

import (
	"KubeMin-Cli/pkg/apiserver/config"
	"KubeMin-Cli/pkg/apiserver/domain/model"
	"fmt"

	corev1 "k8s.io/api/core/v1"
)

func init() {
	Register(&EnvProcessor{})
}

// EnvProcessor is a trait processor that handles environment variables
// from ConfigMaps and Secrets. It acts as a pure resource generator.
type EnvProcessor struct{}

// Name returns the name of the trait processor.
func (e *EnvProcessor) Name() string { return "env" }

// Process generates EnvFromSource definitions based on the trait properties
// and returns them in a TraitResult. It does not modify the workload directly.
func (e *EnvProcessor) Process(ctx *TraitContext) (*TraitResult, error) {
	// 1. Cast TraitData to the expected type.
	envTraits, ok := ctx.TraitData.([]model.EnvFromSourceSpec)
	if !ok {
		return nil, fmt.Errorf("unexpected type for env trait: %T", ctx.TraitData)
	}

	// 2. Build the EnvFrom sources from the trait specifications.
	var envFromSources []corev1.EnvFromSource
	for _, envTrait := range envTraits {
		if envTrait.SourceName == "" {
			return nil, fmt.Errorf("env trait requires a sourceName")
		}
		switch envTrait.Type {
		case config.StorageTypeConfig:
			envFromSources = append(envFromSources, corev1.EnvFromSource{
				ConfigMapRef: &corev1.ConfigMapEnvSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: envTrait.SourceName},
				},
			})
		case config.StorageTypeSecret:
			envFromSources = append(envFromSources, corev1.EnvFromSource{
				SecretRef: &corev1.SecretEnvSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: envTrait.SourceName},
				},
			})
		default:
			return nil, fmt.Errorf("unsupported env type: %s", envTrait.Type)
		}
	}

	// 3. Return the generated resources in a TraitResult.
	// The key for the map is the component name, which acts as the default
	// target container name for top-level trait processing.
	return &TraitResult{
		EnvFromSources: map[string][]corev1.EnvFromSource{
			ctx.Component.Name: envFromSources,
		},
	}, nil
}
