# OAM Workflow 集成方案说明

## 背景
- 用户请求模型需要引入 `workflow` 数组以声明组件的编排顺序，并支持串行/并行执行。
- 兼容现有未显式提供 workflow 的请求，保持默认顺序运行。
- 保持 Job 优先级（PVC/ConfigMap/Secret 优先）逻辑不变，避免并行阶段出现资源竞争错误。

## 核心实现
- 在 `pkg/apiserver/config/consts.go` 中新增 `WorkflowMode` 类型与 `StepByStep`/`DAG` 常量，并提供 `ParseWorkflowMode` 与 `IsParallel` 辅助方法。
- 扩展 `pkg/apiserver/interfaces/api/dto/v1/types.go`，使 `workflow` 字段可接收 `mode`、`components`、`subSteps` 等 OAM 风格参数，同时兼容旧的 `properties.policies` 写法。
- `pkg/apiserver/domain/service/application.go` 新增 `convertWorkflowStepsFromRequest`，构造统一的 `model.WorkflowSteps` 数据结构；未提供 workflow 时仍通过组件顺序自动生成。
- `model.WorkflowStep` 支持 `Mode` 和 `SubSteps` 字段，并提供 `ComponentNames` 辅助方法以解析关联组件。
- `pkg/apiserver/event/workflow/workflow.go`：
  - 引入 `StepExecution` 表示单个执行阶段，区分并行/串行。
  - `GenerateJobTasks` 按步骤构建 Job 队列，串行步骤拆分为多个 `StepExecution`，并行步骤合并执行。
  - `WorkflowCtl.Run` 按步骤依次执行，串行步骤并发度为 1，并行步骤使用 Job 数量作为并发度，同时保持优先级顺序。
  - 新增单元测试 `workflow_test.go` 覆盖默认串行与并行两种场景。

## 新的请求结构示例
```json
{
  "name": "m2507151323j3fnrk-mysql",
  "namespace": "default",
  "alias": "mysql",
  "version": "5.7.2",
  "project": "OneProject",
  "component": [
    { "name": "config", "type": "config", "replicas": 1, "properties": { "conf": { "key": "value" } } },
    { "name": "mysql", "type": "store", "replicas": 1, "image": "mysql:8", "properties": { "ports": [{ "port": 3306 }] } }
  ],
  "workflow": [
    {
      "name": "config-step",
      "mode": "StepByStep",
      "components": ["config"]
    },
    {
      "name": "database",
      "mode": "DAG",
      "components": ["mysql", "mysql-readonly"]
    }
  ]
}
```
- `mode` 取值 `StepByStep` 表示串行、`DAG` 表示并行。
- `components`/`properties.policies`/`subSteps` 三者任选其一即可表达组件集合，系统会自动去重。
- 如果 `workflow` 为空，将按组件声明顺序串行执行。

## 执行顺序与并发说明
- 工作流按步骤顺序执行；每个步骤内部仍按 Job 优先级（High → Normal → Low）分批处理。
- 串行步骤：每个组件拆分为单独步骤，并发度固定为 1。
- 并行步骤：将同一优先级的 Job 放入同一批次，并发度取该批次 Job 数量，确保并发部署。
- PVC、ConfigMap、Secret 仍归入高优先级，在任意模式下均会先于 Deployment 等资源执行。

## 测试
- `go test ./pkg/apiserver/event/workflow/...`
- `go test ./pkg/apiserver/domain/service/...`

> 说明：完整 `go test ./...` 在当前沙箱环境下因 Redis mock 绑定受限会失败，本次仅运行与改动相关的包测试。

## PR 建议流程
1. 本地创建分支（沙箱禁止直接写 `.git/refs`，实际环境可执行）：
   ```bash
   git checkout -b feature/oam-workflow
   ```
2. 执行 `go test` 并确认无误后提交：
   ```bash
   git add ...
   git commit -m "feat: add oam workflow orchestration"
   ```
3. 推送并在远端创建 PR，标题建议：`feat: add oam workflow orchestration`。
4. PR 描述中包含：
   - 需求背景与新 schema 示例
   - 并行/串行执行逻辑说明
   - 运行的测试命令
   - 风险与向后兼容性说明

## 后续可选事项
- 引入最大并发配置，避免并行步骤一次性提交过多 Job。
- 扩展 `workflow` schema，支持 OAM 的 `dependsOn`、条件语句等高级特性。
- 在 UI/CLI 增加校验与模板示例，降低复杂请求配置错误的概率。
