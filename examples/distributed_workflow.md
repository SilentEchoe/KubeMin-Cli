# KubeMin-Cli 分布式工作流使用示例

## 概述

KubeMin-Cli支持两种工作流模式：
1. **本地模式**（默认）：单节点执行，适合开发和小规模部署
2. **分布式模式**：多节点协作，适合生产环境和大规模部署

## 快速开始

### 1. 本地模式

默认情况下，KubeMin-Cli以本地模式运行：

```bash
./kubemin-cli
```

### 2. 分布式模式

#### 启动Redis

```bash
docker run -d --name redis -p 6379:6379 redis:7-alpine
```

#### 配置环境变量

```bash
export REDIS_ADDR=localhost:6379
```

#### 启动KubeMin-Cli

```bash
./kubemin-cli --redis-addr=localhost:6379 --max-workers=10
```

## 架构说明

### 核心组件

1. **SimpleDistributedWorkflow** - 简化的分布式工作流实现
   - 继承自原有的Workflow
   - 增加Redis队列支持
   - 实现分布式锁机制

2. **Leader选举** - 基于Kubernetes Lease资源
   - 只有Leader节点负责调度任务
   - 所有节点都可以执行任务

3. **Redis队列** - 任务分发
   - `workflow:queue` - 主任务队列
   - 分布式锁避免任务重复执行

## 代码集成

### 在现有项目中启用分布式

修改 `cmd/main.go`:

```go
package main

import (
    "KubeMin-Cli/pkg/apiserver"
    "KubeMin-Cli/pkg/apiserver/config"
    "os"
)

func main() {
    cfg := config.NewConfig()
    
    // 从环境变量或命令行参数设置Redis地址
    redisAddr := os.Getenv("REDIS_ADDR")
    if redisAddr != "" {
        cfg.RedisAddr = redisAddr
        cfg.EnableDistributed = true
    }
    
    server := apiserver.New(*cfg)
    server.Run(ctx, errChan)
}
```

### 自定义任务处理

```go
// 在workflow包中扩展任务处理逻辑
func (sdw *SimpleDistributedWorkflow) executeTask(ctx context.Context, task *model.WorkflowQueue, workerID int) {
    klog.Infof("Worker %d executing task %s", workerID, task.TaskID)
    
    // 使用分布式锁
    lockKey := fmt.Sprintf("lock:%s", task.TaskID)
    if !sdw.acquireLock(ctx, lockKey, 30*time.Second) {
        return
    }
    defer sdw.releaseLock(ctx, lockKey)
    
    // 执行实际的工作流逻辑
    controller := NewWorkflowController(task, sdw.KubeClient, sdw.Store)
    controller.Run(ctx, 1)
}
```

## 部署示例

### Docker Compose

```yaml
version: '3.8'
services:
  redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"
  
  kubemin-node1:
    build: .
    environment:
      - REDIS_ADDR=redis:6379
      - NODE_ID=node1
    command: ["./kubemin-cli", "--redis-addr=redis:6379"]
  
  kubemin-node2:
    build: .
    environment:
      - REDIS_ADDR=redis:6379
      - NODE_ID=node2
    command: ["./kubemin-cli", "--redis-addr=redis:6379"]
```

### Kubernetes

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: kubemin-cli
  namespace: kubemin-system
spec:
  replicas: 3
  selector:
    matchLabels:
      app: kubemin-cli
  template:
    metadata:
      labels:
        app: kubemin-cli
    spec:
      containers:
      - name: kubemin-cli
        image: kubemin-cli:latest
        env:
        - name: REDIS_ADDR
          value: "redis-service:6379"
        - name: MAX_WORKERS
          value: "10"
```

## 监控和调试

### 查看Redis队列状态

```bash
redis-cli
> LLEN workflow:queue
> KEYS lock:*
```

### 查看日志

```bash
# 查看特定节点的日志
kubectl logs -n kubemin-system deployment/kubemin-cli -f

# 筛选分布式相关日志
kubectl logs -n kubemin-system deployment/kubemin-cli | grep -E "distributed|leader|worker"
```

### 性能调优

1. **调整Worker数量**
   ```bash
   --max-workers=20  # 根据CPU和任务类型调整
   ```

2. **Redis连接池**
   ```go
   // 在distributed_simple.go中调整
   PoolSize: 20,
   MinIdleConns: 10,
   ```

3. **任务超时设置**
   ```go
   lockTTL := 30 * time.Second // 根据任务执行时间调整
   ```

## 故障处理

### Redis连接失败

系统会自动降级到本地模式：
```
Failed to init Redis, falling back to local mode
```

### Leader选举失败

检查Kubernetes权限：
```bash
kubectl get lease -n min-cli-system
```

### 任务执行失败

查看死信队列或失败任务：
```bash
redis-cli
> LRANGE workflow:failed 0 -1
```

## 扩展性

### 添加Kafka支持

实现MessageQueue接口即可支持其他消息队列：

```go
type KafkaQueue struct {
    // Kafka client
}

func (k *KafkaQueue) Publish(ctx context.Context, topic string, message []byte) error {
    // Kafka publish logic
}

func (k *KafkaQueue) Subscribe(ctx context.Context, topic string, handler func([]byte)) error {
    // Kafka subscribe logic
}
```

### 自定义调度策略

扩展SimpleDistributedWorkflow的调度逻辑：

```go
func (sdw *SimpleDistributedWorkflow) customScheduler(ctx context.Context) {
    // 自定义调度逻辑
    // 例如：基于优先级、资源使用率等
}
```

## 最佳实践

1. **幂等性设计** - 确保任务可重复执行
2. **合理设置超时** - 避免任务长时间占用资源
3. **监控队列长度** - 及时发现性能瓶颈
4. **定期清理** - 清理过期的锁和失败任务
5. **优雅关闭** - 确保任务完成后再关闭节点

## 总结

分布式工作流系统通过以下特性提升了KubeMin-Cli的可扩展性：

- ✅ 与现有系统完全兼容
- ✅ 支持动态伸缩
- ✅ 故障自动恢复
- ✅ 简单易用的API
- ✅ 可扩展的架构设计