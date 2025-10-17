package traits

import (
	"KubeMin-Cli/pkg/apiserver/domain/model"
	spec "KubeMin-Cli/pkg/apiserver/domain/spec"
	"encoding/json"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"testing"
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
		Component: &model.ApplicationComponent{Name: "mysql"},
		TraitData: allStorageTraits,
	}

	// 3. Run the processor
	result, err := storageProcessor.Process(ctx)
	assert.NoError(t, err)
	assert.NotNil(t, result)

	// 4. Assert the results are correct

	// 4.1. Check the Volumes list. It should contain 'conf' but not 'data'.
	assert.Len(t, result.Volumes, 1, "Should only be one volume in the pod spec volumes list")
	assert.Equal(t, "conf", result.Volumes[0].Name, "The only volume should be the 'conf' ephemeral disk")

	// 4.2. Check the AdditionalObjects list. It should contain the 'data' PVC template.
	assert.Len(t, result.AdditionalObjects, 1, "Should be one additional object (the PVC template)")

	pvc, ok := result.AdditionalObjects[0].(*corev1.PersistentVolumeClaim)
	assert.True(t, ok, "The additional object should be a PersistentVolumeClaim")
	assert.Equal(t, "data", pvc.Name, "The PVC should be named 'data'")

	annotations := pvc.GetAnnotations()
	assert.NotNil(t, annotations)
	assert.Equal(t, "template", annotations["storage.kubemin.cli/pvc-role"], "The PVC should have the 'template' role annotation")
}
