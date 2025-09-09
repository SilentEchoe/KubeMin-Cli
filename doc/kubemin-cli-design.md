# KubeMin-Cli 设计与分布式工作流架构

本文记录 KubeMin-Cli 在分布式工作流方面的核心设计、关键决策、接口抽象、权衡取舍，以及未来可演进的方向，便于团队在实现与运维中达成一致。

## 背景与目标

- 目标：提供稳定、可扩展、可观测的分布式工作流执行框架，支持单机与分布式两种模式的自然切换。
- 约束：
  - 采用 Kubernetes Lease 选主；不依赖外部协调服务。
  - 队列具备“单次消费/可接管”的语义，避免重复执行。
  - 允许本地/开发环境在无中间件条件下正常运行。

## 总体架构

- API Server（REST + 业务逻辑）
  - 依赖注入（IoC）：`pkg/apiserver/utils/container` 提供容器，统一注入 datastore、kubeClient、queue 等。
  - 选主与角色：`client-go` 的 `leaderelection` + `Lease` 实现领导者选举与心跳。
- 分布式队列（统一抽象）
  - 接口：`pkg/apiserver/infrastructure/messaging/queue.go`
  - 实现：`RedisStreams`（默认分布式实现）、`NoopQueue`（本地/单机占位）
- 工作流执行
  - Dispatcher（仅领导者运行，分布式下不消费）：扫描 DB Waiting 任务 → 入队。
  - Worker（跟随者运行，或单机下由领导者兼任）：消费队列 → 更新状态 → 执行 Job → 结束 Ack。

## 关键决策与理由

1) 单一队列抽象（queue.Queue）

- 决策：统一放入 `pkg/apiserver/infrastructure/messaging`，以 Stream 语义（分组消费、确认）作为抽象。
- 好处：
  - 降低心智负担：Dispatcher/Worker 只依赖一套接口。
  - 便于扩展：后续接入 Kafka 时无需改动上层逻辑。

2) Redis Streams 作为默认分布式实现

- 决策：基于 Redis Streams 的 XADD/XGROUP/XREADGROUP/XACK/XAUTOCLAIM，实现“单次消费、可接管、可观测”。
- 好处：
  - 与 Pub/Sub 相比具备队列语义，消息持久度更强，支持 pending/claim。
  - 易于维护与部署，学习曲线平缓。

3) 单机/本地回退（NoopQueue）

- 决策：在无 Redis 的环境下，自动回退到原有 DB 轮询执行逻辑。
- 好处：提升研发/测试便利性，不引入额外依赖。

4) 副本与角色策略（Odd Replica + 阈值）

- 决策：
  - 启动时若副本数为偶数，最后启动的实例直接退出，保证最终副本为奇数（便于选主与容灾对称）。
  - 分布式阈值：`replicas >= 3` 为分布式模式（领导者仅分发），`replicas == 1` 为单机模式（领导者分发+执行）。
  - 领导者周期校验（每 30s）动态切换角色，适配伸缩。
- 好处：规避偶数副本在多数投票、故障场景中的边界问题，简化扩容策略。

5) 仅用 Lease 做选主/心跳，不承载副本数

- 决策：副本数通过当前 Pod → OwnerReferences → Controller（Deployment/StatefulSet/DaemonSet）读取期望副本。
- 好处：与控制器期望值保持一致，避免额外元数据同步。

## 队列接口与实现

接口定义：`pkg/apiserver/infrastructure/messaging/queue.go`

- `type Message struct { ID string; Payload []byte }`
- `EnsureGroup(ctx, group) error`：确保流与消费组存在。
- `Enqueue(ctx, payload) (id string, error)`：入队消息，返回消息 ID。
- `ReadGroup(ctx, group, consumer string, count int, block time.Duration) ([]Message, error)`：组内拉取消息。
- `Ack(ctx, group string, ids ...string) error`：确认处理完成。
- `AutoClaim(ctx, group, consumer string, minIdle time.Duration, count int) ([]Message, error)`：接管长时间未确认的 pending 消息。
- `Stats(ctx, group) (backlog, pending int64, err error)`：观测队列长度与组内待确认数。
- `Close(ctx) error`：释放资源。

实现映射：

- RedisStreams：`pkg/apiserver/infrastructure/messaging/redis_streams.go`
  - XADD → Enqueue；XGROUP CREATE MKSTREAM → EnsureGroup；XREADGROUP → ReadGroup；XACK → Ack；XAUTOCLAIM → AutoClaim；XLEN/XPENDING → Stats。
- NoopQueue：`pkg/apiserver/infrastructure/messaging/noop.go`
  - 占位实现，方便本地运行。

Kafka（规划）：

- EnsureGroup：创建 topic + group（存在则忽略）。
- Enqueue：Producer.Send（ID 可用 `partition:offset` 形式）。
- ReadGroup：ConsumerGroup 拉取；Ack：提交 offset；AutoClaim：空实现（交由协调器重平衡）。

## 工作流执行路径

1) Dispatcher（领导者）

- 每 3 秒扫描 DB 的 `Waiting` 任务：`service.WaitingTasks()`。
- 生成 `TaskDispatch` 载荷并 `Enqueue()` 入 Streams，日志打印 `taskId` 与 `streamID`。
- 分布式下领导者不执行任务；单机模式下领导者也启动 worker（见下）。

2) Worker（跟随者 | 单机模式下由领导者兼任）

- 主循环：`ReadGroup(..., block=2s)` 批量拉取 → 解析 payload → DB 加载 task → `updateQueueAndRunTask()`。
- 任务状态机：
  - `Waiting → Queued`（进入执行队列）
  - 运行完成后由 `WorkflowCtl.updateWorkflowStatus()` 落最终态（`Completed/Failed/Cancelled`）。
- 悬挂恢复：每 15s 调用 `AutoClaim(minIdle=60s)` 接管 pending 消息，按同路径处理。
- 可观测：消费日志包含 consumer、message id、task id。

## 选主与角色切换

- 选主：`leaderelection` + `Lease`。
- 领导者回调：
  - 确保组存在：`Queue.EnsureGroup("workflow-workers")`；
  - 定时打印队列指标：`Stats(backlog, pending)`；
  - 副本判定：`replicas >= 3` 仅分发，`== 1` 分发+执行；
  - 每 30s 重测副本数，动态切换角色。
- Follower 回调：当 `replicas >= 3` 启动 worker 订阅。

## 配置与环境

- `config.Config`：
  - `Messaging.Type`：`redis`（分布式）| 其他（单机 Noop）。
  - `Messaging.ChannelPrefix`：Stream key 前缀（默认 `kubemin`）。
  - `Cache.*`：Redis 连接参数（addr/username/password/db）。
  - `LeaderConfig.*`：Lease 相关参数（ID/lock name/namespace 等）。
- 环境变量：
  - `POD_NAME`, `POD_NAMESPACE`：用于查找控制器副本数（Downward API 注入）。

## 可观测性与运维

- 日志：
  - 分发：`taskId`、`streamID`。
  - 消费/接管：`consumer`、`message id`、`taskId`。
- 指标（初版日志形式）：
  - `Stats` 周期性打印 backlog（XLEN）与 pending（XPENDING.Count）。
- 后续可接入：
  - Prometheus metrics（backlog/pending/consume latency/retry 等）。
  - Trace（已在 HTTP 层面接入，可延伸到工作流执行路径）。

## 安全与健壮性

- 配置与密钥：通过环境变量/配置注入，不在日志打印敏感信息。
- 退出策略：偶数副本时新实例立即退出，减少不确定性。
- 容灾：领导者故障后新领导者接管分发；Worker 故障后通过 `AutoClaim` 接管 pending 消息。
- 资源与限流：KubeQPS/Burst 可配置，避免对 APIServer 施压。

## 局限与权衡

- 当前不重试：执行失败即 Ack，不会重新入队；需要上层幂等与补救措施。
- DB 与队列一致性：入队与状态改变间存在弱一致性，需依赖幂等、状态检查与人工兜底。
- Redis 可用性：作为中间件需要高可用部署，建议主从/哨兵或托管服务。

## 未来演进建议

1) Kafka 支持

- 基于现有 Queue 接口实现 Kafka 版（Producer/ConsumerGroup），在配置中切换 `Messaging.Type=kafka` 即可。

2) 重试与死信（DLQ）

- 在 `Ack` 前根据执行结果与重试次数决定是否重投；失败达到上限写入 DLQ 并报警。

3) 幂等与去重

- 以 `taskId` 为幂等键，增加 DB 侧 CAS 或状态转移约束，避免重复执行。

4) 优先级与限速

- 通过多 Stream/多分组实现优先级；在 Worker 端做并发度/速率控制（令牌桶/漏斗）。

5) 指标与告警

- 导出 Prometheus 指标：backlog、pending、消费延迟、执行耗时、失败率、claim 次数等；结合告警阈值。

6) 运维工具

- 加入队列巡检/清理工具（查看 backlog、pending、claim 分布，手动 reassign）。

7) 更细粒度的伸缩策略

- 根据 backlog/pending/处理速率，自动伸缩 Worker 副本（HPA/外部控制器）。

## 测试策略

- 单元测试：
  - Queue 接口的模拟实现；Redis Streams 的行为单测（可使用本地 Redis）。
  - Workflow 调度/消费路径的幂等与状态机测试。
- 集成测试：
  - 带 Redis 的 end-to-end（创建任务 → 入队 → 消费 → 状态终态）。
  - 领导者切换、Worker 崩溃恢复、AutoClaim 接管。

## 部署建议

- Pod 环境：注入 `POD_NAME/POD_NAMESPACE`（Downward API）。
- 资源：为 API Server/Worker 设置合理的 CPU/内存 requests/limits。
- 探针：liveness/readiness；可在 readiness 中检查 Redis 连通性（分布式模式）。
- 反亲和：多副本分散，减少同节点故障相关性。
- 奇偶策略：保证最终副本为奇数，避免最后一个副本被动退出影响体验（可在 CI/CD 阶段预校验）。

## 兼容性与迁移

- 历史 `messaging` 包已移除，改用 `queue`。Dispatcher/Worker 的业务逻辑保持一致，对外 API 无破坏性变更。
- 本地/开发环境无需 Redis，默认走 NoopQueue + DB 轮询执行逻辑。

---

如需补充架构图、时序图或 Prometheus 指标定义，可在本篇基础上追加附录，或拆分到 `docs/` 目录的专题文档中。
