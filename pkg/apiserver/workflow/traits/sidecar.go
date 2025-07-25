package traits

import (
	"KubeMin-Cli/pkg/apiserver/domain/model"
	"fmt"

	corev1 "k8s.io/api/core/v1"
)

func init() {
	Register(&SidecarProcessor{})
}

// SidecarProcessor handles the logic for the 'sidecar' trait.
type SidecarProcessor struct{}

// Name returns the name of the trait.
func (s *SidecarProcessor) Name() string {
	return "sidecar"
}

// Process adds sidecar containers to the workload's pod template.
func (s *SidecarProcessor) Process(workload interface{}, traitData interface{}, component *model.ApplicationComponent) error {
	sidecarTraits, ok := traitData.([]model.SidecarSpec)
	if !ok {
		return fmt.Errorf("unexpected type for sidecar trait: %T", traitData)
	}

	podTemplate, err := GetPodTemplateSpec(workload)
	if err != nil {
		return err
	}

	for _, sc := range sidecarTraits {
		if sc.Image == "" {
			return fmt.Errorf("sidecar for component %s must have an image", component.Name)
		}

		sidecarContainer := corev1.Container{
			Name:    sc.Name,
			Image:   sc.Image,
			Command: sc.Command,
			Args:    sc.Args,
			Env:     toKubeEnvVars(sc.Env),
		}

		podTemplate.Spec.Containers = append(podTemplate.Spec.Containers, sidecarContainer)
	}

	return nil
}

// toKubeEnvVars converts a map[string]string to a slice of corev1.EnvVar.
func toKubeEnvVars(env map[string]string) []corev1.EnvVar {
	if env == nil {
		return nil
	}
	vars := make([]corev1.EnvVar, 0, len(env))
	for k, v := range env {
		vars = append(vars, corev1.EnvVar{Name: k, Value: v})
	}
	return vars
}