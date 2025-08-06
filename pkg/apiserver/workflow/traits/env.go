package traits

import (
	"KubeMin-Cli/pkg/apiserver/config"
	"KubeMin-Cli/pkg/apiserver/domain/model"
	"fmt"

	corev1 "k8s.io/api/core/v1"
)

func init() {
	Register(&EnvFromProcessor{})
	Register(&EnvsProcessor{})
}

// EnvFromProcessor handles the logic for the 'envFrom' trait.
type EnvFromProcessor struct{}

// Name returns the name of the trait.
func (p *EnvFromProcessor) Name() string {
	return "envFrom"
}

// Process handles the 'envFrom' trait.
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

// EnvsProcessor handles the logic for the 'envs' trait.
type EnvsProcessor struct{}

// Name returns the name of the trait.
func (p *EnvsProcessor) Name() string {
	return "envs"
}

// Process handles the 'envs' trait.
func (p *EnvsProcessor) Process(ctx *TraitContext) (*TraitResult, error) {
	envsTraits, ok := ctx.TraitData.([]model.EnvVarSpec)
	if !ok {
		return nil, fmt.Errorf("unexpected type for envs trait: %T", ctx.TraitData)
	}

	var envVars []corev1.EnvVar
	for _, trait := range envsTraits {
		envVar := corev1.EnvVar{Name: trait.Name}
		if trait.Value != "" {
			envVar.Value = trait.Value
		} else if trait.ValueFrom != nil {
			envVar.ValueFrom = &corev1.EnvVarSource{}
			if trait.ValueFrom.SecretKeyRef != nil {
				envVar.ValueFrom.SecretKeyRef = &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: trait.ValueFrom.SecretKeyRef.Name},
					Key:                  trait.ValueFrom.SecretKeyRef.Key,
				}
			} else if trait.ValueFrom.FieldRef != nil {
				envVar.ValueFrom.FieldRef = &corev1.ObjectFieldSelector{
					APIVersion: "v1",
					FieldPath:  trait.ValueFrom.FieldRef.FieldPath,
				}
			}
		}
		envVars = append(envVars, envVar)
	}

	return &TraitResult{
		EnvVars: map[string][]corev1.EnvVar{
			ctx.Component.Name: envVars,
		},
	}, nil
}
