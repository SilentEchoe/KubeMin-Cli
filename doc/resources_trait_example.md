# Resources Trait 使用说明

## 概述

Resources Trait 是一个简化的资源控制特征，用于为容器设置 CPU、Memory 和 GPU 资源限制。所有资源值默认作为 `limits` 设置。

## 语法

```json
{
  "traits": {
    "resources": {
      "cpu": "500m",
      "memory": "512Mi",
      "gpu": "1"
    }
  }
}
```

## 支持的资源类型

- **CPU**: 支持 Kubernetes 标准格式，如 `"100m"` (0.1 core)、`"1"` (1 core)、`"2.5"` (2.5 cores)
- **Memory**: 支持标准格式，如 `"64Mi"`、`"1Gi"`、`"2Gi"`
- **GPU**: 支持数量，如 `"1"`、`"2"` (需要集群支持 NVIDIA GPU 资源)

## 使用场景

### 1. 主容器资源设置

```json
{
  "name": "my-app",
  "image": "nginx:latest",
  "traits": {
    "resources": {
      "cpu": "500m",
      "memory": "512Mi"
    }
  }
}
```

### 2. Sidecar 容器资源设置

```json
{
  "name": "my-app",
  "image": "nginx:latest",
  "traits": {
    "sidecar": [
      {
        "name": "log-collector",
        "image": "fluentd:v1",
        "traits": {
          "resources": {
            "cpu": "200m",
            "memory": "256Mi"
          }
        }
      }
    ]
  }
}
```

### 3. Init 容器资源设置

```json
{
  "name": "my-app",
  "image": "nginx:latest",
  "traits": {
    "init": [
      {
        "name": "db-migrate",
        "properties": {
          "image": "migrate:v1"
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
```

### 4. 完整示例（包含多种特征）

```json
{
  "name": "web-app",
  "image": "web:v1",
  "traits": {
    "resources": {
      "cpu": "1",
      "memory": "1Gi"
    },
    "sidecar": [
      {
        "name": "cache",
        "image": "redis:alpine",
        "traits": {
          "resources": {
            "cpu": "500m",
            "memory": "512Mi"
          },
          "storage": [
            {
              "name": "cache-data",
              "type": "persistent",
              "mountPath": "/data",
              "size": "10Gi"
            }
          ]
        }
      }
    ]
  }
}
```

## 注意事项

1. **资源值格式**: 必须使用有效的 Kubernetes 资源数量格式
2. **默认行为**: 所有资源值都作为 `limits` 设置，不设置 `requests`
3. **GPU 支持**: 需要集群配置 NVIDIA GPU 设备插件
4. **单一配置**: 每个组件或容器只能有一个 resources trait 配置
5. **嵌套支持**: 可以在 sidecar 和 init 容器的嵌套 traits 中使用

## 错误处理

- 无效的 CPU 值（如 `"abc"`）会返回解析错误
- 无效的 Memory 值（如 `"100"`）会返回解析错误  
- 无效的 GPU 值会返回解析错误
- 所有错误都会在 trait 处理阶段被捕获并返回
