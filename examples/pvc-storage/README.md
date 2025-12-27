# PVC Storage 测试用例

本目录包含用于测试 StatefulSet PVC Volume 命名问题的测试用例。

## 背景

在 StatefulSet 中使用 `tmp_create: true` 创建持久化存储时，volumeClaimTemplate 的名称必须与 VolumeMount 的名称一致，否则会导致 Pod 创建失败。

详细的问题分析请参阅：[StatefulSet PVC Volume 命名问题深度分析](../../docs/statefulset-pvc-volume-naming.md)

## 测试用例列表

| 文件 | 场景 | 目的 |
|------|------|------|
| `01-basic-statefulset-pvc.json` | 基本 StatefulSet + tmp_create | 验证单个 PVC 的正确创建 |
| `02-multi-volume-statefulset.json` | 多 Volume StatefulSet | 验证多个 tmp_create volume 同时工作 |
| `03-mixed-pvc-mode.json` | 混合模式 | 验证 tmp_create + 引用已有 PVC |
| `04-multi-container-shared-volume.json` | 多容器共享 Volume | 验证主容器 + Sidecar + Init 共享 volume |
| `05-dependency-chain-with-pvc.json` | 依赖链 | 验证复杂工作流中的 PVC 处理 |
| `06-deployment-ephemeral-only.json` | Deployment + ephemeral | Deployment 推荐的存储方式 |

## 使用方法

### 1. 启动 API Server

```shell
go run ./cmd/main.go
```

### 2. 执行测试

```shell
# 基本测试
curl -X POST http://127.0.0.1:8080/api/v1/applications/validation/try \
  -H "Content-Type: application/json" \
  -d @examples/pvc-storage/01-basic-statefulset-pvc.json

# 获取任务 ID 后查询状态
TASK_ID="<返回的任务ID>"
curl http://127.0.0.1:8080/api/v1/workflow/tasks/${TASK_ID}/status
```

### 3. 验证 Kubernetes 资源

```shell
# 检查 StatefulSet volumeClaimTemplates
kubectl get sts -o jsonpath='{range .items[*]}{.metadata.name}: {.spec.volumeClaimTemplates[*].metadata.name}{"\n"}{end}'

# 检查 Pod volumeMounts
kubectl get pod <pod-name> -o jsonpath='{.spec.containers[*].volumeMounts[*].name}'

# 检查 PVC
kubectl get pvc
```

## 预期结果

### 正确的 StatefulSet YAML 结构

```yaml
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: mysql-primary-xxx
spec:
  volumeClaimTemplates:
    - metadata:
        name: mysql-data  # ✅ 与 VolumeMount 名称一致
      spec:
        accessModes: ["ReadWriteOnce"]
        resources:
          requests:
            storage: 5Gi
  template:
    spec:
      containers:
        - name: mysql-primary
          volumeMounts:
            - name: mysql-data  # ✅ 匹配 volumeClaimTemplate.name
              mount_path: /var/lib/mysql
```

### 常见错误

如果看到以下错误，说明 PVC 命名逻辑存在问题：

```
spec.containers[0].volumeMounts[0].name: Not found: "mysql-data"
```

## 注意事项

1. **tmp_create 仅适用于 StatefulSet**：Deployment 不支持 volumeClaimTemplates，使用 tmp_create 会导致警告
2. **Volume 名称去重**：多个容器引用同一个 volume 时，只会创建一个 volumeClaimTemplate
3. **sub_path 支持**：可以使用 sub_path 在同一个 PVC 上隔离不同容器的数据

## 相关文档

- [StatefulSet PVC Volume 命名问题深度分析](../../docs/statefulset-pvc-volume-naming.md)
- [Workflow 测试指南](../../docs/workflow-testing-guide.md)

