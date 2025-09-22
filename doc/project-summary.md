# KubeMin-Cli 项目总结

## 架构总览
- 项目定位为面向开发者的 Kubernetes 应用管理平台，通过 CLI 与 API Server 统一交付和编排工作负载。
- 入口程序位于 `cmd/main.go`，先注册工作流 Trait 处理器，再启动 `cmd/server/app` 下的 API Server（Cobra 命令）。
- API Server 基于 Gin 提供 REST 接口，并通过自建 IoC 容器注入 `datastore`、`kubeClient`、`queue` 等依赖，支持按需替换实现。
- 配置结构体 `pkg/apiserver/config.Config` 统一承载服务地址、MySQL 存储、Redis 缓存、消息队列、Tracing、选主等参数，支持 CLI Flag 动态覆盖。

## 关键组件
- **领域层 (`pkg/apiserver/domain`)**：包含应用、工作流等核心模型及服务；使用 `utils` 中的随机工具生成主键，通过 `repository` 调用 `datastore` 完成持久化。
- **数据访问层 (`pkg/apiserver/infrastructure/datastore`)**：默认实现基于 GORM 的 MySQL，自动迁移已注册模型。
- **接口层 (`pkg/apiserver/interfaces/api`)**：使用 Gin 路由注册应用与工作流相关的 REST API，并提供输入校验、DTO 到模型的装配器及常用中间件（日志、Gzip、Tracing）。
- **工作流执行 (`pkg/apiserver/event/workflow`)**：
  - 领导者实例通过 Redis Streams（或本地轮询的 NoopQueue）调度 `Waiting` 状态的任务。
  - Worker 以消费组方式拿到任务、更新状态、调用 `job` 包生成/提交 Deployment、Service、PVC 等 Kubernetes 资源。
  - 支持 AutoClaim 接管长时间未确认的消息，并输出队列 backlog/pending 日志指标。
- **Trait 体系 (`pkg/apiserver/workflow/traits`)**：对组件启用存储、资源、探针、环境变量等特性，工作流构建 Job 时读取这些扩展配置。

## 运行时特性
- 通过 `leaderelection` + Lease 实现主选举，启动时若检测到偶数副本会主动退出以保持奇数副本数。
- `pkg/apiserver/utils` 提供缓存、日志清理、PVC 工具、随机字符串生成等辅助能力。
- 支持 Jaeger 分布式追踪初始化（`pkg/apiserver/infrastructure/observability`），并在 HTTP 层注入 `otelgin` 中间件。

## 开发与测试
- 常用命令：`go run ./cmd/main.go` 启动本地服务；`make build-apiserver` 交叉编译；`go test ./... -race -cover` 运行测试。
- 项目内已有针对 Redis 队列、Trait 处理、Job 生成等模块的单元测试，遵循标准 `testing` + `testify` 组合。
