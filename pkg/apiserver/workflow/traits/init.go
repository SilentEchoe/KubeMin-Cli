package traits

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"

	spec "KubeMin-Cli/pkg/apiserver/domain/spec"
	"KubeMin-Cli/pkg/apiserver/utils"
)

// InitProcessor creates init containers (pre-main) and applies nested traits
// to them (e.g., storage/env/probes/resources), excluding further init recursion.
type InitProcessor struct{}

// Name returns the name of the trait
func (i *InitProcessor) Name() string {
	return "init"
}

// Process adds init containers to the workload, recursively applying any nested traits.
func (i *InitProcessor) Process(ctx *TraitContext) (*TraitResult, error) {
	initTraits, ok := ctx.TraitData.([]spec.InitTraitSpec)
	if !ok {
		return nil, fmt.Errorf("unexpected type for init trait: %T", ctx.TraitData)
	}

	// This is the final result that will be returned, aggregating all outcomes.
	finalResult := &TraitResult{
		VolumeMounts:   make(map[string][]corev1.VolumeMount),
		EnvFromSources: make(map[string][]corev1.EnvFromSource),
	}

	for _, initTrait := range initTraits {
		if initTrait.Properties.Image == "" {
			return nil, fmt.Errorf("init container for component %s must have an image", ctx.Component.Name)
		}

		initContainerName := initTrait.Name
		if initContainerName == "" {
			initContainerName = fmt.Sprintf("%s-init-%s", ctx.Component.Name, utils.RandStringBytes(4))
		}

		// Convert env map to env vars
		var envVars []corev1.EnvVar
		for k, v := range initTrait.Properties.Env {
			envVars = append(envVars, corev1.EnvVar{Name: k, Value: v})
		}

		// Recursively apply nested traits, excluding the 'init' trait itself to prevent infinite loops.
		nestedResult, err := applyTraitsRecursive(ctx.Component, ctx.Workload, &initTrait.Traits, []string{"init"})
		if err != nil {
			return nil, fmt.Errorf("failed to process nested traits for init container %s: %w", initContainerName, err)
		}
		if nestedResult == nil {
			nestedResult = &TraitResult{}
		}

		// The init container itself gets the volume mounts from its nested traits.
		var volumeMounts []corev1.VolumeMount
		for _, mounts := range nestedResult.VolumeMounts {
			volumeMounts = append(volumeMounts, mounts...)
		}

		// The init container also gets the EnvFrom sources from its nested traits.
		var envFromSources []corev1.EnvFromSource
		for _, envFromBatch := range nestedResult.EnvFromSources {
			envFromSources = append(envFromSources, envFromBatch...)
		}

		// The init container also gets the Env vars from its nested traits.
		for _, envVarsBatch := range nestedResult.EnvVars {
			envVars = append(envVars, envVarsBatch...)
		}

		initContainer := corev1.Container{
			Name:            initContainerName,
			Image:           initTrait.Properties.Image,
			Command:         initTrait.Properties.Command,
			Env:             envVars, // Now contains envs from both properties and traits
			EnvFrom:         envFromSources,
			VolumeMounts:    volumeMounts,
			ImagePullPolicy: corev1.PullIfNotPresent,
		}

		// Apply nested resource requirements to the init container if present
		if nestedResult.ResourceRequirements != nil {
			initContainer.Resources = *nestedResult.ResourceRequirements
		}

		// Add the created container to the final result.
		finalResult.InitContainers = append(finalResult.InitContainers, initContainer)

		// Merge volumes and additional objects from the nested traits into the final result.
		finalResult.Volumes = append(finalResult.Volumes, nestedResult.Volumes...)
		finalResult.AdditionalObjects = append(finalResult.AdditionalObjects, nestedResult.AdditionalObjects...)

		klog.V(3).Infof("Constructed init container %s for component %s", initContainerName, ctx.Component.Name)
	}

	return finalResult, nil
}
