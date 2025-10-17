package traits

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"

	spec "KubeMin-Cli/pkg/apiserver/domain/spec"
	"KubeMin-Cli/pkg/apiserver/utils"
)

// SidecarProcessor materializes additional containers attached to the Pod.
// It also supports nested traits (except nested sidecars) applied to the sidecar itself.
type SidecarProcessor struct{}

// Name returns the name of the trait.
func (s *SidecarProcessor) Name() string {
	return "sidecar"
}

// Process adds sidecar containers to the workload, recursively applying any nested traits.
func (s *SidecarProcessor) Process(ctx *TraitContext) (*TraitResult, error) {
	sidecarTraits, ok := ctx.TraitData.([]spec.SidecarTraitsSpec)
	if !ok {
		return nil, fmt.Errorf("unexpected type for sidecar trait: %T", ctx.TraitData)
	}

	finalResult := &TraitResult{
		VolumeMounts:   make(map[string][]corev1.VolumeMount),
		EnvFromSources: make(map[string][]corev1.EnvFromSource),
		EnvVars:        make(map[string][]corev1.EnvVar),
	}

	for _, sidecarSpec := range sidecarTraits {
		if sidecarSpec.Image == "" {
			return nil, fmt.Errorf("sidecar for component %s must have an image", ctx.Component.Name)
		}
		// As per the design, sidecars cannot have nested sidecars.
		if len(sidecarSpec.Traits.Sidecar) > 0 {
			return nil, fmt.Errorf("sidecar '%s' must not contain nested sidecars", sidecarSpec.Name)
		}

		sidecarName := sidecarSpec.Name
		if sidecarName == "" {
			sidecarName = fmt.Sprintf("%s-sidecar-%s", ctx.Component.Name, utils.RandStringBytes(4))
		}

		// Convert env map to env vars
		var envVars []corev1.EnvVar
		for k, v := range sidecarSpec.Env {
			envVars = append(envVars, corev1.EnvVar{Name: k, Value: v})
		}

		// Recursively apply nested traits, excluding 'sidecar' and 'init' itself.
		nestedResult, err := applyTraitsRecursive(ctx.Component, ctx.Workload, &sidecarSpec.Traits, []string{"sidecar", "init"})
		if err != nil {
			return nil, fmt.Errorf("failed to process nested traits for sidecar %s: %w", sidecarName, err)
		}

		// The sidecar container gets the volume mounts from its nested traits.
		// The component name is used as the key for the main container's resources.
		var volumeMounts []corev1.VolumeMount
		if nestedResult != nil {
			if mounts, ok := nestedResult.VolumeMounts[ctx.Component.Name]; ok {
				volumeMounts = mounts
			}
		}

		// The sidecar container also gets the EnvFrom and EnvVars from its nested traits.
		var envFromSources []corev1.EnvFromSource
		if nestedResult != nil {
			if envFrom, ok := nestedResult.EnvFromSources[ctx.Component.Name]; ok {
				envFromSources = envFrom
			}
			if nestedEnvVars, ok := nestedResult.EnvVars[ctx.Component.Name]; ok {
				envVars = append(envVars, nestedEnvVars...)
			}
		}

		sidecarContainer := corev1.Container{
			Name:            sidecarName,
			Image:           sidecarSpec.Image,
			Command:         sidecarSpec.Command,
			Args:            sidecarSpec.Args,
			Env:             envVars,
			EnvFrom:         envFromSources,
			VolumeMounts:    volumeMounts,
			ImagePullPolicy: corev1.PullIfNotPresent,
		}

		// Apply probes if present
		if nestedResult != nil {
			if nestedResult.LivenessProbe != nil {
				sidecarContainer.LivenessProbe = nestedResult.LivenessProbe
			}
			if nestedResult.ReadinessProbe != nil {
				sidecarContainer.ReadinessProbe = nestedResult.ReadinessProbe
			}
			if nestedResult.StartupProbe != nil {
				sidecarContainer.StartupProbe = nestedResult.StartupProbe
			}
		}

		// Apply nested resource requirements to the sidecar if present
		if nestedResult != nil && nestedResult.ResourceRequirements != nil {
			sidecarContainer.Resources = *nestedResult.ResourceRequirements
		}

		// Add the created container to the final result.
		finalResult.Containers = append(finalResult.Containers, sidecarContainer)

		// Merge volumes and additional objects from the nested traits into the final result.
		if nestedResult != nil {
			finalResult.Volumes = append(finalResult.Volumes, nestedResult.Volumes...)
			finalResult.AdditionalObjects = append(finalResult.AdditionalObjects, nestedResult.AdditionalObjects...)
		}

		klog.V(3).Infof("Constructed sidecar container %s for component %s", sidecarName, ctx.Component.Name)
	}

	return finalResult, nil
}
