package traits

import (
	"KubeMin-Cli/pkg/apiserver/domain/model"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
)

func init() {
	Register(&ConfigProcessor{})
}

// ConfigProcessor handles the logic for the 'config' trait
type ConfigProcessor struct{}

// Name returns the name of the trait
func (c *ConfigProcessor) Name() string {
	return "config"
}

// Process creates ConfigMaps and adds them as volumes
func (c *ConfigProcessor) Process(ctx *TraitContext) error {
	configTraits, ok := ctx.TraitData.([]model.ConfigMapSpec)
	if !ok {
		return fmt.Errorf("unexpected type for config trait: %T", ctx.TraitData)
	}

	for i, ct := range configTraits {
		configName := fmt.Sprintf("%s-config-%d", ctx.Component.Name, i)

		// Create ConfigMap
		configMap := corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name: configName,
			},
			Data: ct.Data,
		}
		ctx.AddConfigMap(configMap)

		// Create volume
		volume := corev1.Volume{
			Name: configName,
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: configName,
					},
				},
			},
		}
		ctx.AddVolume(volume)

		// Create volume mount
		volumeMount := corev1.VolumeMount{
			Name:      configName,
			MountPath: fmt.Sprintf("/config/%s", configName),
			ReadOnly:  true,
		}
		ctx.AddVolumeMount(0, volumeMount) // Mount to first container

		klog.V(3).Infof("Added config map %s to component %s", configName, ctx.Component.Name)
	}

	return nil
}
