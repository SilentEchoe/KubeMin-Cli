package traits

import (
	"KubeMin-Cli/pkg/apiserver/config"
	"KubeMin-Cli/pkg/apiserver/domain/model"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func init() {
	Register(&EnvProcessor{})
}

// EnvProcessor handles the 'env' trait.
type EnvProcessor struct{}

// Name returns the name of the trait.
func (e *EnvProcessor) Name() string { return "env" }

// Process applies the environment configuration to the main application container.
func (e *EnvProcessor) Process(ctx *TraitContext) (*TraitResult, error) {
	envTraits, ok := ctx.TraitData.([]model.EnvFromSourceSpec)
	if !ok {
		return nil, fmt.Errorf("unexpected type for env trait: %T", ctx.TraitData)
	}

	podSpec, err := getPodTemplateSpecFromWorkload(ctx.Workload)
	if err != nil {
		return nil, err
	}

	// Find the main container by matching the image from the component definition.
	mainContainer := findMainContainer(podSpec, ctx.Component.Image)
	if mainContainer == nil {
		return nil, fmt.Errorf("main container with image '%s' not found in workload", ctx.Component.Image)
	}

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

	// Append the EnvFrom sources to the identified main container.
	mainContainer.EnvFrom = append(mainContainer.EnvFrom, envFromSources...)

	return &TraitResult{}, nil
}

// findMainContainer iterates through containers in a PodSpec and returns the one
// whose image matches the provided component image.
func findMainContainer(podSpec *corev1.PodTemplateSpec, componentImage string) *corev1.Container {
	for i, container := range podSpec.Spec.Containers {
		if container.Image == componentImage {
			return &podSpec.Spec.Containers[i] // Return a pointer to the container in the slice.
		}
	}
	return nil
}

// getPodTemplateSpecFromWorkload extracts the PodTemplateSpec from a supported workload type.
func getPodTemplateSpecFromWorkload(workload runtime.Object) (*corev1.PodTemplateSpec, error) {
	switch w := workload.(type) {
	case *appsv1.Deployment:
		return &w.Spec.Template, nil
	case *appsv1.StatefulSet:
		return &w.Spec.Template, nil
	default:
		return nil, fmt.Errorf("unsupported workload type for env trait: %T", workload)
	}
}
