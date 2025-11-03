package traits

import (
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"

	"KubeMin-Cli/pkg/apiserver/domain/model"
	spec "KubeMin-Cli/pkg/apiserver/domain/spec"
)

func TestStorageProcessor_ExistingPVCUsesGivenName(t *testing.T) {
	storageProcessor := &StorageProcessor{}
	pvcTrait := spec.StorageTraitSpec{
		Type:   "persistent",
		Name:   "shared-cache",
		Create: false,
	}
	ctx := &TraitContext{
		Component: &model.ApplicationComponent{
			Name:      "worker",
			AppID:     "app-2",
			Namespace: "jobs",
		},
		TraitData: []spec.StorageTraitSpec{pvcTrait},
	}

	result, err := storageProcessor.Process(ctx)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Len(t, result.Volumes, 1)

	vol := result.Volumes[0]
	require.Equal(t, "shared-cache", vol.Name)
	require.NotNil(t, vol.VolumeSource.PersistentVolumeClaim)
	require.Equal(t, "shared-cache", vol.VolumeSource.PersistentVolumeClaim.ClaimName)

	require.Len(t, result.AdditionalObjects, 1)
	pvc, ok := result.AdditionalObjects[0].(*corev1.PersistentVolumeClaim)
	require.True(t, ok)
	require.Equal(t, "shared-cache", pvc.Name)
}
