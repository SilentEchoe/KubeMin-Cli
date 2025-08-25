# ConfigMap Job 使用说明

## 概述

ConfigMap Job 支持两种创建 ConfigMap 的方式：

1. **从 Map 创建**：用户直接传入 `map[string]string` 数据
2. **从 URL 文件创建**：用户提供文件链接，系统自动下载并创建 ConfigMap

## 特性

- 支持两种创建方式
- 自动文件大小检查（限制在 1MB 以内）
- 自动文件名提取
- 完整的验证机制
- 向后兼容现有实现

## 使用方法

### 1. 从 Map 创建 ConfigMap

```go
import "KubeMin-Cli/pkg/apiserver/domain/model"

// 创建配置
configMapInfo := &model.ConfigMapJobInfo{
    Type: "map",
    FromMap: &model.ConfigMapFromMap{
        Name:      "app-config",
        Namespace: "default",
        Labels: map[string]string{
            "app": "myapp",
        },
        Data: map[string]string{
            "config.yaml": "apiVersion: v1\nkind: Config",
            "env.txt":     "DEBUG=true",
        },
    },
}

// 验证配置
if err := configMapInfo.Validate(); err != nil {
    log.Fatalf("配置验证失败: %v", err)
}

// 创建 ConfigMap 数据
configMapData, err := configMapInfo.CreateConfigMap()
if err != nil {
    log.Fatalf("创建 ConfigMap 失败: %v", err)
}
```

### 2. 从 URL 文件创建 ConfigMap

```go
// 创建配置
configMapInfo := &model.ConfigMapJobInfo{
    Type: "url",
    FromURL: &model.ConfigMapFromURL{
        Name:      "external-config",
        Namespace: "default",
        URL:       "https://example.com/configs/app-config.yaml",
        FileName:  "app-config.yaml", // 可选，不提供则自动提取
    },
}

// 验证配置
if err := configMapInfo.Validate(); err != nil {
    log.Fatalf("配置验证失败: %v", err)
}

// 创建 ConfigMap 数据（会自动下载文件）
configMapData, err := configMapInfo.CreateConfigMap()
if err != nil {
    log.Fatalf("创建 ConfigMap 失败: %v", err)
}
```

## Job 配置

### 在 Workflow 中使用

```yaml
apiVersion: v1
kind: Workflow
metadata:
  name: deploy-app-with-config
spec:
  steps:
    - name: create-configmap
      jobType: deploy_configmap
      properties:
        configMapInfo:
          type: "map"
          fromMap:
            name: "app-config"
            namespace: "default"
            data:
              config.yaml: |
                apiVersion: v1
                kind: Config
                database:
                  host: mysql-service
                  port: 3306
```

### 从 URL 创建

```yaml
apiVersion: v1
kind: Workflow
metadata:
  name: deploy-app-with-external-config
spec:
  steps:
    - name: create-external-configmap
      jobType: deploy_configmap
      properties:
        configMapInfo:
          type: "url"
          fromURL:
            name: "external-config"
            namespace: "default"
            url: "https://example.com/configs/app-config.yaml"
            fileName: "app-config.yaml"  # 可选
```

## 验证规则

### Map 方式验证
- `name` 必须提供
- `data` 不能为空
- 所有 key 不能为空
- 总数据大小不能超过 1MB

### URL 方式验证
- `name` 必须提供
- `url` 必须是有效的 URL
- 下载的文件大小不能超过 1MB

## 错误处理

### 常见错误

1. **文件大小超限**
   ```
   file size 1048577 bytes exceeds ConfigMap maximum size 1048576 bytes
   ```

2. **无效 URL**
   ```
   invalid ConfigMap configuration: invalid URL format
   ```

3. **网络错误**
   ```
   failed to create ConfigMap data: failed to read file from URL: HTTP request failed with status: 404
   ```

### 错误处理建议

```go
configMapData, err := configMapInfo.CreateConfigMap()
if err != nil {
    if strings.Contains(err.Error(), "exceeds ConfigMap maximum size") {
        // 处理文件过大错误
        log.Printf("文件过大，请压缩或分割文件")
    } else if strings.Contains(err.Error(), "HTTP request failed") {
        // 处理网络错误
        log.Printf("网络请求失败，请检查URL和网络连接")
    } else {
        // 处理其他错误
        log.Printf("创建 ConfigMap 失败: %v", err)
    }
    return err
}
```

## 性能考虑

### 文件下载
- 使用 `io.LimitReader` 限制读取大小
- 支持超时控制
- 自动关闭 HTTP 连接

### 内存使用
- 限制单个文件最大 1MB
- 避免大文件导致内存溢出
- 支持流式处理

## 安全考虑

### URL 验证
- 支持 HTTP 和 HTTPS
- 建议使用 HTTPS 链接
- 避免访问不可信的 URL

### 文件内容
- 验证文件类型和大小
- 避免恶意文件内容
- 支持内容类型检查

## 测试

运行测试用例：

```bash
cd pkg/apiserver/domain/model
go test -v -run TestConfigMap
```

## 示例代码

完整的使用示例请参考 `examples/configmap_examples.yaml` 文件。

## 注意事项

1. **文件大小限制**：ConfigMap 最大支持 1MB 数据
2. **网络依赖**：URL 方式需要网络连接
3. **错误处理**：建议实现完善的错误处理机制
4. **向后兼容**：新实现兼容现有的 `corev1.ConfigMap` 类型
5. **命名空间**：如果不指定命名空间，将使用 Job 的默认命名空间
