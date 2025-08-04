package traits

import (
	"KubeMin-Cli/pkg/apiserver/domain/model"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
)

func init() {
	Register(&EnvProcessor{})
}

// EnvProcessor handles the logic for the 'env' trait
type EnvProcessor struct{}

// Name returns the name of the trait
func (e *EnvProcessor) Name() string {
	return "env"
}

// Process adds environment variables from ConfigMaps or Secrets
func (e *EnvProcessor) Process(ctx *TraitContext) error {
	envTraits, ok := ctx.TraitData.([]model.EnvFromSourceSpec)
	if !ok {
		return fmt.Errorf("unexpected type for env trait: %T", ctx.TraitData)
	}

	podTemplate, err := ctx.GetPodTemplate()
	if err != nil {
		return err
	}

	if len(podTemplate.Spec.Containers) == 0 {
		return fmt.Errorf("component %s has no containers to add environment variables to", ctx.Component.Name)
	}

	// Add env vars to the first container
	mainContainer := &podTemplate.Spec.Containers[0]

	for _, et := range envTraits {
		switch et.Type {
		case "configMap":
			envFrom := corev1.EnvFromSource{
				ConfigMapRef: &corev1.ConfigMapEnvSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: et.Name,
					},
				},
			}
			mainContainer.EnvFrom = append(mainContainer.EnvFrom, envFrom)

		case "secret":
			envFrom := corev1.EnvFromSource{
				SecretRef: &corev1.SecretEnvSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: et.Name,
					},
				},
			}
			mainContainer.EnvFrom = append(mainContainer.EnvFrom, envFrom)

		default:
			return fmt.Errorf("unsupported env source type: %s", et.Type)
		}

		klog.V(3).Infof("Added env from %s %s to component %s", et.Type, et.Name, ctx.Component.Name)
	}

	return nil
}
