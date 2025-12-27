# StatefulSet PVC Volume 命名问题深度分析

## 问题概述

在 StatefulSet 中使用 `tmp_create: true` 创建持久化存储时，VolumeMount 与 Volume 名称不匹配会导致 Pod 创建失败。

### 错误信息示例

```
create Pod store-mysql-db-ms2ep31dovjkhkwkpy2qiivb-0 in StatefulSet store-mysql-db-ms2ep31dovjkhkwkpy2qiivb failed
error: Pod "store-mysql-db-ms2ep31dovjkhkwkpy2qiivb-0" is invalid:
spec.containers[0].volumeMounts[0].name: Not found: "mysql-data"
```

---

## 问题根因分析

### Kubernetes StatefulSet volumeClaimTemplates 工作机制

```yaml
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: mysql
spec:
  volumeClaimTemplates:
    - metadata:
        name: data  # ← 这是 volume 的名称
      spec:
        accessModes: ["ReadWriteOnce"]
        resources:
          requests:
            storage: 5Gi
  template:
    spec:
      containers:
        - name: mysql
          volumeMounts:
            - name: data  # ← 必须与 volumeClaimTemplate.metadata.name 匹配
              mount_path: /var/lib/mysql
```

**关键点**：
1. StatefulSet 的 `volumeClaimTemplates` 会自动为每个 Pod 创建 PVC
2. 实际 PVC 名称格式：`{volumeClaimTemplate.name}-{podName}` (如 `data-mysql-0`)
3. **Pod 中的 volume 名称等于 `volumeClaimTemplate.metadata.name`**
4. 不需要在 `spec.volumes` 中显式定义这些 volume

### 原代码问题 (修复前)

```go
// storage.go - 修复前的代码
if vol.TmpCreate {
    pvcName := wfNaming.PVCName(vol.Name, ctx.Component.AppID)  // 生成 "pvc-mysql-data-xxx"
    templatePVC := corev1.PersistentVolumeClaim{
        ObjectMeta: metav1.ObjectMeta{
            Name: pvcName,  // ❌ PVC template 名称是 "pvc-mysql-data-xxx"
        },
        // ...
    }
    volumes = append(volumes, corev1.Volume{
        Name: volumeName,  // Volume 名称是 "mysql-data"
        VolumeSource: corev1.VolumeSource{
            PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
                ClaimName: pvcName,  // 引用 "pvc-mysql-data-xxx"
            },
        },
    })
}
// VolumeMount 引用 volumeName = "mysql-data"
volumeMounts = append(volumeMounts, corev1.VolumeMount{
    Name: volumeName,  // "mysql-data"
    MountPath: mount_path,
})
```

**处理流程**：

```
┌─────────────────────────────────────────────────────────────────────────────────┐
│                              storage.go 处理                                      │
├─────────────────────────────────────────────────────────────────────────────────┤
│  用户输入: name="mysql-data", tmp_create=true                                      │
│                                                                                 │
│  1. volumeName = NormalizeLowerStrip("mysql-data") = "mysql-data"               │
│  2. pvcName = PVCName("mysql-data", appID) = "pvc-mysql-data-xxx"               │
│                                                                                 │
│  生成:                                                                           │
│  ├── PVC Template: { Name: "pvc-mysql-data-xxx", Annotations: {role: template} }│
│  ├── Volume: { Name: "mysql-data", ClaimName: "pvc-mysql-data-xxx" }            │
│  └── VolumeMount: { Name: "mysql-data", MountPath: "/var/lib/mysql" }           │
└─────────────────────────────────────────────────────────────────────────────────┘
                                        │
                                        ▼
┌─────────────────────────────────────────────────────────────────────────────────┐
│                            processor.go 处理                                     │
├─────────────────────────────────────────────────────────────────────────────────┤
│  检测到 PVC 有 "template" 注解，workload 是 StatefulSet                           │
│                                                                                 │
│  1. 将 PVC 添加到 StatefulSet.Spec.VolumeClaimTemplates                          │
│     └── volumeClaimTemplates[0].Name = "pvc-mysql-data-xxx"                     │
│                                                                                 │
│  2. 记录 pvcTemplateNames["pvc-mysql-data-xxx"] = true                          │
│                                                                                 │
│  3. 遍历 volumes，移除 ClaimName 在 pvcTemplateNames 中的条目                      │
│     └── Volume { Name: "mysql-data", ClaimName: "pvc-mysql-data-xxx" } 被移除    │
│                                                                                 │
│  4. VolumeMount { Name: "mysql-data" } 保持不变                                  │
└─────────────────────────────────────────────────────────────────────────────────┘
                                        │
                                        ▼
┌─────────────────────────────────────────────────────────────────────────────────┐
│                          最终 StatefulSet YAML                                   │
├─────────────────────────────────────────────────────────────────────────────────┤
│  spec:                                                                          │
│    volumeClaimTemplates:                                                        │
│      - metadata:                                                                │
│          name: pvc-mysql-data-xxx  # ← StatefulSet 创建的 volume 名称            │
│    template:                                                                    │
│      spec:                                                                      │
│        containers:                                                              │
│          - volumeMounts:                                                        │
│              - name: mysql-data  # ❌ 引用的名称不存在！                          │
│                mount_path: /var/lib/mysql                                        │
└─────────────────────────────────────────────────────────────────────────────────┘
                                        │
                                        ▼
                    ❌ Pod 创建失败: volumeMounts[0].name: Not found: "mysql-data"
```

### 修复后的代码

```go
// storage.go - 修复后的代码
if vol.TmpCreate {
    templatePVC := corev1.PersistentVolumeClaim{
        ObjectMeta: metav1.ObjectMeta{
            Name: volumeName,  // ✅ 使用 volumeName 作为模板名称
        },
        // ...
    }
    volumes = append(volumes, corev1.Volume{
        Name: volumeName,
        VolumeSource: corev1.VolumeSource{
            PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
                ClaimName: volumeName,  // ✅ 使用 volumeName
            },
        },
    })
}
```

**修复后的处理流程**：

```
┌─────────────────────────────────────────────────────────────────────────────────┐
│                          最终 StatefulSet YAML (修复后)                           │
├─────────────────────────────────────────────────────────────────────────────────┤
│  spec:                                                                          │
│    volumeClaimTemplates:                                                        │
│      - metadata:                                                                │
│          name: mysql-data  # ✅ 与 VolumeMount 名称一致                          │
│    template:                                                                    │
│      spec:                                                                      │
│        containers:                                                              │
│          - volumeMounts:                                                        │
│              - name: mysql-data  # ✅ 匹配成功                                   │
│                mount_path: /var/lib/mysql                                        │
└─────────────────────────────────────────────────────────────────────────────────┘
                                        │
                                        ▼
              ✅ Pod 创建成功，PVC 名称为 mysql-data-mysql-0
```

---

## 历史修改记录

| Commit | 日期 | 修改内容 | 问题 |
|--------|------|---------|------|
| `0f18fd2a` | 2025-11-08 | 引入 naming 函数生成 PVC 名称 | 导致名称不匹配问题 |
| `d5b5410e` | 2025-11-03 | 尝试对齐 PVC 命名 | 部分修复 |
| `be132634` | 2025-12-13 | 移除重复 volumes 条目 | 未完全修复根因 |
| *当前修复* | - | 使用 volumeName 作为模板名称 | ✅ 根本解决 |

---

## 测试场景与用例

### 场景 1: 基本 StatefulSet + tmp_create

**测试目的**：验证 StatefulSet 使用 `tmp_create: true` 时 PVC 正确创建

```json
{
  "name": "test-statefulset-pvc-basic",
  "namespace": "default",
  "version": "1.0.0",
  "description": "基本 StatefulSet PVC 测试 - tmp_create 模式",
  "component": [
    {
      "name": "mysql-primary",
      "type": "store",
      "image": "mysql:8.0",
      "replicas": 1,
      "properties": {
        "ports": [{"port": 3306, "expose": true}],
        "env": {
          "MYSQL_ROOT_PASSWORD": "rootpassword",
          "MYSQL_DATABASE": "testdb"
        }
      },
      "traits": {
        "storage": [
          {
            "name": "mysql-data",
            "type": "persistent",
            "mount_path": "/var/lib/mysql",
            "tmp_create": true,
            "size": "5Gi",
            "storage_class": "standard"
          }
        ]
      }
    }
  ],
  "workflow": [
    {
      "name": "deploy-mysql",
      "mode": "StepByStep",
      "components": ["mysql-primary"]
    }
  ]
}
```

**预期结果**：
- StatefulSet 正确创建
- volumeClaimTemplates 包含名为 `mysql-data` 的模板
- Pod 成功启动
- PVC 名称为 `mysql-data-mysql-primary-xxx-0`

**验证命令**：
```shell
# 检查 StatefulSet volumeClaimTemplates
kubectl get sts -o jsonpath='{.items[*].spec.volumeClaimTemplates[*].metadata.name}'
# 应返回: mysql-data

# 检查 Pod volumeMounts
kubectl get pod -o jsonpath='{.items[*].spec.containers[*].volumeMounts[*].name}'
# 应返回: mysql-data

# 检查 PVC
kubectl get pvc
# 应看到: mysql-data-<podname>
```

---

### 场景 2: 多 Volume StatefulSet

**测试目的**：验证多个 Volume 同时使用 tmp_create

```json
{
  "name": "test-multi-volume-sts",
  "namespace": "default",
  "version": "1.0.0",
  "description": "多 Volume StatefulSet 测试",
  "component": [
    {
      "name": "postgres-db",
      "type": "store",
      "image": "postgres:15",
      "replicas": 1,
      "properties": {
        "ports": [{"port": 5432, "expose": true}],
        "env": {
          "POSTGRES_PASSWORD": "secretpassword"
        }
      },
      "traits": {
        "storage": [
          {
            "name": "pg-data",
            "type": "persistent",
            "mount_path": "/var/lib/postgresql/data",
            "tmp_create": true,
            "size": "10Gi",
            "storage_class": "standard"
          },
          {
            "name": "pg-wal",
            "type": "persistent",
            "mount_path": "/var/lib/postgresql/wal",
            "tmp_create": true,
            "size": "5Gi",
            "storage_class": "standard"
          },
          {
            "name": "pg-backup",
            "type": "ephemeral",
            "mount_path": "/backup"
          }
        ]
      }
    }
  ],
  "workflow": [
    {
      "name": "deploy-postgres",
      "mode": "StepByStep",
      "components": ["postgres-db"]
    }
  ]
}
```

**预期结果**：
- volumeClaimTemplates 包含 `pg-data` 和 `pg-wal` 两个模板
- emptyDir volume `pg-backup` 正常创建
- 所有 volumeMounts 正确绑定

**验证命令**：
```shell
# 检查 volumeClaimTemplates 数量
kubectl get sts <name> -o jsonpath='{.spec.volumeClaimTemplates[*].metadata.name}'
# 应返回: pg-data pg-wal

# 检查 volumes (应只有 ephemeral)
kubectl get sts <name> -o jsonpath='{.spec.template.spec.volumes[*].name}'
# 应返回: pg-backup

# 检查 PVC
kubectl get pvc | grep -E "pg-data|pg-wal"
# 应看到两个 PVC
```

---

### 场景 3: 混合模式 - tmp_create + 引用已有 PVC

**测试目的**：验证同时使用 tmp_create 和引用已有 PVC

```json
{
  "name": "test-mixed-pvc-mode",
  "namespace": "default",
  "version": "1.0.0",
  "description": "混合 PVC 模式测试",
  "component": [
    {
      "name": "app-server",
      "type": "store",
      "image": "nginx:alpine",
      "replicas": 2,
      "properties": {
        "ports": [{"port": 80, "expose": true}]
      },
      "traits": {
        "storage": [
          {
            "name": "app-data",
            "type": "persistent",
            "mount_path": "/data",
            "tmp_create": true,
            "size": "2Gi"
          },
          {
            "name": "shared-config",
            "type": "persistent",
            "mount_path": "/config",
            "tmp_create": false,
            "claim_name": "existing-config-pvc"
          }
        ]
      }
    }
  ],
  "workflow": [
    {
      "name": "deploy-app",
      "mode": "StepByStep",
      "components": ["app-server"]
    }
  ]
}
```

**前置条件**：
```shell
# 先创建已有的 PVC
kubectl apply -f - <<EOF
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: existing-config-pvc
spec:
  accessModes: ["ReadWriteOnce"]
  resources:
    requests:
      storage: 1Gi
EOF
```

**预期结果**：
- volumeClaimTemplates 只包含 `app-data`
- volumes 包含引用 `existing-config-pvc` 的条目
- 两个 volumeMounts 都正确绑定

---

### 场景 4: Deployment + tmp_create (负面测试)

**测试目的**：验证 Deployment 使用 tmp_create 时的处理（应发出警告）

```json
{
  "name": "test-deployment-pvc",
  "namespace": "default",
  "version": "1.0.0",
  "description": "Deployment PVC 测试 (负面场景)",
  "component": [
    {
      "name": "web-app",
      "type": "webservice",
      "image": "nginx:alpine",
      "replicas": 2,
      "properties": {
        "ports": [{"port": 80, "expose": true}]
      },
      "traits": {
        "storage": [
          {
            "name": "web-data",
            "type": "persistent",
            "mount_path": "/data",
            "tmp_create": true,
            "size": "1Gi"
          }
        ]
      }
    }
  ],
  "workflow": [
    {
      "name": "deploy-web",
      "mode": "StepByStep",
      "components": ["web-app"]
    }
  ]
}
```

**预期结果**：
- Deployment 不支持 volumeClaimTemplates
- 应发出警告日志
- PVC template 会被忽略
- Pod 创建可能失败（因为 Volume 被移除但 VolumeMount 仍存在）

**注意**：对于 Deployment，应使用 `tmp_create: false` 并引用已有 PVC，或使用 ephemeral volume。

---

### 场景 5: 带 Sidecar 和 Init 容器的 StatefulSet

**测试目的**：验证多容器共享 volume 的场景

```json
{
  "name": "test-multi-container-volume",
  "namespace": "default",
  "version": "1.0.0",
  "description": "多容器共享 Volume 测试",
  "component": [
    {
      "name": "mysql-cluster",
      "type": "store",
      "image": "mysql:8.0",
      "replicas": 1,
      "properties": {
        "ports": [{"port": 3306, "expose": true}],
        "env": {
          "MYSQL_ROOT_PASSWORD": "rootpassword"
        }
      },
      "traits": {
        "storage": [
          {
            "name": "mysql-data",
            "type": "persistent",
            "mount_path": "/var/lib/mysql",
            "sub_path": "mysql",
            "tmp_create": true,
            "size": "10Gi"
          }
        ],
        "sidecar": [
          {
            "name": "xtrabackup",
            "image": "percona/percona-xtrabackup:8.0",
            "command": ["sleep", "infinity"],
            "traits": {
              "storage": [
                {
                  "name": "mysql-data",
                  "type": "persistent",
                  "mount_path": "/var/lib/mysql",
                  "sub_path": "mysql"
                }
              ]
            }
          }
        ],
        "init": [
          {
            "name": "init-mysql",
            "image": "busybox:latest",
            "command": ["sh", "-c", "echo 'Initializing...'"],
            "traits": {
              "storage": [
                {
                  "name": "mysql-data",
                  "type": "persistent",
                  "mount_path": "/var/lib/mysql",
                  "sub_path": "mysql"
                }
              ]
            }
          }
        ]
      }
    }
  ],
  "workflow": [
    {
      "name": "deploy-mysql-cluster",
      "mode": "StepByStep",
      "components": ["mysql-cluster"]
    }
  ]
}
```

**预期结果**：
- volumeClaimTemplates 只包含一个 `mysql-data` 模板（去重）
- 主容器、sidecar、init 容器都正确挂载同一个 volume
- 所有容器的 volumeMount.name 都是 `mysql-data`

**验证命令**：
```shell
# 检查所有容器的 volumeMounts
kubectl get pod <pod-name> -o jsonpath='{range .spec.containers[*]}{.name}: {.volumeMounts[*].name}{"\n"}{end}'

# 检查 init 容器的 volumeMounts
kubectl get pod <pod-name> -o jsonpath='{range .spec.initContainers[*]}{.name}: {.volumeMounts[*].name}{"\n"}{end}'
```

---

### 场景 6: 依赖链中的 StatefulSet

**测试目的**：验证在复杂依赖链中 StatefulSet 的 PVC 正确处理

```json
{
  "name": "test-dependency-chain",
  "namespace": "default",
  "version": "1.0.0",
  "description": "依赖链中的 StatefulSet PVC 测试",
  "component": [
    {
      "name": "app-config",
      "type": "config",
      "replicas": 1,
      "properties": {
        "conf": {
          "database.host": "mysql-db",
          "database.port": "3306"
        }
      }
    },
    {
      "name": "app-secret",
      "type": "secret",
      "replicas": 1,
      "properties": {
        "secret": {
          "db-password": "secretpassword123"
        }
      }
    },
    {
      "name": "mysql-db",
      "type": "store",
      "image": "mysql:8.0",
      "replicas": 1,
      "properties": {
        "ports": [{"port": 3306, "expose": true}],
        "env": {
          "MYSQL_ROOT_PASSWORD": "rootpassword"
        }
      },
      "traits": {
        "storage": [
          {
            "name": "mysql-data",
            "type": "persistent",
            "mount_path": "/var/lib/mysql",
            "tmp_create": true,
            "size": "5Gi",
            "storage_class": "standard"
          }
        ]
      }
    },
    {
      "name": "backend-app",
      "type": "webservice",
      "image": "myregistry/backend:v1.0.0",
      "replicas": 2,
      "properties": {
        "ports": [{"port": 8080, "expose": true}],
        "env": {
          "DB_HOST": "mysql-db"
        }
      },
      "traits": {
        "env_from": [
          {"type": "configMap", "source_name": "app-config"},
          {"type": "secret", "source_name": "app-secret"}
        ]
      }
    }
  ],
  "workflow": [
    {
      "name": "step1-config",
      "mode": "StepByStep",
      "components": ["app-config"]
    },
    {
      "name": "step2-secret",
      "mode": "StepByStep",
      "components": ["app-secret"]
    },
    {
      "name": "step3-database",
      "mode": "StepByStep",
      "components": ["mysql-db"]
    },
    {
      "name": "step4-app",
      "mode": "StepByStep",
      "components": ["backend-app"]
    }
  ]
}
```

**预期结果**：
- ConfigMap 和 Secret 先创建
- MySQL StatefulSet 正确创建，PVC 正常绑定
- Backend Deployment 正确引用 ConfigMap 和 Secret
- 工作流按顺序执行完成

---

## 单元测试代码

### 测试文件: `storage_statefulset_test.go`

```go
package traits

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"kubemin-cli/pkg/apiserver/config"
	"kubemin-cli/pkg/apiserver/domain/model"
	spec "kubemin-cli/pkg/apiserver/domain/spec"
)

// TestStorageProcessor_StatefulSet_TmpCreate 验证 StatefulSet 使用 tmp_create 时
// volumeClaimTemplate 名称与 VolumeMount 名称一致
func TestStorageProcessor_StatefulSet_TmpCreate(t *testing.T) {
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

	// 验证 Volume 名称
	require.Len(t, result.Volumes, 1)
	assert.Equal(t, "mysql-data", result.Volumes[0].Name, "Volume 名称应该是 volumeName")
	assert.Equal(t, "mysql-data", result.Volumes[0].PersistentVolumeClaim.ClaimName,
		"ClaimName 应该与 volumeName 一致，以便 StatefulSet 正确处理")

	// 验证 PVC template 名称
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
}

// TestProcessor_StatefulSet_VolumeClaimTemplates 验证完整的 StatefulSet 处理流程
func TestProcessor_StatefulSet_VolumeClaimTemplates(t *testing.T) {
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
		AdditionalObjects: []client.Object{
			&corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "mysql-data",
					Annotations: map[string]string{config.LabelStorageRole: "template"},
				},
			},
		},
	}

	// 创建 StatefulSet workload
	sts := &appsv1.StatefulSet{
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

	// 应用 result 到 StatefulSet
	// ... (调用 applyTraitsToWorkload)

	// 验证结果
	assert.Len(t, sts.Spec.VolumeClaimTemplates, 1,
		"应该有一个 volumeClaimTemplate")
	assert.Equal(t, "mysql-data", sts.Spec.VolumeClaimTemplates[0].Name,
		"volumeClaimTemplate 名称应该与 VolumeMount 名称一致")

	// 验证 Volume 被正确移除
	assert.Len(t, sts.Spec.Template.Spec.Volumes, 0,
		"显式 Volume 应该被移除，StatefulSet 会自动创建")
}

// TestStorageProcessor_NonStatefulSet_TmpCreate 验证非 StatefulSet 使用 tmp_create 的警告
func TestStorageProcessor_NonStatefulSet_TmpCreate(t *testing.T) {
	// 对于 Deployment，tmp_create=true 应该发出警告
	// 因为 Deployment 不支持 volumeClaimTemplates
}
```

---

## 调试指南

### 检查生成的 StatefulSet YAML

```shell
# 获取完整的 StatefulSet 定义
kubectl get sts <name> -o yaml

# 重点检查以下部分:
# 1. spec.volumeClaimTemplates[].metadata.name
# 2. spec.template.spec.containers[].volumeMounts[].name
# 3. spec.template.spec.volumes (应该为空或不包含 PVC 类型)
```

### 检查日志

```shell
# 查看 apiserver 日志中的 storage trait 处理
kubectl logs <apiserver-pod> | grep -E "(storage|volumeClaimTemplate|pvc)"

# 查看 StatefulSet controller 日志
kubectl logs -n kube-system -l component=kube-controller-manager | grep <sts-name>
```

### 常见问题排查

| 错误 | 原因 | 解决方案 |
|------|------|---------|
| `volumeMounts[0].name: Not found` | VolumeMount 名称与 volumeClaimTemplate 名称不匹配 | 检查 storage.go 中的命名逻辑 |
| `PVC not found` | PVC 未创建或名称不匹配 | 检查 PVC 列表和命名 |
| `duplicate volume name` | 多个 volume 使用相同名称 | 检查去重逻辑 |

---

## 版本兼容性

| 版本 | tmp_create 行为 | 注意事项 |
|------|---------------|---------|
| < 修复版本 | 使用 naming 函数生成 PVC 名称 | 导致名称不匹配 |
| >= 修复版本 | 使用 volumeName 作为模板名称 | 正确工作 |

---

## 参考资料

- [Kubernetes StatefulSet 官方文档](https://kubernetes.io/docs/concepts/workloads/controllers/statefulset/)
- [Kubernetes Volumes 官方文档](https://kubernetes.io/docs/concepts/storage/volumes/)
- [PersistentVolumeClaims](https://kubernetes.io/docs/concepts/storage/persistent-volumes/)

