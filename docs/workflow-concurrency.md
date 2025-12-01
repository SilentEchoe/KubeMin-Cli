# Workflow 并发配置

串行步骤（`workflowSteps[].mode=step`）默认一次只运行一个 Job。随着组件数量增多，这类步骤会成为瓶颈。为了解决该问题，新增了 `workflow-sequential-max-concurrency` 参数，用于限制“串行步骤内部”允许同时运行的 Job 数量。

## 配置方式

- **配置文件 / 结构体**：`config.Config.Workflow.SequentialMaxConcurrency`
- **启动参数**：`--workflow-sequential-max-concurrency=<N>`
- **默认值**：`1`

示例（片段）：

```yaml
workflow:
  sequentialMaxConcurrency: 3
```

或在命令行中：

```bash
./kubemin-apiserver --workflow-sequential-max-concurrency=3
```

## 行为说明

1. 参数仅作用于串行步骤。DAG/并行步骤仍然按照 Job 数量全速并发。
2. 优先级顺序保持不变：`max-high → high → normal → low`。即使串行步骤允许并发，也会等高优先级 Job 全部完成后才开始低优先级 Job。
3. 设置值 ≤ 0 会在启动校验时被拒绝；可根据实际集群容量适当提高（常见范围 2~5）。
4. 如果某个优先级下的 Job 数量少于该值，则只会并发运行实际数量。

## 何时调整

- 串行步骤内存在多个互不依赖的组件（例如多份 ConfigMap、多个无状态服务），需要缩短该步骤耗时。
- 集群资源充裕，允许同时创建多个对象（如 Ingress、PVC）。
- 希望结合 DAG 步骤进一步提升吞吐量，但又不想重构已有串行流程。

提升并发后，请关注 Kubernetes API QPS、资源限额以及下游依赖的可承受能力，避免短时间内提交过多对象。若观察到 API 速率限制或集群压力，可适当降低该值或继续依赖默认串行执行。 

