# Workflow Task De-duplication Proposal

Goal: avoid the same workflow task being executed twice across dispatcher/worker instances while keeping Job-level idempotency as a safety net.

## Pain points
- `UpdateTaskStatus` does a Get + Put without CAS; multiple dispatchers can flip a `waiting` task to `queued` concurrently.
- `processDispatchMessage` runs tasks regardless of current status; repeated Redis deliveries still execute.
- `InitQueue` rewrites all `running` tasks to `waiting`; concurrent boots can double-queue tasks.

## Proposed mitigations
1) **DB CAS on state transitions**
   - Add `version INT DEFAULT 1` to `workflow_queue`.
   - Repository adds `UpdateTaskStatusCAS(ctx, store, taskID, from, to)` using a single conditional UPDATE (e.g. `WHERE task_id=? AND status=?` or `AND version=?`, then `version=version+1`). Return `RowsAffected==1` as success.
   - Replace dispatcher `markTaskStatus` usage with CAS so only one actor claims a task.

2) **Worker-side status guard**
   - After loading the task in `processDispatchMessage`, proceed only when `task.Status==queued` (optionally allow `waiting` if you want auto-claim to re-queue). Otherwise log and ack to drain duplicate messages.

3) **InitQueue coordination**
   - Wrap `InitQueue` with a short TTL distributed lock (Redis `SET key val NX EX`) or leader-only flag to prevent multiple instances from re-queuing the same tasks.

4) **Optional stream de-dup**
   - Relying on CAS + status check is usually enough. If needed, use `taskID` as the Redis Stream entry ID or keep a short-lived processed set for belt-and-suspenders.

## Change sketch
- DDL: `ALTER TABLE workflow_queue ADD COLUMN version INT DEFAULT 1;`
- Repository: new `UpdateTaskStatusCAS` helper with conditional UPDATE + version bump.
- Dispatcher: use CAS in `claimAndProcessTask`.
- Worker: status check before `updateQueueAndRunTask`.
- Init: add best-effort distributed lock helper around `InitQueue`.

## Risks / notes
- Requires SQL datastore; CAS helper should no-op or fall back gracefully on non-SQL drivers.
- Ensure DDL runs before rollout; version bump is backward compatible (`DEFAULT 1`).
- Status machine is still permissive; consider centralizing allowed transitions later.
