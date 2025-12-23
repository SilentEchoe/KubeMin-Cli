[English](./README.md) | [简体中文](./README_zh.md)

# KubeMin-Cli

**面向开发者的 Kubernetes 应用平台**

KubeMin-Cli 是一个基于云原生的 PaaS 平台，以轻量化工作流的方式来描述应用，结构模型基于 **OAM（开放应用模型）**，原生设计的工作流作为核心驱动应用的完整生命周期：初始化资源、创建/更新工作负载、发布事件、监听状态等。

## 核心能力

### 1. Traits：可组合的能力原子

Traits 结构将 Kubernetes 中的 Pod、Service、Volume 等底层概念抽象化，将存储、容器边车、环境变量、初始化、密钥等以 Trait 的方式定义，使组件不再依赖复杂的 YAML，而是用组合的方式来构建复杂应用。还提供了"引入模板"功能，能快速创建多个相同形态的新应用（如多套 MySQL），以此来保证生成的资源可用、无冲突、可追溯。

### 2. Workflow：任务驱动的应用生命周期

每次"部署""更新""扩缩容"等操作都被抽象成一条工作流实例：支持并行任务、支持状态一致性、组件之间的依赖、持久化执行记录等。并在基础设施层使用 List-Watch 模式来维护应用状态的整个生命周期，实时同步应用组件之间的状态。

## 架构概览

### 工作流引擎

KubeMin-Cli 工作流引擎是整个应用交付系统的核心组件，负责将声明的应用配置转换为实际运行在 Kubernetes 集群上的资源。它充当了"编排者"的角色，协调多个组件的创建、更新和删除操作，确保应用的正确部署。

#### 运行模式

1. **本地模式**：使用 NoopQueue，直接扫描数据库执行任务，适用于单实例部署、开发测试。
2. **分布式模式**：使用 Redis Streams、Kafka 等；支持任务分发和故障恢复，适用于多实例部署、生产环境。

#### 关键特性

- **资源依赖管理**：确保 ConfigMap、Secret、PVC 等依赖资源先于 Deployment、StatefulSet 创建
- **执行顺序控制**：支持串行和并行两种执行模式，满足不同场景需求
- **状态追踪**：完整记录每个任务和 Job 的执行状态，便于问题排查
- **故障恢复**：支持任务重试、取消和资源清理，保证系统一致性
- **分布式扩展**：支持多实例部署，通过 Redis Streams/Kafka 实现任务分发

#### 工作流定义示例

```json
{
  "workflow": [
    {
      "name": "config-step",
      "mode": "StepByStep",
      "components": ["config", "secret"]
    },
    {
      "name": "database",
      "mode": "DAG",
      "components": ["mysql", "redis"]
    },
    {
      "name": "services",
      "mode": "StepByStep",
      "components": ["backend", "frontend"]
    }
  ]
}
```

### OAM Traits

KubeMin-Cli 提供了全面的 Traits 集合来增强组件功能：

#### 可用 Traits

| Trait | 说明 | K8s 资源 |
|-------|------|----------|
| Storage | 存储挂载 | PVC、EmptyDir、ConfigMap、Secret Volume |
| Init | 初始化容器 | InitContainer |
| Sidecar | 边车容器 | Container |
| Envs | 单个环境变量 | EnvVar |
| EnvFrom | 批量环境变量导入 | EnvFromSource |
| Probes | 健康检查探针 | LivenessProbe、ReadinessProbe、StartupProbe |
| Resources | 计算资源限制 | ResourceRequirements |
| Ingress | 入口流量路由 | Ingress |
| RBAC | 权限访问控制 | ServiceAccount、Role、RoleBinding、ClusterRole、ClusterRoleBinding |

#### Trait 处理顺序

1. Storage（存储）
2. EnvFrom（批量环境变量）
3. Envs（环境变量）
4. Resources（资源限制）
5. Probes（健康探针）
6. RBAC（权限控制）
7. Init（初始化容器）
8. Sidecar（边车容器）
9. Ingress（入口流量）

## 快速开始

### 前置要求

- Kubernetes 集群（v1.20+）
- MySQL 数据库
- Redis（分布式模式需要）

### 安装

```bash
# 克隆仓库
git clone https://github.com/your-org/KubeMin-Cli.git
cd KubeMin-Cli

# 构建二进制文件
go build -o kubemin-cli cmd/main.go

# 运行服务器
./kubemin-cli
```

### 部署第一个应用

```bash
# 创建应用
curl -X POST http://localhost:8080/applications \
  -H "Content-Type: application/json" \
  -d '{
    "name": "demo-app",
    "namespace": "default",
    "component": [
      {
        "name": "web",
        "type": "webservice",
        "image": "nginx:latest",
        "replicas": 2,
        "properties": {
          "ports": [{"port": 80, "expose": true}]
        }
      }
    ],
    "workflow": [
      {
        "name": "deploy",
        "mode": "StepByStep",
        "components": ["web"]
      }
    ]
  }'
```

## 配置

### 环境变量

- `MYSQL_DSN`：MySQL 连接字符串
- `REDIS_ADDR`：Redis 服务器地址（分布式模式）
- `KUBECONFIG`：kubeconfig 文件路径

### 工作流引擎配置

| 参数 | 默认值 | 说明 |
|-----------|---------|-------------|
| `--workflow-sequential-max-concurrency` | 1 | 串行步骤内部最大并发数 |
| `--workflow-max-concurrent` | 10 | 最大并发工作流数 |
| `--msg-type` | redis | 消息队列类型（noop/redis/kafka） |

## 开发

### 构建

```bash
# 为当前系统构建
go build -o kubemin-cli cmd/main.go

# 构建 Linux 版本
make build-linux

# 构建 macOS 版本
make build-darwin

# 构建 Windows 版本
make build-windows
```

### 测试

```bash
# 运行所有测试（带竞态检测和覆盖率）
go test ./... -race -cover

# 运行特定包的测试
go test ./pkg/apiserver/workflow/... -v
```

### 代码质量

```bash
# 格式化代码
go fmt ./...

# 运行静态分析
go vet ./...

# 整理依赖
go mod tidy
```

## 架构详情

### 整洁架构结构

```
pkg/apiserver/
├── domain/           # 业务逻辑和领域模型
│   ├── model/        # 领域实体
│   ├── service/      # 业务逻辑服务
│   └── repository/   # 数据访问接口
├── infrastructure/   # 外部集成
│   ├── persistence/  # 数据库层（GORM + MySQL）
│   ├── messaging/    # 队列实现
│   ├── kubernetes/   # K8s 客户端和工具
│   └── tracing/      # OpenTelemetry 集成
├── interfaces/api/   # REST API 层
│   ├── handlers/     # HTTP 请求处理器
│   └── middleware/   # Gin 中间件
├── workflow/         # 工作流执行引擎
│   ├── dispatcher/   # 任务分发逻辑
│   ├── worker/       # 任务执行工作器
│   └── traits/       # 组件特征处理器
└── utils/            # 共享工具
```

### 关键模式

1. **依赖注入**：自定义 IoC 容器管理服务生命周期
2. **队列抽象**：统一接口支持 Redis Streams 和本地队列
3. **特征系统**：通过特征处理器实现可扩展的组件增强
4. **领导者选举**：基于 Kubernetes Lease 的分布式模式领导者选举

## 贡献

我们欢迎贡献！请查看 [CONTRIBUTING.md](CONTRIBUTING.md) 了解指南。

## 许可证

本项目采用 Apache License 2.0 许可证 - 详情请见 [LICENSE](LICENSE) 文件。
