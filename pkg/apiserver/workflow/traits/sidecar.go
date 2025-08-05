package traits

import (
	"KubeMin-Cli/pkg/apiserver/domain/model"
	"KubeMin-Cli/pkg/apiserver/utils"
	"fmt"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
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

// Process converts sidecar specs into containers, recursively applying any nested traits.
func (s *SidecarProcessor) Process(ctx *TraitContext) (*TraitResult, error) {
	sidecarTraits, ok := ctx.TraitData.([]model.SidecarSpec)
	if !ok {
		return nil, fmt.Errorf("unexpected type for sidecar trait: %T", ctx.TraitData)
	}

	finalResult := &TraitResult{
		VolumeMounts: make(map[string][]corev1.VolumeMount),
	}

	for _, sc := range sidecarTraits {
		// Prevent nested sidecars within a sidecar.
		if len(sc.Traits.Sidecar) > 0 {
			return nil, fmt.Errorf("sidecar '%s' must not contain nested sidecars", sc.Name)
		}

		sidecarName := sc.Name
		if sidecarName == "" {
			sidecarName = fmt.Sprintf("%s-sidecar-%s", ctx.Component.Name, utils.RandStringBytes(4))
		}

		// Build env
		var containerEnvs []corev1.EnvVar
		for k, v := range sc.Env {
			containerEnvs = append(containerEnvs, corev1.EnvVar{Name: k, Value: v})
		}

		// Recursively apply nested traits, excluding the 'sidecar' trait itself.
		nestedResult, err := applyTraitsRecursive(ctx.Component, ctx.Workload, &sc.Traits, []string{"sidecar"})
		if err != nil {
			return nil, fmt.Errorf("failed to process nested traits for sidecar %s: %w", sidecarName, err)
		}

		// The sidecar container gets the volume mounts from its nested traits.
		var volumeMounts []corev1.VolumeMount
		if nestedResult != nil {
			for _, mounts := range nestedResult.VolumeMounts {
				volumeMounts = append(volumeMounts, mounts...)
			}
		}

		c := corev1.Container{
			Name:         sidecarName,
			Image:        sc.Image,
			Command:      sc.Command,
			Args:         sc.Args,
			Env:          containerEnvs,
			VolumeMounts: volumeMounts,
		}

		// Add the created container to the final result.
		finalResult.Containers = append(finalResult.Containers, c)

		// Merge volumes and additional objects from the nested traits into the final result.
		if nestedResult != nil {
			finalResult.Volumes = append(finalResult.Volumes, nestedResult.Volumes...)
			finalResult.AdditionalObjects = append(finalResult.AdditionalObjects, nestedResult.AdditionalObjects...)
		}

		klog.V(3).Infof("Constructed sidecar container '%s' for component '%s'", sidecarName, ctx.Component.Name)
	}

	return finalResult, nil
}
