package traits

import (
	"KubeMin-Cli/pkg/apiserver/config"
	"KubeMin-Cli/pkg/apiserver/domain/model"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
)

func init() {
	Register(&StorageProcessor{})
}

// StorageProcessor handles the logic for the 'storage' trait
type StorageProcessor struct{}

// Name returns the name of the trait
func (s *StorageProcessor) Name() string {
	return "storage"
}

// Process adds volumes and volume mounts to the workload
func (s *StorageProcessor) Process(ctx *TraitContext) error {
	storageTraits, ok := ctx.TraitData.([]model.StorageTrait)
	if !ok {
		return fmt.Errorf("unexpected type for storage trait: %T", ctx.TraitData)
	}

	podTemplate, err := ctx.GetPodTemplate()
	if err != nil {
		return err
	}

	if len(podTemplate.Spec.Containers) == 0 {
		return fmt.Errorf("component %s has no containers defined to mount storage into", ctx.Component.Name)
	}

	for _, st := range storageTraits {
		volName := st.Name
		if volName == "" {
			volName = fmt.Sprintf("%s-storage-%s", ctx.Component.Name, generateRandomSuffix())
		}

		mountPath := st.MountPath
		if mountPath == "" {
			mountPath = fmt.Sprintf("/mnt/%s", volName)
		}

		volType := config.StorageTypeMapping[st.Type]

		switch volType {
		case config.VolumeTypePVC:
			// Create PVC
			if st.Size == "" {
				st.Size = "1Gi"
			}
			qty, err := resource.ParseQuantity(st.Size)
			if err != nil {
				return fmt.Errorf("invalid storage size %s: %w", st.Size, err)
			}

			pvc := corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name: volName,
				},
				Spec: corev1.PersistentVolumeClaimSpec{
					AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
					Resources: corev1.VolumeResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceStorage: qty,
						},
					},
				},
			}
			ctx.AddPVC(pvc)

			// Create volume mount
			volumeMount := corev1.VolumeMount{
				Name:      volName,
				MountPath: mountPath,
				SubPath:   st.SubPath,
				ReadOnly:  st.ReadOnly,
			}
			ctx.AddVolumeMount(0, volumeMount) // Mount to first container

		case config.VolumeTypeEmptyDir:
			// Create empty dir volume
			volume := corev1.Volume{
				Name: volName,
				VolumeSource: corev1.VolumeSource{
					EmptyDir: &corev1.EmptyDirVolumeSource{},
				},
			}
			ctx.AddVolume(volume)

			// Create volume mount
			volumeMount := corev1.VolumeMount{
				Name:      volName,
				MountPath: mountPath,
				SubPath:   st.SubPath,
				ReadOnly:  st.ReadOnly,
			}
			ctx.AddVolumeMount(0, volumeMount)

		case config.VolumeTypeConfigMap:
			// Create config map volume
			volume := corev1.Volume{
				Name: volName,
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: volName,
						},
						DefaultMode: parseInt32(config.DefaultStorageMode),
					},
				},
			}
			ctx.AddVolume(volume)

			// Create volume mount
			volumeMount := corev1.VolumeMount{
				Name:      volName,
				MountPath: mountPath,
				SubPath:   st.SubPath,
				ReadOnly:  st.ReadOnly,
			}
			ctx.AddVolumeMount(0, volumeMount)

		case config.VolumeTypeSecret:
			// Create secret volume
			volume := corev1.Volume{
				Name: volName,
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName:  volName,
						DefaultMode: parseInt32(config.DefaultStorageMode),
					},
				},
			}
			ctx.AddVolume(volume)

			// Create volume mount
			volumeMount := corev1.VolumeMount{
				Name:      volName,
				MountPath: mountPath,
				SubPath:   st.SubPath,
				ReadOnly:  st.ReadOnly,
			}
			ctx.AddVolumeMount(0, volumeMount)
		}

		klog.V(3).Infof("Added storage volume %s to component %s", volName, ctx.Component.Name)
	}

	return nil
}

func parseInt32(s int32) *int32 {
	// Implementation for parsing int32 - you can use your existing utils
	var result int32 = 420
	return &result
}
