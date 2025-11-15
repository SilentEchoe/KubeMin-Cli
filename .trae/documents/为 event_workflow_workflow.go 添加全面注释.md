## 目标
- 为 `pkg/apiserver/event/workflow/workflow.go` 全文件补充中文注释，覆盖包级说明、类型/函数用途、参数与返回含义、并发与状态转换、关键逻辑与异常路径。
- 保持注释与当前代码风格一致（文件内已有中英混合注释），以中文为主，导出符号采用 GoDoc 风格短句。

## 注释风格
- 包级：简述模块职责、依赖（Kubernetes/OTel/消息队列/Datastore）。
- 导出类型/函数：以名称开头的 GoDoc 风格一句或两句解释，必要时补充行为/约束。
- 非导出函数：在函数前添加简短说明；复杂分支在关键处单行注释。
- 并发/状态机：明确说明启动、停止、重试、并行/串行策略、任务状态流转。
- 日志与追踪：点明 trace/span 的用途与上下文传播。

## 修改范围
- 仅更新 `workflow.go`，不改动逻辑代码，不创建新文件。
- 为以下元素补充注释：包、常量/变量、结构体字段、所有函数与方法。

## 逐项注释清单
- 包级注释：模块概览、运行模式（本地/队列）、核心数据流。
- `type Workflow`：字段用途与依赖注入；`Start`/`InitQueue`/`WorkflowTaskSender`/`Dispatcher`：任务发现、状态迁移、队列发布。
- `TaskDispatch` 及 `Marshal/Unmarshal`：消息负载格式与用途。
- Worker 侧：`StartWorker`/`dispatchTopic`/`consumerGroup`/`consumerName`/`processDispatchMessage`：消费者组、消息处理、容错策略。
- 控制器：`type WorkflowCtl`/`NewWorkflowController`/`updateWorkflowTask`/`Run`：trace 建立、日志上下文、并发执行、失败中止与状态落库。
- 状态操作：`updateWorkflowStatus`/`mutateTask`/`snapshotTask`/`setStatus`/`isWorkflowTerminal`：线程安全与终止条件。
- 任务生成：`GenerateJobTasks`：读取工作流与组件、构造 `StepExecution`；`jobExecutionBuilder` 全部方法与字段。
- 步骤编排：`processStep`/`processSubSteps`/`processFlatStep`：并行/串行策略、命名规则。
- 执行桶：`addParallelExecution`/`addSequentialExecution`/`addExecution`：作业桶构建、统计与日志。
- 模式/命名：`normalizeWorkflowMode`/`parallelGroupName`/`sequentialStepName`：默认值与回退策略。
- 任务与运行：`NewJobTask`/`updateQueueAndRunTask`：初始状态、并发配置、异步启动。
- 属性与对象：`ParseProperties`/`CreateObjectJobsFromResult`：资源派生（PVC/Ingress/RBAC）与优先级。
- 组件编译：`appendComponentGroup`/`buildJobsForComponent`：按组件类型生成 Job（Web/Store/Conf/Secret/Service）。
- 队列服务生成：`queueServiceJobs`：附加对象先行、主服务 Job。
- 工具函数：`jobPriorityLevels`/`newJobBuckets`/`mergeJobBuckets`/`bucketsEmpty`/`countJobs`/`logGeneratedJobs`/`determineStepConcurrency`/`sortedPriorities`/`nameOrFallback`。

## 验证
- 运行代码静态检查与编译：`go build ./...`，确保新增注释不影响构建。
- 可选运行 `golangci-lint`（若仓库已配置）检查注释风格与未使用代码提示。

## 影响与风险
- 仅添加注释，无逻辑改动，风险极低。
- 注释将帮助后续维护者理解任务生命周期、并发模型与失败处理路径。