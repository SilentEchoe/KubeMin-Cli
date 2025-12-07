# 版本更新功能测试指南

## 概述

本文档以实际操作流程的方式，演示如何使用版本更新 API。

---

## 场景一：简单镜像更新

### 步骤 1：创建应用

首先，创建一个版本为 `1.0.0` 的应用：

**请求**：`POST /api/v1/applications`

```json
{
  "name": "my-backend-app",
  "namespace": "default",
  "version": "1.0.0",
  "description": "My backend application",
  "component": [
    {
      "name": "backend",
      "type": "webservice",
      "image": "myapp/backend:v1.0.0",
      "replicas": 2,
      "properties": {
        "ports": [
          {"port": 8080}
        ],
        "env": {
          "ENV": "production",
          "LOG_LEVEL": "info"
        }
      }
    }
  ]
}
```

**响应**：

```json
{
  "id": "abc123xyz456",
  "name": "my-backend-app",
  "version": "1.0.0",
  "workflow_id": "wf-789def",
  "createTime": "2024-01-15T10:30:00Z",
  "updateTime": "2024-01-15T10:30:00Z"
}
```

### 步骤 2：更新镜像版本

现在，将 backend 组件的镜像从 `v1.0.0` 更新到 `v1.1.0`：

**请求**：`POST /api/v1/applications/abc123xyz456/version`

```json
{
  "version": "1.1.0",
  "strategy": "rolling",
  "components": [
    {
      "name": "backend",
      "image": "myapp/backend:v1.1.0"
    }
  ],
  "description": "Update backend image to v1.1.0"
}
```

**响应**：

```json
{
  "appId": "abc123xyz456",
  "version": "1.1.0",
  "previousVersion": "1.0.0",
  "strategy": "rolling",
  "taskId": "task-update-001",
  "updatedComponents": ["backend"],
  "addedComponents": [],
  "removedComponents": []
}
```

### 步骤 3：查看更新状态

使用返回的 `taskId` 查询工作流执行状态：

**请求**：`GET /api/v1/workflow/tasks/task-update-001/status`

**响应**：

```json
{
  "taskId": "task-update-001",
  "status": "completed",
  "workflowId": "wf-789def",
  "workflowName": "my-backend-app-workflow",
  "appId": "abc123xyz456",
  "components": [
    {
      "name": "backend",
      "type": "deploy",
      "status": "completed",
      "startTime": 1705312200,
      "endTime": 1705312260
    }
  ]
}
```

---

## 场景二：扩容副本数

### 当前状态

应用 `my-backend-app` 当前版本为 `1.1.0`，backend 组件有 2 个副本。

### 执行扩容

将副本数从 2 扩展到 5：

**请求**：`POST /api/v1/applications/abc123xyz456/version`

```json
{
  "version": "1.1.1",
  "components": [
    {
      "name": "backend",
      "replicas": 5
    }
  ],
  "description": "Scale backend to 5 replicas for high traffic"
}
```

**响应**：

```json
{
  "appId": "abc123xyz456",
  "version": "1.1.1",
  "previousVersion": "1.1.0",
  "strategy": "rolling",
  "taskId": "task-scale-002",
  "updatedComponents": ["backend"],
  "addedComponents": [],
  "removedComponents": []
}
```

---

## 场景三：添加缓存组件

### 当前状态

应用 `my-backend-app` 当前版本为 `1.1.1`，只有一个 backend 组件。

### 添加 Redis 缓存

**请求**：`POST /api/v1/applications/abc123xyz456/version`

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
        "ports": [
          {"port": 6379}
        ]
      },
      "traits": {
        "resources": {
          "cpu": "100m",
          "memory": "256Mi"
        }
      }
    }
  ],
  "description": "Add Redis cache component"
}
```

**响应**：

```json
{
  "appId": "abc123xyz456",
  "version": "2.0.0",
  "previousVersion": "1.1.1",
  "strategy": "rolling",
  "taskId": "task-add-003",
  "updatedComponents": [],
  "addedComponents": ["redis-cache"],
  "removedComponents": []
}
```

### 验证组件列表

**请求**：`GET /api/v1/applications/abc123xyz456/components`

**响应**：

```json
{
  "components": [
    {
      "id": 1,
      "appId": "abc123xyz456",
      "name": "backend",
      "namespace": "default",
      "image": "myapp/backend:v1.1.0",
      "replicas": 5,
      "type": "webservice"
    },
    {
      "id": 2,
      "appId": "abc123xyz456",
      "name": "redis-cache",
      "namespace": "default",
      "image": "redis:7-alpine",
      "replicas": 1,
      "type": "store"
    }
  ]
}
```

---

## 场景四：删除废弃组件

### 当前状态

假设应用有一个废弃的 `legacy-worker` 组件需要移除。

### 删除组件

**请求**：`POST /api/v1/applications/abc123xyz456/version`

```json
{
  "version": "2.1.0",
  "components": [
    {
      "action": "remove",
      "name": "legacy-worker"
    }
  ],
  "description": "Remove deprecated legacy-worker component"
}
```

**响应**：

```json
{
  "appId": "abc123xyz456",
  "version": "2.1.0",
  "previousVersion": "2.0.0",
  "strategy": "rolling",
  "taskId": "task-remove-004",
  "updatedComponents": [],
  "addedComponents": [],
  "removedComponents": ["legacy-worker"]
}
```

---

## 场景五：混合操作（更新 + 新增 + 删除）

### 当前状态

应用版本 `2.1.0`，包含 backend 和 redis-cache 组件。

### 执行架构重构

一次性完成：
- 更新 backend 镜像
- 新增 message-queue 组件
- 删除 old-scheduler 组件

**请求**：`POST /api/v1/applications/abc123xyz456/version`

```json
{
  "version": "3.0.0",
  "strategy": "rolling",
  "components": [
    {
      "action": "update",
      "name": "backend",
      "image": "myapp/backend:v3.0.0",
      "replicas": 3,
      "env": {
        "API_VERSION": "v3",
        "FEATURE_NEW_UI": "enabled"
      }
    },
    {
      "action": "add",
      "name": "message-queue",
      "type": "store",
      "image": "rabbitmq:3-management",
      "replicas": 1,
      "properties": {
        "ports": [
          {"port": 5672},
          {"port": 15672}
        ]
      }
    },
    {
      "action": "remove",
      "name": "old-scheduler"
    }
  ],
  "autoExec": true,
  "description": "Major architecture refactoring - v3.0.0"
}
```

**响应**：

```json
{
  "appId": "abc123xyz456",
  "version": "3.0.0",
  "previousVersion": "2.1.0",
  "strategy": "rolling",
  "taskId": "task-refactor-005",
  "updatedComponents": ["backend"],
  "addedComponents": ["message-queue"],
  "removedComponents": ["old-scheduler"]
}
```

---

## 场景六：仅更新版本号（不部署）

### 用例

需要记录一个版本号变更，但不触发实际部署（例如文档更新）。

**请求**：`POST /api/v1/applications/abc123xyz456/version`

```json
{
  "version": "3.0.1",
  "autoExec": false,
  "description": "Documentation update - no deployment needed"
}
```

**响应**：

```json
{
  "appId": "abc123xyz456",
  "version": "3.0.1",
  "previousVersion": "3.0.0",
  "strategy": "rolling",
  "taskId": "",
  "updatedComponents": [],
  "addedComponents": [],
  "removedComponents": []
}
```

> **注意**：`taskId` 为空，表示没有触发工作流执行。

---

## 场景七：金丝雀发布

### 用例

使用金丝雀策略，先部署少量副本测试新版本。

**请求**：`POST /api/v1/applications/abc123xyz456/version`

```json
{
  "version": "3.1.0-canary",
  "strategy": "canary",
  "components": [
    {
      "name": "backend",
      "image": "myapp/backend:v3.1.0",
      "replicas": 1
    }
  ],
  "description": "Canary release - testing v3.1.0 with 1 replica"
}
```

**响应**：

```json
{
  "appId": "abc123xyz456",
  "version": "3.1.0-canary",
  "previousVersion": "3.0.1",
  "strategy": "canary",
  "taskId": "task-canary-006",
  "updatedComponents": ["backend"],
  "addedComponents": [],
  "removedComponents": []
}
```

---

## 错误场景

### 错误 1：应用不存在

**请求**：`POST /api/v1/applications/non-existent-app/version`

```json
{
  "version": "1.0.0"
}
```

**响应**（HTTP 404）：

```json
{
  "HTTPCode": 404,
  "BusinessCode": 10005,
  "Message": "application name is not exist"
}
```

### 错误 2：缺少版本号

**请求**：`POST /api/v1/applications/abc123xyz456/version`

```json
{
  "components": [
    {"name": "backend", "image": "new-image"}
  ]
}
```

**响应**（HTTP 400）：

```json
{
  "HTTPCode": 400,
  "BusinessCode": 10000,
  "Message": "Key: 'UpdateVersionRequest.Version' Error:Field validation for 'Version' failed on the 'required' tag"
}
```

### 错误 3：组件不存在（更新时跳过）

**请求**：`POST /api/v1/applications/abc123xyz456/version`

```json
{
  "version": "1.2.0",
  "components": [
    {"name": "non-existent-component", "image": "some-image"}
  ]
}
```

**响应**（HTTP 200 - 成功但无更新）：

```json
{
  "appId": "abc123xyz456",
  "version": "1.2.0",
  "previousVersion": "1.1.0",
  "strategy": "rolling",
  "taskId": "",
  "updatedComponents": [],
  "addedComponents": [],
  "removedComponents": []
}
```

> **注意**：不存在的组件会被跳过，不会报错。日志中会记录警告。

---

## cURL 命令示例

### 创建应用

```bash
curl -X POST "http://localhost:8080/api/v1/applications" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "my-backend-app",
    "namespace": "default",
    "version": "1.0.0",
    "component": [
      {
        "name": "backend",
        "type": "webservice",
        "image": "myapp/backend:v1.0.0",
        "replicas": 2,
        "properties": {
          "ports": [{"port": 8080}]
        }
      }
    ]
  }'
```

### 更新版本

```bash
# 替换 APP_ID 为实际的应用 ID
APP_ID="abc123xyz456"

curl -X POST "http://localhost:8080/api/v1/applications/${APP_ID}/version" \
  -H "Content-Type: application/json" \
  -d '{
    "version": "1.1.0",
    "strategy": "rolling",
    "components": [
      {"name": "backend", "image": "myapp/backend:v1.1.0"}
    ]
  }'
```

### 查看任务状态

```bash
TASK_ID="task-update-001"

curl "http://localhost:8080/api/v1/workflow/tasks/${TASK_ID}/status"
```

### 查看应用组件

```bash
curl "http://localhost:8080/api/v1/applications/${APP_ID}/components"
```

---

## 单元测试文件

| 文件 | 说明 |
|------|------|
| `pkg/apiserver/domain/service/application_version_test.go` | Service 层测试 |
| `pkg/apiserver/interfaces/api/workflow_test.go` | API 层测试 |

### 运行测试

```bash
# 运行版本更新相关测试
go test ./pkg/apiserver/domain/service/... -v -run TestUpdateVersion -count=1
go test ./pkg/apiserver/interfaces/api/... -v -run TestUpdateVersion -count=1

# 运行完整测试套件
go test ./pkg/apiserver/domain/service/... ./pkg/apiserver/interfaces/api/... -v -count=1
```

---

## 测试检查清单

| 场景 | 测试用例 | 状态 |
|------|---------|------|
| 镜像更新 | `TestUpdateVersionWithImageUpdate` | ✅ |
| 副本数更新 | `TestUpdateVersionWithReplicasUpdate` | ✅ |
| 版本记录 | `TestUpdateVersionWithPreviousVersion` | ✅ |
| 新增组件 | `TestUpdateVersionAddComponent` | ✅ |
| 删除组件 | `TestUpdateVersionRemoveComponent` | ✅ |
| 混合操作 | `TestUpdateVersionMixedOperations` | ✅ |
| 应用不存在 | `TestUpdateVersionMissingApp` | ✅ |
| 跳过不存在组件 | `TestUpdateVersionSkipNonExistentComponent` | ✅ |
| 更新描述 | `TestUpdateVersionWithDescription` | ✅ |
| 默认策略 | `TestUpdateVersionDefaultStrategy` | ✅ |
| 无变更 | `TestUpdateVersionNoChanges` | ✅ |
| API 完整流程 | `TestUpdateVersionEndpoint` | ✅ |
| API 最简请求 | `TestUpdateVersionEndpointMinimalRequest` | ✅ |
