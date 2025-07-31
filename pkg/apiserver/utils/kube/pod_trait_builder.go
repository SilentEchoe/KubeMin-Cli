package kube

import (
	"KubeMin-Cli/pkg/apiserver/config"
	"KubeMin-Cli/pkg/apiserver/domain/model"
	"KubeMin-Cli/pkg/apiserver/utils"
	"fmt"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
)

// BuildAllInitContainers 构建所有初始化容器(作为主容器的前置依赖或准备步骤，用于完成某些初始化行为)
func BuildAllInitContainers(specs []model.InitTrait) ([]corev1.Container, []corev1.Volume, []corev1.PersistentVolumeClaim) {
	if len(specs) == 0 {
		return nil, nil, nil
	}

	var initContainers []corev1.Container
	var volumes []corev1.Volume
	var claims []corev1.PersistentVolumeClaim
	volumeNameSet := map[string]bool{}

	//获取特征中所有初始化信息,屏蔽掉初始化容器中附带的初始化容器，防止无限递归。
	for k, sc := range specs {
		initContainerName := sc.Name
		if initContainerName == "" {
			initContainerName = fmt.Sprintf("%s-init", utils.RandStringBytes(6))
		}

		// 构建Env
		var envs []corev1.EnvVar
		for k, v := range sc.Env {
			envs = append(envs, corev1.EnvVar{Name: k, Value: v})
		}

		// 构建挂载
		mounts, vols, pvcs := BuildStorageResources(initContainerName, &sc.Traits[k])

		init := corev1.Container{
			Name:         sc.Name,
			Image:        sc.Image,
			Env:          envs,
			Command:      sc.Command,
			VolumeMounts: mounts,
		}
		initContainers = append(initContainers, init)
		for _, v := range vols {
			if !volumeNameSet[v.Name] {
				volumes = append(volumes, v)
				volumeNameSet[v.Name] = true
			}
		}
		claims = append(claims, pvcs...)
	}

	return initContainers, volumes, claims
}

// BuildAllSidecars 构建所有 sidecar 容器及其资源
func BuildAllSidecars(compName string, specs []model.SidecarSpec) ([]corev1.Container, []corev1.Volume, []corev1.PersistentVolumeClaim, error) {
	var containers []corev1.Container
	var volumes []corev1.Volume
	var claims []corev1.PersistentVolumeClaim
	volumeNameSet := map[string]bool{}

	for _, sc := range specs {
		if len(sc.Traits.Sidecar) > 0 {
			return nil, nil, nil, fmt.Errorf("sidecar '%s' must not contain nested sidecars", sc.Name)
		}
		sidecarName := sc.Name
		if sidecarName == "" {
			sidecarName = fmt.Sprintf("%s-sidecar-%s", compName, utils.RandStringBytes(4))
		}

		// 构建 env
		var containerEnvs []corev1.EnvVar
		for k, v := range sc.Env {
			containerEnvs = append(containerEnvs, corev1.EnvVar{Name: k, Value: v})
		}

		// 构建挂载
		mounts, vols, pvcs := BuildStorageResources(sidecarName, &sc.Traits)

		c := corev1.Container{
			Name:         sidecarName,
			Image:        sc.Image,
			Command:      sc.Command,
			Args:         sc.Args,
			Env:          containerEnvs,
			VolumeMounts: mounts,
		}

		containers = append(containers, c)
		for _, v := range vols {
			if !volumeNameSet[v.Name] {
				volumes = append(volumes, v)
				volumeNameSet[v.Name] = true
			}
		}
		claims = append(claims, pvcs...)
	}

	return containers, volumes, claims, nil
}

// BuildStorageResources 构建存储的信息(Pvc,hostPath....)
func BuildStorageResources(serviceName string, traits *model.Traits) ([]corev1.VolumeMount, []corev1.Volume, []corev1.PersistentVolumeClaim) {
	var volumeMounts []corev1.VolumeMount
	var volumes []corev1.Volume
	var volumeClaims []corev1.PersistentVolumeClaim

	if traits != nil && len(traits.Storage) > 0 {
		// 首先检查是否有 PVC 类型的存储配置
		hasPVC := false
		for _, vol := range traits.Storage {
			volType := config.StorageTypeMapping[vol.Type]
			if volType == config.VolumeTypePVC {
				hasPVC = true
				break
			}
		}

		// 处理存储配置
		for _, vol := range traits.Storage {
			volType := config.StorageTypeMapping[vol.Type]

			// 如果存在 PVC 类型，则跳过 EmptyDir 类型的配置
			if hasPVC && volType == config.VolumeTypeEmptyDir {
				klog.Infof("Skipping EmptyDir storage '%s' because PVC storage is configured", vol.Name)
				continue
			}

			volName := vol.Name
			if volName == "" {
				volName = fmt.Sprintf("%s-%s", serviceName, utils.RandStringBytes(5))
			}
			mountPath := defaultOr(vol.MountPath, fmt.Sprintf("/mnt/%s", volName))
			switch volType {
			case config.VolumeTypePVC:
				qty, _ := resource.ParseQuantity(defaultOr(vol.Size, "1Gi"))
				volumeClaims = append(volumeClaims, corev1.PersistentVolumeClaim{
					ObjectMeta: metav1.ObjectMeta{Name: volName},
					Spec: corev1.PersistentVolumeClaimSpec{
						AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
						Resources: corev1.VolumeResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceStorage: qty,
							},
						},
					},
				})
				volumeMounts = append(volumeMounts, corev1.VolumeMount{Name: volName, MountPath: mountPath, SubPath: vol.SubPath})
			case config.VolumeTypeEmptyDir:
				volumes = append(volumes, corev1.Volume{
					Name:         volName,
					VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
				})
				volumeMounts = append(volumeMounts, corev1.VolumeMount{Name: volName, MountPath: mountPath})
			case config.VolumeTypeConfigMap:
				volumes = append(volumes, corev1.Volume{
					Name: volName,
					VolumeSource: corev1.VolumeSource{
						ConfigMap: &corev1.ConfigMapVolumeSource{
							LocalObjectReference: corev1.LocalObjectReference{Name: volName},
							DefaultMode:          ParseInt32(config.DefaultStorageMode),
						},
					},
				})
				volumeMounts = append(volumeMounts, corev1.VolumeMount{Name: volName, MountPath: mountPath})
			case config.VolumeTypeSecret:
				volumes = append(volumes, corev1.Volume{
					Name: volName,
					VolumeSource: corev1.VolumeSource{
						Secret: &corev1.SecretVolumeSource{
							SecretName:  volName,
							DefaultMode: ParseInt32(config.DefaultStorageMode),
						},
					},
				})
				volumeMounts = append(volumeMounts, corev1.VolumeMount{Name: volName, MountPath: mountPath})
			}
		}
	}
	return volumeMounts, volumes, volumeClaims
}

func defaultOr(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
