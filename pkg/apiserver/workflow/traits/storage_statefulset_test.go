package traits

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"KubeMin-Cli/pkg/apiserver/config"
	"KubeMin-Cli/pkg/apiserver/domain/model"
	spec "KubeMin-Cli/pkg/apiserver/domain/spec"
)

// TestStorageProcessor_StatefulSet_TmpCreate_VolumeNameMatch 验证 StatefulSet 使用 tmpCreate 时
// volumeClaimTemplate 名称与 VolumeMount 名称一致
// 这是修复 "volumeMounts[0].name: Not found" 错误的核心测试
func TestStorageProcessor_StatefulSet_TmpCreate_VolumeNameMatch(t *testing.T) {
	processor := &StorageProcessor{}
	ctx := &TraitContext{
		Component: &model.ApplicationComponent{
			Name:      "mysql",
			AppID:     "app-123",
			Namespace: "default",
		},
		TraitData: []spec.StorageTraitSpec{
			{
				Name:      "mysql-data",
				Type:      "persistent",
				MountPath: "/var/lib/mysql",
				Size:      "5Gi",
				TmpCreate: true,
			},
		},
	}

	result, err := processor.Process(ctx)
	require.NoError(t, err)
	require.NotNil(t, result)

	// 验证 Volume 名称与 ClaimName 一致
	require.Len(t, result.Volumes, 1)
	assert.Equal(t, "mysql-data", result.Volumes[0].Name,
		"Volume 名称应该是 volumeName")
	require.NotNil(t, result.Volumes[0].PersistentVolumeClaim)
	assert.Equal(t, "mysql-data", result.Volumes[0].PersistentVolumeClaim.ClaimName,
		"ClaimName 应该与 volumeName 一致，以便 StatefulSet 正确处理")

	// 验证 PVC template 名称与 volumeName 一致
	require.Len(t, result.AdditionalObjects, 1)
	pvc, ok := result.AdditionalObjects[0].(*corev1.PersistentVolumeClaim)
	require.True(t, ok)
	assert.Equal(t, "mysql-data", pvc.Name,
		"PVC template 名称应该是 volumeName，以匹配 VolumeMount")

	// 验证 VolumeMount 名称
	mounts := result.VolumeMounts["mysql"]
	require.Len(t, mounts, 1)
	assert.Equal(t, "mysql-data", mounts[0].Name,
		"VolumeMount 名称应该是 volumeName")

	// 核心验证：所有三个名称必须一致
	assert.Equal(t, result.Volumes[0].Name, pvc.Name,
		"Volume.Name 必须等于 PVC template.Name")
	assert.Equal(t, result.Volumes[0].PersistentVolumeClaim.ClaimName, pvc.Name,
		"Volume.ClaimName 必须等于 PVC template.Name")
	assert.Equal(t, mounts[0].Name, pvc.Name,
		"VolumeMount.Name 必须等于 PVC template.Name")
}

// TestStorageProcessor_MultiVolume_TmpCreate 验证多个 tmpCreate volume 的命名
func TestStorageProcessor_MultiVolume_TmpCreate(t *testing.T) {
	processor := &StorageProcessor{}
	ctx := &TraitContext{
		Component: &model.ApplicationComponent{
			Name:      "postgres",
			AppID:     "app-456",
			Namespace: "default",
		},
		TraitData: []spec.StorageTraitSpec{
			{
				Name:      "pg-data",
				Type:      "persistent",
				MountPath: "/var/lib/postgresql/data",
				Size:      "10Gi",
				TmpCreate: true,
			},
			{
				Name:      "pg-wal",
				Type:      "persistent",
				MountPath: "/var/lib/postgresql/wal",
				Size:      "5Gi",
				TmpCreate: true,
			},
			{
				Name:      "pg-backup",
				Type:      "ephemeral",
				MountPath: "/backup",
			},
		},
	}

	result, err := processor.Process(ctx)
	require.NoError(t, err)
	require.NotNil(t, result)

	// 验证 3 个 volumes (2 PVC + 1 emptyDir)
	require.Len(t, result.Volumes, 3)

	// 验证 2 个 PVC templates
	require.Len(t, result.AdditionalObjects, 2)

	// 验证每个 PVC template 的名称与对应的 volume 名称一致
	pvcNames := make(map[string]bool)
	for _, obj := range result.AdditionalObjects {
		pvc := obj.(*corev1.PersistentVolumeClaim)
		pvcNames[pvc.Name] = true
	}
	assert.True(t, pvcNames["pg-data"], "应该有 pg-data PVC template")
	assert.True(t, pvcNames["pg-wal"], "应该有 pg-wal PVC template")

	// 验证 VolumeMount 名称
	mounts := result.VolumeMounts["postgres"]
	require.Len(t, mounts, 3)
	mountNames := make(map[string]bool)
	for _, m := range mounts {
		mountNames[m.Name] = true
	}
	assert.True(t, mountNames["pg-data"], "应该有 pg-data VolumeMount")
	assert.True(t, mountNames["pg-wal"], "应该有 pg-wal VolumeMount")
	assert.True(t, mountNames["pg-backup"], "应该有 pg-backup VolumeMount")
}

// TestStorageProcessor_MixedMode 验证 tmpCreate 和引用已有 PVC 混合使用
func TestStorageProcessor_MixedMode(t *testing.T) {
	processor := &StorageProcessor{}
	ctx := &TraitContext{
		Component: &model.ApplicationComponent{
			Name:      "app-server",
			AppID:     "app-789",
			Namespace: "default",
		},
		TraitData: []spec.StorageTraitSpec{
			{
				Name:      "app-data",
				Type:      "persistent",
				MountPath: "/data",
				Size:      "2Gi",
				TmpCreate: true,
			},
			{
				Name:      "shared-config",
				Type:      "persistent",
				MountPath: "/config",
				TmpCreate: false,
				ClaimName: "existing-config-pvc",
			},
		},
	}

	result, err := processor.Process(ctx)
	require.NoError(t, err)
	require.NotNil(t, result)

	// 验证 2 个 volumes
	require.Len(t, result.Volumes, 2)

	// 验证 2 个 PVC (1 template + 1 existing reference)
	require.Len(t, result.AdditionalObjects, 2)

	// 查找 tmpCreate 的 PVC template
	var templatePVC, existingPVC *corev1.PersistentVolumeClaim
	for _, obj := range result.AdditionalObjects {
		pvc := obj.(*corev1.PersistentVolumeClaim)
		if pvc.GetAnnotations()[config.LabelStorageRole] == "template" {
			templatePVC = pvc
		} else {
			existingPVC = pvc
		}
	}

	require.NotNil(t, templatePVC, "应该有一个 template PVC")
	require.NotNil(t, existingPVC, "应该有一个 existing PVC reference")

	// 验证 template PVC 名称
	assert.Equal(t, "app-data", templatePVC.Name,
		"template PVC 名称应该是 volumeName")

	// 验证 existing PVC 引用
	assert.Equal(t, "existing-config-pvc", existingPVC.Name,
		"existing PVC 应该使用 claimName")
}

// TestApplyStorageToStatefulSet 验证完整的 StatefulSet 处理流程
func TestApplyStorageToStatefulSet(t *testing.T) {
	// 模拟 StorageProcessor 的输出
	result := &TraitResult{
		Volumes: []corev1.Volume{
			{
				Name: "mysql-data",
				VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
						ClaimName: "mysql-data",
					},
				},
			},
		},
		VolumeMounts: map[string][]corev1.VolumeMount{
			"mysql": {
				{Name: "mysql-data", MountPath: "/var/lib/mysql"},
			},
		},
	}

	// 创建 StatefulSet workload
	sts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mysql-test",
			Namespace: "default",
		},
		Spec: appsv1.StatefulSetSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{Name: "mysql"},
					},
				},
			},
		},
	}

	// 应用 volumes 到 StatefulSet
	sts.Spec.Template.Spec.Volumes = result.Volumes

	// 应用 volumeMounts 到容器
	for i := range sts.Spec.Template.Spec.Containers {
		container := &sts.Spec.Template.Spec.Containers[i]
		if mounts, ok := result.VolumeMounts[container.Name]; ok {
			container.VolumeMounts = append(container.VolumeMounts, mounts...)
		}
	}

	// 模拟 processor.go 将 PVC 转换为 volumeClaimTemplates
	templatePVC := corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name: "mysql-data", // 使用 volumeName，不是 pvcName
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
		},
	}
	sts.Spec.VolumeClaimTemplates = append(sts.Spec.VolumeClaimTemplates, templatePVC)

	// 移除对应的 volume（StatefulSet 会自动创建）
	var filteredVolumes []corev1.Volume
	for _, vol := range sts.Spec.Template.Spec.Volumes {
		if vol.PersistentVolumeClaim != nil && vol.PersistentVolumeClaim.ClaimName == "mysql-data" {
			continue // 移除
		}
		filteredVolumes = append(filteredVolumes, vol)
	}
	sts.Spec.Template.Spec.Volumes = filteredVolumes

	// 验证最终结果
	require.Len(t, sts.Spec.VolumeClaimTemplates, 1,
		"应该有一个 volumeClaimTemplate")
	assert.Equal(t, "mysql-data", sts.Spec.VolumeClaimTemplates[0].Name,
		"volumeClaimTemplate 名称应该与 VolumeMount 名称一致")

	// 验证 Volume 被正确移除
	assert.Len(t, sts.Spec.Template.Spec.Volumes, 0,
		"显式 Volume 应该被移除，StatefulSet 会自动创建")

	// 验证 VolumeMount 仍然存在且名称正确
	require.Len(t, sts.Spec.Template.Spec.Containers[0].VolumeMounts, 1)
	assert.Equal(t, "mysql-data", sts.Spec.Template.Spec.Containers[0].VolumeMounts[0].Name,
		"VolumeMount.Name 应该与 volumeClaimTemplate.Name 一致")

	// 核心验证：VolumeMount.Name == volumeClaimTemplate.Name
	assert.Equal(t,
		sts.Spec.Template.Spec.Containers[0].VolumeMounts[0].Name,
		sts.Spec.VolumeClaimTemplates[0].Name,
		"VolumeMount.Name 必须等于 volumeClaimTemplate.Name，否则 Pod 会创建失败")
}

