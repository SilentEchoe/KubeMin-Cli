package traits

import (
	"KubeMin-Cli/pkg/apiserver/config"
	spec "KubeMin-Cli/pkg/apiserver/spec"
	"fmt"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

// StorageProcessor wires storage into Pods via Volumes/VolumeMounts. It supports
// PVC (existing or dynamic), EmptyDir, ConfigMap, and Secret volume sources.
type StorageProcessor struct{}

// Name returns the name of the trait.
func (s *StorageProcessor) Name() string {
	return "storage"
}

// Process converts []spec.StorageTraitSpec into Volumes/VolumeMounts and optionally
// PersistentVolumeClaims (returned as additional objects for non-StatefulSets).
func (s *StorageProcessor) Process(ctx *TraitContext) (*TraitResult, error) {
	storageTraits, ok := ctx.TraitData.([]spec.StorageTraitSpec)
	if !ok {
		return nil, fmt.Errorf("unexpected type for storage trait: %T", ctx.TraitData)
	}

	if len(storageTraits) == 0 {
		return nil, nil
	}

	var volumes []corev1.Volume
	var volumeMounts []corev1.VolumeMount
	var pvcs []corev1.PersistentVolumeClaim
	var additionalObjects []client.Object

	processedVolumes := make(map[string]bool)

	for _, vol := range storageTraits {
		if processedVolumes[vol.Name] {
			continue
		}
		processedVolumes[vol.Name] = true
		volType := config.StorageTypeMapping[vol.Type]

		volName := vol.Name
		if volName == "" {
			return nil, fmt.Errorf("storage trait of type '%s' requires a name", vol.Type)
		}
		mountPath := defaultOr(vol.MountPath, fmt.Sprintf("/mnt/%s", volName))

		switch volType {
		case config.VolumeTypePVC:
			qty, err := resource.ParseQuantity(defaultOr(vol.Size, "1Gi"))
			if err != nil {
				return nil, fmt.Errorf("invalid size %q for volume %s: %w", vol.Size, volName, err)
			}
			pvcSpec := corev1.PersistentVolumeClaimSpec{
				AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
				Resources:   corev1.VolumeResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceStorage: qty}},
			}
			if vol.StorageClass != "" {
				pvcSpec.StorageClassName = &vol.StorageClass
			}

			pvc := corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{Name: volName, Namespace: ctx.Component.Namespace},
				Spec:       pvcSpec,
			}

			if vol.Create {
				pvc.Annotations = map[string]string{config.LabelStorageRole: "template"}
			} else {
				volumes = append(volumes, corev1.Volume{
					Name:         volName,
					VolumeSource: corev1.VolumeSource{PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: volName}},
				})
			}
			pvcs = append(pvcs, pvc)

		case config.VolumeTypeEmptyDir:
			volumes = append(volumes, corev1.Volume{
				Name:         volName,
				VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
			})

		case config.VolumeTypeConfigMap:
			sourceName := vol.SourceName
			if sourceName == "" {
				sourceName = volName
			}
			volumes = append(volumes, corev1.Volume{
				Name: volName,
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{Name: sourceName},
						DefaultMode:          ParseInt32(config.DefaultStorageMode),
					},
				},
			})

		case config.VolumeTypeSecret:
			sourceName := vol.SourceName
			if sourceName == "" {
				sourceName = volName
			}
			volumes = append(volumes, corev1.Volume{
				Name: volName,
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName:  sourceName,
						DefaultMode: ParseInt32(config.DefaultStorageMode),
					},
				},
			})
		}
		// The VolumeMount is always created, regardless of the volume type.
		volumeMounts = append(volumeMounts, corev1.VolumeMount{Name: volName, MountPath: mountPath, SubPath: vol.SubPath, ReadOnly: vol.ReadOnly})
	}

	// Convert PVCs to generic client.Object for the result
	for i := range pvcs {
		additionalObjects = append(additionalObjects, &pvcs[i])
	}

	// The container name is not known here. The aggregator will place the mounts.
	// We use the component name as a temporary key for mounts intended for the main container.
	volumeMountMap := make(map[string][]corev1.VolumeMount)
	if len(volumeMounts) > 0 {
		volumeMountMap[ctx.Component.Name] = volumeMounts
	}

	return &TraitResult{
		Volumes:           volumes,
		VolumeMounts:      volumeMountMap,
		AdditionalObjects: additionalObjects,
	}, nil
}

func defaultOr(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func ParseInt32(i int32) *int32 {
	return &i
}
