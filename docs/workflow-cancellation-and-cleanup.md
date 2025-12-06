# 工作流取消与资源清理实现说明

## 背景

在部署任务超时或用户主动取消时，数据库中的任务状态会更新，但 Kubernetes 中已经创建的资源不会回滚，导致系统状态不一致。本次改动实现了失败清理、Redis 取消信号以及对外取消接口，确保工作流执行的可控性和一致性。

## 资源清理机制

- 引入上下文级别的清理跟踪器（`WithCleanupTracker`），每个 Job 执行都会记录本次新建的资源。
- 各 Job 控制器在创建 Deployment/StatefulSet/Service/PVC/ConfigMap/Secret 时调用 `MarkResourceCreated`，更新则标记为 `Observed`，避免误删既有资源。
- `Clean(ctx)` 钩子在 Job 失败、超时、panic 时触发，通过跟踪信息删除对应资源，并忽略 `NotFound` 错误以保证幂等。
- `StatusError` 类型封装错误与目标状态（如 `timeout`、`cancelled`），`runJob` 根据错误统一更新状态并决定是否执行清理。

## Redis 取消信号

- 新增 `pkg/apiserver/workflow/signal` 包，用 Redis key (`kubemin:workflow:cancel:<taskID>`) 表示运行中的工作流任务。
- Worker 启动时调用 `signal.Watch`：
  - 通过 `SETNX` 声明执行权，并周期性刷新 TTL。
  - 侦测 key 被删除或值被改写（取消标记），自动触发 `context.Cancel`，同时记录取消原因。
- 服务端调用 `signal.Cancel` 将 key 改写为 `cancelled:<reason>`，worker 能立即收到信号。
- 如果未配置 Redis，Watcher 自动退化为本地模式（不影响执行，但无法跨实例取消）。

## 对外取消接口

> **注意**：`/workflow/cancel` 目前返回 501 Not Implemented。请使用以下应用级接口：

- `POST /applications/:appID/workflow/cancel` 允许指定 `taskId`、可选 `user` 与 `reason`。
- API 默认使用 `config.DefaultTaskRevoker` 作为用户，调用 `WorkflowService.CancelWorkflowTaskForApp`。
- Service 层同时更新数据库任务状态和 Redis 取消信号，failure 会返回给调用方。

### 可用的工作流接口

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/applications/:appID/workflow` | 创建工作流 |
| PUT | `/applications/:appID/workflow` | 更新工作流 |
| POST | `/applications/:appID/workflow/exec` | 执行工作流任务 |
| POST | `/applications/:appID/workflow/cancel` | 取消工作流任务 |
| GET | `/workflow/tasks/:taskID/status` | 查询任务状态 |

## 关键代码路径

- 清理追踪：`pkg/apiserver/event/workflow/job/cleanup_tracker.go`
- 运行时取消：`pkg/apiserver/event/workflow/job/job.go`、`pkg/apiserver/workflow/signal`
- 控制器更新：`job_deploy.go`、`job_statefulset.go`、`job_service.go`、`job_pvc.go`、`job_configmap.go`、`job_secret.go`
- 服务接口：`pkg/apiserver/domain/service/workflow.go`
- API 路由：`pkg/apiserver/interfaces/api/workflow.go`

## 测试覆盖

- `cleanup_tracker_test.go` 验证资源标记与清理能力。
- `cancel_test.go` 使用 `miniredis` 验证 Watch/Cancel 行为及取消原因。
- `workflow_test.go` 针对 `/workflow/cancel` 接口做 JSON 结构校验。
- 运行 `go test ./...` 在受限沙箱中会触发 Redis mock 端口绑定失败；需在具备本地监听权限的环境运行以获得完整测试结果。

## 注意事项

- 依赖 Redis Keyspace 变更的感知，通过轮询实现，无需额外配置通知。
- 若需与旧版本兼容，请确保在发布前清理遗留的 `kubemin:workflow:cancel:*` key。
- Cleanup 过程设置了 30s 超时，避免磁盘或网络异常导致 worker 长时间阻塞。
