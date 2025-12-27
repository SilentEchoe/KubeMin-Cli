# 版本更新 API 设计文档

## 概述

版本更新 API 提供了一种简洁、优雅的方式来更新应用版本，支持组件的更新、新增和删除操作，并可通过工作流自动部署。

## API 端点

```
POST /api/v1/applications/:appID/version
```

## 功能特性

### 更新策略

| 策略 | 值 | 说明 |
|------|------|------|
| 滚动更新 | `rolling` | 默认策略，逐步替换 Pod，保证服务可用性 |
| 重建更新 | `recreate` | 先删除所有旧 Pod，再创建新 Pod |
| 金丝雀更新 | `canary` | 先更新部分 Pod，验证后再全量更新 |
| 蓝绿部署 | `blue-green` | 创建新版本，切换流量后销毁旧版本 |

### 组件操作

| 操作 | 值 | 说明 |
|------|------|------|
| 更新 | `update` | 默认操作，更新现有组件的配置 |
| 新增 | `add` | 向应用添加新组件 |
| 删除 | `remove` | 从应用移除组件 |

## 请求参数

### UpdateVersionRequest

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `version` | string | **是** | 新版本号 |
| `strategy` | string | 否 | 更新策略，默认 `rolling` |
| `components` | array | 否 | 组件更新规格列表 |
| `auto_exec` | bool | 否 | 是否自动执行工作流，默认 `true` |
| `description` | string | 否 | 更新说明 |

### ComponentUpdateSpec

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `action` | string | 否 | 操作类型：`update`/`add`/`remove`，默认 `update` |
| `name` | string | 是 | 组件名称 |
| `image` | string | 否 | 新镜像地址 |
| `replicas` | int32 | 否 | 新副本数 |
| `env` | object | 否 | 环境变量覆盖（合并更新） |
| `type` | string | 否 | 组件类型（新增时必填）：`webservice`/`store`/`config`/`secret` |
| `properties` | object | 否 | 组件属性（新增时可选） |
| `traits` | object | 否 | 组件特性（新增时可选） |

## 响应参数

### UpdateVersionResponse

| 字段 | 类型 | 说明 |
|------|------|------|
| `app_id` | string | 应用 ID |
| `version` | string | 新版本号 |
| `previous_version` | string | 更新前版本号 |
| `strategy` | string | 使用的更新策略 |
| `task_id` | string | 工作流任务 ID（如果触发了工作流执行） |
| `updated_components` | array | 已更新的组件名称列表 |
| `added_components` | array | 新增的组件名称列表 |
| `removed_components` | array | 已删除的组件名称列表 |

## 错误码

| HTTP 状态码 | 业务码 | 说明 |
|------------|-------|------|
| 404 | 10005 | 应用不存在 |
| 500 | 10011 | 版本更新失败 |
| 400 | 10012 | 无效的更新策略 |
| 404 | 10013 | 组件不存在 |
| 400 | 10014 | 没有可更新的组件 |
| 400 | 10015 | 组件已存在（新增时） |
| 400 | 10016 | 无效的组件操作类型 |

---

## JSON 请求示例

### 1. 简单镜像更新

更新单个组件的镜像版本：

```json
{
  "version": "1.1.0",
  "strategy": "rolling",
  "components": [
    {
      "name": "backend",
      "image": "myapp/backend:v1.1.0"
    }
  ]
}
```

**响应示例：**

```json
{
  "app_id": "abc123xyz",
  "version": "1.1.0",
  "previous_version": "1.0.0",
  "strategy": "rolling",
  "task_id": "task-def456",
  "updated_components": ["backend"],
  "added_components": [],
  "removed_components": []
}
```

### 2. 扩容副本数

仅扩容某个组件的副本数：

```json
{
  "version": "1.0.1",
  "components": [
    {
      "name": "backend",
      "replicas": 5
    }
  ]
}
```

### 3. 更新多个组件

同时更新多个组件的镜像和配置：

```json
{
  "version": "2.0.0",
  "strategy": "rolling",
  "components": [
    {
      "name": "backend",
      "image": "myapp/backend:v2.0.0",
      "replicas": 3
    },
    {
      "name": "frontend",
      "image": "myapp/frontend:v2.0.0"
    }
  ],
  "description": "Major version upgrade"
}
```

### 4. 新增组件

向应用添加新的组件（如 Redis 缓存）：

```json
{
  "version": "2.0.0",
  "components": [
    {
      "action": "add",
      "name": "redis-cache",
      "type": "store",
      "image": "redis:7-alpine",
      "replicas": 1,
      "properties": {
        "ports": [{"port": 6379}]
      }
    }
  ]
}
```

### 5. 删除组件

从应用中移除不需要的组件：

```json
{
  "version": "2.0.0",
  "components": [
    {
      "action": "remove",
      "name": "legacy-service"
    }
  ]
}
```

### 6. 混合操作（更新 + 新增 + 删除）

在一次请求中同时执行多种操作：

```json
{
  "version": "3.0.0",
  "strategy": "rolling",
  "components": [
    {
      "action": "update",
      "name": "backend",
      "image": "myapp/backend:v3.0.0"
    },
    {
      "action": "add",
      "name": "message-queue",
      "type": "store",
      "image": "rabbitmq:3-management",
      "replicas": 1
    },
    {
      "action": "remove",
      "name": "deprecated-worker"
    }
  ],
  "auto_exec": true,
  "description": "Architecture refactoring"
}
```

**响应示例：**

```json
{
  "app_id": "abc123xyz",
  "version": "3.0.0",
  "previous_version": "2.0.0",
  "strategy": "rolling",
  "task_id": "task-789xyz",
  "updated_components": ["backend"],
  "added_components": ["message-queue"],
  "removed_components": ["deprecated-worker"]
}
```

### 7. 金丝雀发布

使用金丝雀策略更新部分副本：

```json
{
  "version": "2.1.0",
  "strategy": "canary",
  "components": [
    {
      "name": "frontend",
      "image": "myapp/frontend:v2.1.0",
      "replicas": 1
    }
  ],
  "auto_exec": true
}
```

### 8. 仅更新版本号（不触发部署）

仅更新应用版本号和描述，不触发工作流：

```json
{
  "version": "2.0.0",
  "auto_exec": false,
  "description": "Major version bump - documentation only"
}
```

### 9. 更新环境变量

更新组件的环境变量配置：

```json
{
  "version": "1.2.0",
  "components": [
    {
      "name": "backend",
      "env": {
        "LOG_LEVEL": "debug",
        "FEATURE_FLAG": "enabled",
        "DB_POOL_SIZE": "20"
      }
    }
  ]
}
```

---

## cURL 命令示例

### 基本镜像更新

```bash
curl -X POST "http://localhost:8080/api/v1/applications/app-123/version" \
  -H "Content-Type: application/json" \
  -d '{
    "version": "1.1.0",
    "strategy": "rolling",
    "components": [
      {"name": "backend", "image": "myapp/backend:v1.1.0"}
    ]
  }'
```

### 新增组件

```bash
curl -X POST "http://localhost:8080/api/v1/applications/app-123/version" \
  -H "Content-Type: application/json" \
  -d '{
    "version": "2.0.0",
    "components": [
      {
        "action": "add",
        "name": "redis",
        "type": "store",
        "image": "redis:7-alpine",
        "replicas": 1
      }
    ]
  }'
```

### 查看工作流执行状态

更新后可以使用返回的 `task_id` 查询执行状态：

```bash
curl "http://localhost:8080/api/v1/workflow/tasks/task-456/status"
```

---

## 最佳实践

1. **生产环境使用滚动更新**：默认的 `rolling` 策略可以保证服务可用性
2. **测试环境可用重建更新**：`recreate` 策略更新速度快，适合测试环境
3. **大版本更新使用金丝雀/蓝绿**：降低风险，便于回滚
4. **设置合理的副本数**：确保更新期间有足够的副本处理流量
5. **添加更新说明**：使用 `description` 字段记录更新原因，便于审计

## 注意事项

- 组件名称会自动转换为小写
- 删除组件不会删除 Kubernetes 中已部署的资源，需要单独清理
- `auto_exec: false` 时仅更新数据库记录，不触发实际部署
- 新增组件时必须指定 `type` 字段

