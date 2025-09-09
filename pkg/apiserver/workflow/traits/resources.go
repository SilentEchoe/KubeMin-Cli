package traits

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	"KubeMin-Cli/pkg/apiserver/spec"
)

// ResourcesProcessor applies compute resources (cpu/memory/gpu) to a container.
type ResourcesProcessor struct{}

// Name returns the name of the trait.
func (r *ResourcesProcessor) Name() string {
	return "resources"
}

// Process converts a single ResourceTraitsSpec into Kubernetes ResourceRequirements.
// By design, all values are applied as Limits (no Requests) for simplicity.
func (r *ResourcesProcessor) Process(ctx *TraitContext) (*TraitResult, error) {
	resourceSpec, ok := ctx.TraitData.(*spec.ResourceTraitsSpec)
	if !ok {
		return nil, fmt.Errorf("unexpected type for resources trait: %T", ctx.TraitData)
	}

	if resourceSpec == nil {
		return nil, nil
	}

	resourceReq := corev1.ResourceRequirements{
		Requests: make(corev1.ResourceList),
		Limits:   make(corev1.ResourceList),
	}

	if resourceSpec.CPU != "" {
		qty, err := resource.ParseQuantity(resourceSpec.CPU)
		if err != nil {
			return nil, fmt.Errorf("invalid cpu resource %q: %w", resourceSpec.CPU, err)
		}
		resourceReq.Requests[corev1.ResourceCPU] = qty
		resourceReq.Limits[corev1.ResourceCPU] = qty
	}

	if resourceSpec.Memory != "" {
		qty, err := resource.ParseQuantity(resourceSpec.Memory)
		if err != nil {
			return nil, fmt.Errorf("invalid memory resource %q: %w", resourceSpec.Memory, err)
		}
		resourceReq.Requests[corev1.ResourceMemory] = qty
		resourceReq.Limits[corev1.ResourceMemory] = qty
	}

	if resourceSpec.GPU != "" {
		qty, err := resource.ParseQuantity(resourceSpec.GPU)
		if err != nil {
			return nil, fmt.Errorf("invalid gpu resource %q: %w", resourceSpec.GPU, err)
		}
		// TODO It doesn't necessarily have to be "nvidia.com/gpu" as the prefix.
		resourceReq.Requests[corev1.ResourceName("nvidia.com/gpu")] = qty
		resourceReq.Limits[corev1.ResourceName("nvidia.com/gpu")] = qty
	}

	return &TraitResult{ResourceRequirements: &resourceReq}, nil
}
