# 分布式工作流集成说明

## 集成点

### 1. 配置层 (config/config.go)
```go
type Config struct {
    // ... 其他配置
    RedisAddr         string  // Redis地址
    EnableDistributed bool    // 是否启用分布式
    MaxWorkers        int     // 最大Worker数
}
```

### 2. 服务层 (server.go)
```go
// buildIoCContainer 中的修改
if s.cfg.RedisAddr != "" {
    // 使用分布式模式
    eventBeans = event.InitEventWithOptions(s.cfg.RedisAddr)
} else {
    // 使用本地模式
    eventBeans = event.InitEvent()
}
```

### 3. 事件层 (event/init_distributed.go)
```go
func InitEventWithOptions(redisAddr string) []interface{} {
    if redisAddr != "" {
        // 创建分布式工作流
        distributedWorkflow := &workflow.SimpleDistributedWorkflow{}
        distributedWorkflow.SetRedisAddr(redisAddr)
        distributedWorkflow.SetMaxWorkers(10)
        workers = append(workers, distributedWorkflow)
        return []interface{}{distributedWorkflow}
    }
    return InitEvent()
}
```

### 4. 工作流层 (workflow/distributed_simple.go)
```go
type SimpleDistributedWorkflow struct {
    Workflow  // 嵌入原有工作流
    
    redisClient *redis.Client
    isLeader    bool
    nodeID      string
    // ...
}
```

## 调用流程

```
main.go
  ↓
server.Run()
  ↓
buildIoCContainer()
  ↓
检查 cfg.RedisAddr
  ↓
有Redis地址 → InitEventWithOptions() → SimpleDistributedWorkflow
  ↓
无Redis地址 → InitEvent() → Workflow (本地模式)
  ↓
setupLeaderElection()
  ↓
OnStartedLeading() → Start() → 启动分布式/本地工作流
```

## 使用方式

### 方式1: 环境变量
```bash
export REDIS_ADDR=localhost:6379
./kubemin-cli
```

### 方式2: 命令行参数
```bash
./kubemin-cli --redis-addr=localhost:6379 --enable-distributed=true
```

### 方式3: 配置文件
```yaml
redisAddr: "localhost:6379"
enableDistributed: true
maxWorkers: 10
```

## 模式切换

### 本地模式 → 分布式模式
1. 部署Redis
2. 设置REDIS_ADDR环境变量
3. 重启服务

### 分布式模式 → 本地模式
1. 移除REDIS_ADDR环境变量
2. 重启服务
3. 系统自动降级到本地模式

## 验证分布式模式

### 1. 检查日志
```bash
# 应该看到以下日志
"Initializing distributed workflow with Redis at localhost:6379"
"Distributed mode enabled with Redis at localhost:6379"
```

### 2. 检查Redis连接
```bash
redis-cli ping
# PONG

redis-cli KEYS "workflow:*"
# 1) "workflow:queue"
```

### 3. 检查Leader选举
```bash
kubectl get lease -n min-cli-system apiserver-lock -o yaml
```

## 故障排查

### 问题1: InitEventWithOptions未被调用
**原因**: server.go中仍在使用InitEvent()
**解决**: 已修改server.go使用配置中的RedisAddr判断

### 问题2: Redis连接失败
**日志**: "Failed to init Redis, falling back to local mode"
**解决**: 
- 检查Redis是否运行
- 检查Redis地址是否正确
- 检查网络连接

### 问题3: 分布式模式未启用
**检查点**:
1. 配置中RedisAddr是否设置
2. InitEventWithOptions是否被调用
3. SimpleDistributedWorkflow是否被创建

## 监控指标

### Redis队列
```bash
# 队列长度
redis-cli LLEN workflow:queue

# 活跃锁
redis-cli KEYS "lock:*"
```

### 系统日志
```bash
# 分布式相关日志
grep -E "distributed|redis|leader" app.log
```

### 性能指标
- 任务处理速率
- 队列积压情况
- Worker利用率
- Redis连接池状态

## 最佳实践

1. **配置管理**
   - 使用环境变量管理Redis地址
   - 不同环境使用不同配置

2. **高可用部署**
   - 至少3个节点参与Leader选举
   - Redis使用主从或集群模式

3. **监控告警**
   - 监控队列长度
   - 监控任务失败率
   - 监控Redis连接状态

4. **优雅升级**
   - 先升级Follower节点
   - 最后升级Leader节点
   - 使用滚动更新策略