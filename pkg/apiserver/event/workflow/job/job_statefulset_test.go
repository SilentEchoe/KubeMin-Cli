package job

import (
	"KubeMin-Cli/pkg/apiserver/domain/model"
	"KubeMin-Cli/pkg/apiserver/utils/kube"
	"testing"
)

func TestBuildStorageResources_PVC_EmptyDir_Mutex(t *testing.T) {
	tests := []struct {
		name           string
		traits         *model.Traits
		wantPVCCount   int
		wantEmptyCount int
	}{
		{
			name: "only emptyDir",
			traits: &model.Traits{
				Storage: []model.StorageTrait{
					{Name: "temp", Type: "ephemeral", MountPath: "/tmp"},
					{Name: "cache", Type: "ephemeral", MountPath: "/cache"},
				},
			},
			wantPVCCount:   0,
			wantEmptyCount: 2,
		},
		{
			name: "only pvc",
			traits: &model.Traits{
				Storage: []model.StorageTrait{
					{Name: "data", Type: "persistent", MountPath: "/data", Size: "1Gi"},
					{Name: "logs", Type: "persistent", MountPath: "/logs", Size: "1Gi"},
				},
			},
			wantPVCCount:   2,
			wantEmptyCount: 0,
		},
		{
			name: "pvc and emptyDir mixed",
			traits: &model.Traits{
				Storage: []model.StorageTrait{
					{Name: "data", Type: "persistent", MountPath: "/data", Size: "1Gi"},
					{Name: "temp", Type: "ephemeral", MountPath: "/tmp"},
					{Name: "cache", Type: "ephemeral", MountPath: "/cache"},
				},
			},
			wantPVCCount:   1,
			wantEmptyCount: 0, // 应该被跳过
		},
		{
			name: "config and emptyDir",
			traits: &model.Traits{
				Storage: []model.StorageTrait{
					{Name: "config", Type: "config", MountPath: "/config"},
					{Name: "temp", Type: "ephemeral", MountPath: "/tmp"},
				},
			},
			wantPVCCount:   0,
			wantEmptyCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, volumes, pvcs := kube.BuildStorageResources("testsvc", tt.traits)
			emptyDirCount := 0
			for _, v := range volumes {
				if v.VolumeSource.EmptyDir != nil {
					emptyDirCount++
				}
			}
			if len(pvcs) != tt.wantPVCCount {
				t.Errorf("PVC count = %d, want %d", len(pvcs), tt.wantPVCCount)
			}
			if emptyDirCount != tt.wantEmptyCount {
				t.Errorf("EmptyDir count = %d, want %d", emptyDirCount, tt.wantEmptyCount)
			}
		})
	}
}
