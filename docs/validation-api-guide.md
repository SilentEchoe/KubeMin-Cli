# 验证 API (Try/DryRun) 使用指南

本文档介绍 KubeMin-Cli 的验证 API，用于在不实际创建资源的情况下验证应用配置和工作流配置的合法性。

## 概述

验证 API 提供了两个端点，允许用户在提交创建/更新请求前预先验证配置的正确性：

1. **Try Application API** - 验证应用创建请求
2. **Try Workflow API** - 验证工作流更新请求

## API 端点

### 1. Try Application API

**端点**: `POST /api/v1/applications/try`

**用途**: 验证应用创建请求是否符合规范（命名规则、Traits 规则、组件配置、工作流引用）

**请求体**: 与创建应用的请求体相同 (`CreateApplicationsRequest`)

**示例调用**:
```bash
curl -X POST http://localhost:8080/api/v1/applications/try \
  -H "Content-Type: application/json" \
  -d '{
    "name": "my-app",
    "namespace": "default",
    "version": "1.0.0",
    "component": [...],
    "workflow": [...]
  }'
```

### 2. Try Workflow API

**端点**: `POST /api/v1/applications/:appID/workflow/try`

**用途**: 验证工作流配置是否引用了存在的组件

**请求体**: `TryWorkflowRequest`

**示例调用**:
```bash
curl -X POST http://localhost:8080/api/v1/applications/your-app-id/workflow/try \
  -H "Content-Type: application/json" \
  -d '{
    "name": "new-workflow",
    "workflow": [...]
  }'
```

## 响应格式

### 验证通过
```json
{
  "valid": true,
  "errors": []
}
```

### 验证失败
```json
{
  "valid": false,
  "errors": [
    {
      "field": "component[0].name",
      "code": "INVALID_NAME_FORMAT",
      "message": "name must match DNS-1123 subdomain (lowercase alphanumeric, may contain hyphens, must start and end with alphanumeric)"
    },
    {
      "field": "component[1].traits.probes[0]",
      "code": "INVALID_PROBE_CONFIG",
      "message": "probe must specify exactly one of exec, httpGet, or tcpSocket"
    },
    {
      "field": "workflow[0].components[2]",
      "code": "COMPONENT_NOT_FOUND",
      "message": "component 'missing-comp' not found in application"
    }
  ]
}
```

## 请求示例

### 示例 1: 简单有效应用

```json
{
  "name": "simple-backend",
  "namespace": "default",
  "version": "1.0.0",
  "project": "demo-project",
  "description": "Simple backend application",
  "component": [
    {
      "name": "backend",
      "type": "webservice",
      "image": "nginx:1.24",
      "nameSpace": "default",
      "replicas": 2,
      "properties": {
        "ports": [
          {
            "port": 8080,
            "expose": true
          }
        ],
        "env": {
          "APP_ENV": "production"
        }
      },
      "traits": {}
    }
  ],
  "workflow": [
    {
      "name": "deploy-backend",
      "mode": "StepByStep",
      "components": ["backend"]
    }
  ]
}
```

### 示例 2: 完整应用配置（包含所有 Traits）

```json
{
  "name": "demo-app",
  "namespace": "default",
  "version": "1.0.0",
  "project": "demo-project",
  "description": "Complete demo application with all traits",
  "component": [
    {
      "name": "app-config",
      "type": "config",
      "nameSpace": "default",
      "replicas": 1,
      "properties": {
        "conf": {
          "database.host": "mysql.default.svc",
          "database.port": "3306"
        }
      }
    },
    {
      "name": "backend",
      "type": "webservice",
      "image": "myregistry/backend:v1.0.0",
      "nameSpace": "default",
      "replicas": 3,
      "properties": {
        "ports": [{"port": 8080, "expose": true}],
        "env": {"APP_ENV": "production"}
      },
      "traits": {
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
        "storage": [
          {
            "type": "persistent",
            "name": "data",
            "mountPath": "/data",
            "tmpCreate": true,
            "size": "10Gi"
          }
        ],
        "envFrom": [
          {
            "type": "configMap",
            "sourceName": "app-config"
          }
        ],
        "rbac": [
          {
            "serviceAccount": "backend-sa",
            "rules": [
              {
                "apiGroups": [""],
                "resources": ["pods"],
                "verbs": ["get", "list", "watch"]
              }
            ]
          }
        ],
        "ingress": [
          {
            "name": "backend-ingress",
            "ingressClassName": "nginx",
            "routes": [
              {
                "path": "/api",
                "backend": {
                  "serviceName": "backend",
                  "servicePort": 8080
                }
              }
            ]
          }
        ]
      }
    }
  ],
  "workflow": [
    {
      "name": "config-step",
      "mode": "StepByStep",
      "components": ["app-config"]
    },
    {
      "name": "deploy-backend",
      "mode": "DAG",
      "components": ["backend"]
    }
  ]
}
```

### 示例 3: 包含 Init 容器和 Sidecar 的应用

```json
{
  "name": "app-with-init-sidecar",
  "namespace": "default",
  "version": "1.0.0",
  "component": [
    {
      "name": "backend",
      "type": "webservice",
      "image": "myregistry/backend:v1.0.0",
      "nameSpace": "default",
      "replicas": 2,
      "traits": {
        "init": [
          {
            "name": "init-config",
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
                }
              ]
            }
          }
        ],
        "sidecar": [
          {
            "name": "logging-sidecar",
            "image": "fluent/fluentd:v1.14",
            "env": {
              "FLUENTD_CONF": "fluent.conf"
            },
            "traits": {
              "resources": {
                "cpu": "100m",
                "memory": "128Mi"
              }
            }
          }
        ]
      }
    }
  ],
  "workflow": [
    {
      "name": "deploy",
      "mode": "StepByStep",
      "components": ["backend"]
    }
  ]
}
```

### 示例 4: 工作流验证请求

```json
{
  "workflowId": "",
  "name": "new-workflow",
  "alias": "New Deployment Workflow",
  "workflow": [
    {
      "name": "config-step",
      "mode": "StepByStep",
      "components": ["app-config", "app-secret"]
    },
    {
      "name": "database-step",
      "mode": "DAG",
      "components": ["mysql", "redis"]
    },
    {
      "name": "services-step",
      "mode": "DAG",
      "components": ["backend", "frontend"]
    }
  ]
}
```

## 无效配置示例

### 示例 1: 无效的应用名称

```json
{
  "name": "My_Invalid_App",
  "namespace": "default",
  "component": [...]
}
```

**预期错误**:
```json
{
  "valid": false,
  "errors": [
    {
      "field": "name",
      "code": "INVALID_NAME_FORMAT",
      "message": "name must match DNS-1123 subdomain (lowercase alphanumeric, may contain hyphens, must start and end with alphanumeric)"
    }
  ]
}
```

### 示例 2: 缺少镜像

```json
{
  "name": "my-app",
  "component": [
    {
      "name": "backend",
      "type": "webservice",
      "image": ""
    }
  ]
}
```

**预期错误**:
```json
{
  "valid": false,
  "errors": [
    {
      "field": "component[0].image",
      "code": "MISSING_IMAGE",
      "message": "image is required for webservice and store component types"
    }
  ]
}
```

### 示例 3: 无效的探针配置

```json
{
  "name": "my-app",
  "component": [
    {
      "name": "backend",
      "type": "webservice",
      "image": "nginx:latest",
      "traits": {
        "probes": [
          {
            "type": "liveness"
          }
        ]
      }
    }
  ]
}
```

**预期错误**:
```json
{
  "valid": false,
  "errors": [
    {
      "field": "component[0].traits.probes[0]",
      "code": "INVALID_PROBE_CONFIG",
      "message": "probe must specify exactly one of exec, httpGet, or tcpSocket"
    }
  ]
}
```

### 示例 4: 嵌套 Sidecar（禁止）

```json
{
  "name": "my-app",
  "component": [
    {
      "name": "backend",
      "type": "webservice",
      "image": "nginx:latest",
      "traits": {
        "sidecar": [
          {
            "name": "sidecar-1",
            "image": "fluent/fluentd:v1.14",
            "traits": {
              "sidecar": [
                {
                  "name": "nested-sidecar",
                  "image": "busybox:latest"
                }
              ]
            }
          }
        ]
      }
    }
  ]
}
```

**预期错误**:
```json
{
  "valid": false,
  "errors": [
    {
      "field": "component[0].traits.sidecar[0].traits.sidecar[0]",
      "code": "NESTED_TRAIT_FORBIDDEN",
      "message": "sidecar trait cannot be nested inside another init or sidecar trait"
    }
  ]
}
```

### 示例 5: 工作流引用不存在的组件

```json
{
  "name": "my-app",
  "component": [
    {
      "name": "backend",
      "type": "webservice",
      "image": "nginx:latest"
    }
  ],
  "workflow": [
    {
      "name": "deploy-all",
      "mode": "StepByStep",
      "components": ["backend", "frontend", "database"]
    }
  ]
}
```

**预期错误**:
```json
{
  "valid": false,
  "errors": [
    {
      "field": "workflow[0].components[1]",
      "code": "COMPONENT_NOT_FOUND",
      "message": "component 'frontend' not found in application"
    },
    {
      "field": "workflow[0].components[2]",
      "code": "COMPONENT_NOT_FOUND",
      "message": "component 'database' not found in application"
    }
  ]
}
```

## 错误码参考

### 命名错误

| 错误码 | 说明 |
|--------|------|
| `INVALID_NAME` | 名称无效 |
| `NAME_TOO_SHORT` | 名称太短 (< 2 字符) |
| `NAME_TOO_LONG` | 名称太长 (> 63 字符) |
| `INVALID_NAME_FORMAT` | 名称格式不符合 DNS-1123 子域名规范 |
| `INVALID_COMPONENT_NAME` | 组件名称无效 |
| `INVALID_STEP_NAME` | 工作流步骤名称无效 |

### 组件错误

| 错误码 | 说明 |
|--------|------|
| `INVALID_COMPONENT_TYPE` | 无效的组件类型 |
| `MISSING_IMAGE` | 缺少镜像 (webservice/store 类型必填) |
| `DUPLICATE_COMPONENT` | 重复的组件名称 |

### Traits 错误

| 错误码 | 说明 |
|--------|------|
| `INVALID_TRAIT_CONFIG` | Trait 配置无效 |
| `MISSING_REQUIRED_FIELD` | 缺少必填字段 |
| `INVALID_STORAGE_TYPE` | 无效的存储类型 |
| `INVALID_STORAGE_SIZE` | 无效的存储大小格式 |
| `INVALID_PROBE_TYPE` | 无效的探针类型 |
| `INVALID_PROBE_CONFIG` | 探针配置无效 |
| `NESTED_TRAIT_FORBIDDEN` | 禁止嵌套 Trait |
| `MISSING_RBAC_RULES` | RBAC 缺少规则 |
| `MISSING_RBAC_VERBS` | RBAC 规则缺少 verbs |
| `MISSING_INGRESS_ROUTES` | Ingress 缺少路由 |
| `MISSING_SERVICE_NAME` | Ingress 路由缺少服务名称 |
| `INVALID_ENVFROM_TYPE` | envFrom 类型无效 |
| `INVALID_ENV_VALUE_SOURCE` | 环境变量值来源无效 |

### 工作流错误

| 错误码 | 说明 |
|--------|------|
| `COMPONENT_NOT_FOUND` | 组件不存在 |
| `INVALID_WORKFLOW_MODE` | 无效的工作流模式 |
| `EMPTY_WORKFLOW_STEP` | 空的工作流步骤 |
| `DUPLICATE_WORKFLOW_STEP` | 重复的工作流步骤名称 |
| `WORKFLOW_STEP_NO_COMPONENT` | 工作流步骤没有组件 |

## 验证规则详解

### 命名规则

所有名称（应用名、组件名、工作流步骤名）必须符合 **DNS-1123 子域名规范**：

- **长度**: 2-63 字符
- **格式**: `^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`
- 只能包含小写字母、数字和连字符
- 必须以字母或数字开头和结尾
- 不能以连字符开头或结尾

**有效示例**: `my-app`, `backend-v1`, `mysql01`

**无效示例**: `My_App`, `-app`, `app-`, `a` (太短)

### 组件类型

| 类型 | 说明 | 是否需要镜像 |
|------|------|--------------|
| `webservice` | 无状态服务 (Deployment) | 是 |
| `store` | 有状态存储服务 (StatefulSet) | 是 |
| `config` | ConfigMap 配置 | 否 |
| `secret` | Secret 密钥 | 否 |

### 工作流模式

| 模式 | 说明 |
|------|------|
| `StepByStep` | 串行模式，组件按顺序依次执行 |
| `DAG` | 并行模式，同一 Step 内的组件并行执行 |

### Traits 嵌套规则

- `init` 和 `sidecar` Trait 支持嵌套以下 Traits:
  - `storage`, `envs`, `envFrom`, `probes`, `resources`, `rbac`, `ingress`
- **禁止**在 `init` 或 `sidecar` 中再嵌套 `init` 或 `sidecar`

### 探针规则

- `type` 必填，可选值: `liveness`, `readiness`, `startup`
- 探测方法**三选一**: `exec`, `httpGet`, `tcpSocket`
- 不能同时指定多个探测方法
- `httpGet` 和 `tcpSocket` 的 `port` 必须为正整数

### 存储规则

- `type` 必填，可选值: `persistent`, `ephemeral`, `config`, `secret`
- `mountPath` 必填
- `persistent` 类型配合 `tmpCreate: true` 时，`size` 需要符合 Kubernetes 资源量格式 (如 `1Gi`, `500Mi`)
- `config` 和 `secret` 类型需要指定 `sourceName` 或 `name`

### RBAC 规则

- `rules` 数组必填且不能为空
- 每个 rule 的 `verbs` 数组必填且不能为空

### Ingress 规则

- `routes` 数组必填且不能为空
- 每个 route 的 `backend.serviceName` 必填

## 使用建议

1. **开发阶段**: 在提交创建应用请求前，先使用 Try API 验证配置
2. **CI/CD 集成**: 在部署流水线中添加验证步骤，提前发现配置错误
3. **调试**: 当创建应用失败时，使用 Try API 获取详细的验证错误信息
4. **批量验证**: 可以批量验证多个配置文件，确保配置规范一致性

## 相关文件

- 示例 JSON 文件: `examples/validation-try/`
- DTO 定义: `pkg/apiserver/interfaces/api/dto/v1/validation.go`
- 验证服务实现: `pkg/apiserver/domain/service/validation.go`
- 单元测试: `pkg/apiserver/domain/service/validation_test.go`

