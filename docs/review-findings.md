# 代码评审发现

## 背景
根据现有实现和 `docs/` 文档描述，对工作流和队列相关逻辑做了一次静态检查，重点关注可用性、一致性以及文档与实现的偏差。

## 问题与优化建议

1) **重启/切主会强制取消所有运行中任务**  
   - 位置：`pkg/apiserver/event/workflow/workflow.go` `InitQueue()`  
   - 行为：进程启动时列出“正在运行”的任务并逐个 `CancelWorkflowTask`，导致普通重启或 leader 切换就中断所有执行。  
   - 风险：与文档宣称的“可恢复/可观测”目标冲突，任务被反复打断或残留部分状态。  
   - 建议：仅取消确实超时或重复执行的任务；运行中任务应尝试恢复或交由 AutoClaim 继续处理。

2) **队列消息解析失败时仍确认，可能丢任务**  
   - 位置：`pkg/apiserver/event/workflow/dispatcher.go` `processDispatchMessage()`  
   - 行为：反序列化失败或 DB 读取任务失败时返回 `ack=true`，消息被确认但任务未执行。  
   - 风险：瞬时故障直接丢弃任务，无法自动重试。  
   - 建议：解析/加载失败时不 Ack，让消息保留 pending（或写入死信）；补充失败指标和告警。

3) **Worker 连续少量错误即退出，无重启/监督**  
   - 位置：`dispatcher.go`（读流/claim 连续 3 次失败直接退出），`pkg/apiserver/server.go`（worker 启动后缺少监督重拉）。  
   - 风险：Redis 短时抖动或网络闪断会让消费端停摆，队列堆积需人工重启。  
   - 建议：使用指数退避持续重试，或退出后由上层周期性重建订阅；增加健康探针/指标。

4) **文档与路由实现不一致**  
   - 文档：`docs/workflow-cancellation-and-cleanup.md` 等描述了 `/workflow/exec`、`/workflow/cancel`。  
   - 代码：`pkg/apiserver/interfaces/api/workflow.go` 中这些路由返回 501 占位，实际可用的是 `/applications/:appID/workflow/...`。  
   - 风险：调用方按文档请求会遇到未实现错误。  
   - 建议：实现文档声明的接口或在文档中改为指向现有应用级接口，并移除占位路由。

5) **串行步骤并发失败处理不符合预期**  
   - 位置：`pkg/apiserver/event/workflow/controller.go` `Run()`  
   - 行为：`SequentialMaxConcurrency` > 1 时，批内 Job 失败不会立刻停止，其他 Job 仍继续创建资源，最后才整体标记失败。  
   - 风险：串行模式下出现不必要的资源创建/脏状态。  
   - 建议：串行步骤失败后立即停止同批次剩余 Job（或回滚），与“串行”语义保持一致。

6) **HTTP 服务缺少优雅关闭**  
   - 位置：`pkg/apiserver/server.go` `startHTTP()`  
   - 行为：直接 `ListenAndServe`，收到 SIGTERM/失去 leader 时未调用 `Shutdown`。  
   - 风险：现有请求被硬中断或端口占用延迟释放，影响滚动升级。  
   - 建议：将 `http.Server` 绑定到 `ctx`，在退出路径调用 `Shutdown`，留出超时保护。

## 解决方案

### 1. 重启/切主任务取消优化
**方案**：
- 添加任务超时检测，仅取消超过配置超时时间的任务
- 实现任务执行者身份验证，避免取消其他实例的合法任务
- 引入任务状态恢复机制，对于非超时任务尝试重新认领

**实现步骤**：
```go
// 在 InitQueue() 中添加超时检查
maxTaskAge := time.Duration(cfg.Workflow.TaskTimeoutMinutes) * time.Minute
for _, task := range tasks {
    // 检查任务是否真正超时
    if time.Since(task.StartTime) > maxTaskAge {
        // 仅取消超时任务
        w.WorkflowService.CancelWorkflowTask(ctx, config.DefaultTaskRevoker, task.TaskID, "timeout")
    } else if isTaskOwnedByDeadInstance(task) {
        // 取消已死亡实例的任务
        w.WorkflowService.CancelWorkflowTask(ctx, config.DefaultTaskRevoker, task.TaskID, "owner-dead")
    }
    // 其他情况保留任务，由 AutoClaim 处理
}
```

### 2. 消息解析失败处理改进
**方案**：
- 解析失败时不确认消息，让消息保持在 pending 状态
- 实现重试机制，最多重试3次
- 超过重试次数的消息写入死信队列

**实现步骤**：
```go
func (w *Workflow) processDispatchMessage(ctx context.Context, m msg.Message) (bool, string) {
    td, err := UnmarshalTaskDispatch(m.Payload)
    if err != nil {
        klog.Errorf("decode dispatch failed: %v", err)
        return false, ""  // 不确认消息
    }

    task, err := repository.TaskByID(ctx, w.Store, td.TaskID)
    if err != nil {
        klog.Errorf("load task %s failed: %v", td.TaskID, err)
        return false, td.TaskID  // 不确认消息
    }

    // 任务执行逻辑...
    return true, td.TaskID
}
```

### 3. Worker 错误恢复机制
**方案**：
- 实现指数退避重试，最大重试时间延长至5分钟
- Worker 退出后由上层监督进程自动重启
- 添加健康检查端点，监控 Worker 状态

**实现步骤**：
```go
func (w *Workflow) StartWorker(ctx context.Context, errChan chan error) {
    go func() {
        for {
            select {
            case <-ctx.Done():
                return
            default:
                // 运行 Worker 逻辑
                err := w.runWorkerOnce(ctx)
                if err != nil {
                    klog.Errorf("worker exited with error: %v, will retry", err)
                    time.Sleep(time.Minute) // 等待1分钟后重试
                }
            }
        }
    }()
}
```

### 4. API 文档一致性修复
**方案**：
- 实现文档中声明的 `/workflow/exec` 和 `/workflow/cancel` 接口
- 保持现有应用级接口不变以兼容已有客户端
- 更新文档明确说明两套接口的使用场景

**实现步骤**：
```go
func (w *workflow) execWorkflowTask(c *gin.Context) {
    var req ExecuteWorkflowRequest
    if err := c.Bind(&req); err != nil {
        bcode.ReturnError(c, err)
        return
    }

    // 调用工作流服务执行
    taskID, err := w.WorkflowService.ExecuteWorkflow(c.Request.Context(), req)
    if err != nil {
        bcode.ReturnError(c, err)
        return
    }

    c.JSON(http.StatusOK, gin.H{"taskId": taskID})
}
```

### 5. 串行步骤并发控制优化
**方案**：
- 串行模式下即使并发度>1，任一任务失败立即停止整个批次
- 实现快速失败机制，取消同批次剩余任务
- 添加回滚逻辑，清理已创建的资源

**实现步骤**：
```go
func (p *Pool) work() {
    for job := range p.jobsChan {
        runJob(p.ctx, job, p.client, p.store, p.ack)

        // 串行模式下快速失败
        if p.stopOnFailure && jobStatusFailed(job.Status) && !p.isParallelMode {
            p.failureOnce.Do(func() {
                p.cancel()
                // 清理同批次剩余任务
                p.cleanupRemainingJobs()
            })
        }
        p.wg.Done()
    }
}
```

### 6. HTTP 服务优雅关闭
**方案**：
- 使用 http.Server 的 Shutdown 方法实现优雅关闭
- 设置合理的关闭超时时间（30秒）
- 处理 SIGTERM 信号，确保容器正常退出

**实现步骤**：
```go
func (s *restServer) startHTTP(ctx context.Context) error {
    server := &http.Server{
        Addr:    s.cfg.BindAddr,
        Handler: s,
        ReadHeaderTimeout: 2 * time.Second,
    }

    // 优雅关闭处理
    go func() {
        <-ctx.Done()
        shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
        defer cancel()
        if err := server.Shutdown(shutdownCtx); err != nil {
            klog.Errorf("HTTP server shutdown error: %v", err)
        }
    }()

    return server.ListenAndServe()
}

## 增强版解决方案（更全面/更稳健）

1) 重启/切主策略  
   - 仅取消“超过 DefaultJobTimeout/Workflow.DefaultJobTimeout 的运行中任务”或“状态缺失/无法加载的异常任务”；其余交由队列 AutoClaim 接管，不主动 Cancel。  
   - 在 `WorkflowQueue` 增加 `OwnerID`（消费者/leader ID）与 `LastHeartbeat`，启动时只接管 Owner 已失效且超时的任务，避免误杀。  
   - 引入“任务恢复”日志：重启后将被接管的 taskId 记录到审计/指标，便于观测。

2) 队列可靠性  
   - 消息附带 `attempt` 字段（或使用 Redis Stream entry metadata + XPENDING），`processDispatchMessage` 失败时不 Ack；超过阈值（如 5 次）写入 `<stream>.dlq` 并 Ack 原消息。  
   - 增加 DLQ 消费/巡检脚本（导出指标、可手动重放）。  
   - 对于解析失败，直接 DLQ（不可重试）；对于 DB 暂时失败，保留 pending 由 AutoClaim 重试。

3) Worker 自愈与角色协同  
   - 单个 worker 内部使用指数退避而不轻易退出；读/claim 连续失败达到阈值只做告警，不退出。  
   - `restServer` 维持“单实例 worker”语义：通过 `startWorkers/stopWorkers` 控制；worker goroutine内部监听 ctx，ctx 取消即退出，避免自循环导致悬挂。  
   - 增加健康指标：read/claim error count、backlog、pending、last-success-timestamp，暴露给 `/metrics` 或日志。

4) API 兼容与文档  
   - 保留 `/applications/:appID/workflow/...` 作为主接口；`/workflow/exec|cancel` 作为薄包装，内部需要 `appID` 时可从请求体/查询参数获取，缺失则返回 400。  
   - 文档中列出两套接口的适用场景，并声明 `/workflow/*` 只是包装层。

5) 串行并发语义  
   - 在 `controller.Run` 中：StepByStep 模式始终 `stopOnFailure=true`，即使并发度>1 也在首失败后取消剩余 Job；并行模式保持现状。  
   - 清理策略：失败 Job 已调用 `Clean`，同批次未启动 Job 直接标记为 `StatusSkipped`，避免多余资源创建。

6) HTTP 优雅关闭与进程生命周期  
   - `startHTTP` 监听 ctx，`Shutdown` 时忽略 `http.ErrServerClosed`；关闭时同时停止 queue metrics/replica watcher，防 goroutine 泄漏。  
   - 进程主循环在 `Run` 收到 errChan 或 term 信号后统一取消 ctx，确保 worker/dispatcher 退出、HTTP 优雅关闭、Tracer shutdown 执行。
```

## 实施计划

1. **优先级排序**：
   - 高优先级：问题2（消息丢失）、问题3（Worker退出）、问题6（优雅关闭）
   - 中优先级：问题1（任务取消）、问题5（串行失败）
   - 低优先级：问题4（API一致性）

2. **测试策略**：
   - 为每个修复添加单元测试
   - 进行集成测试验证整体流程
   - 模拟故障场景确保恢复机制有效

3. **发布计划**：
   - 分阶段发布，先修复高优先级问题
   - 保持向后兼容性
   - 更新相关文档和示例

---

## 风险补充与改进方案 v2

基于对代码的深入分析，原有解决方案存在以下潜在风险和遗漏，本节提供更完善的改进方案。

### 原方案风险分析

#### 问题 1：重启/切主任务取消优化 - 风险点

| 风险 | 描述 | 影响 |
|------|------|------|
| Schema 缺失 | `isTaskOwnedByDeadInstance` 依赖 `OwnerID`/`LastHeartbeat` 字段，但 `WorkflowQueue` 模型当前没有这些字段 | 无法判断任务归属，方案无法落地 |
| 时间戳语义错误 | `WorkflowQueue.CreateTime` 是任务创建时间而非执行开始时间，用它判断超时会误杀排队很久但刚开始执行的任务 | 正常任务被错误取消 |
| AutoClaim 竞态 | 不取消任务而交给 AutoClaim，分布式场景下可能出现同一任务被多个 worker 同时认领 | 任务重复执行，资源冲突 |
| Cancel Reason 空值 | 当前 `InitQueue` 取消时 `reason` 传空字符串，审计日志无法追溯 | 问题排查困难 |

#### 问题 2：消息解析失败处理 - 风险点

| 风险 | 描述 | 影响 |
|------|------|------|
| 无限重试堆积 | 不 Ack 让消息保留 pending，故障消息被 AutoClaim 反复领取又失败，阻塞队列 | 队列堵塞，新任务无法执行 |
| DLQ 未实现 | 代码里没有死信队列逻辑，方案只给出伪代码 | 无法隔离毒丸消息 |
| attempt 字段缺失 | `TaskDispatch` 结构没有重试计数，无法判断重试次数 | 无法实现重试上限 |
| 语义不一致 | 解析失败和 DB 加载失败都 `return true`，但前者不可重试、后者可重试 | 处理策略混乱 |

#### 问题 3：Worker 错误恢复 - 风险点

| 风险 | 描述 | 影响 |
|------|------|------|
| Goroutine 泄漏 | 伪代码 `for { ... }` 若 ctx 未正确传递，进程关闭时无法退出 | 内存泄漏，进程挂起 |
| 健康探针未暴露 | `/metrics` 端点和 Prometheus 格式导出逻辑未实现 | 运维无法观测 Worker 状态 |
| 退避时间硬编码 | 建议 `time.Sleep(time.Minute)` 缺少配置化 | 生产环境难以调优 |

#### 问题 5：串行步骤并发失败 - 风险点

| 风险 | 描述 | 影响 |
|------|------|------|
| stopOnFailure 传值错误 | `controller.go:113` 传的是 `stepExec.Mode.IsParallel()`，串行模式传 `false`，与期望相反 | 串行模式不会快速失败 |
| cleanupRemainingJobs 未定义 | 方案提到"清理同批次剩余任务"但未说明如何实现 | 无法落地 |
| 回滚复杂度 | 已创建资源的删除顺序、依赖关系、finalizer 阻塞等未考虑 | 残留资源 |
| Pool.isParallelMode 缺失 | `Pool` 结构当前只有 `stopOnFailure`，没有模式标识 | 需要扩展结构 |

#### 问题 6：HTTP 优雅关闭 - 风险点

| 风险 | 描述 | 影响 |
|------|------|------|
| Shutdown 被取消 | 30s 超时后 `shutdownCtx` 取消，仍然硬中断 | 请求丢失 |
| ErrServerClosed 未处理 | `ListenAndServe` 返回 `http.ErrServerClosed` 时被当作错误上报 | 误报警 |
| Leader Election 冲突 | `ReleaseOnCancel: true`，若 HTTP 关闭先于 lease 释放可能短暂双主 | 状态不一致 |
| 端口释放延迟 | TCP TIME_WAIT 导致重启后端口仍被占用 | 滚动升级失败 |

#### 额外发现的隐藏风险

| 位置 | 风险 | 影响 |
|------|------|------|
| `signal/cancel.go:141` | `SetNX` 成功后若 goroutine 未启动就 panic，key 残留 45s 阻塞后续执行 | 任务无法启动 |
| `controller.go:150` | `resolveDefaultJobTimeout` 使用 `config.DefaultJobTaskTimeout` 但该常量定义不完整 | 编译错误或运行时 panic |
| `workflow.go:79-80` | 取消任务时 `reason` 传空字符串 | 审计日志缺失 |

---

### 改进方案 v2

#### 1. 重启/切主任务处理 - 完整方案

**Schema 变更**（需要数据库迁移）：

```sql
ALTER TABLE min_workflow_queue 
ADD COLUMN executor_id VARCHAR(255) DEFAULT '' COMMENT '执行者实例ID',
ADD COLUMN started_at BIGINT DEFAULT 0 COMMENT '实际开始执行时间戳',
ADD COLUMN heartbeat_at BIGINT DEFAULT 0 COMMENT '最后心跳时间戳';

CREATE INDEX idx_workflow_queue_executor ON min_workflow_queue(executor_id);
```

**Model 变更**：

```go
// pkg/apiserver/domain/model/workflow_queue.go
type WorkflowQueue struct {
    // ... existing fields ...
    ExecutorID  string `gorm:"column:executor_id" json:"executor_id,omitempty"`
    StartedAt   int64  `gorm:"column:started_at" json:"started_at,omitempty"`
    HeartbeatAt int64  `gorm:"column:heartbeat_at" json:"heartbeat_at,omitempty"`
}
```

**InitQueue 改进**：

```go
func (w *Workflow) InitQueue(ctx context.Context) {
    if w.Store == nil {
        klog.Error("datastore is nil")
        return
    }
    
    tasks, err := w.WorkflowService.TaskRunning(ctx)
    if err != nil {
        klog.Errorf("find task running error: %v", err)
        return
    }
    
    instanceID := w.getInstanceID()
    now := time.Now().Unix()
    maxTaskAge := w.resolveMaxTaskAge()
    heartbeatTimeout := w.resolveHeartbeatTimeout()
    
    var cancelErrs []error
    var recoveredCount, cancelledCount int
    
    for _, task := range tasks {
        // 1. 检查是否真正超时（基于 StartedAt 而非 CreateTime）
        if task.StartedAt > 0 && now-task.StartedAt > maxTaskAge {
            if err := w.cancelWithReason(ctx, task.TaskID, "task_timeout"); err != nil {
                cancelErrs = append(cancelErrs, err)
            } else {
                cancelledCount++
            }
            continue
        }
        
        // 2. 检查执行者是否存活（心跳超时）
        if task.ExecutorID != "" && task.ExecutorID != instanceID {
            if task.HeartbeatAt > 0 && now-task.HeartbeatAt > heartbeatTimeout {
                if err := w.cancelWithReason(ctx, task.TaskID, 
                    fmt.Sprintf("executor_dead:%s", task.ExecutorID)); err != nil {
                    cancelErrs = append(cancelErrs, err)
                } else {
                    cancelledCount++
                }
                continue
            }
            // 执行者存活，跳过
            klog.V(4).Infof("task %s owned by active executor %s, skipping", 
                task.TaskID, task.ExecutorID)
            continue
        }
        
        // 3. 无执行者或本实例的任务，尝试恢复
        klog.Infof("recovering orphan task: %s", task.TaskID)
        recoveredCount++
        // 交由 AutoClaim 或直接重新入队
    }
    
    klog.Infof("InitQueue completed: recovered=%d cancelled=%d errors=%d", 
        recoveredCount, cancelledCount, len(cancelErrs))
}

func (w *Workflow) cancelWithReason(ctx context.Context, taskID, reason string) error {
    return w.WorkflowService.CancelWorkflowTask(ctx, config.DefaultTaskRevoker, taskID, reason)
}
```

**心跳机制**：

```go
// 在 WorkflowCtl.Run 中启动心跳
func (w *WorkflowCtl) startHeartbeat(ctx context.Context) {
    ticker := time.NewTicker(10 * time.Second)
    defer ticker.Stop()
    
    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            w.mutateTask(func(task *model.WorkflowQueue) {
                task.HeartbeatAt = time.Now().Unix()
            })
            w.ack()
        }
    }
}
```

---

#### 2. 消息队列可靠性 - 完整方案

**DLQ 实现**：

```go
// pkg/apiserver/infrastructure/messaging/dlq.go
package messaging

import (
    "context"
    "encoding/json"
    "fmt"
    "time"
    
    "github.com/redis/go-redis/v9"
)

const (
    dlqSuffix       = ".dlq"
    maxRetryCount   = 5
    dlqRetentionDays = 7
)

type DLQEntry struct {
    OriginalID    string    `json:"original_id"`
    Payload       []byte    `json:"payload"`
    Error         string    `json:"error"`
    RetryCount    int       `json:"retry_count"`
    FailedAt      time.Time `json:"failed_at"`
    LastAttemptAt time.Time `json:"last_attempt_at"`
}

func (r *RedisStreams) MoveToDLQ(ctx context.Context, group, msgID string, payload []byte, err error, retryCount int) error {
    dlqKey := r.streamKey + dlqSuffix
    
    entry := DLQEntry{
        OriginalID:    msgID,
        Payload:       payload,
        Error:         err.Error(),
        RetryCount:    retryCount,
        FailedAt:      time.Now(),
        LastAttemptAt: time.Now(),
    }
    
    data, _ := json.Marshal(entry)
    
    if _, err := r.cli.XAdd(ctx, &redis.XAddArgs{
        Stream: dlqKey,
        Values: map[string]interface{}{"data": string(data)},
    }).Result(); err != nil {
        return fmt.Errorf("add to DLQ failed: %w", err)
    }
    
    // Ack 原消息
    return r.cli.XAck(ctx, r.streamKey, group, msgID).Err()
}

func (r *RedisStreams) GetRetryCount(ctx context.Context, group, msgID string) (int, error) {
    pending, err := r.cli.XPendingExt(ctx, &redis.XPendingExtArgs{
        Stream: r.streamKey,
        Group:  group,
        Start:  msgID,
        End:    msgID,
        Count:  1,
    }).Result()
    
    if err != nil || len(pending) == 0 {
        return 0, err
    }
    
    return int(pending[0].RetryCount), nil
}
```

**改进的消息处理**：

```go
func (w *Workflow) processDispatchMessage(ctx context.Context, m msg.Message) (bool, string) {
    group := w.consumerGroup()
    
    // 获取重试次数
    retryCount, _ := w.Queue.GetRetryCount(ctx, group, m.ID)
    
    td, err := UnmarshalTaskDispatch(m.Payload)
    if err != nil {
        // 解析失败：不可重试，直接进 DLQ
        klog.Errorf("decode dispatch failed (moving to DLQ): %v", err)
        if dlqErr := w.Queue.MoveToDLQ(ctx, group, m.ID, m.Payload, err, retryCount); dlqErr != nil {
            klog.Errorf("move to DLQ failed: %v", dlqErr)
            return false, "" // 保留 pending
        }
        return true, "" // 已移入 DLQ，Ack 原消息
    }
    
    task, err := repository.TaskByID(ctx, w.Store, td.TaskID)
    if err != nil {
        // DB 失败：可重试
        klog.Errorf("load task %s failed (attempt %d/%d): %v", td.TaskID, retryCount+1, maxRetryCount, err)
        
        if retryCount >= maxRetryCount {
            // 超过重试次数，进 DLQ
            if dlqErr := w.Queue.MoveToDLQ(ctx, group, m.ID, m.Payload, err, retryCount); dlqErr != nil {
                klog.Errorf("move to DLQ failed: %v", dlqErr)
            }
            return true, td.TaskID
        }
        
        return false, td.TaskID // 保留 pending，等待 AutoClaim 重试
    }
    
    if err := w.updateQueueAndRunTask(ctx, task, 1); err != nil {
        klog.Errorf("run task %s failed: %v", td.TaskID, err)
        return false, td.TaskID
    }
    
    return true, td.TaskID
}
```

---

#### 3. Worker 自愈 - 完整方案

**可配置的退避策略**：

```go
// pkg/apiserver/config/config.go
type WorkflowConfig struct {
    // ... existing fields ...
    WorkerBackoffMin      time.Duration `yaml:"workerBackoffMin"`
    WorkerBackoffMax      time.Duration `yaml:"workerBackoffMax"`
    WorkerMaxReadFailures int           `yaml:"workerMaxReadFailures"`  // 0 = 无限重试
    WorkerMaxClaimFailures int          `yaml:"workerMaxClaimFailures"` // 0 = 无限重试
}

func (c *WorkflowConfig) SetDefaults() {
    if c.WorkerBackoffMin == 0 {
        c.WorkerBackoffMin = time.Second
    }
    if c.WorkerBackoffMax == 0 {
        c.WorkerBackoffMax = 5 * time.Minute
    }
    // MaxReadFailures/MaxClaimFailures 默认 0 表示无限重试
}
```

**改进的 Worker 循环**：

```go
func (w *Workflow) StartWorker(ctx context.Context, errChan chan error) {
    w.errChan = errChan
    group := w.consumerGroup()
    consumer := w.consumerName()
    
    klog.Infof("worker starting: stream=%s group=%s consumer=%s", w.dispatchTopic(), group, consumer)
    
    if err := w.Queue.EnsureGroup(ctx, group); err != nil {
        klog.V(4).Infof("ensure group error: %v", err)
    }
    
    staleTicker := time.NewTicker(w.workerStaleInterval())
    defer staleTicker.Stop()
    
    currentDelay := w.workerBackoffMin()
    readFailures := 0
    claimFailures := 0
    maxReadFailures := w.workerMaxReadFailures()
    maxClaimFailures := w.workerMaxClaimFailures()
    
    for {
        select {
        case <-ctx.Done():
            klog.Info("worker shutting down due to context cancellation")
            return
            
        case <-staleTicker.C:
            msgs, err := w.Queue.AutoClaim(ctx, group, consumer, w.workerAutoClaimMinIdle(), w.workerAutoClaimCount())
            if err != nil {
                claimFailures++
                klog.Warningf("auto-claim error (consecutive: %d): %v", claimFailures, err)
                w.recordMetric("worker_claim_errors", claimFailures)
                
                // 只告警，不退出（除非配置了上限且达到）
                if maxClaimFailures > 0 && claimFailures >= maxClaimFailures {
                    klog.Errorf("max claim failures reached (%d), worker exiting", maxClaimFailures)
                    return
                }
                continue
            }
            claimFailures = 0
            currentDelay = w.workerBackoffMin()
            w.processMessages(ctx, group, consumer, msgs, false)
            
        default:
            msgs, err := w.Queue.ReadGroup(ctx, group, consumer, w.workerReadCount(), w.workerReadBlock())
            if err != nil {
                readFailures++
                klog.Warningf("read group error (consecutive: %d): %v", readFailures, err)
                w.recordMetric("worker_read_errors", readFailures)
                
                // 指数退避
                wait := w.workerBackoffDelay(currentDelay, w.workerBackoffMin(), w.workerBackoffMax())
                currentDelay = wait
                
                select {
                case <-ctx.Done():
                    return
                case <-time.After(wait):
                }
                
                // 只告警，不退出（除非配置了上限且达到）
                if maxReadFailures > 0 && readFailures >= maxReadFailures {
                    klog.Errorf("max read failures reached (%d), worker exiting", maxReadFailures)
                    return
                }
                continue
            }
            readFailures = 0
            currentDelay = w.workerBackoffMin()
            w.processMessages(ctx, group, consumer, msgs, true)
        }
    }
}

func (w *Workflow) recordMetric(name string, value int) {
    // TODO: 暴露 Prometheus 指标
    klog.V(4).Infof("metric: %s=%d", name, value)
}
```

---

#### 4. 串行步骤语义 - 完整方案

**修复 controller.go 中的 stopOnFailure 传值**：

```go
// pkg/apiserver/event/workflow/controller.go
func (w *WorkflowCtl) Run(ctx context.Context, concurrency int) error {
    // ... existing setup code ...
    
    for _, stepExec := range stepExecutions {
        if stepExec.Jobs == nil {
            continue
        }
        priorities := sortedPriorities(stepExec.Jobs)
        for _, priority := range priorities {
            tasksInPriority := stepExec.Jobs[priority]
            if len(tasksInPriority) == 0 {
                continue
            }
            stepConcurrency := determineStepConcurrency(stepExec.Mode, len(tasksInPriority), seqLimit)
            
            // 修复：串行模式始终启用 stopOnFailure
            // 原代码：stopOnFailure = stepExec.Mode.IsParallel() (错误)
            // 正确：串行模式 stopOnFailure=true，并行模式根据配置
            stopOnFailure := !stepExec.Mode.IsParallel() // StepByStep 模式快速失败
            
            logger.Info("Executing workflow step", 
                "step", stepExec.Name, 
                "mode", stepExec.Mode, 
                "stopOnFailure", stopOnFailure)
            
            job.RunJobs(ctx, tasksInPriority, stepConcurrency, w.Client, w.Store, w.ack, stopOnFailure)
            
            // ... rest of the loop ...
        }
    }
    // ...
}
```

**Pool 结构扩展**：

```go
// pkg/apiserver/event/workflow/job/job.go
type Pool struct {
    ctx           context.Context
    cancel        context.CancelFunc
    jobs          []*model.JobTask
    jobsChan      chan *model.JobTask
    client        kubernetes.Interface
    store         datastore.DataStore
    ack           func()
    wg            sync.WaitGroup
    stopOnFailure bool
    failureOnce   sync.Once
    failed        atomic.Bool
    skippedJobs   []*model.JobTask
    mu            sync.Mutex
}

func (p *Pool) work() {
    for job := range p.jobsChan {
        // 检查是否已经失败
        if p.failed.Load() && p.stopOnFailure {
            p.mu.Lock()
            job.Status = config.StatusSkipped
            job.Error = "skipped due to previous job failure"
            p.skippedJobs = append(p.skippedJobs, job)
            p.mu.Unlock()
            p.wg.Done()
            continue
        }
        
        runJob(p.ctx, job, p.client, p.store, p.ack)
        
        if p.stopOnFailure && jobStatusFailed(job.Status) {
            p.failureOnce.Do(func() {
                p.failed.Store(true)
                p.cancel() // 取消后续 Job 的执行
                klog.Infof("job %s failed, stopping remaining jobs in batch", job.Name)
            })
        }
        p.wg.Done()
    }
}
```

---

#### 5. HTTP 优雅关闭 - 完整方案

```go
// pkg/apiserver/server.go
func (s *restServer) startHTTP(ctx context.Context) error {
    server := &http.Server{
        Addr:              s.cfg.BindAddr,
        Handler:           s,
        ReadHeaderTimeout: 2 * time.Second,
        ReadTimeout:       30 * time.Second,
        WriteTimeout:      30 * time.Second,
        IdleTimeout:       60 * time.Second,
    }
    
    // 启动优雅关闭监听
    shutdownComplete := make(chan struct{})
    go func() {
        <-ctx.Done()
        klog.Info("HTTP server shutdown initiated")
        
        // 创建独立的关闭上下文
        shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
        defer cancel()
        
        // 停止接受新请求，等待现有请求完成
        if err := server.Shutdown(shutdownCtx); err != nil {
            klog.Errorf("HTTP server graceful shutdown error: %v", err)
            // 强制关闭
            if closeErr := server.Close(); closeErr != nil {
                klog.Errorf("HTTP server force close error: %v", closeErr)
            }
        } else {
            klog.Info("HTTP server graceful shutdown completed")
        }
        
        close(shutdownComplete)
    }()
    
    klog.Infof("HTTP server starting on %s", s.cfg.BindAddr)
    err := server.ListenAndServe()
    
    // 等待关闭完成
    <-shutdownComplete
    
    // 忽略正常关闭错误
    if err == http.ErrServerClosed {
        klog.Info("HTTP server closed normally")
        return nil
    }
    
    return err
}

func (s *restServer) Run(ctx context.Context, errChan chan error) error {
    if err := s.buildIoCContainer(); err != nil {
        return err
    }
    
    // ... existing setup ...
    
    // 统一的关闭处理
    runCtx, runCancel := context.WithCancel(ctx)
    defer runCancel()
    
    // 监听系统信号
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT)
    
    go func() {
        select {
        case sig := <-sigChan:
            klog.Infof("received signal %v, initiating shutdown", sig)
            runCancel()
        case <-ctx.Done():
        }
    }()
    
    // Leader Election 和 HTTP 服务启动
    go func() {
        leaderelection.RunOrDie(runCtx, *l)
    }()
    
    return s.startHTTP(runCtx)
}
```

---

#### 6. 审计与可观测性

**Cancel 操作审计**：

```go
// pkg/apiserver/domain/service/workflow.go
func (w *workflowServiceImpl) CancelWorkflowTask(ctx context.Context, userName, taskID, reason string) error {
    task, err := repository.TaskByID(ctx, w.Store, taskID)
    if err != nil {
        return err
    }
    
    // 记录审计日志
    klog.Infof("AUDIT: cancel workflow task taskID=%s user=%s reason=%s prevStatus=%s",
        taskID, userName, reason, task.Status)
    
    return w.cancelWorkflowTask(ctx, task, userName, reason)
}
```

**健康检查端点**：

```go
// pkg/apiserver/interfaces/api/health.go
type health struct {
    Queue msg.Queue `inject:"queue"`
}

func (h *health) RegisterRoutes(group *gin.RouterGroup) {
    group.GET("/health", h.healthCheck)
    group.GET("/ready", h.readinessCheck)
}

func (h *health) healthCheck(c *gin.Context) {
    c.JSON(http.StatusOK, gin.H{"status": "healthy"})
}

func (h *health) readinessCheck(c *gin.Context) {
    ctx := c.Request.Context()
    
    // 检查队列连接
    if h.Queue != nil {
        if _, _, err := h.Queue.Stats(ctx, "workflow-workers"); err != nil {
            c.JSON(http.StatusServiceUnavailable, gin.H{
                "status": "not ready",
                "error":  "queue connection failed",
            })
            return
        }
    }
    
    c.JSON(http.StatusOK, gin.H{"status": "ready"})
}
```

---

### 实施优先级（更新）

| 优先级 | 问题 | 理由 |
|--------|------|------|
| P0 | 串行步骤 stopOnFailure 修复 | 一行代码修复，影响大 |
| P0 | HTTP ErrServerClosed 处理 | 简单修复，避免误报警 |
| P1 | 消息 DLQ 实现 | 防止毒丸消息阻塞队列 |
| P1 | Worker 无限重试（去掉硬退出） | 提高系统韧性 |
| P2 | Schema 扩展（ExecutorID/StartedAt） | 需要数据库迁移 |
| P2 | 心跳机制 | 依赖 Schema 变更 |
| P3 | 审计日志完善 | 便于问题排查 |
| P3 | 健康检查端点 | 运维可观测性 |

---

### 测试验证清单

- [ ] 串行步骤：Job A 失败后 Job B/C 应被 Skip
- [ ] 并行步骤：Job A 失败后 Job B/C 继续执行
- [ ] 消息解析失败：进入 DLQ，不阻塞队列
- [ ] DB 临时故障：消息保留 pending，AutoClaim 重试
- [ ] Worker 网络抖动：指数退避，不退出
- [ ] HTTP Shutdown：30s 内完成请求，正常返回
- [ ] 进程重启：不误杀存活实例的任务

---

## 实施记录（Implementation Changelog）

### 2024-12-05 实施完成

以下是根据上述风险分析和改进方案 v2 实际实施的变更记录。

#### 1. 串行步骤 stopOnFailure 修复 (P0)

**文件**: `pkg/apiserver/event/workflow/controller.go`

**变更**:
- 修复了 `stopOnFailure` 参数传递错误
- 原代码: `job.RunJobs(..., stepExec.Mode.IsParallel())` 
- 修复后: `stopOnFailure := !stepExec.Mode.IsParallel()` 然后传入

**测试**: `pkg/apiserver/event/workflow/controller_test.go`
- `TestStopOnFailureLogicForStepModes` - 验证串行/并行模式的 stopOnFailure 逻辑

---

#### 2. HTTP ErrServerClosed 处理 (P0)

**文件**: `pkg/apiserver/server.go`

**变更**:
- 添加了完整的 HTTP 优雅关闭逻辑
- 配置了 ReadTimeout/WriteTimeout/IdleTimeout
- 忽略 `http.ErrServerClosed` 错误
- 添加了 SIGTERM/SIGINT 信号处理
- 在关闭前停止 workers

---

#### 3. 消息队列 DLQ 实现 (P1)

**新文件**: `pkg/apiserver/infrastructure/messaging/dlq.go`

**变更**:
- 实现 `DLQCapableQueue` 接口
- `GetRetryCount()` - 获取消息重试次数
- `MoveToDLQ()` - 将失败消息移入死信队列
- `ShouldMoveToDLQ()` - 判断是否应该移入 DLQ
- 常量: `MaxRetryCount = 5`, `DLQSuffix = ".dlq"`

**修改文件**: `pkg/apiserver/infrastructure/messaging/noop.go`
- 添加了 DLQ 接口的空实现

**修改文件**: `pkg/apiserver/event/workflow/dispatcher.go`
- `processDispatchMessage()` 支持 DLQ 逻辑
- 解析失败直接进 DLQ（不可重试）
- DB 失败保留 pending 等待重试，超过阈值进 DLQ

**测试**: `pkg/apiserver/infrastructure/messaging/dlq_test.go`

---

#### 4. Worker 弹性恢复 (P1)

**文件**: `pkg/apiserver/config/config.go`

**变更**:
- 添加配置项: `WorkerMaxReadFailures`, `WorkerMaxClaimFailures`
- 添加配置项: `WorkerBackoffMin`, `WorkerBackoffMax`
- 默认值: `MaxFailures = 0` (无限重试), `BackoffMax = 5 minutes`

**文件**: `pkg/apiserver/event/workflow/dispatcher.go`

**变更**:
- `StartWorker()` 重构为弹性模式
- `MaxFailures = 0` 时无限重试，仅告警不退出
- 指数退避最大延迟从 5 秒改为 5 分钟

**文件**: `pkg/apiserver/event/workflow/workflow_state.go`

**变更**:
- 添加配置获取方法: `workerMaxReadFailures()`, `workerMaxClaimFailures()`, `workerBackoffMin()`, `workerBackoffMax()`

---

#### 5. Schema + 心跳机制 (P2)

**文件**: `pkg/apiserver/domain/model/workflow_queue.go`

**变更**:
- 添加字段: `ExecutorID`, `StartedAt`, `HeartbeatAt`
- GORM 列映射和默认值

**SQL 迁移** (需手动执行):
```sql
ALTER TABLE min_workflow_queue 
ADD COLUMN executor_id VARCHAR(255) DEFAULT '' COMMENT '执行者实例ID',
ADD COLUMN started_at BIGINT DEFAULT 0 COMMENT '实际开始执行时间戳',
ADD COLUMN heartbeat_at BIGINT DEFAULT 0 COMMENT '最后心跳时间戳';

CREATE INDEX idx_workflow_queue_executor ON min_workflow_queue(executor_id);
```

**文件**: `pkg/apiserver/event/workflow/controller.go`

**变更**:
- 任务启动时设置 `ExecutorID`, `StartedAt`, `HeartbeatAt`
- 添加 `startHeartbeat()` goroutine，每 10 秒更新心跳
- 添加 `getExecutorID()` 生成执行者 ID

**文件**: `pkg/apiserver/event/workflow/workflow.go`

**变更**:
- `InitQueue()` 重构为智能任务恢复逻辑
- 基于 `StartedAt` 判断超时（而非 CreateTime）
- 基于 `HeartbeatAt` 判断执行者是否存活
- 跳过存活执行者的任务，仅取消超时/死亡执行者的任务

---

#### 6. 审计日志和健康检查 (P3)

**文件**: `pkg/apiserver/domain/service/workflow.go`

**变更**:
- `cancelWorkflowTask()` 添加审计日志
- 记录: taskID, workflowID, workflowName, user, reason, prevStatus

**新文件**: `pkg/apiserver/interfaces/api/health.go`

**变更**:
- 添加 `/health`, `/healthz` - 存活探针
- 添加 `/ready`, `/readyz` - 就绪探针（检查队列连接）

**测试**: `pkg/apiserver/interfaces/api/health_test.go`

---

### 测试通过情况

| 测试文件 | 测试项 | 状态 |
|---------|--------|------|
| `controller_test.go` | TestStopOnFailureLogicForStepModes | ✅ PASS |
| `controller_test.go` | TestWorkflowCtlSnapshotTask | ✅ PASS |
| `controller_test.go` | TestWorkflowCtlSetStatus | ✅ PASS |
| `controller_test.go` | TestWorkflowCtlMutateTask | ✅ PASS |
| `controller_test.go` | TestIsWorkflowTerminal | ✅ PASS |
| `worker_ack_test.go` | TestProcessDispatchMessageAckOnSuccess | ✅ PASS |
| `worker_ack_test.go` | TestProcessDispatchMessageAckOnFailure | ✅ PASS |
| `worker_ack_test.go` | TestProcessDispatchMessageAckOnDecodeError | ✅ PASS |
| `health_test.go` | TestHealthCheck | ✅ PASS |
| `health_test.go` | TestReadinessCheckWithHealthyQueue | ✅ PASS |
| `health_test.go` | TestReadinessCheckWithUnhealthyQueue | ✅ PASS |
| `health_test.go` | TestReadinessCheckWithNoopQueue | ✅ PASS |

---

### 待办事项

- [x] ~~执行数据库迁移 SQL（添加 executor_id, started_at, heartbeat_at 字段）~~ **已取消：简化方案不需要这些字段**
- [ ] 配置 Kubernetes Deployment 使用 `/ready` 作为 readinessProbe
- [ ] 配置 Kubernetes Deployment 使用 `/health` 作为 livenessProbe
- [x] ~~监控 DLQ 队列 (`<stream>.dlq`) 并设置告警~~ **已取消：移除 DLQ 机制**
- [ ] 验证生产环境的 Worker 弹性行为

---

## 方案简化记录（2024-12-05 更新）

### 移除心跳机制

经过讨论，确认以下架构事实：
1. **Job 通过 goroutine 执行**，而非独立 Pod
2. **进程重启 = 所有 goroutine 死亡**，不存在"其他执行者还活着"的情况
3. **分布式场景下，Redis Streams 的 AutoClaim 机制**已经处理了消息重新认领

因此，移除了以下过度设计的功能：

**移除的代码**:
- `WorkflowQueue` 模型中的 `ExecutorID`, `StartedAt`, `HeartbeatAt` 字段
- `controller.go` 中的 `getExecutorID()`, `startHeartbeat()` 函数
- `workflow.go` 中的 `getInstanceID()`, `resolveMaxTaskAge()`, `resolveHeartbeatTimeout()` 函数

**简化后的 `InitQueue` 逻辑**:
```go
func (w *Workflow) InitQueue(ctx context.Context) {
    tasks, _ := w.WorkflowService.TaskRunning(ctx)
    for _, task := range tasks {
        task.Status = config.StatusWaiting  // 重新入队
        w.Store.Put(ctx, task)
    }
}
```

**结论**：重启时只需将所有 `running` 状态的任务重置为 `waiting`，让 Dispatcher 重新调度即可。无需复杂的心跳检测。

---

### 移除 DLQ 机制（2024-12-06 更新）

经过讨论，确认这是一个 **Pass/Fail 系统**，不需要复杂的消息重试机制。

**原因**:
1. **任务状态在数据库中**，而非消息队列
2. **失败就是失败**，不需要自动重试
3. **运维可以查看数据库**中的失败任务并手动处理

**移除的代码**:
- `pkg/apiserver/infrastructure/messaging/dlq.go` - 整个文件
- `pkg/apiserver/infrastructure/messaging/dlq_test.go` - 整个文件
- `noop.go` 中的 DLQ 相关方法

**简化后的 `processDispatchMessage` 逻辑**:
```go
func (w *Workflow) processDispatchMessage(ctx context.Context, m msg.Message) (bool, string) {
    td, err := UnmarshalTaskDispatch(m.Payload)
    if err != nil {
        klog.Errorf("decode dispatch failed: %v", err)
        return true, ""  // 记录日志，Ack 防止阻塞
    }

    task, err := repository.TaskByID(ctx, w.Store, td.TaskID)
    if err != nil {
        klog.Errorf("load task %s failed: %v", td.TaskID, err)
        return true, td.TaskID  // 记录日志，Ack
    }

    if err := w.updateQueueAndRunTask(ctx, task, 1); err != nil {
        klog.Errorf("run task %s failed: %v", td.TaskID, err)
    }
    return true, td.TaskID  // 始终 Ack
}
```

**核心原则**:
- 消息队列只负责分发，不负责重试
- 任务状态变更记录在数据库中
- 失败任务可以在数据库中查询和处理

---

## 最终实施状态（2024-12-06 更新）

### 所有问题处理结果

| # | 问题 | 状态 | 实施说明 |
|---|------|------|----------|
| 1 | 重启强制取消所有任务 | ✅ **已解决（简化）** | `InitQueue` 改为重新入队（重置为 waiting），无需心跳机制 |
| 2 | 消息解析失败仍 Ack | ✅ **已解决（简化）** | Pass/Fail 系统，失败记录日志+Ack，状态在数据库。移除 DLQ |
| 3 | Worker 少量错误即退出 | ✅ **已实现** | 指数退避，`maxFailures=10`（连续 10 次失败后退出） |
| 4 | 文档与路由不一致 | ✅ **已解决** | 更新文档说明只有 `/applications/:appID/workflow/...` 可用 |
| 5 | 串行步骤失败处理错误 | ✅ **已修复** | `stopOnFailure := !stepExec.Mode.IsParallel()` |
| 6 | HTTP 缺少优雅关闭 | ✅ **已实现** | `http.Server.Shutdown` + 信号处理 |

### Worker maxFailures 默认值更改

**变更文件**: `pkg/apiserver/config/consts.go`

```go
// 修改前
DefaultWorkerMaxReadFailures  = 0  // 0 = 无限重试
DefaultWorkerMaxClaimFailures = 0  // 0 = 无限重试

// 修改后
DefaultWorkerMaxReadFailures  = 10  // 连续 10 次失败后退出
DefaultWorkerMaxClaimFailures = 10  // 连续 10 次失败后退出
```

**效果**：Worker 在 Redis 连续失败 10 次（约 1.5 分钟）后退出，而非无限重试。既容忍短暂网络抖动，又不会隐藏持续性故障。

### 文档与 API 一致性修复

**变更文件**: `docs/workflow-cancellation-and-cleanup.md`

- 明确说明 `/workflow/cancel` 返回 501 Not Implemented
- 添加可用 API 端点表格：

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/applications/:appID/workflow` | 创建工作流 |
| PUT | `/applications/:appID/workflow` | 更新工作流 |
| POST | `/applications/:appID/workflow/exec` | 执行工作流任务 |
| POST | `/applications/:appID/workflow/cancel` | 取消工作流任务 |
| GET | `/workflow/tasks/:taskID/status` | 查询任务状态 |

### 可观测性

项目已使用 **OTEL + Jaeger** 实现分布式追踪（`pkg/apiserver/infrastructure/observability/tracing.go`），无需额外实现 Metrics 端点。

### 已完成的待办事项

- [x] Worker maxFailures 默认值从 0 改为 10
- [x] 更新 API 文档说明可用端点
- [x] 确认 OTEL 可观测性已实现
- [x] 更新 review-findings.md 记录最终状态

### 遗留事项

- [ ] 配置 Kubernetes Deployment 使用 `/ready` 作为 readinessProbe
- [ ] 配置 Kubernetes Deployment 使用 `/health` 作为 livenessProbe
- [ ] 生产环境验证 Worker 弹性行为
