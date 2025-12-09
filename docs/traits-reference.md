# OAM Traits 参考文档

## 概述

KubeMin-Cli 基于 OAM (Open Application Model) 设计，将 Kubernetes 中的底层概念抽象为 **Traits（特征）**。Traits 是可组合的能力原子，使组件不再依赖复杂的 YAML，而是用声明式的方式来构建复杂应用。

本文档详细介绍系统支持的所有 Traits 特性及其使用方法。

## Traits 总览

| Trait 名称 | 说明 | 对应 Kubernetes 资源 |
|-----------|------|---------------------|
| [Storage](#storage-存储) | 存储挂载 | PVC, EmptyDir, ConfigMap, Secret Volume |
| [Init](#init-初始化容器) | 初始化容器 | InitContainer |
| [Sidecar](#sidecar-边车容器) | 边车容器 | Container |
| [Envs](#envs-简化环境变量) | 单个环境变量定义 | EnvVar |
| [EnvFrom](#envfrom-环境变量批量导入) | 批量导入环境变量 | EnvFromSource |
| [Probes](#probes-健康探针) | 健康检查探针 | LivenessProbe, ReadinessProbe, StartupProbe |
| [Resources](#resources-资源限制) | 计算资源限制 | ResourceRequirements |
| [Ingress](#ingress-入口流量) | 入口流量路由 | Ingress |
| [RBAC](#rbac-权限控制) | 权限访问控制 | ServiceAccount, Role, RoleBinding, ClusterRole, ClusterRoleBinding |

---

## Storage 存储

> 详细文档请参考 [架构文档 - Storage 存储](架构文档.md#storage-存储)

Storage Trait 用于将存储卷挂载到容器中，支持多种存储类型。

### 存储类型

| type | 对应 Kubernetes 资源 | 说明 |
|------|---------------------|------|
| persistent | PersistentVolumeClaim + VolumeMount | 持久化存储，数据不会因 Pod 重启丢失 |
| ephemeral | EmptyDir + VolumeMount | 临时存储，Pod 删除后数据丢失 |
| config | ConfigMap + VolumeMount | 挂载 ConfigMap 到容器 |
| secret | Secret + VolumeMount | 挂载 Secret 到容器 |

### 字段详解

| 字段 | 类型 | 限制 | 默认值 | 说明 |
|------|------|------|--------|------|
| name | string | 必填 | - | 存储卷的唯一名称标识符 |
| type | string | 必填 | - | 存储类型：persistent/ephemeral/config/secret |
| mountPath | string | 可选 | `/mnt/<name>` | 容器内挂载路径 |
| subPath | string | 可选 | `""` | 挂载 Volume 内的子路径 |
| readOnly | bool | 可选 | `false` | 是否只读挂载 |
| sourceName | string | 可选 | `name` | ConfigMap/Secret 的实际资源名称 |
| size | string | 可选 | `"1Gi"` | PVC 存储容量（仅 persistent 类型） |
| tmpCreate | bool | 可选 | `false` | 是否动态创建 PVC（仅 persistent 类型） |
| storageClass | string | 可选 | - | StorageClass 名称（仅 persistent 类型） |

### 使用示例

#### 持久化存储

```json
{
  "storage": [
    {
      "type": "persistent",
      "name": "mysql-data",
      "mountPath": "/var/lib/mysql",
      "tmpCreate": true,
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
- StorageClass：`fast-ssd`
- 容量：`10Gi`

#### 临时存储

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
- 挂载路径：`/tmp/cache`
- 特性：Pod 删除后数据丢失

#### ConfigMap 挂载

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

- Volume 类型：`ConfigMap`
- ConfigMap 名称：`my-configmap`
- 挂载路径：`/etc/config`
- 挂载模式：只读
- 默认文件权限：`0644`

---

## Init 初始化容器

Init Trait 用于定义在主容器启动前运行的初始化容器。初始化容器按顺序执行，每个必须成功完成后才会启动下一个。

### 对应 Kubernetes 资源

- **InitContainer**：Pod 的初始化容器

### 字段详解

| 字段 | 类型 | 限制 | 默认值 | 说明 |
|------|------|------|--------|------|
| name | string | 必填 | - | 初始化容器名称，如为空则自动生成 |
| properties | object | 必填 | - | 容器属性配置 |
| properties.image | string | 必填 | - | 容器镜像 |
| properties.command | []string | 可选 | - | 容器启动命令 |
| properties.env | map[string]string | 可选 | - | 环境变量键值对 |
| traits | object | 可选 | - | 嵌套 Traits，支持 storage、envs、envFrom、resources |

### 逻辑详解

1. 系统为每个 Init Trait 创建一个 InitContainer
2. 如果 `name` 为空，自动生成格式为 `<组件名>-init-<随机字符>` 的名称
3. 支持嵌套 Traits（但不能嵌套 init 自身，防止无限循环）
4. 嵌套的 storage/envs/resources 等 Traits 会应用到该初始化容器

### 使用示例

#### 基础初始化容器

```json
{
  "init": [
    {
      "name": "init-permissions",
      "properties": {
        "image": "busybox:latest",
        "command": ["sh", "-c", "chmod -R 755 /data"],
        "env": {
          "DATA_DIR": "/data"
        }
      }
    }
  ]
}
```

**生成结果**：

- InitContainer 名称：`init-permissions`
- 镜像：`busybox:latest`
- 启动命令：`sh -c "chmod -R 755 /data"`
- 环境变量：`DATA_DIR=/data`
- ImagePullPolicy：`IfNotPresent`

#### 带存储挂载的初始化容器

```json
{
  "init": [
    {
      "name": "init-data",
      "properties": {
        "image": "busybox:latest",
        "command": ["sh", "-c", "cp /config/* /app/config/"]
      },
      "traits": {
        "storage": [
          {
            "type": "config",
            "name": "app-config",
            "sourceName": "my-configmap",
            "mountPath": "/config",
            "readOnly": true
          },
          {
            "type": "ephemeral",
            "name": "shared-data",
            "mountPath": "/app/config"
          }
        ]
      }
    }
  ]
}
```

**生成结果**：

- InitContainer 名称：`init-data`
- 镜像：`busybox:latest`
- Volume 1：ConfigMap `my-configmap` 挂载到 `/config`（只读）
- Volume 2：EmptyDir `shared-data` 挂载到 `/app/config`
- 用途：将 ConfigMap 内容复制到共享存储供主容器使用

#### 数据库迁移初始化

```json
{
  "init": [
    {
      "name": "db-migration",
      "properties": {
        "image": "flyway/flyway:latest",
        "command": ["flyway", "migrate"]
      },
      "traits": {
        "envs": [
          {
            "name": "FLYWAY_URL",
            "valueFrom": {
              "secret": {
                "name": "db-credentials",
                "key": "jdbc-url"
              }
            }
          },
          {
            "name": "FLYWAY_USER",
            "valueFrom": {
              "secret": {
                "name": "db-credentials",
                "key": "username"
              }
            }
          }
        ],
        "resources": {
          "cpu": "100m",
          "memory": "256Mi"
        }
      }
    }
  ]
}
```

**生成结果**：

- InitContainer 名称：`db-migration`
- 镜像：`flyway/flyway:latest`
- 环境变量 `FLYWAY_URL`：从 Secret `db-credentials` 的 `jdbc-url` 键读取
- 环境变量 `FLYWAY_USER`：从 Secret `db-credentials` 的 `username` 键读取
- 资源限制：CPU `100m`，内存 `256Mi`
- 用途：在主应用启动前执行数据库迁移

### 注意事项

1. **镜像必填**：`properties.image` 是必填字段，否则会报错
2. **顺序执行**：多个 Init 容器按数组顺序执行
3. **嵌套限制**：Init 容器的嵌套 Traits 不能包含 `init` 自身
4. **共享存储**：Init 容器可以与主容器共享 Volume，用于数据准备

---

## Sidecar 边车容器

Sidecar Trait 用于定义与主容器并行运行的辅助容器，常用于日志收集、代理、监控等场景。

### 对应 Kubernetes 资源

- **Container**：Pod 中的额外容器

### 字段详解

| 字段 | 类型 | 限制 | 默认值 | 说明 |
|------|------|------|--------|------|
| name | string | 必填 | - | 边车容器名称，如为空则自动生成 |
| image | string | 必填 | - | 容器镜像 |
| command | []string | 可选 | - | 容器启动命令 |
| args | []string | 可选 | - | 命令参数 |
| env | map[string]string | 可选 | - | 环境变量键值对 |
| traits | object | 可选 | - | 嵌套 Traits，支持 storage、envs、envFrom、probes、resources |

### 逻辑详解

1. 系统为每个 Sidecar Trait 创建一个额外的 Container
2. 如果 `name` 为空，自动生成格式为 `<组件名>-sidecar-<随机字符>` 的名称
3. 支持嵌套 Traits（但不能嵌套 sidecar 和 init）
4. 边车容器可以有独立的健康探针和资源限制

### 使用示例

#### 日志收集边车

```json
{
  "sidecar": [
    {
      "name": "fluentd",
      "image": "fluent/fluentd:v1.14",
      "env": {
        "FLUENTD_CONF": "fluent.conf"
      },
      "traits": {
        "storage": [
          {
            "type": "ephemeral",
            "name": "app-logs",
            "mountPath": "/var/log/app"
          }
        ],
        "resources": {
          "cpu": "100m",
          "memory": "128Mi"
        }
      }
    }
  ]
}
```

**生成结果**：

- Container 名称：`fluentd`
- 镜像：`fluent/fluentd:v1.14`
- 环境变量：`FLUENTD_CONF=fluent.conf`
- Volume：EmptyDir `app-logs` 挂载到 `/var/log/app`
- 资源限制：CPU `100m`，内存 `128Mi`
- 用途：收集主容器写入 `/var/log/app` 的日志

#### Envoy 代理边车

```json
{
  "sidecar": [
    {
      "name": "envoy-proxy",
      "image": "envoyproxy/envoy:v1.25.0",
      "command": ["envoy"],
      "args": ["-c", "/etc/envoy/envoy.yaml"],
      "traits": {
        "storage": [
          {
            "type": "config",
            "name": "envoy-config",
            "sourceName": "envoy-configmap",
            "mountPath": "/etc/envoy"
          }
        ],
        "probes": [
          {
            "type": "readiness",
            "httpGet": {
              "path": "/ready",
              "port": 9901
            },
            "initialDelaySeconds": 5,
            "periodSeconds": 10
          }
        ],
        "resources": {
          "cpu": "200m",
          "memory": "256Mi"
        }
      }
    }
  ]
}
```

**生成结果**：

- Container 名称：`envoy-proxy`
- 镜像：`envoyproxy/envoy:v1.25.0`
- 启动命令：`envoy -c /etc/envoy/envoy.yaml`
- Volume：ConfigMap `envoy-configmap` 挂载到 `/etc/envoy`
- ReadinessProbe：HTTP GET `/ready:9901`，延迟 5s，间隔 10s
- 资源限制：CPU `200m`，内存 `256Mi`
- 用途：为主容器提供代理功能

#### 监控指标导出边车

```json
{
  "sidecar": [
    {
      "name": "prometheus-exporter",
      "image": "prom/node-exporter:v1.5.0",
      "args": ["--path.procfs=/host/proc", "--path.sysfs=/host/sys"],
      "traits": {
        "probes": [
          {
            "type": "liveness",
            "httpGet": {
              "path": "/metrics",
              "port": 9100
            },
            "periodSeconds": 30
          }
        ]
      }
    }
  ]
}
```

**生成结果**：

- Container 名称：`prometheus-exporter`
- 镜像：`prom/node-exporter:v1.5.0`
- 参数：`--path.procfs=/host/proc --path.sysfs=/host/sys`
- LivenessProbe：HTTP GET `/metrics:9100`，间隔 30s
- 用途：为 Prometheus 提供监控指标

### 注意事项

1. **镜像必填**：`image` 是必填字段
2. **禁止嵌套边车**：Sidecar 的嵌套 Traits 不能包含 `sidecar`
3. **资源规划**：边车容器会占用 Pod 资源，需要合理规划
4. **共享网络**：边车与主容器共享同一网络命名空间，可通过 localhost 通信

---

## Envs 简化环境变量

Envs Trait 提供用户友好的方式定义单个环境变量，支持多种值来源。

### 对应 Kubernetes 资源

- **EnvVar**：容器环境变量

### 字段详解

| 字段 | 类型 | 限制 | 默认值 | 说明 |
|------|------|------|--------|------|
| name | string | 必填 | - | 环境变量名称 |
| valueFrom | object | 必填 | - | 值来源，四种来源选其一 |
| valueFrom.static | *string | 可选 | - | 静态字符串值 |
| valueFrom.secret | object | 可选 | - | 从 Secret 中读取 |
| valueFrom.secret.name | string | 必填 | - | Secret 资源名称 |
| valueFrom.secret.key | string | 必填 | - | Secret 中的 key |
| valueFrom.config | object | 可选 | - | 从 ConfigMap 中读取 |
| valueFrom.config.name | string | 必填 | - | ConfigMap 资源名称 |
| valueFrom.config.key | string | 必填 | - | ConfigMap 中的 key |
| valueFrom.field | *string | 可选 | - | 从 Pod 字段读取（FieldRef） |

### 值来源类型

| 类型 | 说明 | Kubernetes 对应 |
|------|------|-----------------|
| static | 静态字符串值 | `value` |
| secret | 从 Secret 引用 | `secretKeyRef` |
| config | 从 ConfigMap 引用 | `configMapKeyRef` |
| field | 从 Pod 字段引用 | `fieldRef` |

### 常用 Field 字段

| 字段路径 | 说明 |
|---------|------|
| `metadata.name` | Pod 名称 |
| `metadata.namespace` | Pod 命名空间 |
| `metadata.uid` | Pod UID |
| `spec.nodeName` | 所在节点名称 |
| `spec.serviceAccountName` | ServiceAccount 名称 |
| `status.podIP` | Pod IP 地址 |
| `status.hostIP` | 宿主机 IP |

### 使用示例

#### 静态值

```json
{
  "envs": [
    {
      "name": "APP_ENV",
      "valueFrom": {
        "static": "production"
      }
    },
    {
      "name": "LOG_LEVEL",
      "valueFrom": {
        "static": "info"
      }
    }
  ]
}
```

**生成结果**：

```yaml
env:
  - name: APP_ENV
    value: "production"
  - name: LOG_LEVEL
    value: "info"
```

#### 从 Secret 读取

```json
{
  "envs": [
    {
      "name": "DB_PASSWORD",
      "valueFrom": {
        "secret": {
          "name": "db-credentials",
          "key": "password"
        }
      }
    },
    {
      "name": "API_KEY",
      "valueFrom": {
        "secret": {
          "name": "api-secrets",
          "key": "key"
        }
      }
    }
  ]
}
```

**生成结果**：

```yaml
env:
  - name: DB_PASSWORD
    valueFrom:
      secretKeyRef:
        name: db-credentials
        key: password
  - name: API_KEY
    valueFrom:
      secretKeyRef:
        name: api-secrets
        key: key
```

#### 从 ConfigMap 读取

```json
{
  "envs": [
    {
      "name": "DATABASE_URL",
      "valueFrom": {
        "config": {
          "name": "app-config",
          "key": "database_url"
        }
      }
    }
  ]
}
```

**生成结果**：

```yaml
env:
  - name: DATABASE_URL
    valueFrom:
      configMapKeyRef:
        name: app-config
        key: database_url
```

#### 从 Pod 字段读取

```json
{
  "envs": [
    {
      "name": "POD_NAME",
      "valueFrom": {
        "field": "metadata.name"
      }
    },
    {
      "name": "POD_IP",
      "valueFrom": {
        "field": "status.podIP"
      }
    },
    {
      "name": "NODE_NAME",
      "valueFrom": {
        "field": "spec.nodeName"
      }
    }
  ]
}
```

**生成结果**：

```yaml
env:
  - name: POD_NAME
    valueFrom:
      fieldRef:
        apiVersion: v1
        fieldPath: metadata.name
  - name: POD_IP
    valueFrom:
      fieldRef:
        apiVersion: v1
        fieldPath: status.podIP
  - name: NODE_NAME
    valueFrom:
      fieldRef:
        apiVersion: v1
        fieldPath: spec.nodeName
```

#### 混合使用

```json
{
  "envs": [
    {
      "name": "APP_NAME",
      "valueFrom": {
        "static": "my-service"
      }
    },
    {
      "name": "DB_HOST",
      "valueFrom": {
        "config": {
          "name": "db-config",
          "key": "host"
        }
      }
    },
    {
      "name": "DB_PASSWORD",
      "valueFrom": {
        "secret": {
          "name": "db-credentials",
          "key": "password"
        }
      }
    },
    {
      "name": "INSTANCE_ID",
      "valueFrom": {
        "field": "metadata.name"
      }
    }
  ]
}
```

### 注意事项

1. **单一来源**：每个环境变量的 `valueFrom` 只能指定一种来源
2. **资源存在**：引用的 Secret 或 ConfigMap 必须在同一命名空间中存在
3. **敏感数据**：敏感信息应使用 `secret` 来源，避免使用 `static`
4. **与 Properties.Env 区别**：`envs` Trait 支持动态引用，而 `properties.env` 仅支持静态值

---

## EnvFrom 环境变量批量导入

EnvFrom Trait 用于从 ConfigMap 或 Secret 批量导入所有键值对作为环境变量。

### 对应 Kubernetes 资源

- **EnvFromSource**：批量环境变量来源

### 字段详解

| 字段 | 类型 | 限制 | 默认值 | 说明 |
|------|------|------|--------|------|
| type | string | 必填 | - | 来源类型：`secret` 或 `configMap` |
| sourceName | string | 必填 | - | ConfigMap 或 Secret 的资源名称 |

### 使用示例

#### 从 ConfigMap 批量导入

```json
{
  "envFrom": [
    {
      "type": "configMap",
      "sourceName": "app-config"
    }
  ]
}
```

**生成结果**：

```yaml
envFrom:
  - configMapRef:
      name: app-config
```

假设 ConfigMap `app-config` 内容为：

```yaml
data:
  DATABASE_HOST: mysql.default.svc
  DATABASE_PORT: "3306"
  LOG_FORMAT: json
```

则容器会自动获得三个环境变量：`DATABASE_HOST`、`DATABASE_PORT`、`LOG_FORMAT`。

#### 从 Secret 批量导入

```json
{
  "envFrom": [
    {
      "type": "secret",
      "sourceName": "app-secrets"
    }
  ]
}
```

**生成结果**：

```yaml
envFrom:
  - secretRef:
      name: app-secrets
```

#### 同时从多个来源导入

```json
{
  "envFrom": [
    {
      "type": "configMap",
      "sourceName": "app-config"
    },
    {
      "type": "secret",
      "sourceName": "app-secrets"
    }
  ]
}
```

**生成结果**：

```yaml
envFrom:
  - configMapRef:
      name: app-config
  - secretRef:
      name: app-secrets
```

### 注意事项

1. **键名冲突**：如果多个来源有相同的 key，后定义的会覆盖先定义的
2. **资源必须存在**：引用的 ConfigMap/Secret 必须事先存在
3. **与 Envs 配合**：可以同时使用 `envFrom` 批量导入和 `envs` 单独定义
4. **无法选择性导入**：`envFrom` 会导入所有键值对，如需选择性导入请使用 `envs`

---

## Probes 健康探针

Probes Trait 用于定义容器的健康检查探针，Kubernetes 根据探针结果决定容器的运行状态。

### 对应 Kubernetes 资源

- **LivenessProbe**：存活探针，失败时重启容器
- **ReadinessProbe**：就绪探针，失败时从 Service 摘除
- **StartupProbe**：启动探针，成功前不执行其他探针

### 字段详解

#### 基础字段

| 字段 | 类型 | 限制 | 默认值 | 说明 |
|------|------|------|--------|------|
| type | string | 必填 | - | 探针类型：`liveness`/`readiness`/`startup` |
| initialDelaySeconds | int32 | 可选 | 0 | 容器启动后延迟多少秒开始探测 |
| periodSeconds | int32 | 可选 | 10 | 探测间隔秒数 |
| timeoutSeconds | int32 | 可选 | 1 | 探测超时秒数 |
| failureThreshold | int32 | 可选 | 3 | 连续失败多少次视为失败 |
| successThreshold | int32 | 可选 | 1 | 连续成功多少次视为成功 |

#### 探测方式（三选一）

| 字段 | 类型 | 说明 |
|------|------|------|
| exec | object | 执行命令探测 |
| exec.command | []string | 要执行的命令 |
| httpGet | object | HTTP GET 探测 |
| httpGet.path | string | HTTP 请求路径 |
| httpGet.port | int | HTTP 请求端口 |
| httpGet.host | string | 主机名（可选） |
| httpGet.scheme | string | HTTP 或 HTTPS（可选） |
| tcpSocket | object | TCP 端口探测 |
| tcpSocket.port | int | TCP 端口 |
| tcpSocket.host | string | 主机名（可选） |

### 探针类型说明

| 类型 | 用途 | 失败后果 |
|------|------|---------|
| liveness | 检测容器是否存活 | 重启容器 |
| readiness | 检测容器是否就绪 | 从 Service 端点移除 |
| startup | 检测应用是否启动完成 | 阻止 liveness/readiness 探测 |

### 使用示例

#### HTTP 健康检查

```json
{
  "probes": [
    {
      "type": "liveness",
      "httpGet": {
        "path": "/healthz",
        "port": 8080
      },
      "initialDelaySeconds": 30,
      "periodSeconds": 10,
      "timeoutSeconds": 5,
      "failureThreshold": 3
    },
    {
      "type": "readiness",
      "httpGet": {
        "path": "/ready",
        "port": 8080
      },
      "initialDelaySeconds": 5,
      "periodSeconds": 5
    }
  ]
}
```

**生成结果**：

```yaml
livenessProbe:
  httpGet:
    path: /healthz
    port: 8080
  initialDelaySeconds: 30
  periodSeconds: 10
  timeoutSeconds: 5
  failureThreshold: 3
readinessProbe:
  httpGet:
    path: /ready
    port: 8080
  initialDelaySeconds: 5
  periodSeconds: 5
```

#### TCP 端口检查

```json
{
  "probes": [
    {
      "type": "liveness",
      "tcpSocket": {
        "port": 3306
      },
      "initialDelaySeconds": 30,
      "periodSeconds": 15
    },
    {
      "type": "readiness",
      "tcpSocket": {
        "port": 3306
      },
      "initialDelaySeconds": 5,
      "periodSeconds": 10
    }
  ]
}
```

**生成结果**：

```yaml
livenessProbe:
  tcpSocket:
    port: 3306
  initialDelaySeconds: 30
  periodSeconds: 15
readinessProbe:
  tcpSocket:
    port: 3306
  initialDelaySeconds: 5
  periodSeconds: 10
```

#### 命令执行检查

```json
{
  "probes": [
    {
      "type": "liveness",
      "exec": {
        "command": ["cat", "/tmp/healthy"]
      },
      "initialDelaySeconds": 5,
      "periodSeconds": 5
    }
  ]
}
```

**生成结果**：

```yaml
livenessProbe:
  exec:
    command:
      - cat
      - /tmp/healthy
  initialDelaySeconds: 5
  periodSeconds: 5
```

#### 慢启动应用配置

```json
{
  "probes": [
    {
      "type": "startup",
      "httpGet": {
        "path": "/healthz",
        "port": 8080
      },
      "initialDelaySeconds": 0,
      "periodSeconds": 10,
      "failureThreshold": 30
    },
    {
      "type": "liveness",
      "httpGet": {
        "path": "/healthz",
        "port": 8080
      },
      "periodSeconds": 10
    },
    {
      "type": "readiness",
      "httpGet": {
        "path": "/ready",
        "port": 8080
      },
      "periodSeconds": 5
    }
  ]
}
```

**生成结果**：

```yaml
startupProbe:
  httpGet:
    path: /healthz
    port: 8080
  periodSeconds: 10
  failureThreshold: 30       # 允许最多 300s (30*10s) 完成启动
livenessProbe:
  httpGet:
    path: /healthz
    port: 8080
  periodSeconds: 10          # startup 成功后才开始执行
readinessProbe:
  httpGet:
    path: /ready
    port: 8080
  periodSeconds: 5           # startup 成功后才开始执行
```

#### HTTPS 健康检查

```json
{
  "probes": [
    {
      "type": "readiness",
      "httpGet": {
        "path": "/health",
        "port": 443,
        "scheme": "HTTPS"
      },
      "periodSeconds": 10
    }
  ]
}
```

### 注意事项

1. **单一探测方式**：每个探针只能指定 `exec`、`httpGet`、`tcpSocket` 中的一个
2. **每种类型唯一**：同一组件的每种探针类型只能定义一个
3. **启动探针优先**：配置 `startup` 探针时，其他探针在启动成功前不会执行
4. **合理配置阈值**：根据应用特性合理设置 `initialDelaySeconds` 和 `failureThreshold`
5. **端点可用性**：确保探测端点在容器启动后尽快可用

---

## Resources 资源限制

Resources Trait 用于定义容器的计算资源限制，包括 CPU、内存和 GPU。

### 对应 Kubernetes 资源

- **ResourceRequirements**：容器资源请求和限制

### 字段详解

| 字段 | 类型 | 限制 | 默认值 | 说明 |
|------|------|------|--------|------|
| cpu | string | 可选 | - | CPU 资源，如 `100m`、`0.5`、`2` |
| memory | string | 可选 | - | 内存资源，如 `128Mi`、`1Gi`、`2G` |
| gpu | string | 可选 | - | GPU 数量，如 `1`、`2` |

### 资源单位说明

#### CPU 单位

| 格式 | 说明 | 示例 |
|------|------|------|
| 小数 | CPU 核心数 | `0.5` = 半个核心 |
| m | 毫核（1核=1000m） | `500m` = 半个核心 |
| 整数 | CPU 核心数 | `2` = 2个核心 |

#### 内存单位

| 格式 | 说明 | 示例 |
|------|------|------|
| Ki | Kibibyte (1024) | `512Ki` |
| Mi | Mebibyte (1024²) | `256Mi` |
| Gi | Gibibyte (1024³) | `2Gi` |
| K | Kilobyte (1000) | `500K` |
| M | Megabyte (1000²) | `256M` |
| G | Gigabyte (1000³) | `1G` |

### 逻辑详解

1. 系统将指定值同时设置为 `requests` 和 `limits`
2. GPU 资源默认使用 `nvidia.com/gpu` 作为资源名称
3. 未指定的字段不会设置限制

### 使用示例

#### Web 应用资源配置

```json
{
  "resources": {
    "cpu": "500m",
    "memory": "512Mi"
  }
}
```

**生成结果**：

```yaml
resources:
  requests:
    cpu: 500m
    memory: 512Mi
  limits:
    cpu: 500m
    memory: 512Mi
```

- QoS 等级：`Guaranteed`（requests = limits）

#### 数据库资源配置

```json
{
  "resources": {
    "cpu": "2",
    "memory": "4Gi"
  }
}
```

**生成结果**：

```yaml
resources:
  requests:
    cpu: "2"
    memory: 4Gi
  limits:
    cpu: "2"
    memory: 4Gi
```

#### GPU 工作负载

```json
{
  "resources": {
    "cpu": "4",
    "memory": "16Gi",
    "gpu": "1"
  }
}
```

**生成结果**：

```yaml
resources:
  requests:
    cpu: "4"
    memory: 16Gi
    nvidia.com/gpu: "1"
  limits:
    cpu: "4"
    memory: 16Gi
    nvidia.com/gpu: "1"
```

- GPU 资源名称默认使用 `nvidia.com/gpu`

#### 轻量级微服务

```json
{
  "resources": {
    "cpu": "100m",
    "memory": "128Mi"
  }
}
```

#### 高性能计算

```json
{
  "resources": {
    "cpu": "8",
    "memory": "32Gi",
    "gpu": "4"
  }
}
```

### 注意事项

1. **格式正确**：资源值必须是有效的 Kubernetes 资源量格式
2. **节点容量**：确保集群节点有足够的资源
3. **GPU 支持**：使用 GPU 需要集群安装相应的设备插件
4. **QoS 等级**：同时设置 requests 和 limits 会使 Pod 获得 `Guaranteed` QoS
5. **嵌套使用**：Resources 可以嵌套在 Init 或 Sidecar 中单独配置

---

## Ingress 入口流量

Ingress Trait 用于创建 Kubernetes Ingress 资源，将外部 HTTP/HTTPS 流量路由到服务。

### 对应 Kubernetes 资源

- **Ingress**：入口资源

### 字段详解

#### 基础字段

| 字段 | 类型 | 限制 | 默认值 | 说明 |
|------|------|------|--------|------|
| name | string | 可选 | `<组件名>-ingress` | Ingress 资源名称 |
| namespace | string | 可选 | 组件命名空间 | Ingress 所在命名空间 |
| hosts | []string | 可选 | - | 全局主机名列表 |
| label | map[string]string | 可选 | - | 标签 |
| annotations | map[string]string | 可选 | - | 注解 |
| ingressClassName | string | 可选 | - | Ingress Class 名称 |
| defaultPathType | string | 可选 | `Prefix` | 默认路径匹配类型 |
| tls | []object | 可选 | - | TLS 配置 |
| routes | []object | 必填 | - | 路由规则 |

#### TLS 配置

| 字段 | 类型 | 说明 |
|------|------|------|
| secretName | string | TLS 证书 Secret 名称 |
| hosts | []string | 该证书适用的主机列表 |

#### Routes 路由规则

| 字段 | 类型 | 限制 | 默认值 | 说明 |
|------|------|------|--------|------|
| path | string | 可选 | `/` | URL 路径 |
| pathType | string | 可选 | `Prefix` | 路径类型：Prefix/Exact/ImplementationSpecific |
| host | string | 可选 | - | 路由级主机名（覆盖全局 hosts） |
| backend | object | 必填 | - | 后端服务配置 |
| backend.serviceName | string | 必填 | - | 服务名称 |
| backend.servicePort | int32 | 可选 | 80 | 服务端口 |
| rewrite | object | 可选 | - | 路径重写配置 |

#### Rewrite 重写配置

| 字段 | 类型 | 说明 |
|------|------|------|
| type | string | 重写类型：replace/regexReplace/prefix |
| match | string | 匹配模式 |
| replacement | string | 替换值 |

### 使用示例

#### 基础路由

```json
{
  "ingress": [
    {
      "name": "my-app-ingress",
      "ingressClassName": "nginx",
      "routes": [
        {
          "path": "/",
          "backend": {
            "serviceName": "my-app-service",
            "servicePort": 80
          }
        }
      ]
    }
  ]
}
```

**生成结果**：

- Ingress 名称：`my-app-ingress`
- Ingress Class：`nginx`
- 路由规则：`/` → `my-app-service:80`
- 路径类型：`Prefix`（默认）

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: my-app-ingress
spec:
  ingressClassName: nginx
  rules:
    - http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: my-app-service
                port:
                  number: 80
```

#### 带主机名的路由

```json
{
  "ingress": [
    {
      "name": "api-ingress",
      "ingressClassName": "nginx",
      "hosts": ["api.example.com"],
      "routes": [
        {
          "path": "/v1",
          "backend": {
            "serviceName": "api-v1",
            "servicePort": 8080
          }
        },
        {
          "path": "/v2",
          "backend": {
            "serviceName": "api-v2",
            "servicePort": 8080
          }
        }
      ]
    }
  ]
}
```

#### TLS 配置

```json
{
  "ingress": [
    {
      "name": "secure-ingress",
      "ingressClassName": "nginx",
      "tls": [
        {
          "secretName": "tls-secret",
          "hosts": ["secure.example.com"]
        }
      ],
      "routes": [
        {
          "host": "secure.example.com",
          "path": "/",
          "backend": {
            "serviceName": "secure-app",
            "servicePort": 443
          }
        }
      ]
    }
  ]
}
```

**生成结果**：

- Ingress 名称：`secure-ingress`
- TLS 证书：Secret `tls-secret` 用于 `secure.example.com`
- 路由：`https://secure.example.com/` → `secure-app:443`

```yaml
spec:
  ingressClassName: nginx
  tls:
    - hosts:
        - secure.example.com
      secretName: tls-secret
  rules:
    - host: secure.example.com
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: secure-app
                port:
                  number: 443
```

#### 路径重写

```json
{
  "ingress": [
    {
      "name": "rewrite-ingress",
      "ingressClassName": "nginx",
      "annotations": {
        "nginx.ingress.kubernetes.io/use-regex": "true"
      },
      "routes": [
        {
          "path": "/api(/.*)",
          "pathType": "ImplementationSpecific",
          "host": "app.example.com",
          "backend": {
            "serviceName": "backend-service",
            "servicePort": 8080
          },
          "rewrite": {
            "type": "regexReplace",
            "replacement": "$1"
          }
        }
      ]
    }
  ]
}
```

#### 多主机多路由

```json
{
  "ingress": [
    {
      "name": "multi-host-ingress",
      "ingressClassName": "nginx",
      "label": {
        "app": "multi-tenant"
      },
      "tls": [
        {
          "secretName": "wildcard-tls",
          "hosts": ["*.example.com"]
        }
      ],
      "routes": [
        {
          "host": "app1.example.com",
          "path": "/",
          "backend": {
            "serviceName": "app1-service",
            "servicePort": 80
          }
        },
        {
          "host": "app2.example.com",
          "path": "/",
          "backend": {
            "serviceName": "app2-service",
            "servicePort": 80
          }
        },
        {
          "host": "api.example.com",
          "path": "/users",
          "backend": {
            "serviceName": "user-service",
            "servicePort": 8080
          }
        },
        {
          "host": "api.example.com",
          "path": "/orders",
          "backend": {
            "serviceName": "order-service",
            "servicePort": 8080
          }
        }
      ]
    }
  ]
}
```

### 注意事项

1. **路由必填**：`routes` 至少包含一个路由规则
2. **Ingress Controller**：需要集群中安装相应的 Ingress Controller（如 nginx-ingress）
3. **TLS 证书**：TLS 配置中引用的 Secret 必须存在且包含有效证书
4. **路径类型**：
   - `Prefix`：前缀匹配（默认）
   - `Exact`：精确匹配
   - `ImplementationSpecific`：由 Ingress Controller 决定
5. **注解差异**：不同 Ingress Controller 的注解语法可能不同

---

## RBAC 权限控制

RBAC Trait 用于创建 Kubernetes RBAC 资源，为组件配置细粒度的权限控制。

### 对应 Kubernetes 资源

- **ServiceAccount**：服务账户
- **Role**：命名空间级角色
- **ClusterRole**：集群级角色
- **RoleBinding**：命名空间级角色绑定
- **ClusterRoleBinding**：集群级角色绑定

### 字段详解

#### 基础字段

| 字段 | 类型 | 限制 | 默认值 | 说明 |
|------|------|------|--------|------|
| serviceAccount | string | 可选 | `<组件名>-sa` | ServiceAccount 名称 |
| namespace | string | 可选 | 组件命名空间 | 资源所在命名空间 |
| clusterScope | bool | 可选 | `false` | 是否创建集群级角色 |
| roleName | string | 可选 | `<sa名>-role` | Role/ClusterRole 名称 |
| bindingName | string | 可选 | `<sa名>-binding` | RoleBinding/ClusterRoleBinding 名称 |
| serviceAccountLabels | map[string]string | 可选 | - | ServiceAccount 标签 |
| serviceAccountAnnotations | map[string]string | 可选 | - | ServiceAccount 注解 |
| roleLabels | map[string]string | 可选 | - | Role 标签 |
| bindingLabels | map[string]string | 可选 | - | RoleBinding 标签 |
| automountServiceAccountToken | *bool | 可选 | - | 是否自动挂载 SA Token |
| rules | []object | 必填 | - | 权限规则列表 |

#### Rules 权限规则

| 字段 | 类型 | 限制 | 默认值 | 说明 |
|------|------|------|--------|------|
| apiGroups | []string | 可选 | - | API 组，`""` 表示核心组 |
| resources | []string | 可选 | - | 资源类型 |
| resourceNames | []string | 可选 | - | 具体资源名称 |
| nonResourceURLs | []string | 可选 | - | 非资源 URL |
| verbs | []string | 必填 | - | 操作动词 |

### 常用 API Groups

| API Group | 包含资源 |
|-----------|---------|
| `""` (core) | pods, services, configmaps, secrets, persistentvolumeclaims |
| `apps` | deployments, daemonsets, statefulsets, replicasets |
| `batch` | jobs, cronjobs |
| `networking.k8s.io` | ingresses, networkpolicies |
| `rbac.authorization.k8s.io` | roles, rolebindings, clusterroles |

### 常用 Verbs

| Verb | 说明 |
|------|------|
| get | 读取单个资源 |
| list | 列出资源 |
| watch | 监听资源变化 |
| create | 创建资源 |
| update | 更新资源 |
| patch | 部分更新资源 |
| delete | 删除单个资源 |
| deletecollection | 删除资源集合 |
| `*` | 所有操作 |

### 使用示例

#### 只读 Pod 权限

```json
{
  "rbac": [
    {
      "serviceAccount": "pod-reader",
      "rules": [
        {
          "apiGroups": [""],
          "resources": ["pods"],
          "verbs": ["get", "list", "watch"]
        }
      ]
    }
  ]
}
```

**生成结果**：

- ServiceAccount：`pod-reader`
- Role：`pod-reader-role`（命名空间级）
- RoleBinding：`pod-reader-binding`
- 权限：对 pods 资源的 get/list/watch 操作

```yaml
# ServiceAccount
apiVersion: v1
kind: ServiceAccount
metadata:
  name: pod-reader

# Role
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: pod-reader-role
rules:
  - apiGroups: [""]
    resources: ["pods"]
    verbs: ["get", "list", "watch"]

# RoleBinding
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: pod-reader-binding
subjects:
  - kind: ServiceAccount
    name: pod-reader
roleRef:
  kind: Role
  name: pod-reader-role
  apiGroup: rbac.authorization.k8s.io
```

#### ConfigMap 和 Secret 读取权限

```json
{
  "rbac": [
    {
      "serviceAccount": "config-reader",
      "rules": [
        {
          "apiGroups": [""],
          "resources": ["configmaps", "secrets"],
          "verbs": ["get", "list"]
        }
      ]
    }
  ]
}
```

#### Deployment 管理权限

```json
{
  "rbac": [
    {
      "serviceAccount": "deployment-manager",
      "rules": [
        {
          "apiGroups": ["apps"],
          "resources": ["deployments"],
          "verbs": ["get", "list", "watch", "create", "update", "patch", "delete"]
        },
        {
          "apiGroups": [""],
          "resources": ["pods"],
          "verbs": ["get", "list", "watch"]
        }
      ]
    }
  ]
}
```

#### 集群级权限

```json
{
  "rbac": [
    {
      "serviceAccount": "cluster-admin-sa",
      "clusterScope": true,
      "rules": [
        {
          "apiGroups": [""],
          "resources": ["nodes"],
          "verbs": ["get", "list", "watch"]
        },
        {
          "apiGroups": [""],
          "resources": ["namespaces"],
          "verbs": ["get", "list"]
        }
      ]
    }
  ]
}
```

**生成结果**：

- ServiceAccount：`cluster-admin-sa`
- ClusterRole：`cluster-admin-sa-role`（集群级）
- ClusterRoleBinding：`cluster-admin-sa-binding`
- 权限：对 nodes 的 get/list/watch，对 namespaces 的 get/list

```yaml
# ClusterRole（注意：不是 Role）
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: cluster-admin-sa-role
rules:
  - apiGroups: [""]
    resources: ["nodes"]
    verbs: ["get", "list", "watch"]
  - apiGroups: [""]
    resources: ["namespaces"]
    verbs: ["get", "list"]

# ClusterRoleBinding（注意：不是 RoleBinding）
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: cluster-admin-sa-binding
subjects:
  - kind: ServiceAccount
    name: cluster-admin-sa
    namespace: <组件命名空间>
roleRef:
  kind: ClusterRole
  name: cluster-admin-sa-role
  apiGroup: rbac.authorization.k8s.io
```

#### 特定资源名称权限

```json
{
  "rbac": [
    {
      "serviceAccount": "specific-config-reader",
      "rules": [
        {
          "apiGroups": [""],
          "resources": ["configmaps"],
          "resourceNames": ["app-config", "feature-flags"],
          "verbs": ["get"]
        }
      ]
    }
  ]
}
```

**生成结果**：

- 权限范围：仅能读取名为 `app-config` 和 `feature-flags` 的 ConfigMap
- 其他 ConfigMap 无法访问

```yaml
rules:
  - apiGroups: [""]
    resources: ["configmaps"]
    resourceNames: ["app-config", "feature-flags"]
    verbs: ["get"]
```

#### 带标签和注解的完整配置

```json
{
  "rbac": [
    {
      "serviceAccount": "backend-sa",
      "roleName": "backend-role",
      "bindingName": "backend-binding",
      "automountServiceAccountToken": false,
      "serviceAccountLabels": {
        "app": "backend",
        "env": "production"
      },
      "serviceAccountAnnotations": {
        "description": "Backend service account"
      },
      "roleLabels": {
        "app": "backend"
      },
      "bindingLabels": {
        "app": "backend"
      },
      "rules": [
        {
          "apiGroups": [""],
          "resources": ["pods", "services"],
          "verbs": ["get", "list", "watch"]
        },
        {
          "apiGroups": [""],
          "resources": ["configmaps"],
          "verbs": ["get"]
        }
      ]
    }
  ]
}
```

**生成结果**：

- ServiceAccount 名称：`backend-sa`（自定义）
- Role 名称：`backend-role`（自定义）
- RoleBinding 名称：`backend-binding`（自定义）
- automountServiceAccountToken：`false`（提高安全性）
- 所有资源带自定义标签

```yaml
# ServiceAccount
apiVersion: v1
kind: ServiceAccount
metadata:
  name: backend-sa
  labels:
    app: backend
    env: production
  annotations:
    description: Backend service account
automountServiceAccountToken: false

# Role
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: backend-role
  labels:
    app: backend
rules:
  - apiGroups: [""]
    resources: ["pods", "services"]
    verbs: ["get", "list", "watch"]
  - apiGroups: [""]
    resources: ["configmaps"]
    verbs: ["get"]

# RoleBinding
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: backend-binding
  labels:
    app: backend
# ...
```

#### 多策略配置

```json
{
  "rbac": [
    {
      "serviceAccount": "primary-sa",
      "rules": [
        {
          "apiGroups": [""],
          "resources": ["pods"],
          "verbs": ["get", "list"]
        }
      ]
    },
    {
      "serviceAccount": "secondary-sa",
      "clusterScope": true,
      "rules": [
        {
          "apiGroups": [""],
          "resources": ["nodes"],
          "verbs": ["get", "list"]
        }
      ]
    }
  ]
}
```

**生成结果**：

创建 6 个 Kubernetes 资源：

| 策略 | ServiceAccount | Role/ClusterRole | Binding |
|------|---------------|------------------|---------|
| 第 1 个 | `primary-sa` | Role `primary-sa-role` | RoleBinding `primary-sa-binding` |
| 第 2 个 | `secondary-sa` | ClusterRole `secondary-sa-role` | ClusterRoleBinding `secondary-sa-binding` |

- Pod 绑定的 ServiceAccount：`primary-sa`（第一个策略的 SA）
- 第一个策略创建命名空间级 Role
- 第二个策略创建集群级 ClusterRole

### 注意事项

1. **Verbs 必填**：每个 rule 必须指定至少一个 verb
2. **最小权限原则**：只授予必要的权限
3. **命名空间隔离**：非 `clusterScope` 的角色只在指定命名空间生效
4. **ServiceAccount 绑定**：第一个 RBAC 策略的 ServiceAccount 会被设置到 Pod
5. **automountServiceAccountToken**：设置为 `false` 可提高安全性
6. **多策略处理**：配置多个 RBAC 策略时，各自创建独立的资源集合

---

## 组合使用示例

### 完整的 Web 应用配置

```json
{
  "storage": [
    {
      "type": "ephemeral",
      "name": "tmp-data",
      "mountPath": "/tmp"
    },
    {
      "type": "config",
      "name": "nginx-config",
      "sourceName": "nginx-conf",
      "mountPath": "/etc/nginx/conf.d",
      "readOnly": true
    }
  ],
  "envs": [
    {
      "name": "APP_ENV",
      "valueFrom": {
        "static": "production"
      }
    },
    {
      "name": "DB_PASSWORD",
      "valueFrom": {
        "secret": {
          "name": "db-credentials",
          "key": "password"
        }
      }
    }
  ],
  "probes": [
    {
      "type": "liveness",
      "httpGet": {
        "path": "/healthz",
        "port": 8080
      },
      "initialDelaySeconds": 30,
      "periodSeconds": 10
    },
    {
      "type": "readiness",
      "httpGet": {
        "path": "/ready",
        "port": 8080
      },
      "initialDelaySeconds": 5,
      "periodSeconds": 5
    }
  ],
  "resources": {
    "cpu": "500m",
    "memory": "512Mi"
  },
  "ingress": [
    {
      "ingressClassName": "nginx",
      "hosts": ["app.example.com"],
      "tls": [
        {
          "secretName": "app-tls",
          "hosts": ["app.example.com"]
        }
      ],
      "routes": [
        {
          "path": "/",
          "backend": {
            "serviceName": "web-app",
            "servicePort": 80
          }
        }
      ]
    }
  ]
}
```

**生成结果**：

| 资源类型 | 名称/配置 |
|---------|----------|
| Volume 1 | EmptyDir `tmp-data` → `/tmp` |
| Volume 2 | ConfigMap `nginx-conf` → `/etc/nginx/conf.d` (只读) |
| 环境变量 | `APP_ENV=production`, `DB_PASSWORD` 从 Secret 读取 |
| LivenessProbe | HTTP `/healthz:8080`, 延迟 30s, 间隔 10s |
| ReadinessProbe | HTTP `/ready:8080`, 延迟 5s, 间隔 5s |
| 资源限制 | CPU `500m`, 内存 `512Mi` |
| Ingress | `https://app.example.com/` → `web-app:80`, TLS 证书 `app-tls` |

### 带初始化和边车的数据库配置

```json
{
  "init": [
    {
      "name": "init-permissions",
      "properties": {
        "image": "busybox:latest",
        "command": ["sh", "-c", "chown -R 999:999 /var/lib/mysql"]
      },
      "traits": {
        "storage": [
          {
            "type": "persistent",
            "name": "mysql-data",
            "mountPath": "/var/lib/mysql",
            "tmpCreate": true,
            "size": "20Gi"
          }
        ]
      }
    }
  ],
  "storage": [
    {
      "type": "persistent",
      "name": "mysql-data",
      "mountPath": "/var/lib/mysql",
      "tmpCreate": true,
      "size": "20Gi"
    }
  ],
  "sidecar": [
    {
      "name": "mysql-exporter",
      "image": "prom/mysqld-exporter:v0.14.0",
      "env": {
        "DATA_SOURCE_NAME": "exporter:password@(localhost:3306)/"
      },
      "traits": {
        "resources": {
          "cpu": "100m",
          "memory": "128Mi"
        }
      }
    }
  ],
  "envFrom": [
    {
      "type": "secret",
      "sourceName": "mysql-credentials"
    }
  ],
  "probes": [
    {
      "type": "liveness",
      "exec": {
        "command": ["mysqladmin", "ping", "-h", "localhost"]
      },
      "initialDelaySeconds": 60,
      "periodSeconds": 10
    },
    {
      "type": "readiness",
      "tcpSocket": {
        "port": 3306
      },
      "initialDelaySeconds": 30,
      "periodSeconds": 5
    }
  ],
  "resources": {
    "cpu": "2",
    "memory": "4Gi"
  },
  "rbac": [
    {
      "serviceAccount": "mysql-sa",
      "rules": [
        {
          "apiGroups": [""],
          "resources": ["configmaps"],
          "verbs": ["get"]
        }
      ]
    }
  ]
}
```

**生成结果**：

| 资源类型 | 名称/配置 | 说明 |
|---------|----------|------|
| PVC | `pvc-mysql-data-<appID>` | 20Gi 持久化存储 |
| InitContainer | `init-permissions` | 修改数据目录权限 |
| 主容器 | - | MySQL 数据库 |
| Sidecar | `mysql-exporter` | Prometheus 指标导出 |
| ServiceAccount | `mysql-sa` | 用于读取 ConfigMap |
| Role | `mysql-sa-role` | ConfigMap 读取权限 |
| RoleBinding | `mysql-sa-binding` | SA 与 Role 绑定 |

**启动流程**：

1. 创建 PVC `pvc-mysql-data-<appID>`（20Gi）
2. 运行 InitContainer `init-permissions`：修改 `/var/lib/mysql` 目录权限为 999:999
3. 启动主容器和边车容器
4. 主容器从 Secret `mysql-credentials` 加载数据库凭证
5. LivenessProbe 通过 `mysqladmin ping` 检查存活
6. ReadinessProbe 通过 TCP 3306 端口检查就绪
7. 边车 `mysql-exporter` 提供 Prometheus 监控指标

---

## 附录

### Traits 处理顺序

系统按以下顺序处理 Traits：

1. Storage（存储）
2. Envs（环境变量）
3. EnvFrom（批量环境变量）
4. Probes（健康探针）
5. Resources（资源限制）
6. Init（初始化容器）
7. Sidecar（边车容器）
8. RBAC（权限控制）
9. Ingress（入口流量）

### 嵌套 Traits 支持矩阵

| 父 Trait | 可嵌套的 Traits | 排除的 Traits |
|---------|----------------|--------------|
| Init | storage, envs, envFrom, resources | init, sidecar |
| Sidecar | storage, envs, envFrom, probes, resources | init, sidecar |

### 版本兼容性

本文档基于 KubeMin-Cli 最新版本编写，Traits 定义位于 `pkg/apiserver/domain/spec/traits.go`。

