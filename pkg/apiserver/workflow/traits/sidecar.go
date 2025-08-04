package traits

import (
	"KubeMin-Cli/pkg/apiserver/domain/model"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
)

func init() {
	Register(&SidecarProcessor{})
}

// SidecarProcessor handles the logic for the 'sidecar' trait
type SidecarProcessor struct{}

// Name returns the name of the trait
func (s *SidecarProcessor) Name() string {
	return "sidecar"
}

// Process adds sidecar containers to the workload
func (s *SidecarProcessor) Process(ctx *TraitContext) error {
	sidecarTraits, ok := ctx.TraitData.([]model.SidecarSpec)
	if !ok {
		return fmt.Errorf("unexpected type for sidecar trait: %T", ctx.TraitData)
	}

	podTemplate, err := ctx.GetPodTemplate()
	if err != nil {
		return err
	}

	if len(podTemplate.Spec.Containers) == 0 {
		return fmt.Errorf("cannot apply sidecar trait to component %s with no main container", ctx.Component.Name)
	}

	// Assume the first container is the main application container
	mainContainer := &podTemplate.Spec.Containers[0]

	for _, sc := range sidecarTraits {
		if sc.Image == "" {
			return fmt.Errorf("sidecar for component %s must have an image", ctx.Component.Name)
		}

		sidecarName := sc.Name
		if sidecarName == "" {
			sidecarName = fmt.Sprintf("%s-sidecar-%s", ctx.Component.Name, generateRandomSuffix())
		}

		// Convert env map to env vars
		var envVars []corev1.EnvVar
		for k, v := range sc.Env {
			envVars = append(envVars, corev1.EnvVar{Name: k, Value: v})
		}

		sidecarContainer := corev1.Container{
			Name:         sidecarName,
			Image:        sc.Image,
			Command:      sc.Command,
			Args:         sc.Args,
			Env:          envVars,
			VolumeMounts: mainContainer.VolumeMounts, // Inherit volume mounts from main container
		}

		ctx.AddContainer(sidecarContainer)
		klog.V(3).Infof("Added sidecar container %s to component %s", sidecarName, ctx.Component.Name)
	}

	return nil
}

func generateRandomSuffix() string {
	// Simple random suffix generation - you can use your existing utils.RandStringBytes
	return "sidecar"
}
