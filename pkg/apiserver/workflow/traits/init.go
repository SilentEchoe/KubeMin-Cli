package traits

import (
	"KubeMin-Cli/pkg/apiserver/domain/model"
	"KubeMin-Cli/pkg/apiserver/utils"
	"fmt"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
)

func init() {
	Register(&InitProcessor{})
}

// InitProcessor handles the logic for the 'init' trait
type InitProcessor struct{}

// Name returns the name of the trait
func (i *InitProcessor) Name() string {
	return "init"
}

// Process adds init containers to the workload
func (i *InitProcessor) Process(ctx *TraitContext) (*TraitResult, error) {
	initTraits, ok := ctx.TraitData.([]model.InitTrait)
	if !ok {
		return nil, fmt.Errorf("unexpected type for init trait: %T", ctx.TraitData)
	}

	var initContainers []corev1.Container
	for _, initTrait := range initTraits {
		if initTrait.Image == "" {
			return nil, fmt.Errorf("init container for component %s must have an image", ctx.Component.Name)
		}

		initContainerName := initTrait.Name
		if initContainerName == "" {
			initContainerName = fmt.Sprintf("%s-init-%s", ctx.Component.Name, utils.RandStringBytes(4))
		}

		// Convert env map to env vars
		var envVars []corev1.EnvVar
		for k, v := range initTrait.Env {
			envVars = append(envVars, corev1.EnvVar{Name: k, Value: v})
		}

		initContainer := corev1.Container{
			Name:    initContainerName,
			Image:   initTrait.Image,
			Command: initTrait.Command,
			Env:     envVars,
		}
		initContainers = append(initContainers, initContainer)
		klog.V(3).Infof("Constructed init container %s for component %s", initContainerName, ctx.Component.Name)
	}

	return &TraitResult{
		InitContainers: initContainers,
	}, nil
}
