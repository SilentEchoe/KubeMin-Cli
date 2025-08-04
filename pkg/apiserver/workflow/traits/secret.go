package traits

import (
	"KubeMin-Cli/pkg/apiserver/domain/model"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
)

func init() {
	Register(&SecretProcessor{})
}

// SecretProcessor handles the logic for the 'secret' trait
type SecretProcessor struct{}

// Name returns the name of the trait
func (s *SecretProcessor) Name() string {
	return "secret"
}

// Process creates Secrets and adds them as volumes
func (s *SecretProcessor) Process(ctx *TraitContext) error {
	secretTraits, ok := ctx.TraitData.([]model.SecretSpec)
	if !ok {
		return fmt.Errorf("unexpected type for secret trait: %T", ctx.TraitData)
	}

	for i, st := range secretTraits {
		secretName := fmt.Sprintf("%s-secret-%d", ctx.Component.Name, i)

		// Create Secret
		secret := corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name: secretName,
			},
			StringData: st.Data, // StringData is more convenient for string values
		}
		ctx.AddSecret(secret)

		// Create volume
		volume := corev1.Volume{
			Name: secretName,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: secretName,
				},
			},
		}
		ctx.AddVolume(volume)

		// Create volume mount
		volumeMount := corev1.VolumeMount{
			Name:      secretName,
			MountPath: fmt.Sprintf("/secrets/%s", secretName),
			ReadOnly:  true,
		}
		ctx.AddVolumeMount(0, volumeMount) // Mount to first container

		klog.V(3).Infof("Added secret %s to component %s", secretName, ctx.Component.Name)
	}

	return nil
}
