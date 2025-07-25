package traits

import (
	"KubeMin-Cli/pkg/apiserver/domain/model"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func init() {
	Register(&StorageProcessor{})
}

// StorageProcessor handles the logic for the 'storage' trait.
type StorageProcessor struct{}

// Name returns the name of the trait.
func (s *StorageProcessor) Name() string {
	return "storage"
}

// Process adds volumes and volume mounts to the workload based on the storage trait.
func (s *StorageProcessor) Process(workload interface{}, traitData interface{}, component *model.ApplicationComponent) error {
	storageTraits, ok := traitData.([]model.StorageTrait)
	if !ok {
		return fmt.Errorf("unexpected type for storage trait: %T", traitData)
	}

	podTemplate, err := GetPodTemplateSpec(workload)
	if err != nil {
		return err
	}

	for _, st := range storageTraits {
		volume := corev1.Volume{
			Name: st.Name,
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: st.Name,
				},
			},
		}
		podTemplate.Spec.Volumes = append(podTemplate.Spec.Volumes, volume)

		if len(podTemplate.Spec.Containers) == 0 {
			return fmt.Errorf("component %s has no containers defined to mount storage into", component.Name)
		}
		volumeMount := corev1.VolumeMount{
			Name:      st.Name,
			MountPath: st.MountPath,
		}
		// Mount the volume into the first container by default.
		podTemplate.Spec.Containers[0].VolumeMounts = append(podTemplate.Spec.Containers[0].VolumeMounts, volumeMount)
	}

	return nil
}

func parseQuantity(size string) (resource.Quantity, error) {
	return resource.ParseQuantity(size)
}