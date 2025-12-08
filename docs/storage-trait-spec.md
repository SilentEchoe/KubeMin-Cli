# StorageTraitSpec 存储特性规范文档

## 概述

`StorageTraitSpec` 是 KubeMin-Cli 中用于描述容器存储挂载特性的核心数据结构。它支持多种 Kubernetes 原生存储类型的配置，包括持久化存储（PVC）、临时存储（EmptyDir）、ConfigMap 和 Secret 挂载。

该规范定义于 `pkg/apiserver/domain/spec/traits.go`，并由 `StorageProcessor` 进行处理，最终转化为 Kubernetes Volume 和 VolumeMount 资源。

---

## 数据结构定义

```go
type StorageTraitSpec struct {
    Name       string `json:"name,omitempty"`
    Type       string `json:"type"`
    MountPath  string `json:"mountPath"`
    SubPath    string `json:"subPath,omitempty"`
    ReadOnly   bool   `json:"readOnly,omitempty"`
    SourceName string `json:"sourceName,omitempty"`

    // 仅适用于 "persistent" 类型
    TmpCreate    bool   `json:"TmpCreate,omitempty"`
    Size         string `json:"size,omitempty"`
    ClaimName    string `json:"claimName,omitempty"`
    StorageClass string `json:"storageClass,omitempty"`
}
```

---

## 字段详解

### 通用字段

| 字段 | 类型 | 必填 | 默认值 | 说明 |
|------|------|------|--------|------|
| `name` | string | ✅ | - | 存储卷的唯一名称标识符，用于生成 Kubernetes Volume 名称。必须符合 DNS-1123 子域名规范（小写字母、数字、连字符）。 |
| `type` | string | ✅ | - | 存储类型，决定底层 Kubernetes 资源类型。支持的值见下方 [存储类型](#存储类型) 章节。 |
| `mountPath` | string | ✅ | `/mnt/<name>` | 容器内挂载路径。若未指定，默认为 `/mnt/<volume-name>`。 |
| `subPath` | string | ❌ | `""` | 挂载 Volume 内的子路径。用于将同一 Volume 的不同子目录挂载到不同位置。 |
| `readOnly` | bool | ❌ | `false` | 是否以只读模式挂载。设置为 `true` 时容器无法写入该挂载点。 |
| `sourceName` | string | ❌ | `name` | 仅用于 ConfigMap/Secret 类型。指定实际 ConfigMap 或 Secret 资源的名称。若为空，则使用 `name` 字段的值。 |

### 持久化存储专用字段

以下字段仅在 `type: "persistent"` 时生效：

| 字段 | 类型 | 必填 | 默认值 | 说明 |
|------|------|------|--------|------|
| `TmpCreate` | bool | ❌ | `false` | **创建模式控制**：<br/>• `true`：动态创建新的 PVC，PVC 名称格式为 `pvc-<name>-<appID>`，并添加 `template` 角色注解<br/>• `false`：创建 PVC 对象，名称直接使用 `name` 字段值 |
| `size` | string | ❌ | `"1Gi"` | PVC 请求的存储容量。无论 `TmpCreate` 为何值，该字段都会生效。格式遵循 Kubernetes 资源量规范（如 `5Gi`、`100Mi`）。 |
| `claimName` | string | ❌ | `name` | ⚠️ **预留字段，当前代码未实现**。设计意图是允许指定已存在的 PVC 名称，但当前 Volume 引用的 PVC 名称完全由 `TmpCreate` 和 `name` 字段决定。 |
| `storageClass` | string | ❌ | 集群默认 | 指定 PVC 使用的 StorageClass。若为空，使用集群默认 StorageClass。 |

---

## 存储类型

KubeMin-Cli 支持以下用户侧存储类型声明，系统会自动映射到相应的 Kubernetes Volume 类型：

| 用户侧类型 | Kubernetes 类型 | 说明 |
|-----------|-----------------|------|
| `persistent` | PersistentVolumeClaim | 持久化存储，数据在 Pod 重启后保留 |
| `ephemeral` | EmptyDir | 临时存储，Pod 删除后数据丢失 |
| `host-mounted` | HostPath | 挂载宿主机目录（不推荐在生产环境使用） |
| `config` | ConfigMap | 将 ConfigMap 数据挂载为文件 |
| `secret` | Secret | 将 Secret 数据挂载为文件 |

---

## 处理逻辑流程

```
┌─────────────────────────────────────────────────────────────────────────┐
│                      StorageProcessor.Process()                          │
└─────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────────┐
│  遍历 []StorageTraitSpec，根据 Name 去重（同名只处理一次）                   │
└─────────────────────────────────────────────────────────────────────────┘
                                    │
            ┌───────────────────────┼───────────────────────┐
            ▼                       ▼                       ▼
    type="persistent"        type="ephemeral"      type="config/secret"
            │                       │                       │
            ▼                       ▼                       ▼
    ┌───────────────┐        ┌─────────────┐        ┌──────────────┐
    │ 创建 PVC 对象  │        │ 创建        │        │ 创建         │
    │ (Size 生效)   │        │ EmptyDir    │        │ ConfigMap/   │
    │               │        │ Volume      │        │ Secret Volume│
    │ TmpCreate?    │        └─────────────┘        └──────────────┘
    │ true: 名称带   │
    │   appID后缀    │
    │ false:直接用   │
    │   name字段     │
    └───────────────┘
            │
            ▼
┌─────────────────────────────────────────────────────────────────────────┐
│  统一创建 VolumeMount（name, mountPath, subPath, readOnly）               │
└─────────────────────────────────────────────────────────────────────────┘
            │
            ▼
┌─────────────────────────────────────────────────────────────────────────┐
│  返回 TraitResult: Volumes + VolumeMounts + AdditionalObjects(PVCs)      │
└─────────────────────────────────────────────────────────────────────────┘
```

### PVC 创建逻辑详解

当 `type="persistent"` 时，系统**始终会创建 PVC 对象**，但根据 `TmpCreate` 字段决定命名和注解：

#### TmpCreate = true（模板创建模式）

1. 生成 PVC 名称：`pvc-<name>-<appID>`（通过 `wfNaming.PVCName()` 函数）
2. 添加注解：`storage.kubemin.cli/pvc-role: template`
3. Volume 引用动态生成的 PVC 名称
4. 适用场景：需要为每个应用实例创建独立存储

#### TmpCreate = false（直接创建模式）

1. 直接使用 `name` 字段作为 PVC 名称
2. 创建基础 PVC 对象（无特殊注解）
3. Volume 引用该 PVC 名称
4. 适用场景：多个应用共享同一存储，或需要精确控制 PVC 名称

> **注意**：两种模式都会创建 PVC 资源，`Size` 和 `StorageClass` 字段在两种模式下都生效。

---

## 使用示例

### 1. 动态创建持久化存储

```json
{
  "storage": [
    {
      "type": "persistent",
      "name": "mysql-data",
      "mountPath": "/var/lib/mysql",
      "TmpCreate": true,
      "size": "10Gi",
      "storageClass": "fast-ssd"
    }
  ]
}
```

**生成结果**：
- PVC 名称：`pvc-mysql-data-<appID>`
- Volume 类型：`PersistentVolumeClaim`
- 挂载路径：`/var/lib/mysql`

### 2. 直接命名 PVC（不带 appID 后缀）

```json
{
  "storage": [
    {
      "type": "persistent",
      "name": "shared-data",
      "mountPath": "/data",
      "size": "5Gi",
      "TmpCreate": false
    }
  ]
}
```

**生成结果**：
- 创建名为 `shared-data` 的 PVC（名称不带 appID 后缀）
- Volume 类型：`PersistentVolumeClaim`
- 适用于多应用共享同一 PVC 的场景

### 3. 临时存储（EmptyDir）

```json
{
  "storage": [
    {
      "type": "ephemeral",
      "name": "cache",
      "mountPath": "/tmp/cache"
    }
  ]
}
```

**生成结果**：
- Volume 类型：`EmptyDir`
- Pod 删除后数据丢失

### 4. ConfigMap 挂载

```json
{
  "storage": [
    {
      "type": "config",
      "name": "app-config",
      "sourceName": "my-configmap",
      "mountPath": "/etc/config",
      "readOnly": true
    }
  ]
}
```

**生成结果**：
- 挂载名为 `my-configmap` 的 ConfigMap
- 默认文件权限：`0644`

### 5. Secret 挂载

```json
{
  "storage": [
    {
      "type": "secret",
      "name": "certs",
      "sourceName": "tls-secret",
      "mountPath": "/etc/ssl/certs",
      "readOnly": true
    }
  ]
}
```

**生成结果**：
- 挂载名为 `tls-secret` 的 Secret
- 默认文件权限：`0644`

### 6. 使用 SubPath 挂载

```json
{
  "storage": [
    {
      "type": "persistent",
      "name": "data",
      "mountPath": "/var/lib/mysql",
      "subPath": "mysql",
      "TmpCreate": true,
      "size": "5Gi"
    }
  ]
}
```

**生成结果**：
- 仅挂载 PVC 中的 `mysql` 子目录到 `/var/lib/mysql`
- 同一 PVC 可被多个容器以不同 subPath 挂载

### 7. 多容器共享存储（Init/Sidecar）

多容器共享存储时，**只需在主容器的 storage 中声明 PVC 创建**，Init/Sidecar 容器使用 `ephemeral` 类型声明挂载意图即可。系统会根据 Volume 名称自动关联。

```json
{
  "storage": [
    {
      "type": "persistent",
      "name": "shared-data",
      "mountPath": "/data",
      "TmpCreate": true,
      "size": "5Gi"
    }
  ],
  "init": [
    {
      "name": "init-data",
      "properties": {
        "image": "busybox:latest"
      },
      "traits": {
        "storage": [
          {
            "type": "ephemeral",
            "name": "shared-data",
            "mountPath": "/init-data"
          }
        ]
      }
    }
  ],
  "sidecar": [
    {
      "name": "backup",
      "image": "backup-agent:v1",
      "traits": {
        "storage": [
          {
            "type": "ephemeral",
            "name": "shared-data",
            "mountPath": "/backup-source",
            "readOnly": true
          }
        ]
      }
    }
  ]
}
```

**说明**：
- 主容器声明 `type: persistent` 并设置 `TmpCreate: true`，系统创建 PVC
- Init/Sidecar 使用 `type: ephemeral` + 相同的 `name` 声明挂载意图
- 系统会自动去重：同名 Volume 只创建一次，各容器的 VolumeMount 独立配置
- 各容器可使用不同的 `mountPath` 和 `readOnly` 设置

> ⚠️ **注意**：如果 Init/Sidecar 也使用 `type: persistent`，由于 `TmpCreate` 默认为 `false`，会尝试创建额外的同名 PVC，导致资源冲突或创建多余对象。

---

## 注意事项

1. **名称规范**：`name` 字段必须符合 Kubernetes DNS-1123 子域名规范，系统会自动进行规范化处理（小写化、移除非法字符）

2. **PVC 生命周期**：动态创建的 PVC（`TmpCreate: true`）生命周期与应用绑定，删除应用时需考虑 PVC 清理策略

3. **StorageClass**：确保指定的 StorageClass 在目标集群中存在，否则 PVC 将处于 Pending 状态

4. **SubPath 与空目录**：使用 `subPath` 时，如果子目录不存在，Kubernetes 不会自动创建，可能导致挂载失败

5. **只读模式**：对于 ConfigMap 和 Secret 类型，建议始终设置 `readOnly: true`

6. **Volume 去重**：当多个容器（主容器、Init、Sidecar）声明同名存储时，系统只创建一个 Volume，但各自的 VolumeMount 独立配置

7. **多容器共享存储**：Init/Sidecar 容器共享主容器的 PVC 时，应使用 `type: ephemeral` 声明挂载意图，避免重复声明 `type: persistent` 导致创建多余的 PVC 对象

8. **claimName 字段**：该字段当前未实现，请勿依赖此字段指定已存在的 PVC 名称

---

## 相关代码位置

| 文件 | 说明 |
|------|------|
| `pkg/apiserver/domain/spec/traits.go` | StorageTraitSpec 结构体定义 |
| `pkg/apiserver/workflow/traits/storage.go` | StorageProcessor 处理逻辑 |
| `pkg/apiserver/config/consts.go` | 存储类型常量映射 |
| `pkg/apiserver/workflow/naming/naming.go` | PVC 命名规则 |

---

## 版本历史

| 版本 | 日期 | 说明 |
|------|------|------|
| v1.0 | 2025-12-08 | 初始版本，支持 persistent/ephemeral/config/secret 类型 |
| v1.1 | 2025-12-08 | 修正文档：标注 claimName 字段未实现；修正多容器共享存储示例 |

