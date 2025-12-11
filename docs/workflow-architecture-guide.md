# KubeMin-Cli 工作流引擎架构详解

## 目录

- [1. 概述与背景](#1-概述与背景)
- [2. 设计理念](#2-设计理念)
- [3. 设计原则](#3-设计原则)
- [4. 架构设计](#4-架构设计)
- [5. 核心组件详解](#5-核心组件详解)
- [6. 执行流程](#6-执行流程)
- [7. 分布式支持](#7-分布式支持)
- [8. 取消与清理机制](#8-取消与清理机制)
- [9. 状态管理](#9-状态管理)
- [10. 并发控制](#10-并发控制)
- [11. 配置参考](#11-配置参考)
- [12. 优势总结](#12-优势总结)

---

## 1. 概述与背景

### 1.1 工作流引擎的定位

KubeMin-Cli 工作流引擎是整个应用交付系统的核心组件，负责将用户声明的应用配置转换为实际运行在 Kubernetes 集群上的资源。它充当了"编排者"的角色，协调多个组件的创建、更新和删除操作，确保应用的正确部署。

工作流引擎解决了以下核心问题：

- **资源依赖管理**：确保 ConfigMap、Secret、PVC 等依赖资源先于 Deployment、StatefulSet 创建
- **执行顺序控制**：支持串行和并行两种执行模式，满足不同场景需求
- **状态追踪**：完整记录每个任务和 Job 的执行状态，便于问题排查
- **故障恢复**：支持任务重试、取消和资源清理，保证系统一致性
- **分布式扩展**：支持多实例部署，通过 Redis Streams 实现任务分发

### 1.2 与 OAM 的集成

工作流引擎借鉴了 OAM (Open Application Model) 的设计思想，支持声明式的工作流定义：

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

工作流支持两种执行模式：

| 模式 | 标识 | 说明 |
|------|------|------|
| 串行模式 | `StepByStep` | 组件按声明顺序依次执行，前一个完成后才执行下一个 |
| 并行模式 | `DAG` | 同一 Step 内的组件并行执行，适合无依赖关系的组件 |

---

## 2. 设计理念

### 2.1 声明式编排

工作流采用声明式的方式定义组件编排顺序，用户只需声明"做什么"，系统自动处理"怎么做"：

```go
// model/workflow.go
type WorkflowStep struct {
    Name         string              `json:"name"`
    Mode         config.WorkflowMode `json:"mode,omitempty"`
    Properties   []Policies          `json:"properties,omitempty"`
    SubSteps     []*WorkflowSubStep  `json:"subSteps,omitempty"`
}
```

这种设计的优势：
- **简化用户操作**：用户无需关心底层资源创建的复杂逻辑
- **提高可维护性**：工作流定义与执行逻辑分离
- **增强可读性**：工作流结构清晰，易于理解和调试

### 2.2 组件化 Job

每种 Kubernetes 资源类型都有对应的 Job 控制器，实现了高度的模块化：

```
pkg/apiserver/event/workflow/job/
├── job.go                 # Job 执行器核心
├── job_deploy.go          # Deployment 控制器
├── job_statefulset.go     # StatefulSet 控制器
├── job_service.go         # Service 控制器
├── job_pvc.go             # PVC 控制器
├── job_configmap.go       # ConfigMap 控制器
├── job_secret.go          # Secret 控制器
├── job_ingress.go         # Ingress 控制器
├── job_rbac.go            # RBAC 资源控制器
├── cleanup_tracker.go     # 清理跟踪器
└── naming.go              # 命名规范
```

每个 Job 控制器实现统一的接口：

```go
// job/job.go
type JobCtl interface {
    Run(ctx context.Context) error      // 执行资源创建/更新
    Clean(ctx context.Context)          // 清理已创建的资源
    SaveInfo(ctx context.Context) error // 保存执行信息到数据库
}
```

### 2.3 可观测性优先

工作流引擎深度集成了 OpenTelemetry 分布式追踪：

```go
// controller.go
func (w *WorkflowCtl) Run(ctx context.Context, concurrency int) error {
    tracer := otel.Tracer("workflow-runner")
    ctx, span := tracer.Start(ctx, workflowName, trace.WithAttributes(
        attribute.String("workflow.name", workflowName),
        attribute.String("workflow.task_id", taskMeta.TaskID),
    ))
    defer span.End()
    
    // 创建带 traceID 的 logger
    logger := klog.FromContext(ctx).WithValues(
        "traceID", span.SpanContext().TraceID().String(),
        "workflowName", workflowName,
        "taskID", taskMeta.TaskID,
    )
    // ...
}
```

每个 Job 执行也会创建子 Span：

```go
// job/job.go
func runJob(ctx context.Context, job *model.JobTask, ...) {
    tracer := otel.Tracer("job-runner")
    ctx, span := tracer.Start(ctx, job.Name, trace.WithAttributes(
        attribute.String("job.name", job.Name),
        attribute.String("job.type", job.JobType),
    ))
    defer span.End()
    // ...
}
```

### 2.4 弹性与容错

工作流引擎支持两种运行模式，可根据部署规模灵活选择：

| 模式 | 适用场景 | 特点 |
|------|----------|------|
| 本地模式 | 单实例部署、开发测试 | 使用 NoopQueue，直接扫描数据库执行任务 |
| 分布式模式 | 多实例部署、生产环境 | 使用 Redis Streams，支持任务分发和故障恢复 |

模式选择逻辑：

```go
// workflow.go
func (w *Workflow) Start(ctx context.Context, errChan chan error) {
    // 如果队列是 noop（本地模式），使用直接数据库扫描执行
    if _, ok := w.Queue.(*msg.NoopQueue); ok {
        go w.WorkflowTaskSender(ctx)
        return
    }
    // Redis Streams 路径：领导者运行 dispatcher；worker 由服务器回调管理
    go w.Dispatcher(ctx)
}
```

---

## 3. 设计原则

### 3.1 单一职责原则

每个 Job 控制器只负责一种资源类型的生命周期管理：

```go
// job/job.go
func initJobCtl(job *model.JobTask, ...) JobCtl {
    switch job.JobType {
    case string(config.JobDeploy):
        return NewDeployJobCtl(job, client, store, ack)
    case string(config.JobDeployService):
        return NewDeployServiceJobCtl(job, client, store, ack)
    case string(config.JobDeployStore):
        return NewDeployStatefulSetJobCtl(job, client, store, ack)
    case string(config.JobDeployPVC):
        return NewDeployPVCJobCtl(job, client, store, ack)
    case string(config.JobDeployConfigMap):
        return NewDeployConfigMapJobCtl(job, client, store, ack)
    case string(config.JobDeploySecret):
        return NewDeploySecretJobCtl(job, client, store, ack)
    // ... 更多类型
    }
}
```

### 3.2 优先级调度

资源按依赖关系分为不同优先级，确保依赖资源先创建：

```go
// config/consts.go
const (
    JobPriorityMaxHigh = 0   // 最高优先级：Secret, ConfigMap
    JobPriorityHigh    = 1   // 高优先级：PVC, ServiceAccount, Role
    JobPriorityNormal  = 10  // 普通优先级：Deployment, StatefulSet, Service
    JobPriorityLow     = 20  // 低优先级：清理任务、通知任务
)
```

Job 构建时自动分配优先级：

```go
// job_builder.go
func buildJobsForComponent(ctx context.Context, component *model.ApplicationComponent, ...) map[int][]*model.JobTask {
    buckets := newJobBuckets()
    
    switch component.ComponentType {
    case config.ConfJob:
        // ConfigMap 分配到最高优先级
        buckets[config.JobPriorityMaxHigh] = append(buckets[config.JobPriorityMaxHigh], jobTask)
    case config.SecretJob:
        // Secret 分配到最高优先级
        buckets[config.JobPriorityMaxHigh] = append(buckets[config.JobPriorityMaxHigh], jobTask)
    case config.ServerJob:
        // Deployment 相关的附加资源（PVC、Ingress）分配到高优先级
        // Deployment 本身分配到普通优先级
        buckets[config.JobPriorityNormal] = append(buckets[config.JobPriorityNormal], jobTask)
    }
    
    return buckets
}
```

执行时按优先级顺序处理：

```go
// controller.go
for _, stepExec := range stepExecutions {
    priorities := sortedPriorities(stepExec.Jobs) // [0, 1, 10, 20]
    for _, priority := range priorities {
        tasksInPriority := stepExec.Jobs[priority]
        // 执行该优先级的所有 Job
        job.RunJobs(ctx, tasksInPriority, stepConcurrency, ...)
    }
}
```

### 3.3 幂等性设计

Job 控制器在执行前会检查资源是否存在，支持重复执行：

```go
// job_deploy.go
func (c *DeployJobCtl) run(ctx context.Context) error {
    deployLast, isAlreadyExists, err := c.deploymentExists(ctx, deployName, deploy.Namespace)
    if err != nil {
        return fmt.Errorf("failed to check deployment existence: %w", err)
    }

    if isAlreadyExists {
        // 已存在：检查是否需要更新
        if isDeploymentChanged(deployLast, deploy) {
            // 使用 Server-Side Apply 更新
            updated, err := c.ApplyDeployment(ctx, deploy)
            // ...
        } else {
            klog.Infof("Deployment %q is up-to-date, skip apply.", deploy.Name)
        }
        // 标记为"已观察"而非"已创建"，避免清理时误删
        markResourceObserved(ctx, config.ResourceDeployment, deploy.Namespace, deploy.Name)
    } else {
        // 不存在：创建新资源
        result, err := c.client.AppsV1().Deployments(deploy.Namespace).Create(ctx, deploy, ...)
        // 标记为"已创建"，失败时需要清理
        MarkResourceCreated(ctx, config.ResourceDeployment, deploy.Namespace, deploy.Name)
    }
    return nil
}
```

### 3.4 状态驱动

任务生命周期由状态机管理：

```
┌─────────┐    创建任务    ┌─────────┐    获取到执行权    ┌─────────┐
│ waiting │ ─────────────> │ queued  │ ────────────────> │ running │
└─────────┘                └─────────┘                   └─────────┘
                                                              │
                    ┌─────────────────────────────────────────┼─────────────────────┐
                    │                                         │                     │
                    ▼                                         ▼                     ▼
              ┌───────────┐                            ┌───────────┐         ┌───────────┐
              │ completed │                            │  failed   │         │ cancelled │
              └───────────┘                            └───────────┘         └───────────┘
                                                              │
                                                              ▼
                                                       ┌───────────┐
                                                       │  timeout  │
                                                       └───────────┘
```

状态定义：

```go
// config/consts.go
const (
    StatusWaiting   Status = "waiting"   // 等待执行
    StatusQueued    Status = "queued"    // 已入队，等待 worker 处理
    StatusRunning   Status = "running"   // 执行中
    StatusCompleted Status = "completed" // 执行完成
    StatusFailed    Status = "failed"    // 执行失败
    StatusTimeout   Status = "timeout"   // 执行超时
    StatusCancelled Status = "cancelled" // 已取消
)
```

---

## 4. 架构设计

### 4.1 整体架构图

```
┌──────────────────────────────────────────────────────────────────────────────┐
│                              API Layer                                        │
│  ┌─────────────────────────────────────────────────────────────────────────┐ │
│  │  POST /applications/:appID/workflow/exec                                │ │
│  │  POST /applications/:appID/workflow/cancel                              │ │
│  │  GET  /workflow/tasks/:taskID/status                                    │ │
│  └─────────────────────────────────────────────────────────────────────────┘ │
└──────────────────────────────────────────────────────────────────────────────┘
                                      │
                                      ▼
┌──────────────────────────────────────────────────────────────────────────────┐
│                           Service Layer                                       │
│  ┌─────────────────────────────────────────────────────────────────────────┐ │
│  │  WorkflowService                                                        │ │
│  │  - CreateWorkflowTask()    创建工作流任务                                 │ │
│  │  - ExecWorkflowTask()      触发工作流执行                                 │ │
│  │  - CancelWorkflowTask()    取消工作流任务                                 │ │
│  │  - GetTaskStatus()         查询任务状态                                   │ │
│  └─────────────────────────────────────────────────────────────────────────┘ │
└──────────────────────────────────────────────────────────────────────────────┘
                                      │
                                      ▼
┌──────────────────────────────────────────────────────────────────────────────┐
│                          Workflow Engine                                      │
│  ┌────────────────────────────┐    ┌────────────────────────────────────┐   │
│  │       Workflow             │    │       WorkflowController            │   │
│  │  ┌──────────────────────┐  │    │  ┌──────────────────────────────┐  │   │
│  │  │ Start()              │  │    │  │ Run()                        │  │   │
│  │  │ - InitQueue()        │  │    │  │ - GenerateJobTasks()         │  │   │
│  │  │ - Dispatcher()       │──┼───>│  │ - RunJobs() by priority      │  │   │
│  │  │ - StartWorker()      │  │    │  │ - updateWorkflowStatus()     │  │   │
│  │  └──────────────────────┘  │    │  └──────────────────────────────┘  │   │
│  └────────────────────────────┘    └────────────────────────────────────┘   │
│                                                   │                          │
│                                                   ▼                          │
│  ┌──────────────────────────────────────────────────────────────────────┐   │
│  │                         Job Controllers                               │   │
│  │  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐   │   │
│  │  │ Deploy   │ │ Store    │ │ Service  │ │ PVC      │ │ Config   │   │   │
│  │  │ JobCtl   │ │ JobCtl   │ │ JobCtl   │ │ JobCtl   │ │ JobCtl   │   │   │
│  │  └──────────┘ └──────────┘ └──────────┘ └──────────┘ └──────────┘   │   │
│  │  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐   │   │
│  │  │ Secret   │ │ Ingress  │ │ SA       │ │ Role     │ │ Binding  │   │   │
│  │  │ JobCtl   │ │ JobCtl   │ │ JobCtl   │ │ JobCtl   │ │ JobCtl   │   │   │
│  │  └──────────┘ └──────────┘ └──────────┘ └──────────┘ └──────────┘   │   │
│  └──────────────────────────────────────────────────────────────────────┘   │
└──────────────────────────────────────────────────────────────────────────────┘
                                      │
                    ┌─────────────────┼─────────────────┐
                    ▼                 ▼                 ▼
┌──────────────────────┐  ┌──────────────────┐  ┌──────────────────────┐
│     MySQL            │  │   Redis          │  │   Kubernetes         │
│  ┌────────────────┐  │  │  ┌────────────┐  │  │  ┌────────────────┐  │
│  │ workflow       │  │  │  │ Streams    │  │  │  │ Deployments    │  │
│  │ workflow_queue │  │  │  │ (任务分发)  │  │  │  │ StatefulSets   │  │
│  │ job_info       │  │  │  ├────────────┤  │  │  │ Services       │  │
│  │ components     │  │  │  │ Cancel     │  │  │  │ ConfigMaps     │  │
│  └────────────────┘  │  │  │ Signals    │  │  │  │ Secrets        │  │
└──────────────────────┘  │  └────────────┘  │  │  │ PVCs           │  │
                          └──────────────────┘  │  └────────────────┘  │
                                                └──────────────────────┘
```

### 4.2 数据模型

#### Workflow - 工作流定义

```go
// domain/model/workflow.go
type Workflow struct {
    ID           string                  `json:"id" gorm:"primaryKey"`
    Name         string                  `json:"name"`
    Namespace    string                  `json:"namespace"`
    Alias        string                  `json:"alias"`
    Disabled     bool                    `json:"disabled"`
    ProjectID    string                  `json:"project_id"`
    AppID        string                  `json:"app_id"`
    UserID       string                  `json:"user_id"`
    Description  string                  `json:"description"`
    WorkflowType config.WorkflowTaskType `json:"workflow_type"`
    Status       config.Status           `json:"status"`
    Steps        *JSONStruct             `json:"steps,omitempty" gorm:"serializer:json"`
    BaseModel
}
```

#### WorkflowQueue - 任务队列

```go
// domain/model/workflow_queue.go
type WorkflowQueue struct {
    TaskID              string                  `gorm:"primaryKey" json:"task_id"`
    ProjectID           string                  `json:"projectId"`
    WorkflowName        string                  `json:"workflow_name"`
    AppID               string                  `json:"app_id"`
    WorkflowID          string                  `json:"workflow_id"`
    WorkflowDisplayName string                  `json:"workflow_display_name"`
    Status              config.Status           `json:"status,omitempty"`
    TaskCreator         string                  `json:"task_creator,omitempty"`
    TaskRevoker         string                  `json:"task_revoker,omitempty"`
    Type                config.WorkflowTaskType `json:"type,omitempty"`
    BaseModel
}
```

#### JobTask - Job 任务定义

```go
// domain/model/job.go
type JobTask struct {
    Name       string        `json:"name"`
    Namespace  string        `json:"namespace"`
    WorkflowID string        `json:"workflow_id"`
    ProjectID  string        `json:"project_id"`
    AppID      string        `json:"app_id"`
    TaskID     string        `json:"task_id"`
    JobType    string        `json:"job_type"`
    Status     config.Status `json:"status"`
    Timeout    int64         `json:"timeout"`
    StartTime  int64         `json:"start_time"`
    EndTime    int64         `json:"end_time"`
    Error      string        `json:"error"`
    JobInfo    interface{}   `json:"job_info"` // 存储具体的 K8s 资源对象
}
```

### 4.3 消息队列抽象

工作流引擎抽象了消息队列接口，支持多种实现：

```go
// infrastructure/messaging/queue.go
type Queue interface {
    // 确保消费者组存在
    EnsureGroup(ctx context.Context, group string) error
    
    // 入队：将消息推送到流
    Enqueue(ctx context.Context, payload []byte) (string, error)
    
    // 读取：从消费者组读取消息
    ReadGroup(ctx context.Context, group, consumer string, count int, block time.Duration) ([]Message, error)
    
    // 确认：标记消息已处理
    Ack(ctx context.Context, group string, ids ...string) error
    
    // 自动认领：认领空闲超时的待处理消息
    AutoClaim(ctx context.Context, group, consumer string, minIdle time.Duration, count int) ([]Message, error)
    
    // 关闭连接
    Close(ctx context.Context) error
    
    // 统计信息
    Stats(ctx context.Context, group string) (backlog int64, pending int64, err error)
}
```

目前支持三种实现：

| 实现 | 说明 | 使用场景 |
|------|------|----------|
| NoopQueue | 空实现，不做任何队列操作 | 本地模式，直接 DB 轮询 |
| RedisStreams | 基于 Redis Streams 的分布式队列 | 生产环境，多实例部署 |
| KafkaQueue | 基于 Apache Kafka 的分布式队列 | 大规模生产环境，需要高吞吐量 |

---

## 5. 核心组件详解

### 5.1 Workflow - 入口调度器

`Workflow` 是工作流引擎的入口，负责启动和协调整个系统：

```go
// event/workflow/workflow.go
type Workflow struct {
    KubeClient      kubernetes.Interface    `inject:"kubeClient"`
    KubeConfig      *rest.Config            `inject:"kubeConfig"`
    Store           datastore.DataStore     `inject:"datastore"`
    WorkflowService service.WorkflowService `inject:""`
    Queue           msg.Queue               `inject:"queue"`
    Cfg             *config.Config          `inject:""`
    taskGroup       *errgroup.Group
    taskGroupCtx    context.Context
    errChan         chan error
    workflowLimiter *semaphore.Weighted     // 并发限制器
}
```

核心方法：

#### Start - 启动工作流引擎

```go
func (w *Workflow) Start(ctx context.Context, errChan chan error) {
    w.InitQueue(ctx)  // 重新入队中断的任务
    w.errChan = errChan
    w.taskGroup, w.taskGroupCtx = errgroup.WithContext(ctx)
    
    // 初始化并发限制器
    if max := w.maxWorkflowConcurrency(); max > 0 {
        w.workflowLimiter = semaphore.NewWeighted(max)
    }
    
    // 优雅关闭处理
    go func() {
        <-ctx.Done()
        if w.taskGroup != nil {
            w.taskGroup.Wait()
        }
    }()
    
    // 根据队列类型选择运行模式
    if _, ok := w.Queue.(*msg.NoopQueue); ok {
        go w.WorkflowTaskSender(ctx)  // 本地模式
        return
    }
    go w.Dispatcher(ctx)  // 分布式模式
}
```

#### InitQueue - 任务恢复

进程重启后，需要将中断的任务重新入队：

```go
func (w *Workflow) InitQueue(ctx context.Context) {
    // 查找所有"运行中"的任务
    tasks, err := w.WorkflowService.TaskRunning(ctx)
    if err != nil || len(tasks) == 0 {
        return
    }
    
    // 将状态重置为 waiting，等待重新调度
    for _, task := range tasks {
        task.Status = config.StatusWaiting
        w.Store.Put(ctx, task)
        klog.Infof("re-queued task: %s (workflow=%s)", task.TaskID, task.WorkflowName)
    }
}
```

### 5.2 WorkflowController - 任务控制器

`WorkflowCtl` 负责单个工作流任务的执行控制：

```go
// event/workflow/controller.go
type WorkflowCtl struct {
    workflowTask             *model.WorkflowQueue
    workflowTaskMutex        sync.RWMutex
    Client                   kubernetes.Interface
    Store                    datastore.DataStore
    prefix                   string
    ack                      func()                  // 状态同步回调
    defaultJobTimeoutSeconds int64
    ctx                      context.Context
}
```

#### Run - 执行工作流

```go
func (w *WorkflowCtl) Run(ctx context.Context, concurrency int) error {
    // 1. 启动追踪 Span
    tracer := otel.Tracer("workflow-runner")
    ctx, span := tracer.Start(ctx, workflowName, ...)
    defer span.End()
    
    // 2. 更新状态为运行中
    w.mutateTask(func(task *model.WorkflowQueue) {
        task.Status = config.StatusRunning
        task.CreateTime = time.Now()
    })
    w.ack()
    
    // 3. 生成 Job 任务
    stepExecutions := GenerateJobTasks(ctx, &taskForGeneration, w.Store, w.defaultJobTimeoutSeconds)
    
    // 4. 按步骤执行
    for _, stepExec := range stepExecutions {
        priorities := sortedPriorities(stepExec.Jobs)
        for _, priority := range priorities {
            tasksInPriority := stepExec.Jobs[priority]
            
            // 确定并发度
            stepConcurrency := determineStepConcurrency(stepExec.Mode, len(tasksInPriority), seqLimit)
            stopOnFailure := !stepExec.Mode.IsParallel()
            
            // 执行该优先级的 Jobs
            job.RunJobs(ctx, tasksInPriority, stepConcurrency, w.Client, w.Store, w.ack, stopOnFailure)
            
            // 检查执行结果
            for _, task := range tasksInPriority {
                if task.Status != config.StatusCompleted {
                    w.setStatus(config.StatusFailed)
                    return fmt.Errorf("workflow %s failed at job %s", workflowName, task.Name)
                }
            }
        }
    }
    
    // 5. 标记完成
    w.updateWorkflowStatus(ctx)
    return nil
}
```

### 5.3 JobBuilder - Job 构建器

`GenerateJobTasks` 函数负责将工作流步骤转换为可执行的 Job 任务：

```go
// event/workflow/job_builder.go
func GenerateJobTasks(ctx context.Context, task *model.WorkflowQueue, ds datastore.DataStore, defaultJobTimeoutSeconds int64) []StepExecution {
    // 1. 加载工作流定义
    workflow := model.Workflow{ID: task.WorkflowID}
    ds.Get(ctx, &workflow)
    
    // 2. 解析工作流步骤
    var workflowSteps model.WorkflowSteps
    json.Unmarshal(stepsBytes, &workflowSteps)
    
    // 3. 加载组件信息
    componentEntities, _ := ds.List(ctx, &model.ApplicationComponent{AppID: task.AppID}, ...)
    componentMap := make(map[string]*model.ApplicationComponent)
    for _, entity := range componentEntities {
        componentMap[component.Name] = component
    }
    
    // 4. 按步骤构建 Job
    var executions []StepExecution
    for _, step := range workflowSteps.Steps {
        mode := step.Mode
        if mode == "" {
            mode = config.WorkflowModeStepByStep
        }
        
        componentNames := step.ComponentNames()
        
        if mode.IsParallel() {
            // 并行模式：所有组件放入同一个 StepExecution
            buckets := newJobBuckets()
            appendComponentGroup(ctx, buckets, componentNames, componentMap, task, ...)
            executions = append(executions, StepExecution{Name: step.Name, Mode: mode, Jobs: buckets})
        } else {
            // 串行模式：每个组件独立成一个 StepExecution
            for _, name := range componentNames {
                buckets := newJobBuckets()
                appendComponentGroup(ctx, buckets, []string{name}, componentMap, task, ...)
                executions = append(executions, StepExecution{Name: name, Mode: config.WorkflowModeStepByStep, Jobs: buckets})
            }
        }
    }
    
    return executions
}
```

#### buildJobsForComponent - 组件转 Job

```go
func buildJobsForComponent(ctx context.Context, component *model.ApplicationComponent, task *model.WorkflowQueue, defaultJobTimeoutSeconds int64) map[int][]*model.JobTask {
    buckets := newJobBuckets()
    
    properties := ParseProperties(ctx, component.Properties)
    
    switch component.ComponentType {
    case config.ServerJob:  // webservice
        serviceJobs := job.GenerateWebService(component, &properties)
        // 附加资源（PVC、Ingress）放入高优先级
        // Deployment 本身放入普通优先级
        queueServiceJobs(logger, buckets, component, task, namespace, config.JobDeploy, serviceJobs, ...)
        
    case config.StoreJob:  // store
        storeJobs := job.GenerateStoreService(component)
        queueServiceJobs(logger, buckets, component, task, namespace, config.JobDeployStore, storeJobs, ...)
        
    case config.ConfJob:  // config
        jobTask := NewJobTask(component.Name, namespace, ...)
        jobTask.JobType = string(config.JobDeployConfigMap)
        jobTask.JobInfo = job.GenerateConfigMap(component, &properties)
        buckets[config.JobPriorityMaxHigh] = append(buckets[config.JobPriorityMaxHigh], jobTask)
        
    case config.SecretJob:  // secret
        jobTask := NewJobTask(component.Name, namespace, ...)
        jobTask.JobType = string(config.JobDeploySecret)
        jobTask.JobInfo = job.GenerateSecret(component, &properties)
        buckets[config.JobPriorityMaxHigh] = append(buckets[config.JobPriorityMaxHigh], jobTask)
    }
    
    // Service 资源（如果有端口暴露）
    if len(properties.Ports) > 0 {
        svcJob := NewJobTask(component.Name, namespace, ...)
        svcJob.JobType = string(config.JobDeployService)
        svcJob.JobInfo = job.GenerateService(component, &properties)
        buckets[config.JobPriorityNormal] = append(buckets[config.JobPriorityNormal], svcJob)
    }
    
    return buckets
}
```

### 5.4 Job Controllers - 资源控制器

以 `DeployJobCtl` 为例说明 Job 控制器的实现：

```go
// event/workflow/job/job_deploy.go
type DeployJobCtl struct {
    namespace string
    job       *model.JobTask
    client    kubernetes.Interface
    store     datastore.DataStore
    ack       func()
}

func (c *DeployJobCtl) Run(ctx context.Context) error {
    c.job.Status = config.StatusRunning
    c.ack()  // 通知状态变更
    
    // 1. 执行资源创建/更新
    if err := c.run(ctx); err != nil {
        c.job.Error = err.Error()
        c.job.Status = config.StatusFailed
        return err
    }
    
    // 2. 等待资源就绪
    if err := c.wait(ctx); err != nil {
        c.job.Error = err.Error()
        c.job.Status = config.StatusFailed
        return err
    }
    
    c.job.Status = config.StatusCompleted
    return nil
}

func (c *DeployJobCtl) run(ctx context.Context) error {
    deploy := c.job.JobInfo.(*appsv1.Deployment)
    deployName := buildWebServiceName(c.job.Name, c.job.AppID)
    deploy.Name = deployName
    
    // 检查是否已存在
    deployLast, isAlreadyExists, err := c.deploymentExists(ctx, deployName, deploy.Namespace)
    
    if isAlreadyExists {
        if isDeploymentChanged(deployLast, deploy) {
            // 使用 Server-Side Apply 更新
            c.ApplyDeployment(ctx, deploy)
        }
        markResourceObserved(ctx, config.ResourceDeployment, deploy.Namespace, deploy.Name)
    } else {
        // 创建新资源
        c.client.AppsV1().Deployments(deploy.Namespace).Create(ctx, deploy, ...)
        MarkResourceCreated(ctx, config.ResourceDeployment, deploy.Namespace, deploy.Name)
    }
    
    return nil
}

func (c *DeployJobCtl) wait(ctx context.Context) error {
    timeout := time.After(time.Duration(c.timeout()) * time.Second)
    ticker := time.NewTicker(2 * time.Second)
    defer ticker.Stop()
    
    for {
        select {
        case <-ctx.Done():
            return NewStatusError(config.StatusCancelled, fmt.Errorf("cancelled: %w", ctx.Err()))
        case <-timeout:
            return NewStatusError(config.StatusTimeout, fmt.Errorf("timeout"))
        case <-ticker.C:
            status, _ := getDeploymentStatus(ctx, c.client, c.job.Namespace, targetName)
            if status != nil && status.Ready {
                return nil
            }
        }
    }
}

func (c *DeployJobCtl) Clean(ctx context.Context) {
    // 只清理本次创建的资源，不清理已存在的
    refs := resourcesForCleanup(ctx, config.ResourceDeployment)
    for _, ref := range refs {
        if !ref.Created {
            continue
        }
        c.client.AppsV1().Deployments(ref.Namespace).Delete(ctx, ref.Name, ...)
    }
}
```

### 5.5 Signal - 取消信号管理

`signal` 包实现了基于 Redis 的跨实例取消信号：

```go
// workflow/signal/cancel.go
type CancelWatcher struct {
    cli      *redis.Client
    key      string              // kubemin:workflow:cancel:<taskID>
    token    string              // 执行权标识
    stopCh   chan struct{}
    state    *cancelState        // 存储取消原因
    taskID   string
    cancelFn context.CancelFunc
}

// Watch 建立取消信号监听
func Watch(ctx context.Context, taskID string) (*CancelWatcher, context.Context, context.CancelFunc, error) {
    if cli == nil {
        // 无 Redis：使用内存注册表
        return localFallback(ctx, taskID)
    }
    
    key := cancelKeyPrefix + taskID
    token := uuid.NewString()
    
    // 使用 SETNX 声明执行权
    ok, err := cli.SetNX(ctx, key, token, defaultExpiry).Result()
    if !ok {
        // 检查是否是取消标记
        existing, _ := cli.Get(ctx, key).Result()
        if isCancelledToken(existing) {
            // 已取消，立即返回取消的 context
            cancelFn()
            return watcher, derivedCtx, func() {}, nil
        }
        return nil, ctx, nil, fmt.Errorf("task %s already running", taskID)
    }
    
    // 启动维护 goroutine
    go watcher.maintain(derivedCtx, cancelFn)
    
    return watcher, derivedCtx, cancelFn, nil
}

func (w *CancelWatcher) maintain(ctx context.Context, cancelFn context.CancelFunc) {
    ticker := time.NewTicker(extendInterval)  // 10s
    defer ticker.Stop()
    
    for {
        select {
        case <-ctx.Done():
            return
        case <-w.stopCh:
            return
        case <-ticker.C:
            w.step(ctx, cancelFn)
        }
    }
}

func (w *CancelWatcher) step(ctx context.Context, cancelFn context.CancelFunc) {
    val, err := w.cli.Get(ctx, w.key).Result()
    
    if err == redis.Nil {
        // key 被删除，触发取消
        cancelFn()
        return
    }
    
    if val != w.token {
        // 值被修改为取消标记
        w.state.set(extractCancelReason(val))
        cancelFn()
        return
    }
    
    // 续期 TTL
    w.cli.Expire(ctx, w.key, defaultExpiry)
}

// Cancel 发送取消信号
func Cancel(ctx context.Context, taskID, reason string) error {
    if cli == nil {
        localCancelRegistryInstance.cancel(taskID, reason)
        return nil
    }
    value := "cancelled:" + reason
    return cli.Set(ctx, cancelKeyPrefix+taskID, value, defaultExpiry).Err()
}
```

---

## 6. 执行流程

### 6.1 任务创建与入队

用户通过 API 触发工作流执行时，系统首先在数据库中创建任务记录，状态设为 `waiting`。这确保了任务的持久化存储，即使系统重启也不会丢失。API 同步返回任务 ID，用户可以通过该 ID 查询任务状态。

```
用户请求                    WorkflowService                   数据库
   │                             │                              │
   │  POST /workflow/exec        │                              │
   │────────────────────────────>│                              │
   │                             │                              │
   │                             │  创建 WorkflowQueue          │
   │                             │  status = waiting            │
   │                             │─────────────────────────────>│
   │                             │                              │
   │                             │<─────────────────────────────│
   │  返回 taskId                │                              │
   │<────────────────────────────│                              │
```

**关键点**：
- 任务首先写入数据库，确保持久化
- 状态初始为 `waiting`，等待被调度
- API 同步返回，用户可立即获得任务 ID

### 6.2 本地模式执行流程

本地模式（`msg-type=noop`）适用于单实例部署或开发测试。`WorkflowTaskSender` 定时轮询数据库，发现 `waiting` 任务后通过 CAS（Compare-And-Swap）操作获取执行权，避免并发冲突。获取成功后直接在本进程内创建 `WorkflowController` 执行任务。

```
WorkflowTaskSender                 数据库                    WorkflowController
       │                             │                              │
       │  定时轮询 (3s)              │                              │
       │────────────────────────────>│                              │
       │  查询 status=waiting        │                              │
       │<────────────────────────────│                              │
       │                             │                              │
       │  CAS 更新 waiting -> queued │                              │
       │────────────────────────────>│                              │
       │                             │                              │
       │  成功获取执行权             │                              │
       │<────────────────────────────│                              │
       │                             │                              │
       │  创建 WorkflowController    │                              │
       │─────────────────────────────────────────────────────────>│
       │                             │                              │
       │                             │  更新 status=running         │
       │                             │<─────────────────────────────│
       │                             │                              │
       │                             │  执行 Jobs                   │
       │                             │<─────────────────────────────│
       │                             │                              │
       │                             │  更新 status=completed       │
       │                             │<─────────────────────────────│
```

**关键点**：
- 无需外部消息队列依赖，简化部署
- CAS 操作确保同一任务不会被重复执行
- 任务在本进程内同步执行

### 6.3 分布式模式执行流程

分布式模式（`msg-type=redis` 或 `msg-type=kafka`）适用于多实例生产部署。Dispatcher 负责发现任务并发布到消息队列，Worker 从消息队列消费并执行。这种职责分离使系统具备水平扩展能力和故障恢复能力。

```
Dispatcher              Redis Streams              Worker                  K8s
    │                        │                        │                      │
    │  轮询 DB: waiting       │                        │                      │
    │──────────>             │                        │                      │
    │  CAS: waiting->queued  │                        │                      │
    │──────────>             │                        │                      │
    │                        │                        │                      │
    │  XADD: taskDispatch    │                        │                      │
    │───────────────────────>│                        │                      │
    │                        │                        │                      │
    │                        │  XREADGROUP            │                      │
    │                        │<───────────────────────│                      │
    │                        │                        │                      │
    │                        │  返回消息              │                      │
    │                        │───────────────────────>│                      │
    │                        │                        │                      │
    │                        │                        │  执行 Jobs           │
    │                        │                        │─────────────────────>│
    │                        │                        │                      │
    │                        │                        │  等待就绪            │
    │                        │                        │<─────────────────────│
    │                        │                        │                      │
    │                        │  XACK                  │                      │
    │                        │<───────────────────────│                      │
```

**关键点**：
- **Dispatcher**：轮询数据库发现 `waiting` 任务，通过 CAS 获取执行权后发布到消息队列
- **消息队列**：作为任务分发通道，支持 Redis Streams 或 Kafka
- **Worker**：从消息队列消费任务，执行完成后 ACK 确认
- **故障恢复**：Worker 崩溃时，Redis 通过 AutoClaim 重新分配消息，Kafka 通过 Rebalance 重新分配分区

**为什么 Dispatcher 要轮询数据库？**
- 任务通过 API 创建，首先写入数据库（保证持久化）
- 消息队列作为"分发通道"而非"任务存储"
- 数据库支持状态查询、取消操作、重启恢复等场景

### 6.4 Step 与 Priority 执行顺序

工作流由多个 Step 组成，每个 Step 包含一组组件。Step 之间按顺序执行，Step 内部根据执行模式（StepByStep 或 DAG）决定组件的执行方式。同时，每个组件生成的 Job 按优先级分组执行，确保依赖资源（如 ConfigMap、Secret）优先创建。

```
Workflow Steps 定义:
┌────────────────────────────────────────────────────────────────────┐
│  Step 1: config-step (StepByStep)     Step 2: services (DAG)      │
│  ┌─────────────────────────────┐      ┌─────────────────────────┐ │
│  │ components: [config,secret] │      │ components: [api,web]   │ │
│  └─────────────────────────────┘      └─────────────────────────┘ │
└────────────────────────────────────────────────────────────────────┘

转换为 StepExecutions:
┌─────────────────────────────────────────────────────────────────────────────┐
│ StepExecution 1   StepExecution 2   StepExecution 3                         │
│ (config)          (secret)          (api + web)                             │
│ mode: StepByStep  mode: StepByStep  mode: DAG                              │
└─────────────────────────────────────────────────────────────────────────────┘

每个 StepExecution 内部按 Priority 执行:
┌─────────────────────────────────────────────────────────────────────────────┐
│ Priority 0 (MaxHigh)  →  Priority 1 (High)  →  Priority 10 (Normal)        │
│ [ConfigMap, Secret]      [PVC, Ingress]         [Deployment, Service]       │
└─────────────────────────────────────────────────────────────────────────────┘
```

**执行模式说明**：

| 模式 | 标识 | 行为 | 适用场景 |
|------|------|------|----------|
| 串行 | `StepByStep` | 组件逐个执行，前一个完成后才执行下一个 | 有依赖关系的组件 |
| 并行 | `DAG` | 同一 Step 内的组件并行执行 | 无依赖关系的组件 |

**优先级说明**：

| 优先级 | 值 | 资源类型 | 原因 |
|--------|-----|----------|------|
| MaxHigh | 0 | ConfigMap, Secret | 被其他资源引用，必须先创建 |
| High | 1 | PVC, ServiceAccount, Role | Deployment/StatefulSet 可能依赖 |
| Normal | 10 | Deployment, StatefulSet, Service | 主要工作负载 |
| Low | 20 | 清理任务、通知任务 | 最后执行 |

**执行顺序总结**：
1. Step 按声明顺序依次执行
2. StepByStep 模式下，组件逐个执行
3. DAG 模式下，组件并行执行
4. 每个组件的 Job 按优先级分组，高优先级先执行
5. 同一优先级的 Job 根据并发配置执行

### 6.5 并发配置对执行的影响

工作流引擎提供 `SequentialMaxConcurrency` 配置，控制 StepByStep 模式下同一优先级内 Job 的最大并发数。

**配置参数**：

| 参数 | 命令行 | 默认值 | 说明 |
|------|--------|--------|------|
| `SequentialMaxConcurrency` | `--workflow-sequential-max-concurrency` | 1 | StepByStep 模式下同优先级 Job 的最大并发数 |

**并发计算规则**：

| 执行模式 | 并发度计算 | 说明 |
|---------|-----------|------|
| DAG（并行） | `并发数 = Job 总数` | 忽略并发配置，所有 Job 全部并行 |
| StepByStep（串行） | `并发数 = min(Job 数量, SequentialMaxConcurrency)` | 受配置限制 |

**重要约束**：无论并发设置多大，**优先级是硬边界**。高优先级的所有 Job 必须全部完成后，才会开始执行低优先级的 Job。

#### 示例 1：StepByStep 模式，SequentialMaxConcurrency=1（默认）

场景：一个应用包含 3 个 config 组件和 2 个 webservice 组件，使用 StepByStep 模式部署。

**创建应用请求 (POST /applications)：**

```json
{
  "name": "demo-app",
  "namespace": "default",
  "version": "1.0.0",
  "project": "demo-project",
  "description": "StepByStep 模式示例应用",
  "component": [
    {
      "name": "app-config-1",
      "type": "config",
      "nameSpace": "default",
      "replicas": 1,
      "properties": {
        "conf": {
          "database.host": "mysql.default.svc",
          "database.port": "3306"
        }
      },
      "traits": {}
    },
    {
      "name": "app-config-2",
      "type": "config",
      "nameSpace": "default",
      "replicas": 1,
      "properties": {
        "conf": {
          "redis.host": "redis.default.svc",
          "redis.port": "6379"
        }
      },
      "traits": {}
    },
    {
      "name": "app-config-3",
      "type": "config",
      "nameSpace": "default",
      "replicas": 1,
      "properties": {
        "conf": {
          "log.level": "info",
          "log.format": "json"
        }
      },
      "traits": {}
    },
    {
      "name": "backend",
      "type": "webservice",
      "image": "myregistry/backend:v1.0.0",
      "nameSpace": "default",
      "replicas": 2,
      "properties": {
        "ports": [{"port": 8080, "expose": true}],
        "env": {
          "APP_ENV": "production"
        }
      },
      "traits": {}
    },
    {
      "name": "frontend",
      "type": "webservice",
      "image": "myregistry/frontend:v1.0.0",
      "nameSpace": "default",
      "replicas": 2,
      "properties": {
        "ports": [{"port": 80, "expose": true}],
        "env": {
          "API_URL": "http://backend:8080"
        }
      },
      "traits": {}
    }
  ],
  "workflow": [
    {
      "name": "deploy-all",
      "mode": "StepByStep",
      "components": ["app-config-1", "app-config-2", "app-config-3", "backend", "frontend"]
    }
  ]
}
```

**执行结果：** 3 个 ConfigMap Job（Priority 0）和 2 个 Deployment Job（Priority 10）

```
时间轴 ───────────────────────────────────────────────────────────────────────────>

Priority 0 (ConfigMap):
  [ConfigMap-1] ─完成─> [ConfigMap-2] ─完成─> [ConfigMap-3] ─完成─┐
                                                                  │
                                                         等待全部完成
                                                                  │
Priority 10 (Deployment):                                         ↓
  [Deployment-1] ─完成─> [Deployment-2] ─完成─> 结束
```

执行过程：
1. Priority 0 的 3 个 ConfigMap Job **逐个串行**执行
2. 全部完成后，才开始 Priority 10
3. Priority 10 的 2 个 Deployment Job **逐个串行**执行

#### 示例 2：StepByStep 模式，SequentialMaxConcurrency=2

同样场景，但设置 `--workflow-sequential-max-concurrency=2`

```
时间轴 ───────────────────────────────────────────────────────────────────────────>

Priority 0 (ConfigMap):
  [ConfigMap-1] ──────┐
                      ├─完成─> [ConfigMap-3] ─完成─┐
  [ConfigMap-2] ──────┘                           │
                                                  │
                                         等待全部完成
                                                  │
Priority 10 (Deployment):                         ↓
  [Deployment-1] ──────┐
                       ├─完成─> 结束
  [Deployment-2] ──────┘
```

执行过程：
1. Priority 0：前 2 个 ConfigMap Job **并行**执行，完成后执行第 3 个
2. Priority 0 全部完成后，才开始 Priority 10
3. Priority 10：2 个 Deployment Job **并行**执行（因为 Job 数 ≤ 并发配置）

#### 示例 3：DAG 模式（忽略并发配置）

场景：DAG 模式下同一 Step 的所有组件并行执行

```
时间轴 ───────────────────────────────────────────────────────────────────────────>

Priority 0 (ConfigMap):
  [ConfigMap-1] ──────┐
  [ConfigMap-2] ──────┼─全部完成─┐
  [ConfigMap-3] ──────┘          │
                                 │
                        等待全部完成
                                 │
Priority 10 (Deployment):        ↓
  [Deployment-1] ──────┐
                       ├─全部完成─> 结束
  [Deployment-2] ──────┘
```

执行过程：
1. Priority 0：所有 ConfigMap Job **全部并行**执行（忽略 SequentialMaxConcurrency）
2. Priority 0 全部完成后，才开始 Priority 10
3. Priority 10：所有 Deployment Job **全部并行**执行

#### 示例 4：多 Step 组合执行

场景：2 个 Step，Step1 是 StepByStep（config 组件），Step2 是 DAG（api + web 组件）

**创建应用请求 (POST /applications)：**

```json
{
  "name": "multi-step-app",
  "namespace": "default",
  "version": "1.0.0",
  "project": "demo-project",
  "description": "多 Step 组合执行示例",
  "component": [
    {
      "name": "config",
      "type": "config",
      "nameSpace": "default",
      "replicas": 1,
      "properties": {
        "conf": {
          "app.name": "multi-step-app",
          "app.env": "production"
        }
      },
      "traits": {}
    },
    {
      "name": "api",
      "type": "webservice",
      "image": "myregistry/api:v1.0.0",
      "nameSpace": "default",
      "replicas": 2,
      "properties": {
        "ports": [{"port": 8080, "expose": true}],
        "env": {
          "SERVICE_NAME": "api"
        }
      },
      "traits": {}
    },
    {
      "name": "web",
      "type": "webservice",
      "image": "myregistry/web:v1.0.0",
      "nameSpace": "default",
      "replicas": 2,
      "properties": {
        "ports": [{"port": 80, "expose": true}],
        "env": {
          "API_URL": "http://api:8080"
        }
      },
      "traits": {}
    }
  ],
  "workflow": [
    {
      "name": "config-step",
      "mode": "StepByStep",
      "components": ["config"]
    },
    {
      "name": "services",
      "mode": "DAG",
      "components": ["api", "web"]
    }
  ]
}
```

**执行结果：**
- Step 1 生成: ConfigMap(Priority 0)
- Step 2 生成: Deployment-api(P10) + Service-api(P10) + Deployment-web(P10) + Service-web(P10)

执行流程（SequentialMaxConcurrency=2）：

```
时间轴 ───────────────────────────────────────────────────────────────────────────>

═══ Step 1: config-step (StepByStep) ═════════════════════════════════════════════

  Priority 0:  [ConfigMap-config] ─完成─┐
                                        │
═══ Step 2: services (DAG) ═════════════╪═════════════════════════════════════════
                                        ↓
  Priority 10: [Deployment-api] ────────┐
               [Deployment-web] ────────┼─全部完成─┐
               [Service-api] ───────────┤          │
               [Service-web] ───────────┘          ↓
                                                 结束
```

执行过程：
1. **Step 1** 执行完成后，才开始 **Step 2**
2. Step 2 是 DAG 模式，同优先级的所有 Job 并行执行

### 6.6 并发控制层级架构

工作流引擎的并发控制分为两个层级：

```
┌─────────────────────────────────────────────────────────────────────────────┐
│              第一层：MaxConcurrentWorkflows（默认 10）                        │
│                        控制同时运行的工作流数量                                │
│                                                                             │
│  ┌───────────┐ ┌───────────┐ ┌───────────┐        ┌───────────┐           │
│  │Workflow 1 │ │Workflow 2 │ │Workflow 3 │  ...   │Workflow 10│           │
│  │(goroutine)│ │(goroutine)│ │(goroutine)│        │(goroutine)│           │
│  │           │ │           │ │           │        │           │           │
│  │ ┌───────┐ │ │ ┌───────┐ │ │ ┌───────┐ │        │ ┌───────┐ │           │
│  │ │Job    │ │ │ │Job    │ │ │ │Job    │ │        │ │Job    │ │           │
│  │ │Pool   │ │ │ │Pool   │ │ │ │Pool   │ │        │ │Pool   │ │           │
│  │ │       │ │ │ │       │ │ │ │       │ │        │ │       │ │           │
│  │ │ ┌───┐ │ │ │ │ ┌───┐ │ │ │ │ ┌───┐ │ │        │ │ ┌───┐ │ │           │
│  │ │ │W1 │ │ │ │ │ │W1 │ │ │ │ │ │W1 │ │ │        │ │ │W1 │ │ │           │
│  │ │ │W2 │ │ │ │ │ │W2 │ │ │ │ │ │W2 │ │ │        │ │ │W2 │ │ │           │
│  │ │ │W3 │ │ │ │ │ │W3 │ │ │ │ │ │...│ │ │        │ │ │...│ │ │           │
│  │ │ └───┘ │ │ │ │ └───┘ │ │ │ │ └───┘ │ │        │ │ └───┘ │ │           │
│  │ └───────┘ │ │ └───────┘ │ │ └───────┘ │        │ └───────┘ │           │
│  └───────────┘ └───────────┘ └───────────┘        └───────────┘           │
│                                                                             │
│              第二层：Job Pool Worker（由执行模式决定）                         │
│              DAG 模式：Worker 数 = Job 数（无限制）                           │
│              StepByStep 模式：Worker 数 = SequentialMaxConcurrency           │
└─────────────────────────────────────────────────────────────────────────────┘
```

#### 并发控制参数说明

| 参数 | 默认值 | 作用层级 | 说明 |
|------|--------|----------|------|
| `MaxConcurrentWorkflows` | 10 | 工作流层 | 同时运行的工作流数量上限 |
| `SequentialMaxConcurrency` | 1 | Job Pool 层 | StepByStep 模式下 Job 并发数 |
| DAG 模式 | 无限制 | Job Pool 层 | 同优先级 Job 全部并行 |

#### 最大并行 Job 数计算公式

```
最大并行 Job 数 = min(请求数, MaxConcurrentWorkflows) × 每个工作流的同优先级并行 Job 数
```

#### 不同场景下的并行 Job 数示例

| 场景 | 并发请求 | 执行模式 | 每工作流 Job 数 | 最大并行 Job |
|------|---------|---------|----------------|-------------|
| 场景 A | 5 | StepByStep（并发=1） | 3 | 5 × 1 = **5** |
| 场景 B | 5 | DAG | 3 | 5 × 3 = **15** |
| 场景 C | 10 | DAG | 3 | 10 × 3 = **30** |
| 场景 D | 15 | DAG | 3 | 10 × 3 = **30**（5个排队） |
| 场景 E | 10 | StepByStep（并发=2） | 4 | 10 × 2 = **20** |

**说明**：
- 场景 D 中，由于 `MaxConcurrentWorkflows=10`，超出的 5 个请求会排队等待
- 实际执行时，还需考虑 Priority 分组，只有同优先级的 Job 才会真正并行
- 每个 webservice 组件可能生成多个 Job（Deployment + Service + PVC/Ingress 等）

#### 代码实现

工作流级别使用信号量控制：

```go
// workflow.go
type Workflow struct {
    workflowLimiter *semaphore.Weighted  // 并发限制器
}

func (w *Workflow) Start(ctx context.Context, errChan chan error) {
    if max := w.maxWorkflowConcurrency(); max > 0 {
        w.workflowLimiter = semaphore.NewWeighted(max)  // 默认 10
    }
}
```

Job Pool 使用 worker 协程池：

```go
// job/job.go
func (p *Pool) Run() {
    for i := 0; i < p.concurrency; i++ {
        go p.work()  // 启动 concurrency 个 worker goroutine
    }
    // 分发任务到 worker
    for _, task := range p.Jobs {
        p.jobsChan <- task
    }
}
```

### 6.7 并发配置建议

| 场景 | 建议值 | 原因 |
|------|--------|------|
| 开发/测试环境 | 1 | 便于调试，日志清晰 |
| 小规模生产 | 2-4 | 平衡执行速度和资源消耗 |
| 大规模部署 | 4-8 | 充分利用 K8s API Server 能力 |
| 资源受限环境 | 1-2 | 避免 API Server 过载 |

**注意事项**：
- 并发数过高可能导致 K8s API Server 压力过大
- 即使设置高并发，Job 的执行仍受 K8s 调度器和资源限制影响
- 建议根据集群规模和 API Server 性能调整
- DAG 模式会显著放大并行度，生产环境需谨慎评估

---

## 7. 分布式支持

### 7.1 Redis Streams 消费者组

工作流引擎使用 Redis Streams 实现分布式任务分发：

```go
// infrastructure/messaging/redis_streams.go
type RedisStreams struct {
    cli    redisCommander
    key    string           // kubemin.workflow.dispatch
    maxLen int64            // 流长度限制
}

// 入队
func (r *RedisStreams) Enqueue(ctx context.Context, payload []byte) (string, error) {
    args := &redis.XAddArgs{
        Stream: r.key,
        Values: map[string]interface{}{"p": payload},
    }
    if r.maxLen > 0 {
        args.MaxLen = r.maxLen  // MAXLEN 限制流长度
    }
    return r.cli.XAdd(ctx, args).Result()
}

// 消费
func (r *RedisStreams) ReadGroup(ctx context.Context, group, consumer string, count int, block time.Duration) ([]Message, error) {
    res, err := r.cli.XReadGroup(ctx, &redis.XReadGroupArgs{
        Group:    group,
        Consumer: consumer,
        Streams:  []string{r.key, ">"},  // > 表示只读取新消息
        Count:    int64(count),
        Block:    block,
        NoAck:    false,
    }).Result()
    // ...
}
```

### 7.2 AutoClaim 消息恢复

当 Worker 崩溃后，未确认的消息可以被其他 Worker 认领：

```go
// event/workflow/dispatcher.go
func (w *Workflow) StartWorker(ctx context.Context, errChan chan error) {
    staleTicker := time.NewTicker(w.workerStaleInterval())  // 15s
    
    for {
        select {
        case <-staleTicker.C:
            // 定期检查并认领过期消息
            mags, err := w.Queue.AutoClaim(ctx, group, consumer, 
                w.workerAutoClaimMinIdle(),  // 60s
                w.workerAutoClaimCount())    // 50
            
            for _, m := range mags {
                if ack, taskID := w.processDispatchMessage(ctx, m); ack {
                    acknowledgements = append(acknowledgements, dispatchAck{id: m.ID, taskID: taskID})
                }
            }
            w.ackDispatchMessages(ctx, group, consumer, acknowledgements)
            
        default:
            // 正常读取新消息
            mags, err := w.Queue.ReadGroup(ctx, group, consumer, 
                w.workerReadCount(),   // 10
                w.workerReadBlock())   // 2s
            // ...
        }
    }
}
```

### 7.3 指数退避重试

Worker 在遇到错误时使用指数退避策略：

```go
func (w *Workflow) StartWorker(ctx context.Context, errChan chan error) {
    backoffMin := w.workerBackoffMin()   // 200ms
    backoffMax := w.workerBackoffMax()   // 5min
    currentDelay := backoffMin
    readFailures := 0
    
    for {
        mags, err := w.Queue.ReadGroup(ctx, ...)
        if err != nil {
            readFailures++
            klog.Warningf("read group error (consecutive: %d): %v", readFailures, err)
            
            // 计算退避时间
            wait := w.workerBackoffDelay(currentDelay, backoffMin, backoffMax)
            currentDelay = wait
            
            select {
            case <-ctx.Done():
                return
            case <-time.After(wait):
            }
            
            // 检查是否达到最大失败次数
            if maxReadFailures > 0 && readFailures >= maxReadFailures {
                klog.Errorf("max read failures reached (%d), worker exiting", maxReadFailures)
                return
            }
            continue
        }
        
        // 成功后重置
        readFailures = 0
        currentDelay = backoffMin
        // ...
    }
}

func (w *Workflow) workerBackoffDelay(current, min, max time.Duration) time.Duration {
    if current < min {
        return min
    }
    next := current * 2  // 指数增长
    if next > max {
        return max
    }
    return next
}
```

---

## 8. 取消与清理机制

### 8.1 取消信号流程

```
用户请求                    WorkflowService               Redis                   Worker
   │                             │                          │                        │
   │ POST /workflow/cancel       │                          │                        │
   │────────────────────────────>│                          │                        │
   │   taskId, reason            │                          │                        │
   │                             │                          │                        │
   │                             │  更新 DB: cancelled      │                        │
   │                             │─────────>                │                        │
   │                             │                          │                        │
   │                             │  SET cancel:taskId       │                        │
   │                             │  "cancelled:reason"      │                        │
   │                             │─────────────────────────>│                        │
   │                             │                          │                        │
   │                             │                          │  maintain() 检测到变更  │
   │                             │                          │───────────────────────>│
   │                             │                          │                        │
   │                             │                          │  触发 ctx.Cancel()     │
   │                             │                          │<───────────────────────│
   │                             │                          │                        │
   │                             │                          │  Job.Clean() 清理资源  │
   │                             │                          │<───────────────────────│
   │                             │                          │                        │
   │  返回成功                   │                          │                        │
   │<────────────────────────────│                          │                        │
```

### 8.2 资源清理跟踪器

清理跟踪器记录每个 Job 创建的资源，便于失败时精确清理：

```go
// event/workflow/job/cleanup_tracker.go
type CleanupTracker struct {
    mu        sync.Mutex
    resources map[config.ResourceKind][]ResourceRef
}

type ResourceRef struct {
    Name      string
    Namespace string
    Created   bool  // true=本次创建, false=已存在只是观察
}

// MarkResourceCreated 标记资源为本次创建
func MarkResourceCreated(ctx context.Context, kind config.ResourceKind, namespace, name string) {
    tracker := trackerFromContext(ctx)
    if tracker == nil {
        return
    }
    tracker.mu.Lock()
    defer tracker.mu.Unlock()
    tracker.resources[kind] = append(tracker.resources[kind], ResourceRef{
        Name:      name,
        Namespace: namespace,
        Created:   true,
    })
}

// markResourceObserved 标记资源为已存在（更新场景）
func markResourceObserved(ctx context.Context, kind config.ResourceKind, namespace, name string) {
    tracker := trackerFromContext(ctx)
    if tracker == nil {
        return
    }
    tracker.mu.Lock()
    defer tracker.mu.Unlock()
    tracker.resources[kind] = append(tracker.resources[kind], ResourceRef{
        Name:      name,
        Namespace: namespace,
        Created:   false,  // 不是本次创建，清理时跳过
    })
}

// resourcesForCleanup 获取需要清理的资源
func resourcesForCleanup(ctx context.Context, kind config.ResourceKind) []ResourceRef {
    tracker := trackerFromContext(ctx)
    if tracker == nil {
        return nil
    }
    tracker.mu.Lock()
    defer tracker.mu.Unlock()
    return tracker.resources[kind]
}
```

### 8.3 Job 清理实现

```go
// event/workflow/job/job.go
func runJob(ctx context.Context, job *model.JobTask, ...) {
    ctx = WithCleanupTracker(ctx)  // 注入清理跟踪器
    
    // 设置取消信号监听
    if taskID := TaskIDFromContext(ctx); taskID != "" {
        watcher, jobCtx, cancelFn, _ = signal.Watch(ctx, taskID)
    }
    
    defer func() {
        // panic 恢复时清理
        if r := recover(); r != nil {
            if !cleaned {
                jobCtl.Clean(jobCtx)
                cleaned = true
            }
            job.Status = config.StatusFailed
        }
    }()
    
    if err := jobCtl.Run(jobCtx); err != nil {
        // 执行失败时清理
        if !cleaned {
            jobCtl.Clean(jobCtx)
            cleaned = true
        }
        
        // 根据错误类型设置状态
        if errors.Is(err, context.Canceled) {
            job.Status = config.StatusCancelled
        } else {
            job.Status = config.StatusFailed
        }
    }
    
    // 失败状态也需要清理
    if !cleaned && jobStatusFailed(job.Status) {
        jobCtl.Clean(jobCtx)
    }
}
```

---

## 9. 状态管理

### 9.1 任务状态转换

```go
// 任务状态定义
const (
    StatusWaiting   Status = "waiting"   // 等待执行
    StatusQueued    Status = "queued"    // 已入队
    StatusRunning   Status = "running"   // 执行中
    StatusCompleted Status = "completed" // 完成
    StatusFailed    Status = "failed"    // 失败
    StatusTimeout   Status = "timeout"   // 超时
    StatusCancelled Status = "cancelled" // 取消
)
```

### 9.2 状态转换规则

| 当前状态 | 可转换到 | 触发条件 |
|----------|----------|----------|
| waiting | queued | Dispatcher 获取到执行权 |
| queued | running | Worker 开始执行 |
| running | completed | 所有 Job 执行成功 |
| running | failed | 任一 Job 执行失败 |
| running | timeout | Job 执行超时 |
| running | cancelled | 收到取消信号 |
| queued | waiting | 分发失败，回滚状态 |

### 9.3 CAS 状态更新

为防止并发冲突，状态转换使用 CAS (Compare-And-Swap)：

```go
// domain/repository/workflow.go
func UpdateTaskStatus(ctx context.Context, store datastore.DataStore, taskID string, from, to config.Status) (bool, error) {
    // 使用条件更新：WHERE task_id=? AND status=?
    result := db.Model(&model.WorkflowQueue{}).
        Where("task_id = ? AND status = ?", taskID, from).
        Update("status", to)
    
    if result.RowsAffected == 0 {
        return false, nil  // 状态已被其他实例修改
    }
    return true, nil
}
```

### 9.4 ACK 回调机制

`ack` 回调确保状态变更及时持久化：

```go
// controller.go
func NewWorkflowController(workflowTask *model.WorkflowQueue, ...) *WorkflowCtl {
    ctl := &WorkflowCtl{
        workflowTask: workflowTask,
        // ...
    }
    ctl.ack = ctl.updateWorkflowTask  // 绑定 ACK 回调
    return ctl
}

func (w *WorkflowCtl) updateWorkflowTask() {
    taskSnapshot := w.snapshotTask()
    
    // 终态不再更新
    if isWorkflowTerminal(taskSnapshot.Status) {
        return
    }
    
    ctx := w.ctx
    if ctx == nil {
        ctx = context.Background()
    }
    w.Store.Put(ctx, &taskSnapshot)
}
```

---

## 10. 并发控制

### 10.1 工作流级别并发限制

使用信号量限制同时运行的工作流数量：

```go
// workflow.go
type Workflow struct {
    workflowLimiter *semaphore.Weighted
    // ...
}

func (w *Workflow) Start(ctx context.Context, errChan chan error) {
    if max := w.maxWorkflowConcurrency(); max > 0 {
        w.workflowLimiter = semaphore.NewWeighted(max)  // 默认 10
    }
    // ...
}

func (w *Workflow) runWorkflowTask(ctx context.Context, task *model.WorkflowQueue, concurrency int) {
    acquired := false
    if w.workflowLimiter != nil {
        // 获取执行槽位
        if err := w.workflowLimiter.Acquire(runnerCtx, 1); err != nil {
            w.reportTaskError(fmt.Errorf("acquire workflow slot: %w", err))
            return
        }
        acquired = true
    }
    
    w.taskGroup.Go(func() error {
        controller := NewWorkflowController(taskCopy, w.KubeClient, w.Store, w.Cfg)
        err := controller.Run(runnerCtx, concurrency)
        
        if acquired {
            w.workflowLimiter.Release(1)  // 释放槽位
        }
        return err
    })
}
```

### 10.2 Step 内 Job 并发控制

```go
// controller.go
func determineStepConcurrency(mode config.WorkflowMode, jobCount, sequentialLimit int) int {
    if jobCount <= 0 {
        return 0
    }
    if mode.IsParallel() {
        return jobCount  // 并行模式：全部并发执行
    }
    // 串行模式：受 sequentialLimit 限制
    if sequentialLimit < 1 {
        sequentialLimit = 1
    }
    if jobCount < sequentialLimit {
        return jobCount
    }
    return sequentialLimit
}
```

### 10.3 Job Pool 并发执行

```go
// job/job.go
type Pool struct {
    Jobs          []*model.JobTask
    concurrency   int
    jobsChan      chan *model.JobTask
    stopOnFailure bool
    wg            sync.WaitGroup
    failureOnce   sync.Once
}

func (p *Pool) Run() {
    defer p.cancel()
    
    // 启动 worker goroutines
    for i := 0; i < p.concurrency; i++ {
        go p.work()
    }
    
    // 分发任务
    for _, task := range p.Jobs {
        if p.stopOnFailure && p.ctx.Err() != nil {
            break  // 停止分发
        }
        p.wg.Add(1)
        p.jobsChan <- task
    }
    
    close(p.jobsChan)
    p.wg.Wait()
}

func (p *Pool) work() {
    for job := range p.jobsChan {
        runJob(p.ctx, job, p.client, p.store, p.ack)
        
        if p.stopOnFailure && jobStatusFailed(job.Status) {
            p.failureOnce.Do(func() {
                p.cancel()  // 通知其他 worker 停止
            })
        }
        p.wg.Done()
    }
}
```

---

## 11. 配置参考

### 11.1 WorkflowRuntimeConfig

```go
// config/config.go
type WorkflowRuntimeConfig struct {
    // 串行步骤内部最大并发数（默认 1）
    SequentialMaxConcurrency int
    
    // 本地模式轮询间隔（默认 3s）
    LocalPollInterval time.Duration
    
    // Dispatcher 扫描间隔（默认 3s）
    DispatchPollInterval time.Duration
    
    // Worker 过期检查间隔（默认 15s）
    WorkerStaleInterval time.Duration
    
    // AutoClaim 最小空闲时间（默认 60s）
    WorkerAutoClaimMinIdle time.Duration
    
    // AutoClaim 批量大小（默认 50）
    WorkerAutoClaimCount int
    
    // Worker 单次读取消息数（默认 10）
    WorkerReadCount int
    
    // Worker 阻塞读取超时（默认 2s）
    WorkerReadBlock time.Duration
    
    // Job 默认超时时间（默认 60s）
    DefaultJobTimeout time.Duration
    
    // 最大并发工作流数（默认 10）
    MaxConcurrentWorkflows int
    
    // Worker 最大连续读取失败次数（0=无限重试）
    WorkerMaxReadFailures int
    
    // Worker 最大连续认领失败次数（0=无限重试）
    WorkerMaxClaimFailures int
    
    // 退避最小时间（默认 200ms）
    WorkerBackoffMin time.Duration
    
    // 退避最大时间（默认 5min）
    WorkerBackoffMax time.Duration
}
```

### 11.2 MessagingConfig

```go
// config/config.go
type MessagingConfig struct {
    // 消息队列类型：noop | redis | kafka
    Type          string
    
    // 消息通道/Topic 前缀
    ChannelPrefix string
    
    // === Redis 配置 ===
    // Redis Stream 最大长度，<=0 表示不限制
    RedisStreamMaxLen int64
    
    // === Kafka 配置 ===
    // Kafka Broker 地址列表
    KafkaBrokers []string
    
    // Kafka 消费者组 ID（默认: kubemin-workflow-workers）
    KafkaGroupID string
    
    // Kafka 偏移量重置策略: earliest | latest（默认: earliest）
    KafkaAutoOffsetReset string
}
```

### 11.3 命令行参数详解

#### 工作流参数

| 参数 | 默认值 | 说明 | 推荐配置 |
|------|--------|------|----------|
| `--workflow-sequential-max-concurrency` | 1 | 串行步骤内部最大并发数 | 生产环境建议 1-3 |
| `--workflow-local-poll-interval` | 3s | 本地模式轮询间隔 | 开发环境可设为 1s |
| `--workflow-dispatch-poll-interval` | 3s | Dispatcher 扫描间隔 | 生产环境建议 3-5s |
| `--workflow-worker-stale-interval` | 15s | Worker 过期检查间隔 | 生产环境建议 15-30s |
| `--workflow-worker-autoclaim-idle` | 60s | AutoClaim 最小空闲时间 | 应大于 Job 最大执行时间 |
| `--workflow-worker-autoclaim-count` | 50 | AutoClaim 批量大小 | 根据任务量调整 |
| `--workflow-worker-read-count` | 10 | Worker 单次读取消息数 | 根据处理能力调整 |
| `--workflow-worker-read-block` | 2s | Worker 阻塞读取超时 | 建议 2-5s |
| `--workflow-default-job-timeout` | 60s | Job 默认超时时间 | 根据业务需求调整 |
| `--workflow-max-concurrent` | 10 | 最大并发工作流数 | 根据资源限制调整 |

#### 消息队列参数

| 参数 | 默认值 | 说明 | 推荐配置 |
|------|--------|------|----------|
| `--msg-type` | redis | 消息队列类型 | noop/redis/kafka |
| `--msg-channel-prefix` | kubemin | 消息通道前缀 | 根据环境区分，如 kubemin-prod |
| `--msg-redis-maxlen` | 50000 | Redis Stream 最大长度 | 根据消息量和内存调整 |

#### Kafka 参数

| 参数 | 默认值 | 说明 | 推荐配置 |
|------|--------|------|----------|
| `--msg-kafka-brokers` | - | Kafka Broker 地址 | 生产环境建议配置多个 |
| `--msg-kafka-group-id` | kubemin-workflow-workers | 消费者组 ID | 不同环境使用不同 ID |
| `--msg-kafka-offset-reset` | earliest | 偏移量重置策略 | 生产环境建议 earliest |

### 11.4 命令行参数示例

```bash
# 工作流相关参数
--workflow-sequential-max-concurrency=3     # 串行步骤并发数
--workflow-local-poll-interval=3s           # 本地轮询间隔
--workflow-dispatch-poll-interval=3s        # Dispatcher 扫描间隔
--workflow-worker-stale-interval=15s        # 过期检查间隔
--workflow-worker-autoclaim-idle=60s        # AutoClaim 空闲时间
--workflow-worker-autoclaim-count=50        # AutoClaim 批量大小
--workflow-worker-read-count=10             # 单次读取数
--workflow-worker-read-block=2s             # 阻塞读取超时
--workflow-default-job-timeout=60s          # Job 默认超时
--workflow-max-concurrent=10                # 最大并发工作流数

# 消息队列相关参数
--msg-type=redis                            # 队列类型：noop|redis|kafka
--msg-channel-prefix=kubemin                # 消息通道前缀
--msg-redis-maxlen=50000                    # Redis Stream 最大长度

# Kafka 相关参数（当 msg-type=kafka 时使用）
--msg-kafka-brokers=localhost:9092          # Kafka broker 地址列表
--msg-kafka-group-id=kubemin-workflow       # Kafka 消费者组 ID
--msg-kafka-offset-reset=earliest           # 偏移量重置策略：earliest|latest
```

### 11.5 配置示例

#### 开发环境配置（本地模式）

```yaml
# config-dev.yaml - 开发环境使用本地模式，无需外部队列依赖
workflow:
  sequentialMaxConcurrency: 1
  localPollInterval: 1s          # 更快的轮询
  dispatchPollInterval: 1s
  workerStaleInterval: 10s
  workerAutoClaimMinIdle: 30s
  workerAutoClaimCount: 10
  workerReadCount: 5
  workerReadBlock: 1s
  defaultJobTimeout: 30s
  maxConcurrentWorkflows: 5
  workerMaxReadFailures: 0
  workerMaxClaimFailures: 0
  workerBackoffMin: 100ms
  workerBackoffMax: 1m

messaging:
  type: noop                     # 本地模式，无需 Redis/Kafka
  channelPrefix: kubemin-dev
```

#### 生产环境配置（Redis 模式）

```yaml
# config-prod-redis.yaml - 中等规模生产环境推荐配置
workflow:
  sequentialMaxConcurrency: 3
  localPollInterval: 3s
  dispatchPollInterval: 3s
  workerStaleInterval: 15s
  workerAutoClaimMinIdle: 60s    # 确保大于 defaultJobTimeout
  workerAutoClaimCount: 50
  workerReadCount: 10
  workerReadBlock: 2s
  defaultJobTimeout: 60s
  maxConcurrentWorkflows: 20     # 根据 K8s API 承载能力调整
  workerMaxReadFailures: 0       # 无限重试，增强弹性
  workerMaxClaimFailures: 0
  workerBackoffMin: 200ms
  workerBackoffMax: 5m

messaging:
  type: redis
  channelPrefix: kubemin-prod
  redisStreamMaxLen: 100000      # 根据内存和消息量调整

# Redis 连接配置（复用 Cache 配置）
cache:
  cacheHost: redis.prod.svc.cluster.local
  cacheProt: 6379
  cacheType: redis
  cacheDB: 0
  cacheTTL: 24h
  keyPrefix: "kubemin:cache:"
```

#### 大规模生产环境配置（Kafka 模式）

```yaml
# config-prod-kafka.yaml - 大规模生产环境推荐配置
workflow:
  sequentialMaxConcurrency: 5
  localPollInterval: 3s
  dispatchPollInterval: 3s
  workerStaleInterval: 20s
  workerAutoClaimMinIdle: 120s   # Kafka rebalance 可能需要更长时间
  workerAutoClaimCount: 100
  workerReadCount: 20            # Kafka 吞吐量高，可增加读取数
  workerReadBlock: 3s
  defaultJobTimeout: 120s
  maxConcurrentWorkflows: 50     # 大规模部署
  workerMaxReadFailures: 0
  workerMaxClaimFailures: 0
  workerBackoffMin: 200ms
  workerBackoffMax: 5m

messaging:
  type: kafka
  channelPrefix: kubemin-prod
  kafkaBrokers:
    - kafka-0.kafka.prod.svc.cluster.local:9092
    - kafka-1.kafka.prod.svc.cluster.local:9092
    - kafka-2.kafka.prod.svc.cluster.local:9092
  kafkaGroupID: kubemin-workflow-workers
  kafkaAutoOffsetReset: earliest
```

### 11.6 配置调优建议

#### 性能调优

| 场景 | 推荐配置 | 说明 |
|------|----------|------|
| 高吞吐量 | `workerReadCount=20`, `maxConcurrentWorkflows=50` | 增加并发处理能力 |
| 低延迟 | `dispatchPollInterval=1s`, `workerReadBlock=1s` | 减少轮询间隔 |
| 资源受限 | `maxConcurrentWorkflows=5`, `workerReadCount=3` | 限制并发避免过载 |
| 长时任务 | `defaultJobTimeout=300s`, `workerAutoClaimMinIdle=360s` | 延长超时时间 |

#### 消息队列选型建议

| 场景 | 推荐 | 原因 |
|------|------|------|
| 开发测试 | noop | 无需外部依赖，快速启动 |
| 中小规模 (<100 任务/s) | redis | 部署简单，延迟低 |
| 大规模 (>100 任务/s) | kafka | 高吞吐量，强持久化 |
| 已有 Redis 基础设施 | redis | 复用现有资源 |
| 已有 Kafka 基础设施 | kafka | 复用现有资源 |
| 需要消息回溯 | kafka | 支持历史消息重放 |

#### 关键配置约束

1. **workerAutoClaimMinIdle > defaultJobTimeout**：确保任务超时后才被认领
2. **workerStaleInterval < workerAutoClaimMinIdle**：确保定期检查过期任务
3. **Redis maxLen**：根据消息量和内存设置，防止内存溢出
4. **Kafka partitions >= workers**：确保每个 Worker 都能分配到分区

---

## 12. 优势总结

### 12.1 多模式部署灵活性

| 特性 | 本地模式 | Redis 分布式 | Kafka 分布式 |
|------|----------|--------------|--------------|
| 依赖 | 仅需 MySQL | MySQL + Redis | MySQL + Kafka |
| 部署 | 单实例 | 多实例 | 多实例 |
| 故障恢复 | 进程重启后恢复 | AutoClaim 自动恢复 | Rebalance 自动恢复 |
| 适用场景 | 开发测试 | 生产环境 | 大规模生产环境 |
| 吞吐量 | 低 | 中等 | 高 |
| 配置 | msg-type=noop | msg-type=redis | msg-type=kafka |

### 12.2 完善的可观测性

- **分布式追踪**：集成 OpenTelemetry，支持 Jaeger 等后端
- **结构化日志**：使用 klog 带 traceID、workflowName、taskID
- **状态查询**：提供 `/workflow/tasks/:taskID/status` API
- **组件级状态**：细粒度追踪每个组件的执行状态

### 12.3 安全的资源清理

- **精确清理**：只清理本次创建的资源，不影响已有资源
- **幂等设计**：支持重复执行，更新场景不会误删
- **超时保护**：清理过程有 30s 超时限制
- **panic 恢复**：异常情况下也能正确清理

### 12.4 高可用的消息处理

- **消费者组**：支持多 Worker 负载均衡
- **消息恢复**：Redis 使用 AutoClaim 自动认领超时消息，Kafka 使用原生 Rebalance 机制
- **指数退避**：错误时智能重试，避免雪崩
- **弹性配置**：可配置最大失败次数，0 表示无限重试
- **多后端支持**：支持 Redis Streams 和 Apache Kafka 两种分布式队列

### 12.5 灵活的执行控制

- **优先级调度**：确保依赖资源优先创建
- **串行/并行模式**：满足不同业务场景
- **并发限制**：保护 K8s API Server 和集群资源
- **取消支持**：支持用户主动取消和超时取消

---

## 附录

### A. 关键代码文件索引

| 功能 | 文件路径 |
|------|----------|
| 工作流入口 | `pkg/apiserver/event/workflow/workflow.go` |
| 任务控制器 | `pkg/apiserver/event/workflow/controller.go` |
| 消息分发 | `pkg/apiserver/event/workflow/dispatcher.go` |
| Job 构建器 | `pkg/apiserver/event/workflow/job_builder.go` |
| Job 执行器 | `pkg/apiserver/event/workflow/job/job.go` |
| Deployment 控制器 | `pkg/apiserver/event/workflow/job/job_deploy.go` |
| StatefulSet 控制器 | `pkg/apiserver/event/workflow/job/job_statefulset.go` |
| 清理跟踪器 | `pkg/apiserver/event/workflow/job/cleanup_tracker.go` |
| 取消信号 | `pkg/apiserver/workflow/signal/cancel.go` |
| 队列接口 | `pkg/apiserver/infrastructure/messaging/queue.go` |
| Redis Streams | `pkg/apiserver/infrastructure/messaging/redis_streams.go` |
| Kafka Queue | `pkg/apiserver/infrastructure/messaging/kafka.go` |
| 数据模型 | `pkg/apiserver/domain/model/workflow.go` |
| 配置定义 | `pkg/apiserver/config/config.go` |
| 常量定义 | `pkg/apiserver/config/consts.go` |

### B. API 接口列表

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/applications/:appID/workflow` | 创建工作流 |
| PUT | `/applications/:appID/workflow` | 更新工作流 |
| POST | `/applications/:appID/workflow/exec` | 执行工作流任务 |
| POST | `/applications/:appID/workflow/cancel` | 取消工作流任务 |
| GET | `/workflow/tasks/:taskID/status` | 查询任务状态 |

### C. 状态码定义

```go
// utils/bcode/002_workflow.go
var (
    ErrWorkflowExist        = NewBcode(40001, "workflow already exists")
    ErrWorkflowNotExist     = NewBcode(40002, "workflow not found")
    ErrCreateWorkflow       = NewBcode(40003, "failed to create workflow")
    ErrExecWorkflow         = NewBcode(40004, "failed to execute workflow")
    ErrWorkflowTaskNotExist = NewBcode(40005, "workflow task not found")
)
```

---

*文档版本：1.0.0*
*最后更新：2025-12*

