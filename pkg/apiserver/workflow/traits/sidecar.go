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

// Process converts sidecar specs into containers and volumes.
func (s *SidecarProcessor) Process(ctx *TraitContext) (*TraitResult, error) {
	sidecarTraits, ok := ctx.TraitData.([]model.SidecarSpec)
	if !ok {
		return nil, fmt.Errorf("unexpected type for sidecar trait: %T", ctx.TraitData)
	}

	var containers []corev1.Container
	var volumes []corev1.Volume
	volumeNameSet := make(map[string]bool)

	for _, sc := range sidecarTraits {
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

		// For now, we assume storage defined within a sidecar is for that sidecar.
		// A more advanced implementation might share volumes across the pod.
		storageProcessor := &StorageProcessor{}
		storageCtx := NewTraitContext(ctx.Component, ctx.Workload, sc.Traits.Storage)
		storageResult, err := storageProcessor.Process(storageCtx)
		if err != nil {
			return nil, fmt.Errorf("failed to process storage for sidecar '%s': %w", sc.Name, err)
		}

		var mounts []corev1.VolumeMount
		if storageResult != nil {
			for _, v := range storageResult.Volumes {
				if !volumeNameSet[v.Name] {
					volumes = append(volumes, v)
					volumeNameSet[v.Name] = true
				}
			}
			// This is a simplification. We assume all volume mounts are for this sidecar.
			for _, m := range storageResult.VolumeMounts {
				mounts = append(mounts, m...)
			}
		}

		c := corev1.Container{
			Name:         sidecarName,
			Image:        sc.Image,
			Command:      sc.Command,
			Args:         sc.Args,
			Env:          containerEnvs,
			VolumeMounts: mounts,
		}

		containers = append(containers, c)
		klog.V(3).Infof("Constructed sidecar container '%s' for component '%s'", sidecarName, ctx.Component.Name)
	}

	return &TraitResult{
		Containers: containers,
		Volumes:    volumes,
	}, nil
}