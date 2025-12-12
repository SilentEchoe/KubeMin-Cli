# KubeMin-Cli 架构图

> 本文档基于代码结构自动生成，包含系统的各层架构图和组件关系图。

## 目录

- [1. 系统整体架构](#1-系统整体架构)
- [2. DDD 分层架构](#2-ddd-分层架构)
- [3. 工作流引擎架构](#3-工作流引擎架构)
- [4. 消息队列架构](#4-消息队列架构)
- [5. OAM 组件与 Traits 关系](#5-oam-组件与-traits-关系)
- [5. Informer 状态同步机制](#5-informer-状态同步机制)
- [6. OAM 组件与 Traits 关系](#6-oam-组件与-traits-关系)
- [7. 项目目录结构](#7-项目目录结构)
- [8. 状态机图](#8-状态机图)

---

## 1. 系统整体架构

### 1.1 高层架构图

```mermaid
flowchart TB
    subgraph Client["客户端层"]
        CLI[CLI 工具]
        UI[Web UI]
        API_Client[API 客户端]
    end

    subgraph APIServer["API Server 层"]
        direction TB
        Router[Gin Router<br/>路由层]
        Middleware[中间件<br/>CORS/Gzip/Tracing]
        Handler[API Handler<br/>接口处理]
    end

    subgraph Domain["领域层"]
        direction TB
        AppService[ApplicationService<br/>应用服务]
        WFService[WorkflowService<br/>工作流服务]
        Repository[Repository<br/>数据仓库]
    end

    subgraph Event["事件层"]
        direction TB
        Dispatcher[Dispatcher<br/>任务分发器]
        Worker[Worker<br/>工作节点]
        Controller[WorkflowController<br/>工作流控制器]
    end

    subgraph Infra["基础设施层"]
        direction TB
        MySQL[(MySQL<br/>数据存储)]
        Redis[(Redis<br/>缓存/消息队列)]
        Kafka[(Kafka<br/>消息队列)]
        Informer[InformerManager<br/>状态同步]
    end

    subgraph K8s["Kubernetes 集群"]
        direction TB
        K8sAPI[Kubernetes API]
        Deployment[Deployment]
        StatefulSet[StatefulSet]
        ConfigMap[ConfigMap]
        Secret[Secret]
        Service[Service]
        Ingress[Ingress]
        PVC[PVC]
    end

    Client --> APIServer
    APIServer --> Domain
    Domain --> Repository
    Repository --> MySQL
    Domain --> Event
    Event <--> Redis
    Event <--> Kafka
    Informer -->|List-Watch| K8sAPI
    Informer -->|状态同步| MySQL
    Event --> K8s
    Domain --> K8s

    classDef client fill:#e1f5fe,stroke:#01579b
    classDef api fill:#fff3e0,stroke:#e65100
    classDef domain fill:#e8f5e9,stroke:#2e7d32
    classDef event fill:#fce4ec,stroke:#c2185b
    classDef infra fill:#f3e5f5,stroke:#7b1fa2
    classDef k8s fill:#e3f2fd,stroke:#1565c0

    class CLI,UI,API_Client client
    class Router,Middleware,Handler api
    class AppService,WFService,Repository domain
    class Dispatcher,Worker,Controller event
    class MySQL,Redis,Kafka,Informer infra
    class K8sAPI,Deployment,StatefulSet,ConfigMap,Secret,Service,Ingress,PVC k8s
```

### 1.2 核心组件交互图

```mermaid
sequenceDiagram
    autonumber
    participant Client as 客户端
    participant API as API Server
    participant Service as Domain Service
    participant Repo as Repository
    participant DB as MySQL
    participant Queue as 消息队列
    participant Worker as Worker
    participant K8s as Kubernetes

    Client->>API: POST /api/v1/applications
    API->>Service: CreateApplications()
    Service->>Repo: 保存应用数据
    Repo->>DB: INSERT
    DB-->>Repo: OK
    Service->>Repo: 创建工作流任务
    Repo->>DB: INSERT (status=waiting)
    DB-->>Repo: OK
    Service-->>API: 返回应用ID + TaskID
    API-->>Client: 200 OK {appID, taskID}

    Note over Queue,Worker: 异步执行工作流

    loop Dispatcher轮询
        Worker->>DB: 查询 waiting 任务
        DB-->>Worker: 任务列表
        Worker->>Queue: 发布任务消息
    end

    Queue-->>Worker: 消费任务消息
    Worker->>K8s: 创建 ConfigMap/Secret
    K8s-->>Worker: OK
    Worker->>K8s: 创建 Deployment/StatefulSet
    K8s-->>Worker: OK
    Worker->>K8s: 创建 Service/Ingress
    K8s-->>Worker: OK
    Worker->>DB: 更新任务状态 (completed)
    Worker->>Queue: ACK 消息
```

---

## 2. DDD 分层架构

### 2.1 分层架构图

```mermaid
flowchart TB
    subgraph Interfaces["接口层 (interfaces/)"]
        direction LR
        API_App[applications.go<br/>应用接口]
        API_WF[workflow.go<br/>工作流接口]
        API_Health[health.go<br/>健康检查]
        DTO[dto/v1<br/>数据传输对象]
        Assembler[assembler/v1<br/>对象转换器]
    end

    subgraph Domain["领域层 (domain/)"]
        direction TB
        
        subgraph Service["service/"]
            AppSvc[ApplicationService<br/>应用服务]
            WFSvc[WorkflowService<br/>工作流服务]
        end
        
        subgraph Model["model/"]
            AppModel[Applications<br/>应用模型]
            WFModel[Workflow<br/>工作流模型]
            CompModel[Component<br/>组件模型]
            QueueModel[WorkflowQueue<br/>任务队列模型]
        end
        
        subgraph Repo["repository/"]
            AppRepo[ApplicationRepository]
            WFRepo[WorkflowRepository]
            CompRepo[ComponentRepository]
            QueueRepo[WorkflowQueueRepository]
        end
    end

    subgraph Infrastructure["基础设施层 (infrastructure/)"]
        direction TB
        
        subgraph Clients["clients/"]
            KubeClient[kube.go<br/>K8s客户端]
            RedisClient[redis.go<br/>Redis客户端]
            KafkaClient[kafka.go<br/>Kafka客户端]
        end
        
        subgraph Datastore["datastore/"]
            DatastoreIF[DataStore接口]
            MySQLImpl[mysql/mysql.go<br/>MySQL实现]
        end
        
        subgraph Messaging["messaging/"]
            QueueIF[Queue接口]
            RedisStreams[redis_streams.go]
            KafkaQueue[kafka.go]
            NoopQueue[noop.go]
        end
    end

    Interfaces --> Domain
    Domain --> Infrastructure

    classDef interface fill:#fff3e0,stroke:#e65100
    classDef domain fill:#e8f5e9,stroke:#2e7d32
    classDef infra fill:#f3e5f5,stroke:#7b1fa2

    class API_App,API_WF,API_Health,DTO,Assembler interface
    class AppSvc,WFSvc,AppModel,WFModel,CompModel,QueueModel,AppRepo,WFRepo,CompRepo,QueueRepo domain
    class KubeClient,RedisClient,KafkaClient,DatastoreIF,MySQLImpl,QueueIF,RedisStreams,KafkaQueue,NoopQueue infra
```

### 2.2 IoC 容器依赖注入图

```mermaid
flowchart LR
    subgraph Container["IoC Container (container.go)"]
        direction TB
        
        subgraph Infra_Beans["基础设施 Beans"]
            kubeClient[kubeClient]
            kubeConfig[kubeConfig]
            datastore[datastore]
            cache[cache]
            queue[queue]
        end
        
        subgraph Repo_Beans["仓库 Beans"]
            AppRepo[ApplicationRepository]
            WFRepo[WorkflowRepository]
            CompRepo[ComponentRepository]
            QueueRepo[WorkflowQueueRepository]
        end
        
        subgraph Service_Beans["服务 Beans"]
            AppSvc[ApplicationService]
            WFSvc[WorkflowService]
        end
        
        subgraph API_Beans["API Beans"]
            AppAPI[applications]
            WFAPI[workflow]
        end
        
        subgraph Event_Beans["事件 Beans"]
            Workflow[Workflow]
        end
    end

    Infra_Beans --> Repo_Beans
    Repo_Beans --> Service_Beans
    Service_Beans --> API_Beans
    Service_Beans --> Event_Beans
    Infra_Beans --> Event_Beans

    classDef infra fill:#f3e5f5,stroke:#7b1fa2
    classDef repo fill:#e8f5e9,stroke:#2e7d32
    classDef service fill:#fff3e0,stroke:#e65100
    classDef api fill:#e1f5fe,stroke:#01579b
    classDef event fill:#fce4ec,stroke:#c2185b

    class kubeClient,kubeConfig,datastore,cache,queue infra
    class AppRepo,WFRepo,CompRepo,QueueRepo repo
    class AppSvc,WFSvc service
    class AppAPI,WFAPI api
    class Workflow event
```

---

## 3. 工作流引擎架构

### 3.1 工作流引擎组件图

```mermaid
flowchart TB
    subgraph WorkflowEngine["工作流引擎 (event/workflow/)"]
        direction TB
        
        subgraph Core["核心组件"]
            Workflow[workflow.go<br/>工作流入口]
            Controller[controller.go<br/>工作流控制器]
            Dispatcher[dispatcher.go<br/>任务分发器]
        end
        
        subgraph JobExecutors["Job 执行器 (job/)"]
            JobRunner[job.go<br/>Job运行器]
            DeployJob[job_deploy.go<br/>Deployment]
            StatefulSetJob[job_statefulset.go<br/>StatefulSet]
            ConfigMapJob[job_configmap.go<br/>ConfigMap]
            SecretJob[job_secret.go<br/>Secret]
            ServiceJob[job_service.go<br/>Service]
            IngressJob[job_ingress.go<br/>Ingress]
            PVCJob[job_pvc.go<br/>PVC]
            RBACJob[job_rbac.go<br/>RBAC]
        end
        
        subgraph StateManagement["状态管理"]
            WorkflowState[workflow_state.go<br/>状态机]
            CleanupTracker[cleanup_tracker.go<br/>清理追踪]
        end
        
        subgraph Signal["信号处理 (signal/)"]
            Cancel[cancel.go<br/>取消信号]
        end
    end

    Workflow --> Dispatcher
    Workflow --> Controller
    Controller --> JobExecutors
    Controller --> StateManagement
    JobExecutors --> Signal

    classDef core fill:#e8f5e9,stroke:#2e7d32
    classDef job fill:#fff3e0,stroke:#e65100
    classDef state fill:#e1f5fe,stroke:#01579b
    classDef signal fill:#fce4ec,stroke:#c2185b

    class Workflow,Controller,Dispatcher core
    class JobRunner,DeployJob,StatefulSetJob,ConfigMapJob,SecretJob,ServiceJob,IngressJob,PVCJob,RBACJob job
    class WorkflowState,CleanupTracker state
    class Cancel signal
```

### 3.2 工作流执行流程图

```mermaid
flowchart TB
    Start([开始]) --> CreateTask[创建任务<br/>status=waiting]
    CreateTask --> DispatchCheck{Dispatcher<br/>发现任务?}
    
    DispatchCheck -->|是| CAS[CAS 获取执行权]
    DispatchCheck -->|否| DispatchCheck
    
    CAS -->|成功| PublishQueue[发布到消息队列]
    CAS -->|失败| DispatchCheck
    
    PublishQueue --> WorkerConsume[Worker 消费消息]
    WorkerConsume --> UpdateQueued[更新状态<br/>status=queued]
    UpdateQueued --> CreateController[创建 WorkflowController]
    CreateController --> UpdateRunning[更新状态<br/>status=running]
    
    UpdateRunning --> GenerateJobs[生成 Job 任务列表]
    GenerateJobs --> SortPriority[按优先级排序]
    
    SortPriority --> ExecuteLoop{还有未执行<br/>的 Priority?}
    ExecuteLoop -->|是| GetJobs[获取当前优先级 Jobs]
    GetJobs --> ExecuteJobs[并发执行 Jobs]
    ExecuteJobs --> CheckResult{全部成功?}
    
    CheckResult -->|是| ExecuteLoop
    CheckResult -->|否| MarkFailed[标记失败<br/>status=failed]
    
    ExecuteLoop -->|否| MarkCompleted[标记完成<br/>status=completed]
    
    MarkFailed --> ACK[ACK 消息]
    MarkCompleted --> ACK
    ACK --> End([结束])

    classDef start fill:#c8e6c9,stroke:#2e7d32
    classDef process fill:#fff3e0,stroke:#e65100
    classDef decision fill:#e1f5fe,stroke:#01579b
    classDef end_node fill:#ffcdd2,stroke:#c62828

    class Start,End start
    class CreateTask,CAS,PublishQueue,WorkerConsume,UpdateQueued,CreateController,UpdateRunning,GenerateJobs,SortPriority,GetJobs,ExecuteJobs,MarkFailed,MarkCompleted,ACK process
    class DispatchCheck,ExecuteLoop,CheckResult decision
```

### 3.3 Job 优先级执行图

```mermaid
flowchart LR
    subgraph Priority0["Priority 0 (MaxHigh)"]
        CM[ConfigMap]
        Secret[Secret]
    end
    
    subgraph Priority1["Priority 1 (High)"]
        PVC[PVC]
        SA[ServiceAccount]
        Role[Role/ClusterRole]
        RB[RoleBinding]
    end
    
    subgraph Priority10["Priority 10 (Normal)"]
        Deploy[Deployment]
        STS[StatefulSet]
        SVC[Service]
    end
    
    subgraph Priority20["Priority 20 (Low)"]
        Ingress[Ingress]
        Cleanup[Cleanup Tasks]
    end

    Priority0 -->|完成后| Priority1
    Priority1 -->|完成后| Priority10
    Priority10 -->|完成后| Priority20

    classDef p0 fill:#ffcdd2,stroke:#c62828
    classDef p1 fill:#fff9c4,stroke:#f9a825
    classDef p10 fill:#c8e6c9,stroke:#2e7d32
    classDef p20 fill:#e1f5fe,stroke:#01579b

    class CM,Secret p0
    class PVC,SA,Role,RB p1
    class Deploy,STS,SVC p10
    class Ingress,Cleanup p20
```

---

## 4. 消息队列架构

### 4.1 消息队列抽象层

```mermaid
classDiagram
    class Queue {
        <<interface>>
        +EnsureGroup(ctx, group) error
        +Enqueue(ctx, payload) (string, error)
        +ReadGroup(ctx, group, consumer, count, block) ([]Message, error)
        +Ack(ctx, group, ids...) error
        +AutoClaim(ctx, group, consumer, minIdle, count) ([]Message, error)
        +Close(ctx) error
        +Stats(ctx, group) (backlog, pending, error)
    }
    
    class RedisStreamsQueue {
        -client *redis.Client
        -streamKey string
        -maxLen int64
        +EnsureGroup()
        +Enqueue()
        +ReadGroup()
        +Ack()
        +AutoClaim()
        +Close()
        +Stats()
    }
    
    class KafkaQueue {
        -producer *kafka.Producer
        -consumer *kafka.Consumer
        -topic string
        -groupID string
        +EnsureGroup()
        +Enqueue()
        +ReadGroup()
        +Ack()
        +AutoClaim()
        +Close()
        +Stats()
    }
    
    class NoopQueue {
        +EnsureGroup()
        +Enqueue()
        +ReadGroup()
        +Ack()
        +AutoClaim()
        +Close()
        +Stats()
    }

    Queue <|.. RedisStreamsQueue : implements
    Queue <|.. KafkaQueue : implements
    Queue <|.. NoopQueue : implements
```

### 4.2 分布式模式架构图 (Redis Streams)

```mermaid
flowchart TB
    subgraph Leader["Leader 节点"]
        Dispatcher[Dispatcher]
        LeaderElection[Leader Election<br/>Kubernetes Lease]
    end
    
    subgraph Workers["Worker 节点 (多实例)"]
        Worker1[Worker 1]
        Worker2[Worker 2]
        Worker3[Worker 3]
    end
    
    subgraph Redis["Redis"]
        Stream[("Redis Stream<br/>kubemin.workflow.dispatch")]
        ConsumerGroup[Consumer Group<br/>workflow-workers]
    end
    
    subgraph MySQL["MySQL"]
        TaskTable[(workflow_queue 表)]
    end

    Dispatcher -->|1. 轮询 waiting 任务| MySQL
    Dispatcher -->|2. XADD 发布任务| Stream
    
    Stream --> ConsumerGroup
    ConsumerGroup -->|XREADGROUP| Worker1
    ConsumerGroup -->|XREADGROUP| Worker2
    ConsumerGroup -->|XREADGROUP| Worker3
    
    Worker1 -->|更新状态| MySQL
    Worker2 -->|更新状态| MySQL
    Worker3 -->|更新状态| MySQL
    
    Worker1 -->|XACK| Stream
    Worker2 -->|XACK| Stream
    Worker3 -->|XACK| Stream

    Worker1 -.->|AutoClaim 故障恢复| ConsumerGroup
    Worker2 -.->|AutoClaim 故障恢复| ConsumerGroup
    Worker3 -.->|AutoClaim 故障恢复| ConsumerGroup

    classDef leader fill:#fff3e0,stroke:#e65100
    classDef worker fill:#e8f5e9,stroke:#2e7d32
    classDef redis fill:#ffcdd2,stroke:#c62828
    classDef mysql fill:#e1f5fe,stroke:#01579b

    class Dispatcher,LeaderElection leader
    class Worker1,Worker2,Worker3 worker
    class Stream,ConsumerGroup redis
    class TaskTable mysql
```

### 4.3 分布式模式架构图 (Kafka)

```mermaid
flowchart TB
    subgraph Leader["Leader 节点"]
        Dispatcher[Dispatcher]
    end
    
    subgraph Workers["Worker 节点 (多实例)"]
        Worker1[Worker 1<br/>Partition 0]
        Worker2[Worker 2<br/>Partition 1]
        Worker3[Worker 3<br/>Partition 2]
    end
    
    subgraph Kafka["Kafka Cluster"]
        Topic[("Topic: kubemin.workflow.dispatch")]
        P0[Partition 0]
        P1[Partition 1]
        P2[Partition 2]
        CG[Consumer Group<br/>kubemin-workflow-workers]
    end
    
    subgraph MySQL["MySQL"]
        TaskTable[(workflow_queue 表)]
    end

    Dispatcher -->|1. 轮询 waiting 任务| MySQL
    Dispatcher -->|2. Produce 消息| Topic
    
    Topic --> P0
    Topic --> P1
    Topic --> P2
    
    P0 -->|Consume| Worker1
    P1 -->|Consume| Worker2
    P2 -->|Consume| Worker3
    
    Worker1 -->|更新状态| MySQL
    Worker2 -->|更新状态| MySQL
    Worker3 -->|更新状态| MySQL
    
    Worker1 -->|Commit Offset| CG
    Worker2 -->|Commit Offset| CG
    Worker3 -->|Commit Offset| CG

    classDef leader fill:#fff3e0,stroke:#e65100
    classDef worker fill:#e8f5e9,stroke:#2e7d32
    classDef kafka fill:#e3f2fd,stroke:#1565c0
    classDef mysql fill:#f3e5f5,stroke:#7b1fa2

    class Dispatcher leader
    class Worker1,Worker2,Worker3 worker
    class Topic,P0,P1,P2,CG kafka
    class TaskTable mysql
```

### 4.4 本地模式 vs 分布式模式

```mermaid
flowchart LR
    subgraph Local["本地模式 (NoopQueue)"]
        direction TB
        L_API[API] --> L_DB[(MySQL)]
        L_DB --> L_Sender[WorkflowTaskSender<br/>定时轮询]
        L_Sender --> L_Controller[WorkflowController<br/>同进程执行]
        L_Controller --> L_K8s[Kubernetes]
    end
    
    subgraph Distributed["分布式模式 (Redis/Kafka)"]
        direction TB
        D_API[API] --> D_DB[(MySQL)]
        D_DB --> D_Dispatcher[Dispatcher<br/>Leader节点]
        D_Dispatcher --> D_Queue[(消息队列)]
        D_Queue --> D_Workers[Workers<br/>多实例]
        D_Workers --> D_K8s[Kubernetes]
    end

    classDef local fill:#e8f5e9,stroke:#2e7d32
    classDef distributed fill:#e1f5fe,stroke:#01579b

    class L_API,L_DB,L_Sender,L_Controller,L_K8s local
    class D_API,D_DB,D_Dispatcher,D_Queue,D_Workers,D_K8s distributed
```

---

## 5. Informer 状态同步机制

### 5.1 概述

Informer 状态同步机制用于实时监听 Kubernetes 资源变化，并将组件运行状态同步到数据库。相比传统轮询方式，Informer 采用 List-Watch 机制，具有以下优势：

- **实时性**：事件驱动，状态变更即时感知
- **高效性**：减少 API Server 请求，降低集群负载
- **可靠性**：本地缓存 + 增量更新，网络抖动自动恢复

### 5.2 架构图

```mermaid
flowchart TB
    subgraph K8s["Kubernetes 集群"]
        APIServer[API Server]
        Deploy[Deployment]
        STS[StatefulSet]
    end

    subgraph InformerLayer["Informer 层 (infrastructure/informer/)"]
        direction TB
        Manager[InformerManager<br/>管理器]
        Factory[SharedInformerFactory<br/>工厂]
        DeployInformer[Deployment Informer]
        STSInformer[StatefulSet Informer]
        Waiter[ResourceReadyWaiter<br/>资源等待器]
    end

    subgraph StatusSync["状态同步"]
        SyncFunc[syncComponentStatus<br/>同步回调]
        DB[(MySQL<br/>min_app_components)]
    end

    subgraph JobLayer["Job 执行层"]
        DeployJob[DeployJobCtl]
        STSJob[StatefulSetJobCtl]
        WaitFunc[wait() 函数]
    end

    APIServer -->|List-Watch| Factory
    Factory --> DeployInformer
    Factory --> STSInformer
    
    DeployInformer -->|Add/Update/Delete 事件| Waiter
    STSInformer -->|Add/Update/Delete 事件| Waiter
    
    Waiter -->|状态变更| SyncFunc
    SyncFunc -->|UPDATE status, ready_replicas| DB
    
    Waiter -->|通知就绪| WaitFunc
    WaitFunc --> DeployJob
    WaitFunc --> STSJob

    classDef k8s fill:#e3f2fd,stroke:#1565c0
    classDef informer fill:#e8f5e9,stroke:#2e7d32
    classDef sync fill:#fff3e0,stroke:#e65100
    classDef job fill:#fce4ec,stroke:#c2185b

    class APIServer,Deploy,STS k8s
    class Manager,Factory,DeployInformer,STSInformer,Waiter informer
    class SyncFunc,DB sync
    class DeployJob,STSJob,WaitFunc job
```

### 5.3 核心组件

| 组件 | 文件 | 职责 |
|------|------|------|
| `Manager` | `infrastructure/informer/manager.go` | 管理 Informer 生命周期，注册事件处理器 |
| `ResourceReadyWaiter` | `infrastructure/informer/waiter.go` | 处理资源事件，通知等待者，同步状态到数据库 |
| `types.go` | `infrastructure/informer/types.go` | 定义状态类型、等待条目、状态更新结构 |

### 5.4 状态同步流程

```mermaid
sequenceDiagram
    autonumber
    participant K8s as Kubernetes
    participant Informer as Informer
    participant Waiter as ResourceReadyWaiter
    participant SyncFunc as syncComponentStatus
    participant DB as MySQL

    Note over K8s,DB: 创建/更新资源
    K8s->>Informer: Deployment 状态变更事件
    Informer->>Waiter: OnDeploymentUpdate(old, new)
    Waiter->>Waiter: ExtractDeploymentStatus()
    Waiter->>Waiter: 从 Labels 提取 appID, componentID
    
    alt 有 StatusSyncFunc
        Waiter->>SyncFunc: 异步调用 go syncFunc(update)
        SyncFunc->>DB: List(AppID) 查询组件
        SyncFunc->>DB: Put(component) 更新状态
    end
    
    alt 有等待者在 WaitForReady
        Waiter->>Waiter: 检查 status.Ready
        alt Ready == true
            Waiter-->>Waiter: 关闭 channel，通知等待者
        end
    end

    Note over K8s,DB: 删除资源
    K8s->>Informer: Deployment 删除事件
    Informer->>Waiter: OnDeploymentDelete(deploy)
    Waiter->>Waiter: 从 Labels 提取组件信息
    Waiter->>SyncFunc: 同步状态为 Failed (replicas=0)
    SyncFunc->>DB: 更新 status=Failed, ready_replicas=0
```

### 5.5 Label 约定

Informer 依赖以下 Labels 来识别和过滤资源：

| Label Key | 说明 | 示例值 |
|-----------|------|--------|
| `kube-min-cli-appId` | 应用 ID | `1us2dy3a2yhczm8yes6spm88` |
| `kube-min-cli-componentId` | 组件 ID | `1` |
| `kube-min-cli-componentName` | 组件名称 | `nginx` |

**重要**：Labels 必须同时设置在资源的 `metadata.labels` 和 `spec.template.metadata.labels` 上：

```go
deployment := &appsv1.Deployment{
    ObjectMeta: metav1.ObjectMeta{
        Name:      deploymentName,
        Namespace: component.Namespace,
        Labels:    labels,  // ← Informer 过滤和状态同步依赖此
    },
    Spec: appsv1.DeploymentSpec{
        Selector: &metav1.LabelSelector{
            MatchLabels: labels,
        },
        Template: corev1.PodTemplateSpec{
            ObjectMeta: metav1.ObjectMeta{
                Labels: labels,  // ← Pod 选择器依赖此
            },
            // ...
        },
    },
}
```

### 5.6 组件状态枚举

```go
// config/consts.go
type ComponentStatus string

const (
    ComponentStatusRunning ComponentStatus = "Running"  // 所有副本就绪
    ComponentStatusPending ComponentStatus = "Pending"  // 部分副本就绪或启动中
    ComponentStatusFailed  ComponentStatus = "Failed"   // 失败或已删除
    ComponentStatusUnknown ComponentStatus = "Unknown"  // 未知状态
)
```

**状态计算逻辑**：

| 条件 | 状态 |
|------|------|
| `ready == true` (ReadyReplicas == Replicas) | Running |
| `readyReplicas > 0` | Pending |
| `replicas > 0 && readyReplicas == 0` | Pending |
| `replicas == 0` (资源被删除或缩容为 0) | Failed |

### 5.7 数据库字段

`min_app_components` 表新增字段：

| 字段 | 类型 | 说明 |
|------|------|------|
| `status` | VARCHAR(32) | 运行状态 (Running/Pending/Failed/Unknown) |
| `ready_replicas` | INT | 就绪副本数 |

**DDL**：

```sql
ALTER TABLE min_app_components 
ADD COLUMN status VARCHAR(32) DEFAULT 'Unknown' COMMENT '运行状态',
ADD COLUMN ready_replicas INT DEFAULT 0 COMMENT '就绪副本数';
```

### 5.8 LabelSelector 过滤优化

为减少内存消耗，Informer 仅监听带有 `kube-min-cli-appId` 标签的资源：

```go
// server.go
s.InformerManager = informer.NewManager(
    kubeClient,
    informer.WithResyncPeriod(30*time.Second),
    informer.WithLabelSelector(config.LabelAppID),  // 只监听 KubeMin 管理的资源
)
```

这样 Informer 不会缓存集群中其他应用的 Deployment/StatefulSet，显著降低内存占用。

### 5.9 事件驱动 vs 轮询对比

| 特性 | 事件驱动 (Informer) | 轮询 (Polling) |
|------|---------------------|----------------|
| 实时性 | 毫秒级 | 取决于轮询间隔 |
| API 调用 | 初始 List + 增量 Watch | 每次轮询都调用 |
| 资源消耗 | 低 (本地缓存) | 高 (频繁请求) |
| 适用场景 | 状态同步、等待就绪 | 简单查询、兜底方案 |

当前实现中，`wait()` 函数优先使用 Informer 事件驱动，当 Informer 不可用时自动降级为轮询：

```go
func (c *DeployJobCtl) wait(ctx context.Context) error {
    waiter := GetGlobalWaiter()
    if waiter != nil {
        // 优先使用 Informer 事件驱动
        return waiter.WaitForDeploymentReady(ctx, namespace, name, timeout)
    }
    // 降级为轮询
    return c.waitPolling(ctx)
}
```

---

## 6. OAM 组件与 Traits 关系

### 6.1 OAM 模型结构

```mermaid
classDiagram
    class Application {
        +string ID
        +string Name
        +string Namespace
        +string Version
        +[]Component Components
        +[]WorkflowStep Workflow
    }
    
    class Component {
        +string Name
        +string Type
        +Properties Properties
        +Traits Traits
    }
    
    class Properties {
        +string Image
        +[]Port Ports
        +map[string]string Env
        +[]string Command
        +[]string Args
    }
    
    class Traits {
        +[]Storage Storage
        +[]Envs Envs
        +[]EnvFrom EnvFrom
        +[]Probes Probes
        +Resources Resources
        +[]Init Init
        +[]Sidecar Sidecar
        +[]RBAC RBAC
        +[]Ingress Ingress
    }
    
    class WorkflowStep {
        +string Name
        +string Mode
        +[]string Components
    }

    Application "1" --> "*" Component : contains
    Application "1" --> "*" WorkflowStep : contains
    Component "1" --> "1" Properties : has
    Component "1" --> "1" Traits : has
```

### 6.2 Traits 处理流程图

```mermaid
flowchart TB
    Input[Component 输入] --> Storage[1. Storage Trait<br/>处理存储挂载]
    Storage --> Envs[2. Envs Trait<br/>处理环境变量]
    Envs --> EnvFrom[3. EnvFrom Trait<br/>批量导入环境变量]
    EnvFrom --> Probes[4. Probes Trait<br/>健康检查探针]
    Probes --> Resources[5. Resources Trait<br/>资源限制]
    Resources --> Init[6. Init Trait<br/>初始化容器]
    Init --> Sidecar[7. Sidecar Trait<br/>边车容器]
    Sidecar --> RBAC[8. RBAC Trait<br/>权限控制]
    RBAC --> Ingress[9. Ingress Trait<br/>入口流量]
    Ingress --> Output[Kubernetes 资源]

    classDef trait fill:#fff3e0,stroke:#e65100
    classDef io fill:#e8f5e9,stroke:#2e7d32

    class Storage,Envs,EnvFrom,Probes,Resources,Init,Sidecar,RBAC,Ingress trait
    class Input,Output io
```

### 6.3 组件类型与生成资源

```mermaid
flowchart LR
    subgraph ComponentTypes["组件类型"]
        WebService[webservice]
        StatefulService[statefulservice]
        Config[config]
        Secret[secret]
    end
    
    subgraph K8sResources["生成的 Kubernetes 资源"]
        Deploy[Deployment]
        STS[StatefulSet]
        SVC[Service]
        CM[ConfigMap]
        Sec[Secret]
        Ing[Ingress]
        PVC[PVC]
        SA[ServiceAccount]
        Role[Role]
        RB[RoleBinding]
    end

    WebService --> Deploy
    WebService --> SVC
    WebService -.-> Ing
    WebService -.-> PVC
    WebService -.-> SA
    
    StatefulService --> STS
    StatefulService --> SVC
    StatefulService -.-> PVC
    
    Config --> CM
    Secret --> Sec

    classDef comp fill:#e1f5fe,stroke:#01579b
    classDef k8s fill:#e8f5e9,stroke:#2e7d32

    class WebService,StatefulService,Config,Secret comp
    class Deploy,STS,SVC,CM,Sec,Ing,PVC,SA,Role,RB k8s
```

---

## 7. 项目目录结构

```
KubeMin-Cli/
├── cmd/                              # 入口点
│   ├── main.go                       # 主程序入口
│   └── server/
│       └── app/
│           ├── options/              # 命令行参数
│           └── server.go             # 服务器启动
│
├── pkg/apiserver/                    # 核心 API 服务器
│   ├── server.go                     # 服务器实现
│   ├── config/                       # 配置管理
│   │
│   ├── interfaces/                   # 接口层 (DDD)
│   │   └── api/
│   │       ├── applications.go       # 应用接口
│   │       ├── workflow.go           # 工作流接口
│   │       ├── health.go             # 健康检查
│   │       ├── dto/v1/               # 数据传输对象
│   │       ├── assembler/v1/         # 对象转换器
│   │       └── middleware/           # 中间件
│   │
│   ├── domain/                       # 领域层 (DDD)
│   │   ├── model/                    # 领域模型
│   │   │   ├── applications.go
│   │   │   ├── workflow.go
│   │   │   └── ...
│   │   ├── repository/               # 仓库接口
│   │   │   ├── application.go
│   │   │   ├── workflow.go
│   │   │   └── ...
│   │   └── service/                  # 领域服务
│   │       ├── application.go
│   │       └── workflow.go
│   │
│   ├── infrastructure/               # 基础设施层 (DDD)
│   │   ├── clients/                  # 外部客户端
│   │   │   ├── kube.go               # Kubernetes 客户端
│   │   │   ├── redis.go              # Redis 客户端
│   │   │   └── kafka.go              # Kafka 客户端
│   │   ├── datastore/                # 数据存储
│   │   │   └── mysql/                # MySQL 实现
│   │   └── messaging/                # 消息队列
│   │       ├── queue.go              # 队列接口
│   │       ├── redis_streams.go      # Redis Streams 实现
│   │       ├── kafka.go              # Kafka 实现
│   │       └── noop.go               # 空实现 (本地模式)
│   │
│   ├── event/                        # 事件层
│   │   ├── event.go                  # 事件入口
│   │   └── workflow/                 # 工作流引擎
│   │       ├── workflow.go           # 工作流入口
│   │       ├── controller.go         # 工作流控制器
│   │       ├── dispatcher.go         # 任务分发器
│   │       └── job/                  # Job 执行器
│   │           ├── job.go
│   │           ├── job_deploy.go
│   │           ├── job_statefulset.go
│   │           ├── job_configmap.go
│   │           └── ...
│   │
│   ├── workflow/                     # 工作流支持
│   │   ├── traits/                   # Traits 处理器
│   │   │   ├── storage.go
│   │   │   ├── env.go
│   │   │   ├── probe.go
│   │   │   ├── resources.go
│   │   │   ├── sidecar.go
│   │   │   ├── rbac.go
│   │   │   └── ingress.go
│   │   ├── naming/                   # 命名规则
│   │   └── signal/                   # 信号处理
│   │
│   └── utils/                        # 工具类
│       ├── cache/                    # 缓存实现
│       ├── container/                # IoC 容器
│       ├── template/                 # 模板引擎
│       └── bcode/                    # 错误码
│
├── deploy/                           # 部署配置
│   ├── helm/                         # Helm Charts
│   └── mysql/                        # MySQL 初始化
│
├── docs/                             # 文档
├── examples/                         # 示例
└── scripts/                          # 脚本
```

---

## 8. 状态机图

### 8.1 工作流任务状态机

```mermaid
stateDiagram-v2
    [*] --> waiting: 创建任务
    
    waiting --> queued: Dispatcher 分发
    waiting --> cancelled: 用户取消
    
    queued --> running: Worker 开始执行
    queued --> waiting: Worker 崩溃 (AutoClaim)
    
    running --> completed: 执行成功
    running --> failed: 执行失败
    running --> timeout: 执行超时
    running --> cancelled: 用户取消
    running --> waiting: 进程重启 (InitQueue)
    
    completed --> [*]
    failed --> [*]
    timeout --> [*]
    cancelled --> [*]
```

### 8.2 Leader 选举状态机

```mermaid
stateDiagram-v2
    [*] --> Follower: 启动
    
    Follower --> Leader: 获得 Lease
    Follower --> Follower: 发现新 Leader
    
    Leader --> Follower: 丢失 Lease
    Leader --> Leader: 续期 Lease
    
    state Leader {
        [*] --> StartDispatcher: 成为 Leader
        StartDispatcher --> Running: Dispatcher 启动
        Running --> StopDispatcher: 丢失 Leader
    }
    
    state Follower {
        [*] --> WorkerMode: 非 Leader
        WorkerMode --> WorkerMode: 处理队列消息
    }
```

---

## 附录：技术栈

| 层次 | 技术选型 |
|------|----------|
| Web 框架 | Gin |
| 数据库 | MySQL |
| 缓存 | Redis |
| 消息队列 | Redis Streams / Kafka |
| 容器编排 | Kubernetes |
| 配置管理 | Viper |
| 日志 | klog |
| 链路追踪 | OpenTelemetry |
| 依赖注入 | 自研 IoC 容器 |
| API 风格 | RESTful |

---

*文档生成时间：2024-12*  
*最后更新：2024-12（新增 Informer 状态同步机制章节）*

