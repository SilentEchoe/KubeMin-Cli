# Kafka 分布式消息队列实现文档

## 目录

- [1. 背景与目标](#1-背景与目标)
- [2. 架构设计](#2-架构设计)
- [3. 实现细节](#3-实现细节)
- [4. 配置说明](#4-配置说明)
- [5. 使用指南](#5-使用指南)
- [6. 与 Redis Streams 对比](#6-与-redis-streams-对比)
- [7. 注意事项](#7-注意事项)
- [8. 改动文件清单](#8-改动文件清单)

---

## 1. 背景与目标

### 1.1 背景

KubeMin-Cli 工作流引擎原本支持两种消息队列模式：

- **NoopQueue**：本地模式，适用于单实例开发测试
- **RedisStreams**：基于 Redis Streams 的分布式模式

随着业务规模扩大，部分场景需要更高的吞吐量和更强的分布式能力，因此引入 Apache Kafka 作为第三种消息队列后端。

### 1.2 目标

- 实现 `Queue` 接口的 Kafka 后端，与现有 Redis Streams 实现保持语义一致
- 提供简洁通用的配置方式，不增加过多复杂性
- 利用 Kafka 原生 Consumer Group 和 Rebalance 机制实现高可用
- 更新相关文档，确保使用者能够顺利切换使用

---

## 2. 架构设计

### 2.1 整体架构

```
┌─────────────────────────────────────────────────────────────┐
│                     Queue Interface                          │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐         │
│  │  NoopQueue  │  │ RedisStreams│  │ KafkaQueue  │  ← NEW  │
│  └─────────────┘  └─────────────┘  └─────────────┘         │
└─────────────────────────────────────────────────────────────┘
         │                  │                  │
         ▼                  ▼                  ▼
┌─────────────┐    ┌─────────────┐    ┌─────────────┐
│   (none)    │    │    Redis    │    │    Kafka    │
└─────────────┘    └─────────────┘    └─────────────┘
```

### 2.2 Kafka 特性映射

| Queue 接口方法 | Kafka 实现方式 |
|---------------|---------------|
| `EnsureGroup` | 创建 kafka.Reader，Consumer Group 自动创建 |
| `Enqueue` | kafka.Writer 写入消息到指定 Topic |
| `ReadGroup` | kafka.Reader.FetchMessage 从 Consumer Group 读取 |
| `Ack` | kafka.Reader.CommitMessages 提交 Offset |
| `AutoClaim` | 返回空，依赖 Kafka 原生 Rebalance 机制 |
| `Stats` | 返回 Reader 的 Lag 统计和 Pending 消息数 |
| `Close` | 关闭 Writer 和 Reader |

### 2.3 消息流转图

```
┌─────────────┐         ┌─────────────┐         ┌─────────────┐
│  Dispatcher │ ──────> │    Kafka    │ ──────> │   Worker    │
│  (Producer) │  WRITE  │   Broker    │   READ  │  (Consumer) │
└─────────────┘         └─────────────┘         └─────────────┘
                              │
                              ▼
                     ┌─────────────────┐
                     │  Consumer Group │
                     │  (Coordination) │
                     └─────────────────┘
```

---

## 3. 实现细节

### 3.1 KafkaQueue 结构体

```go
type KafkaQueue struct {
    cfg    KafkaConfig
    writer *kafka.Writer

    mu     sync.RWMutex
    reader *kafka.Reader

    pendingMu       sync.Mutex
    pendingMessages map[string]kafka.Message
}
```

核心设计要点：

1. **延迟初始化 Reader**：Reader 在 `EnsureGroup` 调用时创建，支持灵活配置
2. **Pending 消息跟踪**：使用 map 跟踪已读取但未确认的消息，支持精确的 Offset 提交
3. **线程安全**：使用读写锁保护 Reader，互斥锁保护 Pending 消息

### 3.2 消息 ID 生成

```go
func (k *KafkaQueue) messageID(msg kafka.Message) string {
    return fmt.Sprintf("%d:%d", msg.Partition, msg.Offset)
}
```

消息 ID 由 `partition:offset` 组成，确保全局唯一性。

### 3.3 AutoClaim 处理策略

与 Redis Streams 不同，Kafka 不直接支持 AutoClaim 语义。本实现采用以下策略：

```go
func (k *KafkaQueue) AutoClaim(ctx context.Context, ...) ([]Message, error) {
    // Kafka 通过 Rebalance 机制自动处理消费者失败的情况
    // 当消费者离开或崩溃时，其分区会被重新分配给组内其他消费者
    return nil, nil
}
```

**工作原理**：
- 当 Worker 崩溃或超时未提交 Offset 时，Kafka Broker 会触发 Rebalance
- 该 Worker 负责的 Partition 会被重新分配给其他健康的 Worker
- 未提交 Offset 的消息会被重新消费

### 3.4 客户端初始化

```go
// clients/kafka.go
func EnsureKafka(cfg KafkaConfig) (*kafka.Dialer, error) {
    // 单例模式，确保全局只有一个 Dialer
    // 验证 Broker 连接性
    // 缓存连接供健康检查使用
}
```

---

## 4. 配置说明

### 4.1 配置结构

```go
type MessagingConfig struct {
    Type          string   // noop|redis|kafka
    ChannelPrefix string   // 消息通道/Topic 前缀
    
    // Redis 配置
    RedisStreamMaxLen int64
    
    // Kafka 配置
    KafkaBrokers         []string  // Broker 地址列表
    KafkaGroupID         string    // Consumer Group ID
    KafkaAutoOffsetReset string    // earliest|latest
}
```

### 4.2 命令行参数

| 参数 | 说明 | 默认值 |
|------|------|--------|
| `--msg-type` | 消息队列类型 | `redis` |
| `--msg-channel-prefix` | 消息通道前缀 | `kubemin` |
| `--msg-kafka-brokers` | Kafka Broker 地址列表 | - |
| `--msg-kafka-group-id` | Consumer Group ID | `kubemin-workflow-workers` |
| `--msg-kafka-offset-reset` | Offset 重置策略 | `earliest` |

### 4.3 配置示例

```bash
# 使用 Kafka 作为消息队列后端
./kubemin-cli \
  --msg-type=kafka \
  --msg-kafka-brokers=kafka-0:9092,kafka-1:9092,kafka-2:9092 \
  --msg-kafka-group-id=kubemin-workflow-workers \
  --msg-kafka-offset-reset=earliest
```

---

## 5. 使用指南

### 5.1 前置要求

1. 部署并启动 Kafka 集群（建议 3 节点以上）
2. 确保应用能够访问 Kafka Broker

### 5.2 切换到 Kafka

1. 修改启动参数：

```bash
--msg-type=kafka \
--msg-kafka-brokers=your-kafka-broker:9092
```

2. 或修改配置文件：

```yaml
messaging:
  type: kafka
  channelPrefix: kubemin
  kafkaBrokers:
    - kafka-0.kafka.svc:9092
    - kafka-1.kafka.svc:9092
    - kafka-2.kafka.svc:9092
  kafkaGroupID: kubemin-workflow-workers
  kafkaAutoOffsetReset: earliest
```

### 5.3 验证连接

启动后，检查日志中是否有以下信息：

```
I kafka dialer initialized, connected to broker: kafka-0:9092
I kafka reader initialized for topic=kubemin.workflow.dispatch group=kubemin-workflow-workers
```

---

## 6. 与 Redis Streams 对比

| 特性 | Redis Streams | Kafka |
|------|---------------|-------|
| **消息持久化** | 内存 + RDB/AOF | 磁盘 |
| **吞吐量** | 中等 (~100K msg/s) | 高 (~1M msg/s) |
| **消费者恢复** | AutoClaim (主动) | Rebalance (被动) |
| **部署复杂度** | 低 | 中等 |
| **适用场景** | 中小规模 | 大规模 |
| **延迟** | 低 | 中等 |
| **消息回溯** | 有限 | 支持 |

### 6.1 选择建议

- **选择 Redis Streams**：
  - 已有 Redis 基础设施
  - 任务量适中（< 10K/s）
  - 需要低延迟
  - 希望简化运维

- **选择 Kafka**：
  - 需要高吞吐量
  - 需要消息持久化和回溯
  - 已有 Kafka 基础设施
  - 大规模分布式部署

---

## 7. 注意事项

### 7.1 Topic 自动创建

默认情况下，Kafka 可能禁用自动创建 Topic。请确保：

1. Kafka 配置 `auto.create.topics.enable=true`，或
2. 预先创建所需的 Topic：

```bash
kafka-topics.sh --create \
  --topic kubemin.workflow.dispatch \
  --partitions 3 \
  --replication-factor 3 \
  --bootstrap-server kafka:9092
```

### 7.2 分区数配置

建议 Topic 分区数 >= Worker 数量，以实现最佳负载均衡。

### 7.3 Consumer Group 协调

- 确保所有 Worker 使用相同的 `--msg-kafka-group-id`
- Rebalance 期间可能有短暂的消息处理延迟

### 7.4 Offset 提交策略

本实现使用手动提交 Offset（`CommitInterval=0`），确保消息被成功处理后才提交。这避免了消息丢失，但需要注意：

- 如果 Worker 在处理消息后、提交 Offset 前崩溃，消息会被重新消费
- 应用层需要实现幂等处理

### 7.5 网络分区处理

Kafka 在网络分区时的行为：

- Producer 写入会超时重试
- Consumer 会触发 Rebalance
- 建议配置合理的超时参数

---

## 8. 改动文件清单

### 8.1 新增文件

| 文件路径 | 说明 |
|----------|------|
| `pkg/apiserver/infrastructure/messaging/kafka.go` | KafkaQueue 实现 |
| `pkg/apiserver/infrastructure/messaging/kafka_test.go` | KafkaQueue 单元测试 |
| `pkg/apiserver/infrastructure/clients/kafka.go` | Kafka 客户端初始化 |
| `docs/kafka-queue-implementation.md` | 本文档 |

### 8.2 修改文件

| 文件路径 | 改动说明 |
|----------|----------|
| `pkg/apiserver/config/config.go` | 新增 Kafka 配置字段和验证逻辑 |
| `pkg/apiserver/server.go` | 新增 `buildKafkaQueue` 方法 |
| `docs/workflow-architecture-guide.md` | 更新队列实现列表、配置说明和推荐配置 |
| `go.mod` | 新增 `github.com/segmentio/kafka-go` 依赖 |

### 8.3 测试覆盖

```bash
# 运行 Kafka 相关测试
go test ./pkg/apiserver/infrastructure/messaging/... -v -run Kafka

# 测试用例包括：
# - TestNewKafkaQueue_Validation       # 配置验证测试
# - TestKafkaQueue_DefaultValues       # 默认值测试
# - TestKafkaQueue_EnsureGroupWithoutReader  # 初始化测试
# - TestKafkaQueue_ReadGroupWithoutInit      # 读取测试
# - TestKafkaQueue_AckWithoutInit            # 确认测试
# - TestKafkaQueue_AutoClaimReturnsEmpty     # AutoClaim 测试
# - TestKafkaQueue_StatsWithoutInit          # 统计测试
# - TestKafkaQueue_CloseWithoutInit          # 关闭测试
# - TestKafkaConfig_AutoOffsetResetValues    # 偏移量配置测试
```

### 8.4 依赖变更

```
+ github.com/segmentio/kafka-go v0.4.49
+ github.com/klauspost/compress v1.15.9  (间接)
+ github.com/pierrec/lz4/v4 v4.1.15     (间接)
```

---

*文档版本：1.0.0*
*创建日期：2025-12*
*作者：KubeMin-Cli Team*





