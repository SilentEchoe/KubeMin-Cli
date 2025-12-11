# KubeMin-Cli 架构图

本文档通过 Mermaid 图表可视化 KubeMin-Cli 的系统架构、组件关系和数据流。

## 目录

- [1. 系统整体架构](#1-系统整体架构)
- [2. 分层架构](#2-分层架构)
- [3. 核心组件关系](#3-核心组件关系)
- [4. 工作流引擎架构](#4-工作流引擎架构)
- [5. 数据模型关系](#5-数据模型关系)
- [6. API 请求处理流程](#6-api-请求处理流程)
- [7. 工作流执行流程](#7-工作流执行流程)
- [8. Leader 选举与高可用](#8-leader-选举与高可用)
- [9. Traits 处理流程](#9-traits-处理流程)
- [10. 部署架构](#10-部署架构)

---

## 1. 系统整体架构

KubeMin-Cli 作为 Kubernetes 应用管理平台，与多个外部系统交互：

```mermaid
graph TB
    subgraph ExternalClients [外部客户端]
        CLI[CLI 工具]
        WebUI[Web UI]
        API_Client[API 客户端]
    end

    subgraph KubeMinCli [KubeMin-Cli API Server]
        RestAPI[REST API<br/>Gin Framework]
        WorkflowEngine[工作流引擎]
        TraitsProcessor[Traits 处理器]
        IoC[IoC 容器<br/>依赖注入]
    end

    subgraph ExternalSystems [外部系统]
        K8s[Kubernetes Cluster<br/>Deployments, Services,<br/>ConfigMaps, Secrets, PVCs]
        MySQL[(MySQL<br/>持久化存储)]
        Redis[(Redis<br/>缓存 + 消息队列)]
        Jaeger[Jaeger<br/>分布式追踪]
    end

    CLI --> RestAPI
    WebUI --> RestAPI
    API_Client --> RestAPI

    RestAPI --> IoC
    IoC --> WorkflowEngine
    IoC --> TraitsProcessor

    WorkflowEngine --> K8s
    TraitsProcessor --> K8s

    IoC --> MySQL
    IoC --> Redis
    RestAPI --> Jaeger
    WorkflowEngine --> Jaeger
```

### 系统交互说明

| 外部系统 | 用途 | 协议 |
|----------|------|------|
| Kubernetes | 部署和管理工作负载 | client-go |
| MySQL | 存储应用、工作流、组件等数据 | GORM |
| Redis | 分布式缓存 + Redis Streams 消息队列 | go-redis |
| Jaeger | 分布式追踪收集 | OpenTelemetry |

---

## 2. 分层架构

项目采用 DDD（领域驱动设计）风格的分层架构：

```mermaid
graph TB
    subgraph Layer1 [接口层 - interfaces/api]
        API_App[ApplicationsAPI]
        API_WF[WorkflowAPI]
        Middleware[中间件<br/>CORS, Logging, Gzip, Tracing]
        DTO[DTO/Assembler<br/>数据传输对象]
    end

    subgraph Layer2 [领域层 - domain]
        subgraph Services [服务 - service]
            AppService[ApplicationService]
            WFService[WorkflowService]
        end
        subgraph Models [模型 - model]
            AppModel[Applications]
            CompModel[ApplicationComponent]
            WFModel[Workflow]
            QueueModel[WorkflowQueue]
        end
        subgraph Repos [仓储 - repository]
            AppRepo[ApplicationRepository]
            WFRepo[WorkflowRepository]
        end
    end

    subgraph Layer3 [事件层 - event]
        WFEngine[Workflow Engine]
        JobCtl[Job Controllers]
        Signal[Cancel Signal]
    end

    subgraph Layer4 [基础设施层 - infrastructure]
        DataStore[DataStore<br/>MySQL/GORM]
        Messaging[Messaging<br/>Redis Streams]
        Clients[Clients<br/>KubeClient, RedisClient]
        Observability[Observability<br/>Tracing]
    end

    API_App --> AppService
    API_WF --> WFService
    
    AppService --> AppRepo
    WFService --> WFRepo
    AppService --> Models
    WFService --> Models

    AppRepo --> DataStore
    WFRepo --> DataStore

    WFEngine --> Messaging
    WFEngine --> Clients
    JobCtl --> Clients

    Middleware --> Observability
```

### 层级职责

| 层级 | 目录 | 职责 |
|------|------|------|
| 接口层 | `interfaces/api/` | HTTP 路由、请求验证、响应序列化 |
| 领域层 | `domain/` | 业务逻辑、领域模型、数据仓储 |
| 事件层 | `event/` | 工作流调度、任务执行、事件处理 |
| 基础设施层 | `infrastructure/` | 数据库、缓存、消息队列、外部客户端 |

---

## 3. 核心组件关系

展示主要组件之间的依赖和调用关系：

```mermaid
graph LR
    subgraph API [API 层]
        AppAPI[ApplicationsAPI]
        WFAPI[WorkflowAPI]
    end

    subgraph Service [服务层]
        AppSvc[ApplicationService]
        WFSvc[WorkflowService]
    end

    subgraph Workflow [工作流引擎]
        Dispatcher[Dispatcher]
        Worker[Worker]
        Controller[WorkflowController]
    end

    subgraph Jobs [Job 控制器]
        DeployJob[DeployJobCtl]
        StoreJob[StatefulSetJobCtl]
        ServiceJob[ServiceJobCtl]
        PVCJob[PVCJobCtl]
        ConfigJob[ConfigMapJobCtl]
        SecretJob[SecretJobCtl]
        IngressJob[IngressJobCtl]
        RBACJob[RBACJobCtl]
    end

    subgraph Traits [Traits 处理器]
        StorageTrait[Storage]
        ProbeTrait[Probes]
        ResourceTrait[Resources]
        EnvTrait[Envs/EnvFrom]
        InitTrait[Init]
        SidecarTrait[Sidecar]
        IngressTrait[Ingress]
        RBACTrait[RBAC]
    end

    AppAPI --> AppSvc
    WFAPI --> WFSvc

    AppSvc --> WFSvc
    WFSvc --> Dispatcher

    Dispatcher --> Worker
    Worker --> Controller

    Controller --> DeployJob
    Controller --> StoreJob
    Controller --> ServiceJob
    Controller --> PVCJob
    Controller --> ConfigJob
    Controller --> SecretJob
    Controller --> IngressJob
    Controller --> RBACJob

    DeployJob --> StorageTrait
    DeployJob --> ProbeTrait
    DeployJob --> ResourceTrait
    DeployJob --> EnvTrait
    DeployJob --> InitTrait
    DeployJob --> SidecarTrait
    DeployJob --> IngressTrait
    DeployJob --> RBACTrait
```

---

## 4. 工作流引擎架构

工作流引擎的内部组件和执行模式：

```mermaid
graph TB
    subgraph WorkflowEngine [工作流引擎]
        subgraph Scheduler [调度器]
            Workflow[Workflow<br/>入口调度器]
            Dispatcher[Dispatcher<br/>任务分发]
            TaskSender[TaskSender<br/>本地模式]
        end

        subgraph Executor [执行器]
            Worker[Worker<br/>消息消费者]
            Controller[WorkflowController<br/>任务控制器]
            JobBuilder[JobBuilder<br/>Job 生成器]
        end

        subgraph JobControllers [Job 控制器]
            Deploy[DeployJobCtl<br/>Deployment]
            Store[StatefulSetJobCtl<br/>StatefulSet]
            Svc[ServiceJobCtl<br/>Service]
            PVC[PVCJobCtl<br/>PVC]
            CM[ConfigMapJobCtl<br/>ConfigMap]
            Secret[SecretJobCtl<br/>Secret]
            Ing[IngressJobCtl<br/>Ingress]
            RBAC[RBACJobCtl<br/>RBAC资源]
        end

        subgraph Support [支撑组件]
            Signal[CancelWatcher<br/>取消信号]
            Cleanup[CleanupTracker<br/>资源清理]
            Pool[Job Pool<br/>并发控制]
        end
    end

    subgraph Queue [消息队列]
        Redis[Redis Streams<br/>分布式模式]
        Noop[NoopQueue<br/>本地模式]
    end

    subgraph Storage [存储]
        DB[(MySQL<br/>任务状态)]
    end

    Workflow --> Dispatcher
    Workflow --> TaskSender

    Dispatcher --> Redis
    TaskSender --> DB

    Redis --> Worker
    Worker --> Controller

    Controller --> JobBuilder
    JobBuilder --> Deploy
    JobBuilder --> Store
    JobBuilder --> Svc
    JobBuilder --> PVC
    JobBuilder --> CM
    JobBuilder --> Secret
    JobBuilder --> Ing
    JobBuilder --> RBAC

    Controller --> Signal
    Deploy --> Cleanup
    Store --> Cleanup
    Controller --> Pool

    Controller --> DB
```

### 双模式执行

```mermaid
flowchart LR
    subgraph LocalMode [本地模式]
        L1[TaskSender] --> L2[轮询 DB]
        L2 --> L3[直接执行]
    end

    subgraph DistributedMode [分布式模式]
        D1[Dispatcher] --> D2[Redis Streams]
        D2 --> D3[Worker 消费]
        D3 --> D4[执行任务]
    end

    Config{配置检测} --> |msg-type=noop| LocalMode
    Config --> |msg-type=redis| DistributedMode
```

---

## 5. 数据模型关系

核心数据模型的 ER 关系图：

```mermaid
erDiagram
    Applications ||--o{ ApplicationComponent : contains
    Applications ||--o{ Workflow : has
    Workflow ||--o{ WorkflowQueue : generates
    WorkflowQueue ||--o{ JobInfo : tracks

    Applications {
        string ID PK
        string Name
        string Namespace
        string Version
        string Alias
        string Project
        string Description
        bool TmpEnable
        timestamp CreateTime
        timestamp UpdateTime
    }

    ApplicationComponent {
        int ID PK
        string AppID FK
        string Name
        string Namespace
        string Image
        int32 Replicas
        string ComponentType
        json Properties
        json Traits
    }

    Workflow {
        string ID PK
        string Name
        string Namespace
        string AppID FK
        string ProjectID
        string WorkflowType
        string Status
        json Steps
    }

    WorkflowQueue {
        string TaskID PK
        string WorkflowID FK
        string WorkflowName
        string AppID
        string ProjectID
        string Status
        string TaskCreator
        string TaskRevoker
        string Type
    }

    JobInfo {
        int ID PK
        string TaskID FK
        string WorkflowID
        string AppID
        string Type
        string ServiceName
        string Status
        int64 StartTime
        int64 EndTime
        string Error
    }
```

### 状态流转

```mermaid
stateDiagram-v2
    [*] --> waiting: 创建任务

    waiting --> queued: Dispatcher 认领
    queued --> running: Worker 开始执行
    
    running --> completed: 所有 Job 成功
    running --> failed: Job 执行失败
    running --> timeout: 执行超时
    running --> cancelled: 用户取消

    queued --> waiting: 分发失败回滚

    completed --> [*]
    failed --> [*]
    timeout --> [*]
    cancelled --> [*]
```

---

## 6. API 请求处理流程

一个典型的应用创建请求的完整处理流程：

```mermaid
sequenceDiagram
    autonumber
    participant Client as 客户端
    participant Gin as Gin Router
    participant MW as 中间件
    participant API as ApplicationsAPI
    participant Svc as ApplicationService
    participant Repo as Repository
    participant DB as MySQL
    participant WF as WorkflowService

    Client->>Gin: POST /api/v1/applications
    Gin->>MW: 请求处理
    MW->>MW: CORS / Logging / Tracing
    MW->>API: 路由分发

    API->>API: 参数绑定 & 验证
    API->>Svc: CreateApplication()

    Svc->>Svc: 生成 AppID
    Svc->>Repo: CreateApplication()
    Repo->>DB: INSERT applications

    loop 每个组件
        Svc->>Svc: 解析 Traits
        Svc->>Repo: CreateComponent()
        Repo->>DB: INSERT components
    end

    Svc->>WF: CreateWorkflow()
    WF->>Repo: CreateWorkflow()
    Repo->>DB: INSERT workflow

    Svc-->>API: 返回 AppID, WorkflowID
    API-->>Client: 200 OK + JSON Response
```

---

## 7. 工作流执行流程

从触发工作流到 Kubernetes 资源部署的完整流程：

```mermaid
sequenceDiagram
    autonumber
    participant Client as 客户端
    participant API as WorkflowAPI
    participant Svc as WorkflowService
    participant DB as MySQL
    participant Dispatcher as Dispatcher
    participant Redis as Redis Streams
    participant Worker as Worker
    participant Controller as WorkflowController
    participant JobBuilder as JobBuilder
    participant JobCtl as JobController
    participant K8s as Kubernetes

    Client->>API: POST /workflow/exec
    API->>Svc: ExecWorkflowTask()
    Svc->>DB: INSERT workflow_queue<br/>status=waiting
    Svc-->>Client: 返回 TaskID

    loop 定时轮询
        Dispatcher->>DB: 查询 status=waiting
        Dispatcher->>DB: CAS 更新 waiting→queued
        Dispatcher->>Redis: XADD 任务消息
    end

    Worker->>Redis: XREADGROUP 消费消息
    Worker->>DB: 更新 status=running
    Worker->>Controller: 执行工作流

    Controller->>JobBuilder: GenerateJobTasks()
    JobBuilder->>DB: 加载 Workflow 定义
    JobBuilder->>DB: 加载 Components
    JobBuilder-->>Controller: 返回 StepExecutions

    loop 按 Priority 执行
        Controller->>JobCtl: RunJobs()
        JobCtl->>K8s: Create/Update 资源
        JobCtl->>JobCtl: 等待资源就绪
        JobCtl->>DB: 保存 JobInfo
    end

    Controller->>DB: 更新 status=completed
    Worker->>Redis: XACK 确认消息
```

### Job 优先级执行顺序

```mermaid
flowchart TB
    subgraph Priority0 [Priority 0 - MaxHigh]
        P0[ConfigMap<br/>Secret]
    end

    subgraph Priority1 [Priority 1 - High]
        P1[PVC<br/>ServiceAccount<br/>Role/ClusterRole<br/>RoleBinding]
    end

    subgraph Priority10 [Priority 10 - Normal]
        P10[Deployment<br/>StatefulSet<br/>Service<br/>Ingress]
    end

    subgraph Priority20 [Priority 20 - Low]
        P20[清理任务<br/>通知任务]
    end

    P0 --> P1 --> P10 --> P20
```

---

## 8. Leader 选举与高可用

多实例部署时的 Leader 选举和角色分配：

```mermaid
graph TB
    subgraph Cluster [KubeMin 集群]
        subgraph Instance1 [实例 1 - Leader]
            L_API1[REST API]
            L_Dispatcher[Dispatcher<br/>任务分发]
            L_Worker1[Worker]
        end

        subgraph Instance2 [实例 2 - Follower]
            F_API2[REST API]
            F_Worker2[Worker]
        end

        subgraph Instance3 [实例 3 - Follower]
            F_API3[REST API]
            F_Worker3[Worker]
        end
    end

    subgraph External [外部依赖]
        K8s_Lease[K8s Lease<br/>选举锁]
        Redis_Q[Redis Streams<br/>任务队列]
        MySQL_DB[(MySQL<br/>共享存储)]
    end

    L_API1 --> K8s_Lease
    F_API2 --> K8s_Lease
    F_API3 --> K8s_Lease

    L_Dispatcher --> Redis_Q
    L_Worker1 --> Redis_Q
    F_Worker2 --> Redis_Q
    F_Worker3 --> Redis_Q

    L_API1 --> MySQL_DB
    F_API2 --> MySQL_DB
    F_API3 --> MySQL_DB
```

### Leader 选举流程

```mermaid
sequenceDiagram
    autonumber
    participant I1 as 实例1
    participant I2 as 实例2
    participant I3 as 实例3
    participant Lease as K8s Lease

    I1->>Lease: 尝试获取 Lease
    I2->>Lease: 尝试获取 Lease
    I3->>Lease: 尝试获取 Lease

    Lease-->>I1: 获取成功 (Leader)
    Lease-->>I2: 获取失败 (Follower)
    Lease-->>I3: 获取失败 (Follower)

    Note over I1: 启动 Dispatcher
    Note over I1,I3: 所有实例启动 Worker

    loop 续约
        I1->>Lease: 续约 Lease
    end

    Note over I1: Leader 故障

    I2->>Lease: 尝试获取 Lease
    Lease-->>I2: 获取成功 (新 Leader)
    Note over I2: 启动 Dispatcher
```

### 角色职责

| 角色 | 职责 | 数量 |
|------|------|------|
| Leader | 运行 Dispatcher 分发任务 | 1 |
| Follower | 只运行 Worker 消费任务 | N-1 |
| Worker | 消费队列、执行工作流 | 所有实例 |

---

## 9. Traits 处理流程

Traits 如何被应用到 Kubernetes 资源：

```mermaid
flowchart TB
    subgraph Input [输入]
        Component[ApplicationComponent<br/>组件定义]
        TraitsConfig[Traits 配置<br/>JSON]
    end

    subgraph TraitsProcessor [Traits 处理器]
        Parser[配置解析器]
        
        subgraph Handlers [Trait 处理器]
            StorageH[Storage Handler<br/>存储挂载]
            ProbeH[Probe Handler<br/>健康探针]
            ResourceH[Resource Handler<br/>资源限制]
            EnvH[Env Handler<br/>环境变量]
            InitH[Init Handler<br/>初始化容器]
            SidecarH[Sidecar Handler<br/>边车容器]
            IngressH[Ingress Handler<br/>入口流量]
            RBACH[RBAC Handler<br/>权限控制]
        end
    end

    subgraph Output [输出]
        Deploy[Deployment/<br/>StatefulSet]
        Svc[Service]
        PVC[PVC]
        CM[ConfigMap]
        Secret[Secret]
        Ing[Ingress]
        SA[ServiceAccount]
        Role[Role/ClusterRole]
        Binding[RoleBinding]
    end

    Component --> Parser
    TraitsConfig --> Parser

    Parser --> StorageH
    Parser --> ProbeH
    Parser --> ResourceH
    Parser --> EnvH
    Parser --> InitH
    Parser --> SidecarH
    Parser --> IngressH
    Parser --> RBACH

    StorageH --> Deploy
    StorageH --> PVC
    ProbeH --> Deploy
    ResourceH --> Deploy
    EnvH --> Deploy
    InitH --> Deploy
    SidecarH --> Deploy
    IngressH --> Ing
    RBACH --> SA
    RBACH --> Role
    RBACH --> Binding
```

### Traits 处理顺序

```mermaid
flowchart LR
    T1[1. Storage] --> T2[2. Envs]
    T2 --> T3[3. EnvFrom]
    T3 --> T4[4. Probes]
    T4 --> T5[5. Resources]
    T5 --> T6[6. Init]
    T6 --> T7[7. Sidecar]
    T7 --> T8[8. RBAC]
    T8 --> T9[9. Ingress]
```

### 嵌套 Traits 支持

```mermaid
graph TB
    subgraph MainContainer [主容器 Traits]
        Storage[Storage]
        Envs[Envs]
        EnvFrom[EnvFrom]
        Probes[Probes]
        Resources[Resources]
        Init[Init]
        Sidecar[Sidecar]
        RBAC[RBAC]
        Ingress[Ingress]
    end

    subgraph InitTraits [Init 容器嵌套 Traits]
        I_Storage[Storage]
        I_Envs[Envs]
        I_EnvFrom[EnvFrom]
        I_Resources[Resources]
    end

    subgraph SidecarTraits [Sidecar 容器嵌套 Traits]
        S_Storage[Storage]
        S_Envs[Envs]
        S_EnvFrom[EnvFrom]
        S_Probes[Probes]
        S_Resources[Resources]
    end

    Init --> InitTraits
    Sidecar --> SidecarTraits
```

---

## 10. 部署架构

### 单实例部署（开发/测试）

```mermaid
graph TB
    subgraph SingleNode [单节点]
        API[KubeMin API Server]
        MySQL[(MySQL)]
    end

    subgraph K8sCluster [Kubernetes]
        Workloads[工作负载]
    end

    Client[客户端] --> API
    API --> MySQL
    API --> K8sCluster
```

### 多实例部署（生产环境）

```mermaid
graph TB
    subgraph LoadBalancer [负载均衡]
        LB[Ingress/LoadBalancer]
    end

    subgraph KubeMinCluster [KubeMin 集群]
        API1[API Server 1<br/>Leader]
        API2[API Server 2<br/>Follower]
        API3[API Server 3<br/>Follower]
    end

    subgraph DataLayer [数据层]
        MySQL[(MySQL<br/>主从复制)]
        Redis[(Redis<br/>Cluster/Sentinel)]
    end

    subgraph Observability [可观测性]
        Jaeger[Jaeger]
        Prometheus[Prometheus]
        Grafana[Grafana]
    end

    subgraph K8sCluster [Kubernetes 集群]
        Workloads[工作负载]
    end

    Client[客户端] --> LB
    LB --> API1
    LB --> API2
    LB --> API3

    API1 --> MySQL
    API2 --> MySQL
    API3 --> MySQL

    API1 --> Redis
    API2 --> Redis
    API3 --> Redis

    API1 --> Jaeger
    API2 --> Jaeger
    API3 --> Jaeger

    API1 --> K8sCluster
    API2 --> K8sCluster
    API3 --> K8sCluster
```

### 部署配置建议

| 环境 | 实例数 | MySQL | Redis | 说明 |
|------|--------|-------|-------|------|
| 开发 | 1 | 单实例 | 可选 | 本地模式，无需 Redis |
| 测试 | 1-3 | 单实例 | 单实例 | 验证分布式功能 |
| 生产 | 3+ (奇数) | 主从/集群 | Sentinel/Cluster | 高可用部署 |

---

## 附录

### A. 目录结构映射

```
KubeMin-Cli/
├── cmd/
│   └── main.go                      # 程序入口
├── pkg/apiserver/
│   ├── server.go                    # API Server 启动
│   ├── config/                      # 配置管理
│   ├── interfaces/api/              # 接口层
│   │   ├── applications.go          # 应用 API
│   │   ├── workflow.go              # 工作流 API
│   │   └── middleware/              # 中间件
│   ├── domain/                      # 领域层
│   │   ├── model/                   # 领域模型
│   │   ├── service/                 # 领域服务
│   │   ├── repository/              # 数据仓储
│   │   └── spec/                    # 规格定义
│   ├── event/workflow/              # 工作流引擎
│   │   ├── workflow.go              # 入口调度
│   │   ├── controller.go            # 任务控制
│   │   ├── dispatcher.go            # 消息分发
│   │   ├── job_builder.go           # Job 构建
│   │   └── job/                     # Job 控制器
│   ├── workflow/                    # 工作流支持
│   │   ├── traits/                  # Traits 处理器
│   │   └── signal/                  # 取消信号
│   ├── infrastructure/              # 基础设施
│   │   ├── datastore/               # 数据存储
│   │   ├── messaging/               # 消息队列
│   │   ├── clients/                 # 外部客户端
│   │   └── observability/           # 可观测性
│   └── utils/                       # 工具函数
└── deploy/                          # 部署配置
```

### B. 技术栈总览

```mermaid
mindmap
  root((KubeMin-Cli))
    语言框架
      Go 1.24
      Gin Web Framework
      GORM ORM
    Kubernetes
      client-go
      controller-runtime
      Leader Election
    存储
      MySQL
      Redis
        Cache
        Streams
    可观测性
      OpenTelemetry
      Jaeger
      klog
    工具
      Cobra CLI
      barnettZQG/inject
```

---

*文档版本：1.0.0*
*最后更新：2025-12*



