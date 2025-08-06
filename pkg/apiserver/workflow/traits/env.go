package traits

import (
	"KubeMin-Cli/pkg/apiserver/config"
	"KubeMin-Cli/pkg/apiserver/domain/model"
	"fmt"

	corev1 "k8s.io/api/core/v1"
)

func init() {
	Register(&EnvsProcessor{})
	Register(&EnvFromProcessor{})
}

// EnvsProcessor handles the user-friendly `envs` trait.
// Its primary job is to translate the simplified spec into native Kubernetes EnvVars.
type EnvsProcessor struct{}

// Name returns the name of the trait.
func (p *EnvsProcessor) Name() string {
	return "envs"
}

// Process translates the []model.SimplifiedEnvSpec into []corev1.EnvVar.
func (p *EnvsProcessor) Process(ctx *TraitContext) (*TraitResult, error) {
	simplifiedEnvs, ok := ctx.TraitData.([]model.SimplifiedEnvSpec)
	if !ok {
		return nil, fmt.Errorf("unexpected type for envs trait: expected []model.SimplifiedEnvSpec, got %T", ctx.TraitData)
	}

	var nativeEnvs []corev1.EnvVar
	for _, spec := range simplifiedEnvs {
		nativeEnv, err := translateToNativeEnvVar(spec)
		if err != nil {
			return nil, fmt.Errorf("failed to translate env spec for '%s': %w", spec.Name, err)
		}
		nativeEnvs = append(nativeEnvs, *nativeEnv)
	}

	return &TraitResult{
		EnvVars: map[string][]corev1.EnvVar{
			ctx.Component.Name: nativeEnvs,
		},
	}, nil
}

// EnvFromProcessor handles the `envFrom` trait for batch importing environment variables.
type EnvFromProcessor struct{}

// Name returns the name of the trait.
func (p *EnvFromProcessor) Name() string {
	return "envFrom"
}

// Process converts []model.EnvFromSourceSpec into []corev1.EnvFromSource.
func (p *EnvFromProcessor) Process(ctx *TraitContext) (*TraitResult, error) {
	envFromTraits, ok := ctx.TraitData.([]model.EnvFromSourceSpec)
	if !ok {
		return nil, fmt.Errorf("unexpected type for envFrom trait: %T", ctx.TraitData)
	}

	var envFromSources []corev1.EnvFromSource
	for _, trait := range envFromTraits {
		if trait.SourceName == "" {
			return nil, fmt.Errorf("envFrom trait requires a sourceName")
		}
		switch trait.Type {
		case config.StorageTypeConfig:
			envFromSources = append(envFromSources, corev1.EnvFromSource{
				ConfigMapRef: &corev1.ConfigMapEnvSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: trait.SourceName},
				},
			})
		case config.StorageTypeSecret:
			envFromSources = append(envFromSources, corev1.EnvFromSource{
				SecretRef: &corev1.SecretEnvSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: trait.SourceName},
				},
			})
		default:
			return nil, fmt.Errorf("unsupported envFrom type: %s", trait.Type)
		}
	}

	return &TraitResult{
		EnvFromSources: map[string][]corev1.EnvFromSource{
			ctx.Component.Name: envFromSources,
		},
	}, nil
}

// translateToNativeEnvVar is the core translation logic for the simplified structure.
func translateToNativeEnvVar(spec model.SimplifiedEnvSpec) (*corev1.EnvVar, error) {
	envVar := &corev1.EnvVar{Name: spec.Name}

	src := spec.ValueFrom

	switch {
	case src.Static != nil:
		envVar.Value = *src.Static
	case src.Field != nil:
		envVar.ValueFrom = &corev1.EnvVarSource{
			FieldRef: &corev1.ObjectFieldSelector{
				APIVersion: "v1",
				FieldPath:  *src.Field,
			},
		}
	case src.Secret != nil:
		envVar.ValueFrom = &corev1.EnvVarSource{
			SecretKeyRef: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: src.Secret.Name},
				Key:                  src.Secret.Key,
			},
		}
	case src.Config != nil:
		envVar.ValueFrom = &corev1.EnvVarSource{
			ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: src.Config.Name},
				Key:                  src.Config.Key,
			},
		}
	default:
		return nil, fmt.Errorf("invalid valueFrom spec for env var '%s': exactly one source must be specified", spec.Name)
	}

	return envVar, nil
}
