package kube

import (
	"KubeMin-Cli/pkg/apiserver/domain/model"
	"fmt"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"
	"testing"
)

// 测试Init容器的构建信息
func TestBuildAllInitContainers_SingleTrait(t *testing.T) {
	mockInitTrait := []model.InitTrait{
		{
			Name: "init-mysql",
			Properties: model.Properties{
				Image:   "kubectl:1.28.5",
				Command: []string{"bash", "-c", "set -ex\n[[ $HOSTNAME =~ ^(.*?)-([0-9]+)$ ]] || exit 1\nprefix_name=${BASH_REMATCH[1]}\nordinal=${BASH_REMATCH[2]}\necho [mysqld] > /mnt/conf.d/server-id.cnf\necho server-id=$((100 + $ordinal)) >> /mnt/conf.d/server-id.cnf\nif [[ ${ordinal} -eq 0 ]]; then\n  cp /mnt/config-map/master.cnf /mnt/conf.d\n  kubectl label pod $HOSTNAME mysql-pod-role=$MASTER_ROLE_NAME --namespace $POD_NAMESPACE --overwrite\nelse\n  cp /mnt/config-map/slave.cnf /mnt/conf.d\n  kubectl label pod $HOSTNAME mysql-pod-role=$SLAVE_ROLE_NAME --namespace $POD_NAMESPACE --overwrite\nfi\n\n[[ -d /var/lib/mysql/mysql ]] && exit 0\noutput_dir=/docker-entrypoint-initdb.d\necho \"use $MYSQL_DATABASE\" > $output_dir/00-init.sql\nfor i in $(seq 1 5); do\n\techo \"尝试下载初始化脚本...第 $i 次\"\n\tcurl -f --connect-timeout 10 --max-time 60 -o \"$output_dir/01-init.sql\" --retry 3 --retry-delay 5 \"$SQL_URL\" && break || sleep 5\ndone\n[ -f \"$output_dir/01-init.sql\" ] || { echo \"下载失败\"; exit 1; }"},
				Env:     map[string]string{"MYSQL_DATABASE": "game", "SQL_URL": "test.sql"},
			},
			Traits: []model.Traits{
				{
					Storage: []model.StorageTrait{
						{
							Type:      "config",
							Name:      "conf",
							MountPath: "/mnt/conf.d",
						},
						{
							Type:      "config",
							Name:      "config-map",
							MountPath: "/mnt/config-map",
						},
						{
							Type:      "config",
							Name:      "init-scripts",
							MountPath: "/docker-entrypoint-initdb.d",
						},
					},
				},
			},
		},
	}
	initContainers, volumes, _ := BuildAllInitContainers(mockInitTrait)

	// 只输出关心字段
	output := map[string]interface{}{
		"initContainers": initContainers,
		"volumes":        volumes,
	}

	yamlBytes, err := yaml.Marshal(output)
	require.NoError(t, err)
	fmt.Println(string(yamlBytes))
}

func TestBuildStatefulSets(t *testing.T) {
	mockInitTrait := []model.InitTrait{
		{
			Name: "init-mysql",
			Properties: model.Properties{
				Image:   "kubectl:1.28.5",
				Command: []string{"bash", "-c", "set -ex\n[[ $HOSTNAME =~ ^(.*?)-([0-9]+)$ ]] || exit 1\nprefix_name=${BASH_REMATCH[1]}\nordinal=${BASH_REMATCH[2]}\necho [mysqld] > /mnt/conf.d/server-id.cnf\necho server-id=$((100 + $ordinal)) >> /mnt/conf.d/server-id.cnf\nif [[ ${ordinal} -eq 0 ]]; then\n  cp /mnt/config-map/master.cnf /mnt/conf.d\n  kubectl label pod $HOSTNAME mysql-pod-role=$MASTER_ROLE_NAME --namespace $POD_NAMESPACE --overwrite\nelse\n  cp /mnt/config-map/slave.cnf /mnt/conf.d\n  kubectl label pod $HOSTNAME mysql-pod-role=$SLAVE_ROLE_NAME --namespace $POD_NAMESPACE --overwrite\nfi\n\n[[ -d /var/lib/mysql/mysql ]] && exit 0\noutput_dir=/docker-entrypoint-initdb.d\necho \"use $MYSQL_DATABASE\" > $output_dir/00-init.sql\nfor i in $(seq 1 5); do\n\techo \"尝试下载初始化脚本...第 $i 次\"\n\tcurl -f --connect-timeout 10 --max-time 60 -o \"$output_dir/01-init.sql\" --retry 3 --retry-delay 5 \"$SQL_URL\" && break || sleep 5\ndone\n[ -f \"$output_dir/01-init.sql\" ] || { echo \"下载失败\"; exit 1; }"},
				Env:     map[string]string{"MYSQL_DATABASE": "game", "SQL_URL": "test.sql"},
			},
			Traits: []model.Traits{
				{
					Storage: []model.StorageTrait{
						{
							Type:      "config",
							Name:      "conf",
							MountPath: "/mnt/conf.d",
						},
						{
							Type:      "config",
							Name:      "config-map",
							MountPath: "/mnt/config-map",
						},
						{
							Type:      "config",
							Name:      "init-scripts",
							MountPath: "/docker-entrypoint-initdb.d",
						},
					},
				},
			},
		},
		{
			Name: "clone-mysql",
			Properties: model.Properties{
				Image:   "xtrabackup:latest",
				Command: []string{"bash", "-c", "set -ex\n[[ -d /var/lib/mysql/mysql ]] && exit 0\n[[ $HOSTNAME =~ ^(.*?)-([0-9]+)$ ]] || exit 1\nprefix_name=${BASH_REMATCH[1]}\nordinal=${BASH_REMATCH[2]}\n[[ $ordinal == 0 ]] && exit 0\nncat --recv-only ${prefix_name}-$(($ordinal-1)).${prefix_name} 3307 | xbstream -x -C /var/lib/mysql\nxtrabackup --prepare --target-dir=/var/lib/mysql"},
				Env:     map[string]string{"MYSQL_DATABASE": "game", "SQL_URL": "test.sql"},
			},
		},
	}
	mockStorageTrait := []model.StorageTrait{
		{
			Name:      "data",
			Type:      "persistent",
			MountPath: "/var/lib/mysql",
			SubPath:   "mysql",
			Size:      "5Gi",
		},
		{
			Name:      "conf",
			Type:      "config",
			MountPath: "/etc/mysql/conf.d",
		},
		{
			Name:      "init-scripts",
			Type:      "config",
			MountPath: "/docker-entrypoint-initdb.d",
		},
	}

	mainTraits := model.Traits{
		Init:    mockInitTrait,
		Storage: mockStorageTrait,
	}

	// 构建初始化容器
	initContainers, _, _ := BuildAllInitContainers(mockInitTrait)

	//主要的容器
	volumeMounts, mainVolumes, _ := BuildStorageResources("mysql", &mainTraits)
	mainContainer := corev1.Container{
		Name:            "mysql",
		Image:           "mysql:5.7.44-mha-tz",
		VolumeMounts:    volumeMounts,
		ImagePullPolicy: corev1.PullAlways,
	}
	allContainers := []corev1.Container{mainContainer}

	statefulSet := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "m2507151323j3fnrk-mysql",
			Namespace: "m2507151323j3fnrk-mysql",
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: ParseInt32(1),
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					InitContainers:                initContainers,
					Containers:                    allContainers,
					TerminationGracePeriodSeconds: ParseInt64(30),
					Volumes:                       mainVolumes,
				},
			},
		},
	}

	yamlBytes, err := yaml.Marshal(statefulSet)
	require.NoError(t, err)
	fmt.Println(string(yamlBytes))
}
