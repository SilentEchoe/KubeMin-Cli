package traits

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"

	"KubeMin-Cli/pkg/apiserver/domain/model"
	spec "KubeMin-Cli/pkg/apiserver/domain/spec"
	wfNaming "KubeMin-Cli/pkg/apiserver/workflow/naming"
)

const userInputJSON = `
{
  "storage": [
    {
      "type": "persistent",
      "name": "data",
      "mountPath": "/var/lib/mysql",
      "subPath": "mysql",
      "size": "5Gi",
      "create": true
    },
    {
      "type": "ephemeral",
      "name": "conf",
      "mountPath": "/etc/mysql/conf.d"
    }
  ],
  "sidecar": [
    {
      "name": "xtrabackup",
      "traits": {
        "storage": [
          {
            "type": "persistent",
            "name": "data",
            "mountPath": "/var/lib/mysql",
            "subPath": "mysql"
          }
        ]
      }
    }
  ],
  "init": [
    {
      "name": "clone-mysql",
      "traits": {
        "storage": [
          {
            "type": "persistent",
            "name": "data",
            "mountPath": "/var/lib/mysql",
            "subPath": "mysql"
          }
        ]
      }
    }
  ]
}
`

func TestStorageProcessor_DuplicateInput(t *testing.T) {
	// 1. Parse the user's JSON to simulate the input
	var traits spec.Traits
	err := json.Unmarshal([]byte(userInputJSON), &traits)
	assert.NoError(t, err)

	// Combine all storage traits from the input, simulating the recursive discovery
	var allStorageTraits []spec.StorageTraitSpec
	allStorageTraits = append(allStorageTraits, traits.Storage...)
	allStorageTraits = append(allStorageTraits, traits.Sidecar[0].Traits.Storage...)
	allStorageTraits = append(allStorageTraits, traits.Init[0].Traits.Storage...)

	// 2. Create the processor and the context
	storageProcessor := &StorageProcessor{}
	ctx := &TraitContext{
		Component: &model.ApplicationComponent{
			Name:      "mysql",
			AppID:     "app-1",
			Namespace: "data",
		},
		TraitData: allStorageTraits,
	}

	// 3. Run the processor
	result, err := storageProcessor.Process(ctx)
	assert.NoError(t, err)
	assert.NotNil(t, result)

	// 4. Assert the results are correct

	// 4.1. Check the Volumes list. It should contain the persistent 'data' PVC and the 'conf' disk exactly once each.
	assert.Len(t, result.Volumes, 2, "Should have one PVC volume and one ephemeral volume")
	assert.Equal(t, "data", result.Volumes[0].Name)
	assert.NotNil(t, result.Volumes[0].VolumeSource.PersistentVolumeClaim)
	assert.Equal(t, wfNaming.PVCName("data", ctx.Component.AppID), result.Volumes[0].VolumeSource.PersistentVolumeClaim.ClaimName)
	assert.Equal(t, "conf", result.Volumes[1].Name)
	assert.NotNil(t, result.Volumes[1].VolumeSource.EmptyDir)

	// 4.2. Check the AdditionalObjects list. It should contain the 'data' PVC template.
	assert.Len(t, result.AdditionalObjects, 1, "Should be one additional object (the PVC template)")

	pvc, ok := result.AdditionalObjects[0].(*corev1.PersistentVolumeClaim)
	assert.True(t, ok, "The additional object should be a PersistentVolumeClaim")
	expectedPVCName := wfNaming.PVCName("data", ctx.Component.AppID)
	assert.Equal(t, expectedPVCName, pvc.Name, "The PVC should be normalized with component/app ID")
	assert.Equal(t, ctx.Component.Namespace, pvc.Namespace, "PVC should inherit component namespace")

	annotations := pvc.GetAnnotations()
	assert.NotNil(t, annotations)
	assert.Equal(t, "template", annotations["storage.kubemin.cli/pvc-role"], "The PVC should have the 'template' role annotation")
}
