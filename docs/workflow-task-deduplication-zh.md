# 工作流任务去重方案

目标：在已有 Job 层幂等的基础上，避免同一个 workflow task 被调度/worker 运行两次。

## 痛点
- `UpdateTaskStatus` 是 Get+Put，没有 CAS；多个 dispatcher 可以同时把 `waiting` 改成 `queued`。
- `processDispatchMessage` 不看当前状态，Redis 重投也会继续执行。
- `InitQueue` 把所有 `running` 改回 `waiting`，多实例同时启动可能重复入队。

## 方案
1) **数据库状态 CAS**
   - 在 `workflow_queue` 增加 `version INT DEFAULT 1`。
   - 仓储新增 `UpdateTaskStatusCAS(ctx, store, taskID, from, to)`，用单条条件 UPDATE（`WHERE task_id=? AND status=?`，可选 `AND version=?`），并 `version=version+1`，`RowsAffected==1` 视为成功。
   - dispatcher 的 `markTaskStatus` 改用 CAS，确保只有一个实例 claim。

2) **Worker 状态校验**
   - `processDispatchMessage` 读出任务后，仅当 `task.Status==queued`（如需允许 requeue，可放宽到 `waiting`）才继续；否则日志+ack 吸收重复消息。

3) **InitQueue 协调**
   - 用短 TTL 的分布式锁（Redis `SET key val NX EX`）或只由 leader 执行，避免多实例重复 requeue。

4) **可选流去重**
   - CAS + 状态检查通常足够；如需更严，可用 `taskID` 作为 Redis Stream entry ID，或维护短暂的 processed set。

## 变更速览
- DDL：`ALTER TABLE workflow_queue ADD COLUMN version INT DEFAULT 1;`
- 仓储：新增 `UpdateTaskStatusCAS`（条件 UPDATE + version 自增）。
- Dispatcher：`claimAndProcessTask` 改用 CAS。
- Worker：执行前增加状态检查。
- Init：给 `InitQueue` 加分布式锁的 best-effort。

## 风险/注意
- 依赖 SQL datastore；CAS helper 需对非 SQL 驱动优雅降级或提前检测。
- 先执行 DDL；`DEFAULT 1` 与旧数据兼容。
- 状态机仍较宽松，后续可集中限制合法迁移。
