# KubeMin-Cli 分布式工作流部署指南

## 概述

KubeMin-Cli 分布式工作流系统基于Redis实现任务队列和分布式锁，结合Kubernetes Leader选举机制，实现高可用的工作流调度系统。

## 架构特点

1. **Leader选举**: 使用Kubernetes Lease资源进行Leader选举，确保只有一个节点负责任务调度
2. **分布式执行**: 所有节点都可以作为Worker执行任务
3. **Redis队列**: 使用Redis List和Sorted Set实现优先级队列
4. **故障恢复**: Leader失效时自动选举新Leader，任务自动重新调度
5. **水平扩展**: 支持动态增加Worker节点

## 部署模式

### 1. 本地模式（默认）

不配置Redis地址时，系统运行在本地模式：

```bash
./kubemin-cli
```

### 2. 分布式模式

配置Redis地址启用分布式模式：

```bash
./kubemin-cli --redis-addr=localhost:6379 --enable-distributed=true
```

### 3. Docker Compose部署

使用docker-compose快速部署完整的分布式环境：

```bash
cd deploy/distributed
docker-compose up -d
```

这将启动：
- Redis（消息队列）
- MySQL（数据存储）
- Jaeger（分布式追踪）
- 3个KubeMin-Cli节点（2个Leader候选+1个Worker）

### 4. Kubernetes部署

```bash
# 创建命名空间
kubectl create namespace kubemin-system

# 部署Redis
kubectl apply -f k8s/redis.yaml

# 部署KubeMin-Cli
kubectl apply -f k8s/deployment.yaml
```

## 配置说明

### 环境变量

- `REDIS_ADDR`: Redis服务地址
- `MYSQL_ADDR`: MySQL数据库地址
- `JAEGER_ENDPOINT`: Jaeger追踪端点
- `NODE_ID`: 节点唯一标识

### 命令行参数

```bash
--redis-addr          Redis服务地址 (如: localhost:6379)
--enable-distributed  启用分布式模式 (默认: false)
--max-workers        最大Worker数量 (默认: 10)
--id                 节点ID，用于Leader选举
```

### 配置文件

创建 `config.yaml`:

```yaml
# Redis配置
redisAddr: "localhost:6379"
enableDistributed: true
maxWorkers: 10

# 数据库配置
datastore:
  type: "mysql"
  url: "root:123456@tcp(localhost:3306)/kubemin_cli"

# Leader选举
leaderConfig:
  lockName: "apiserver-lock"
  duration: 5s
```

## 监控和管理

### 查看队列状态

```bash
# 连接Redis查看队列
redis-cli
> LLEN workflow:queue:normal
> LLEN workflow:queue:priority
```

### 查看Leader状态

```bash
# 查看Kubernetes Lease
kubectl get lease -n min-cli-system apiserver-lock -o yaml
```

### 查看日志

```bash
# Docker环境
docker logs kubemin-node1

# Kubernetes环境
kubectl logs -n kubemin-system deployment/kubemin-cli
```

### 访问Jaeger UI

浏览器访问: http://localhost:16686

## 性能调优

### Redis优化

1. 配置持久化：
```redis
appendonly yes
appendfsync everysec
```

2. 调整连接池：
```yaml
redisPoolSize: 20
redisMinIdleConns: 10
```

### Worker调优

根据任务负载调整Worker数量：
- CPU密集型任务: Workers = CPU核心数
- IO密集型任务: Workers = CPU核心数 * 2-4

### 队列优化

1. 使用优先级队列分离紧急任务
2. 设置合理的任务超时时间
3. 配置死信队列处理失败任务

## 故障处理

### Leader失效

当Leader节点失效时：
1. 其他候选节点自动参与选举
2. 新Leader接管任务调度
3. 未完成的任务自动重新调度

### Redis故障

如果Redis不可用：
1. 系统自动降级到本地模式（如果配置了fallback）
2. 或等待Redis恢复
3. 恢复后自动重连

### Worker故障

Worker节点故障时：
1. 超时任务自动重新入队
2. 其他Worker接管任务执行
3. 支持动态增减Worker

## 扩展能力

### 支持其他消息队列

系统设计了消息队列抽象接口，可以扩展支持：
- Kafka
- RabbitMQ
- NATS
- Pulsar

实现 `MessageQueue` 接口即可：
```go
type MessageQueue interface {
    Publish(ctx context.Context, queue string, message []byte, opts ...PublishOption) error
    Subscribe(ctx context.Context, queue string, handler MessageHandler, opts ...SubscribeOption) error
    // ...
}
```

### 自定义任务处理器

注册自定义任务处理器：
```go
worker.RegisterHandler("custom-task", func(ctx context.Context, task *TaskMessage) (*TaskResult, error) {
    // 自定义处理逻辑
    return &TaskResult{Status: "completed"}, nil
})
```

## 最佳实践

1. **高可用部署**: 至少部署3个节点参与Leader选举
2. **任务幂等**: 确保任务可重复执行
3. **监控告警**: 配置队列长度、任务失败率等监控
4. **资源隔离**: 使用Kubernetes资源限制
5. **日志收集**: 使用ELK或其他日志系统收集分析

## 问题排查

### 任务不执行

1. 检查Redis连接
2. 确认有Worker在线
3. 查看Leader选举状态
4. 检查任务队列是否有积压

### 性能问题

1. 增加Worker数量
2. 优化Redis配置
3. 检查网络延迟
4. 分析任务执行时间

### 内存泄漏

1. 检查队列积压
2. 分析goroutine数量
3. 使用pprof分析

## 支持和贡献

- 问题反馈: 创建GitHub Issue
- 贡献代码: 提交Pull Request
- 文档改进: 编辑此README