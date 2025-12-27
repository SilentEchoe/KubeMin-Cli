# KubeMin-Cli 文档向量化 Job 设计文档

## 目录

- [1. 概述与需求分析](#1-概述与需求分析)
- [2. 架构设计](#2-架构设计)
- [3. 数据模型设计](#3-数据模型设计)
- [4. 详细实现步骤](#4-详细实现步骤)
- [5. 文本分块策略](#5-文本分块策略)
- [6. 嵌入模型部署](#6-嵌入模型部署)
- [7. 工作流触发机制](#7-工作流触发机制)
- [8. API 接口设计](#8-api-接口设计)
- [9. 配置参考](#9-配置参考)
- [10. 部署清单](#10-部署清单)

---

## 1. 概述与需求分析

### 1.1 背景与目标

随着大语言模型（LLM）和检索增强生成（RAG）技术的普及，将企业文档转换为向量嵌入并存储在向量数据库中，已成为构建智能知识库、语义搜索和问答系统的关键基础设施。

本设计旨在扩展 KubeMin-Cli 工作流引擎，增加文档向量化 Job 能力，使其能够：

- **文档解析**：支持 PDF、Word、Markdown、HTML、纯文本等多种文档格式
- **文本分块**：将长文档智能切分为适合嵌入的文本块
- **向量嵌入**：调用嵌入模型将文本转换为高维向量
- **向量存储**：将向量及元数据持久化到 Milvus 向量数据库

### 1.2 业务场景

| 场景 | 描述 | 典型用例 |
|------|------|----------|
| **RAG 知识库** | 将企业文档向量化，为 LLM 提供检索上下文 | 智能客服、文档问答 |
| **语义搜索** | 基于语义相似度的文档检索 | 知识管理、合规审计 |
| **文档聚类** | 基于向量相似度的文档自动分类 | 内容推荐、去重 |
| **多模态处理** | 图文混合文档的向量化处理 | 产品手册、技术文档 |

### 1.3 与现有工作流的差异

| 特性 | K8s 部署工作流 | 向量化工作流 |
|------|----------------|--------------|
| **任务类型** | 创建/更新 K8s 资源 | 数据处理（解析、嵌入、存储） |
| **执行位置** | Kubernetes API Server | 嵌入模型服务 + 向量数据库 |
| **输入** | 组件配置 JSON | 文档文件（URL/S3/本地路径） |
| **输出** | K8s 资源对象 | 向量嵌入 (float[]) + 元数据 |
| **状态判断** | Pod Ready / Deployment Available | 向量化完成百分比 |
| **依赖服务** | Kubernetes 集群 | 嵌入模型、向量数据库、对象存储 |

### 1.4 支持的文档类型

| 格式 | 扩展名 | 解析方案 | 特殊处理 |
|------|--------|----------|----------|
| PDF | `.pdf` | Apache Tika / PyMuPDF | OCR 支持（可选） |
| Word | `.docx`, `.doc` | Apache Tika / python-docx | 表格、样式保留 |
| Markdown | `.md` | 原生解析 | 代码块、链接提取 |
| HTML | `.html`, `.htm` | BeautifulSoup / Readability | 正文提取、去噪 |
| 纯文本 | `.txt` | 直接读取 | 编码检测 |
| JSON/JSONL | `.json`, `.jsonl` | 结构化解析 | 字段映射 |

### 1.5 向量存储方案：Milvus

选择 Milvus 作为向量数据库，原因如下：

| 特性 | Milvus | 备选方案对比 |
|------|--------|--------------|
| **性能** | 毫秒级检索，支持万亿级向量 | Qdrant/pgvector 适合中小规模 |
| **生态** | 云原生，K8s 原生部署 | 与现有架构契合 |
| **功能** | 混合检索、分区、索引类型丰富 | 支持复杂查询场景 |
| **社区** | CNCF 毕业项目，活跃维护 | 长期技术支持 |

---

## 2. 架构设计

### 2.1 整体架构图

```
┌──────────────────────────────────────────────────────────────────────────────────┐
│                              API Layer                                            │
│  ┌──────────────────────────────────────────────────────────────────────────────┐ │
│  │  POST /vectorize/tasks                  创建向量化任务                        │ │
│  │  GET  /vectorize/tasks/:taskID/status   查询任务状态                          │ │
│  │  POST /vectorize/tasks/:taskID/cancel   取消任务                              │ │
│  └──────────────────────────────────────────────────────────────────────────────┘ │
└──────────────────────────────────────────────────────────────────────────────────┘
                                       │
                                       ▼
┌──────────────────────────────────────────────────────────────────────────────────┐
│                           Workflow Engine (复用现有)                              │
│  ┌─────────────────────────────┐    ┌─────────────────────────────────────────┐  │
│  │       Dispatcher            │    │       Worker                            │  │
│  │  ┌───────────────────────┐  │    │  ┌───────────────────────────────────┐  │  │
│  │  │ 轮询 DB 发现任务       │  │    │  │ 消费 Kafka/Redis 消息             │  │  │
│  │  │ 发布到消息队列         │──┼───>│  │ 执行 VectorJobCtl                 │  │  │
│  │  └───────────────────────┘  │    │  └───────────────────────────────────┘  │  │
│  └─────────────────────────────┘    └─────────────────────────────────────────┘  │
└──────────────────────────────────────────────────────────────────────────────────┘
                                       │
                    ┌──────────────────┼──────────────────┐
                    ▼                  ▼                  ▼
┌─────────────────────────┐  ┌─────────────────────┐  ┌─────────────────────────┐
│   Document Parser       │  │   Embedding Model   │  │   Vector Database       │
│  ┌───────────────────┐  │  │  ┌───────────────┐  │  │  ┌───────────────────┐  │
│  │ Apache Tika       │  │  │  │ BGE-M3        │  │  │  │ Milvus            │  │
│  │ Unstructured.io   │  │  │  │ Ollama        │  │  │  │ Collection        │  │
│  │ PyMuPDF           │  │  │  │ HuggingFace   │  │  │  │ Partition         │  │
│  └───────────────────┘  │  │  │ TEI           │  │  │  └───────────────────┘  │
└─────────────────────────┘  │  └───────────────┘  │  └─────────────────────────┘
                             └─────────────────────┘
```

### 2.2 模块扩展设计

基于现有工作流引擎，需要扩展以下模块：

```
pkg/apiserver/
├── config/
│   └── consts.go                    # [修改] 新增 VectorJob, JobVectorize 类型
│
├── domain/
│   ├── model/
│   │   └── vectorize.go             # [新增] 向量化任务数据模型
│   └── spec/
│       └── vectorize.go             # [新增] 向量化属性定义
│
├── event/workflow/
│   ├── job_builder.go               # [修改] 扩展 buildJobsForComponent
│   └── job/
│       ├── job.go                   # [修改] 扩展 initJobCtl
│       ├── job_vectorize.go         # [新增] VectorJobCtl 实现
│       └── job_vectorize_test.go    # [新增] 单元测试
│
├── infrastructure/
│   ├── vectorize/                   # [新增] 向量化基础设施
│   │   ├── client.go                # 嵌入模型客户端接口
│   │   ├── ollama.go                # Ollama 实现
│   │   ├── tei.go                   # HuggingFace TEI 实现
│   │   ├── parser.go                # 文档解析器
│   │   ├── chunker.go               # 文本分块器
│   │   └── config.go                # 向量化配置
│   └── storage/
│       ├── milvus.go                # [新增] Milvus 向量存储客户端
│       └── milvus_test.go           # [新增] 单元测试
│
└── interfaces/api/
    └── vectorize.go                 # [新增] 向量化 API 接口
```

### 2.3 向量化 Job 执行流程

```
┌─────────────────────────────────────────────────────────────────────────────────┐
│                        VectorJobCtl 执行流程                                     │
└─────────────────────────────────────────────────────────────────────────────────┘

时间轴 ─────────────────────────────────────────────────────────────────────────────>

Phase 1: 初始化                Phase 2: 文档处理              Phase 3: 向量存储
┌─────────────────┐            ┌─────────────────┐            ┌─────────────────┐
│  1. 解析任务配置  │            │  3. 获取文档内容  │            │  6. 连接 Milvus  │
│  2. 初始化客户端  │ ────────> │  4. 文本分块      │ ────────> │  7. 批量写入向量  │
│     - Parser    │            │  5. 调用嵌入模型  │            │  8. 更新索引     │
│     - Embedder  │            │     生成向量     │            │  9. 返回统计信息  │
│     - Milvus    │            │                 │            │                 │
└─────────────────┘            └─────────────────┘            └─────────────────┘
       │                              │                              │
       ▼                              ▼                              ▼
   状态: prepare               状态: running                  状态: completed
   进度: 0%                    进度: 10% -> 90%               进度: 100%
```

### 2.4 与现有工作流的集成点

| 集成点 | 文件位置 | 修改内容 |
|--------|----------|----------|
| Job 类型定义 | `config/consts.go` | 新增 `VectorJob` 和 `JobVectorize` |
| Job 控制器初始化 | `job/job.go:initJobCtl` | 添加 `case string(config.JobVectorize)` |
| 组件构建逻辑 | `job_builder.go:buildJobsForComponent` | 添加 `case config.VectorJob` |
| 消息队列 | `infrastructure/messaging/` | 复用现有 Kafka/Redis 队列 |
| 状态管理 | `workflow_state.go` | 复用现有状态转换逻辑 |

---

## 3. 数据模型设计

### 3.1 向量化任务配置 (VectorizeSpec)

```go
// pkg/apiserver/domain/spec/vectorize.go

package spec

// VectorizeSpec 向量化任务配置
type VectorizeSpec struct {
    // 数据源配置
    Source SourceConfig `json:"source"`
    
    // 文本处理配置
    Processing ProcessingConfig `json:"processing"`
    
    // 嵌入模型配置
    Embedding EmbeddingConfig `json:"embedding"`
    
    // 向量存储配置
    Storage VectorStorageConfig `json:"storage"`
    
    // 任务行为配置
    Behavior BehaviorConfig `json:"behavior,omitempty"`
}

// SourceConfig 数据源配置
type SourceConfig struct {
    // 数据源类型: file, s3, http, configmap
    Type string `json:"type"`
    
    // 文件路径或 URL 列表
    Paths []string `json:"paths,omitempty"`
    
    // S3 配置
    S3 *S3SourceConfig `json:"s3,omitempty"`
    
    // HTTP 配置
    HTTP *HTTPSourceConfig `json:"http,omitempty"`
    
    // ConfigMap 引用
    ConfigMapRef *ConfigMapRefConfig `json:"configMapRef,omitempty"`
    
    // 文件过滤
    FileFilter *FileFilterConfig `json:"fileFilter,omitempty"`
}

// S3SourceConfig S3 数据源配置
type S3SourceConfig struct {
    Bucket    string `json:"bucket"`
    Prefix    string `json:"prefix,omitempty"`
    Region    string `json:"region,omitempty"`
    Endpoint  string `json:"endpoint,omitempty"`
    SecretRef string `json:"secretRef,omitempty"` // 引用 K8s Secret
}

// HTTPSourceConfig HTTP 数据源配置
type HTTPSourceConfig struct {
    URLs    []string          `json:"urls"`
    Headers map[string]string `json:"headers,omitempty"`
    Timeout string            `json:"timeout,omitempty"` // e.g., "30s"
}

// ConfigMapRefConfig ConfigMap 引用配置
type ConfigMapRefConfig struct {
    Name      string `json:"name"`
    Namespace string `json:"namespace,omitempty"`
    Key       string `json:"key,omitempty"` // 如果为空，处理所有 keys
}

// FileFilterConfig 文件过滤配置
type FileFilterConfig struct {
    Extensions []string `json:"extensions,omitempty"` // [".pdf", ".docx", ".md"]
    MaxSize    string   `json:"maxSize,omitempty"`    // e.g., "10Mi"
    MinSize    string   `json:"minSize,omitempty"`    // e.g., "1Ki"
}

// ProcessingConfig 文本处理配置
type ProcessingConfig struct {
    // 分块策略: fixed, sentence, paragraph, semantic
    ChunkStrategy string `json:"chunkStrategy,omitempty"`
    
    // 分块大小（字符数）
    ChunkSize int `json:"chunkSize,omitempty"`
    
    // 分块重叠（字符数）
    ChunkOverlap int `json:"chunkOverlap,omitempty"`
    
    // 语言（用于分句）
    Language string `json:"language,omitempty"` // zh, en, auto
    
    // 是否启用 OCR
    EnableOCR bool `json:"enableOCR,omitempty"`
    
    // 元数据提取
    ExtractMetadata bool `json:"extractMetadata,omitempty"`
}

// EmbeddingConfig 嵌入模型配置
type EmbeddingConfig struct {
    // 模型提供者: ollama, tei, openai, custom
    Provider string `json:"provider"`
    
    // 模型名称
    Model string `json:"model"`
    
    // 服务端点
    Endpoint string `json:"endpoint,omitempty"`
    
    // API Key（引用 Secret）
    APIKeySecretRef *SecretKeyRef `json:"apiKeySecretRef,omitempty"`
    
    // 向量维度（可选，用于验证）
    Dimension int `json:"dimension,omitempty"`
    
    // 批量大小
    BatchSize int `json:"batchSize,omitempty"`
    
    // 超时时间
    Timeout string `json:"timeout,omitempty"` // e.g., "60s"
}

// SecretKeyRef Secret 键引用
type SecretKeyRef struct {
    Name string `json:"name"`
    Key  string `json:"key"`
}

// VectorStorageConfig 向量存储配置
type VectorStorageConfig struct {
    // 存储类型: milvus, qdrant, pgvector
    Type string `json:"type"`
    
    // 连接配置
    Milvus *MilvusConfig `json:"milvus,omitempty"`
    
    // Collection 名称
    Collection string `json:"collection"`
    
    // Partition 名称（可选）
    Partition string `json:"partition,omitempty"`
    
    // 索引类型
    IndexType string `json:"indexType,omitempty"` // IVF_FLAT, HNSW, etc.
    
    // 距离度量
    MetricType string `json:"metricType,omitempty"` // L2, IP, COSINE
}

// MilvusConfig Milvus 连接配置
type MilvusConfig struct {
    Host     string `json:"host"`
    Port     int    `json:"port,omitempty"` // 默认 19530
    User     string `json:"user,omitempty"`
    Password string `json:"password,omitempty"`
    
    // 或使用 Secret 引用
    SecretRef string `json:"secretRef,omitempty"`
    
    // 连接池配置
    MaxConnections int `json:"maxConnections,omitempty"`
}

// BehaviorConfig 任务行为配置
type BehaviorConfig struct {
    // 失败策略: fail, skip, retry
    OnError string `json:"onError,omitempty"`
    
    // 最大重试次数
    MaxRetries int `json:"maxRetries,omitempty"`
    
    // 是否覆盖已存在的向量
    Overwrite bool `json:"overwrite,omitempty"`
    
    // 并发处理数
    Concurrency int `json:"concurrency,omitempty"`
    
    // 进度回调 URL
    ProgressCallback string `json:"progressCallback,omitempty"`
}
```

### 3.2 向量化任务记录 (VectorizeTask)

```go
// pkg/apiserver/domain/model/vectorize.go

package model

import (
    "kubemin-cli/pkg/apiserver/config"
    "time"
)

func init() {
    RegisterModel(&VectorizeTask{}, &VectorizeRecord{})
}

// VectorizeTask 向量化任务记录
type VectorizeTask struct {
    ID          string            `json:"id" gorm:"primaryKey"`
    Name        string            `json:"name"`
    Namespace   string            `json:"namespace"`
    ProjectID   string            `json:"project_id"`
    AppID       string            `json:"app_id"`
    TaskID      string            `json:"task_id"`       // 关联的 WorkflowQueue TaskID
    Status      config.Status     `json:"status"`
    Progress    int               `json:"progress"`      // 0-100
    
    // 配置快照
    Spec        *JSONStruct       `json:"spec" gorm:"serializer:json"`
    
    // 统计信息
    Stats       *VectorizeStats   `json:"stats" gorm:"serializer:json"`
    
    // 错误信息
    Error       string            `json:"error,omitempty"`
    
    // 时间信息
    StartTime   *time.Time        `json:"start_time,omitempty"`
    EndTime     *time.Time        `json:"end_time,omitempty"`
    
    BaseModel
}

// VectorizeStats 向量化统计信息
type VectorizeStats struct {
    TotalDocuments   int   `json:"total_documents"`
    ProcessedDocs    int   `json:"processed_documents"`
    FailedDocs       int   `json:"failed_documents"`
    TotalChunks      int   `json:"total_chunks"`
    TotalVectors     int   `json:"total_vectors"`
    TotalTokens      int64 `json:"total_tokens"`
    ProcessingTimeMs int64 `json:"processing_time_ms"`
    EmbeddingTimeMs  int64 `json:"embedding_time_ms"`
    StorageTimeMs    int64 `json:"storage_time_ms"`
}

// VectorizeRecord 向量化记录（每个文档）
type VectorizeRecord struct {
    ID           string        `json:"id" gorm:"primaryKey"`
    TaskID       string        `json:"task_id" gorm:"index"`
    DocumentPath string        `json:"document_path"`
    DocumentHash string        `json:"document_hash"` // 用于去重
    Status       config.Status `json:"status"`
    ChunkCount   int           `json:"chunk_count"`
    VectorCount  int           `json:"vector_count"`
    Error        string        `json:"error,omitempty"`
    Metadata     *JSONStruct   `json:"metadata" gorm:"serializer:json"`
    BaseModel
}

func (v *VectorizeTask) PrimaryKey() string {
    return v.ID
}

func (v *VectorizeTask) TableName() string {
    return tableNamePrefix + "vectorize_tasks"
}

func (v *VectorizeTask) ShortTableName() string {
    return "vectorize_task"
}

func (v *VectorizeTask) Index() map[string]interface{} {
    index := make(map[string]interface{})
    if v.ID != "" {
        index["id"] = v.ID
    }
    if v.TaskID != "" {
        index["task_id"] = v.TaskID
    }
    if v.AppID != "" {
        index["app_id"] = v.AppID
    }
    return index
}

func (v *VectorizeRecord) PrimaryKey() string {
    return v.ID
}

func (v *VectorizeRecord) TableName() string {
    return tableNamePrefix + "vectorize_records"
}

func (v *VectorizeRecord) ShortTableName() string {
    return "vectorize_record"
}

func (v *VectorizeRecord) Index() map[string]interface{} {
    index := make(map[string]interface{})
    if v.ID != "" {
        index["id"] = v.ID
    }
    if v.TaskID != "" {
        index["task_id"] = v.TaskID
    }
    return index
}
```

### 3.3 向量元数据 Schema (Milvus)

```go
// Milvus Collection Schema
type VectorDocument struct {
    ID           string    // 主键，UUID
    DocumentID   string    // 源文档 ID
    ChunkIndex   int       // 分块索引
    Content      string    // 原始文本内容
    Vector       []float32 // 向量嵌入
    
    // 元数据
    SourcePath   string            // 源文件路径
    SourceType   string            // 文件类型
    ProjectID    string            // 项目 ID
    AppID        string            // 应用 ID
    TaskID       string            // 任务 ID
    CreatedAt    int64             // 创建时间戳
    Metadata     map[string]string // 自定义元数据
}
```

---

## 4. 详细实现步骤

### 4.1 修改清单

#### 步骤 1：新增 Job 类型定义

**文件**: `pkg/apiserver/config/consts.go`

```go
// 在现有 JobType 定义后添加

// 向量化组件类型
VectorJob JobType = "vectorize"

// 向量化 Job 类型
JobVectorize JobType = "vectorize_deploy"
```

#### 步骤 2：创建向量化 Spec 定义

**新文件**: `pkg/apiserver/domain/spec/vectorize.go`

（见 3.1 节完整代码）

#### 步骤 3：创建向量化数据模型

**新文件**: `pkg/apiserver/domain/model/vectorize.go`

（见 3.2 节完整代码）

#### 步骤 4：实现嵌入模型客户端接口

**新文件**: `pkg/apiserver/infrastructure/vectorize/client.go`

```go
package vectorize

import (
    "context"
)

// EmbeddingClient 嵌入模型客户端接口
type EmbeddingClient interface {
    // Embed 将文本转换为向量
    Embed(ctx context.Context, texts []string) ([][]float32, error)
    
    // EmbedSingle 单个文本嵌入
    EmbedSingle(ctx context.Context, text string) ([]float32, error)
    
    // Dimension 返回向量维度
    Dimension() int
    
    // ModelName 返回模型名称
    ModelName() string
    
    // Close 关闭客户端连接
    Close() error
}

// EmbeddingConfig 嵌入客户端配置
type EmbeddingClientConfig struct {
    Provider  string // ollama, tei, openai
    Model     string
    Endpoint  string
    APIKey    string
    BatchSize int
    Timeout   time.Duration
}

// NewEmbeddingClient 根据配置创建嵌入客户端
func NewEmbeddingClient(cfg EmbeddingClientConfig) (EmbeddingClient, error) {
    switch cfg.Provider {
    case "ollama":
        return NewOllamaClient(cfg)
    case "tei":
        return NewTEIClient(cfg)
    case "openai":
        return NewOpenAIClient(cfg)
    default:
        return nil, fmt.Errorf("unsupported embedding provider: %s", cfg.Provider)
    }
}
```

#### 步骤 5：实现 Ollama 嵌入客户端

**新文件**: `pkg/apiserver/infrastructure/vectorize/ollama.go`

```go
package vectorize

import (
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "net/http"
    "time"
)

// OllamaClient Ollama 嵌入客户端
type OllamaClient struct {
    endpoint   string
    model      string
    httpClient *http.Client
    dimension  int
}

// OllamaEmbedRequest Ollama 嵌入请求
type OllamaEmbedRequest struct {
    Model  string `json:"model"`
    Prompt string `json:"prompt"`
}

// OllamaEmbedResponse Ollama 嵌入响应
type OllamaEmbedResponse struct {
    Embedding []float32 `json:"embedding"`
}

func NewOllamaClient(cfg EmbeddingClientConfig) (*OllamaClient, error) {
    if cfg.Endpoint == "" {
        cfg.Endpoint = "http://localhost:11434"
    }
    if cfg.Timeout == 0 {
        cfg.Timeout = 60 * time.Second
    }
    
    client := &OllamaClient{
        endpoint: cfg.Endpoint,
        model:    cfg.Model,
        httpClient: &http.Client{
            Timeout: cfg.Timeout,
        },
    }
    
    // 获取向量维度
    dim, err := client.detectDimension(context.Background())
    if err != nil {
        return nil, fmt.Errorf("detect dimension: %w", err)
    }
    client.dimension = dim
    
    return client, nil
}

func (c *OllamaClient) Embed(ctx context.Context, texts []string) ([][]float32, error) {
    results := make([][]float32, len(texts))
    for i, text := range texts {
        vec, err := c.EmbedSingle(ctx, text)
        if err != nil {
            return nil, fmt.Errorf("embed text %d: %w", i, err)
        }
        results[i] = vec
    }
    return results, nil
}

func (c *OllamaClient) EmbedSingle(ctx context.Context, text string) ([]float32, error) {
    reqBody := OllamaEmbedRequest{
        Model:  c.model,
        Prompt: text,
    }
    
    body, err := json.Marshal(reqBody)
    if err != nil {
        return nil, err
    }
    
    req, err := http.NewRequestWithContext(ctx, "POST", c.endpoint+"/api/embeddings", bytes.NewReader(body))
    if err != nil {
        return nil, err
    }
    req.Header.Set("Content-Type", "application/json")
    
    resp, err := c.httpClient.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()
    
    if resp.StatusCode != http.StatusOK {
        return nil, fmt.Errorf("ollama API error: %s", resp.Status)
    }
    
    var result OllamaEmbedResponse
    if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
        return nil, err
    }
    
    return result.Embedding, nil
}

func (c *OllamaClient) detectDimension(ctx context.Context) (int, error) {
    vec, err := c.EmbedSingle(ctx, "dimension test")
    if err != nil {
        return 0, err
    }
    return len(vec), nil
}

func (c *OllamaClient) Dimension() int {
    return c.dimension
}

func (c *OllamaClient) ModelName() string {
    return c.model
}

func (c *OllamaClient) Close() error {
    return nil
}
```

#### 步骤 6：实现 HuggingFace TEI 客户端

**新文件**: `pkg/apiserver/infrastructure/vectorize/tei.go`

```go
package vectorize

import (
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "net/http"
    "time"
)

// TEIClient HuggingFace Text Embeddings Inference 客户端
type TEIClient struct {
    endpoint   string
    model      string
    httpClient *http.Client
    dimension  int
}

// TEIEmbedRequest TEI 嵌入请求
type TEIEmbedRequest struct {
    Inputs []string `json:"inputs"`
}

func NewTEIClient(cfg EmbeddingClientConfig) (*TEIClient, error) {
    if cfg.Endpoint == "" {
        return nil, fmt.Errorf("TEI endpoint is required")
    }
    if cfg.Timeout == 0 {
        cfg.Timeout = 60 * time.Second
    }
    
    client := &TEIClient{
        endpoint: cfg.Endpoint,
        model:    cfg.Model,
        httpClient: &http.Client{
            Timeout: cfg.Timeout,
        },
    }
    
    // 获取向量维度
    dim, err := client.detectDimension(context.Background())
    if err != nil {
        return nil, fmt.Errorf("detect dimension: %w", err)
    }
    client.dimension = dim
    
    return client, nil
}

func (c *TEIClient) Embed(ctx context.Context, texts []string) ([][]float32, error) {
    reqBody := TEIEmbedRequest{
        Inputs: texts,
    }
    
    body, err := json.Marshal(reqBody)
    if err != nil {
        return nil, err
    }
    
    req, err := http.NewRequestWithContext(ctx, "POST", c.endpoint+"/embed", bytes.NewReader(body))
    if err != nil {
        return nil, err
    }
    req.Header.Set("Content-Type", "application/json")
    
    resp, err := c.httpClient.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()
    
    if resp.StatusCode != http.StatusOK {
        return nil, fmt.Errorf("TEI API error: %s", resp.Status)
    }
    
    var results [][]float32
    if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
        return nil, err
    }
    
    return results, nil
}

func (c *TEIClient) EmbedSingle(ctx context.Context, text string) ([]float32, error) {
    vecs, err := c.Embed(ctx, []string{text})
    if err != nil {
        return nil, err
    }
    if len(vecs) == 0 {
        return nil, fmt.Errorf("empty embedding result")
    }
    return vecs[0], nil
}

func (c *TEIClient) detectDimension(ctx context.Context) (int, error) {
    vecs, err := c.Embed(ctx, []string{"dimension test"})
    if err != nil {
        return 0, err
    }
    if len(vecs) == 0 || len(vecs[0]) == 0 {
        return 0, fmt.Errorf("empty embedding result")
    }
    return len(vecs[0]), nil
}

func (c *TEIClient) Dimension() int {
    return c.dimension
}

func (c *TEIClient) ModelName() string {
    return c.model
}

func (c *TEIClient) Close() error {
    return nil
}
```

#### 步骤 7：实现 Milvus 向量存储客户端

**新文件**: `pkg/apiserver/infrastructure/storage/milvus.go`

```go
package storage

import (
    "context"
    "fmt"
    
    "github.com/milvus-io/milvus-sdk-go/v2/client"
    "github.com/milvus-io/milvus-sdk-go/v2/entity"
)

// MilvusClient Milvus 向量存储客户端
type MilvusClient struct {
    client     client.Client
    collection string
    partition  string
    dimension  int
}

// MilvusConfig Milvus 配置
type MilvusConfig struct {
    Host       string
    Port       int
    User       string
    Password   string
    Collection string
    Partition  string
    Dimension  int
    IndexType  string // IVF_FLAT, HNSW, etc.
    MetricType string // L2, IP, COSINE
}

// VectorRecord 向量记录
type VectorRecord struct {
    ID         string
    DocumentID string
    ChunkIndex int
    Content    string
    Vector     []float32
    Metadata   map[string]string
}

func NewMilvusClient(ctx context.Context, cfg MilvusConfig) (*MilvusClient, error) {
    if cfg.Port == 0 {
        cfg.Port = 19530
    }
    
    addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
    
    var c client.Client
    var err error
    
    if cfg.User != "" {
        c, err = client.NewGrpcClient(ctx, addr, client.WithCredential(cfg.User, cfg.Password))
    } else {
        c, err = client.NewGrpcClient(ctx, addr)
    }
    if err != nil {
        return nil, fmt.Errorf("connect to milvus: %w", err)
    }
    
    mc := &MilvusClient{
        client:     c,
        collection: cfg.Collection,
        partition:  cfg.Partition,
        dimension:  cfg.Dimension,
    }
    
    // 确保 Collection 存在
    if err := mc.ensureCollection(ctx, cfg); err != nil {
        c.Close()
        return nil, err
    }
    
    return mc, nil
}

func (m *MilvusClient) ensureCollection(ctx context.Context, cfg MilvusConfig) error {
    exists, err := m.client.HasCollection(ctx, m.collection)
    if err != nil {
        return fmt.Errorf("check collection: %w", err)
    }
    
    if exists {
        return nil
    }
    
    // 创建 Collection Schema
    schema := &entity.Schema{
        CollectionName: m.collection,
        Description:    "Document vectors for RAG",
        Fields: []*entity.Field{
            {
                Name:       "id",
                DataType:   entity.FieldTypeVarChar,
                PrimaryKey: true,
                AutoID:     false,
                TypeParams: map[string]string{"max_length": "64"},
            },
            {
                Name:     "document_id",
                DataType: entity.FieldTypeVarChar,
                TypeParams: map[string]string{"max_length": "64"},
            },
            {
                Name:     "chunk_index",
                DataType: entity.FieldTypeInt32,
            },
            {
                Name:     "content",
                DataType: entity.FieldTypeVarChar,
                TypeParams: map[string]string{"max_length": "65535"},
            },
            {
                Name:     "source_path",
                DataType: entity.FieldTypeVarChar,
                TypeParams: map[string]string{"max_length": "1024"},
            },
            {
                Name:     "project_id",
                DataType: entity.FieldTypeVarChar,
                TypeParams: map[string]string{"max_length": "64"},
            },
            {
                Name:     "app_id",
                DataType: entity.FieldTypeVarChar,
                TypeParams: map[string]string{"max_length": "64"},
            },
            {
                Name:     "vector",
                DataType: entity.FieldTypeFloatVector,
                TypeParams: map[string]string{"dim": fmt.Sprintf("%d", m.dimension)},
            },
        },
    }
    
    if err := m.client.CreateCollection(ctx, schema, 2); err != nil {
        return fmt.Errorf("create collection: %w", err)
    }
    
    // 创建索引
    indexType := entity.IvfFlat
    if cfg.IndexType == "HNSW" {
        indexType = entity.HNSW
    }
    
    metricType := entity.L2
    if cfg.MetricType == "IP" {
        metricType = entity.IP
    } else if cfg.MetricType == "COSINE" {
        metricType = entity.COSINE
    }
    
    idx, err := entity.NewIndexIvfFlat(metricType, 128)
    if indexType == entity.HNSW {
        idx, err = entity.NewIndexHNSW(metricType, 16, 256)
    }
    if err != nil {
        return fmt.Errorf("create index params: %w", err)
    }
    
    if err := m.client.CreateIndex(ctx, m.collection, "vector", idx, false); err != nil {
        return fmt.Errorf("create index: %w", err)
    }
    
    // 加载 Collection
    if err := m.client.LoadCollection(ctx, m.collection, false); err != nil {
        return fmt.Errorf("load collection: %w", err)
    }
    
    return nil
}

// Insert 批量插入向量
func (m *MilvusClient) Insert(ctx context.Context, records []VectorRecord) error {
    if len(records) == 0 {
        return nil
    }
    
    ids := make([]string, len(records))
    documentIDs := make([]string, len(records))
    chunkIndices := make([]int32, len(records))
    contents := make([]string, len(records))
    sourcePaths := make([]string, len(records))
    projectIDs := make([]string, len(records))
    appIDs := make([]string, len(records))
    vectors := make([][]float32, len(records))
    
    for i, r := range records {
        ids[i] = r.ID
        documentIDs[i] = r.DocumentID
        chunkIndices[i] = int32(r.ChunkIndex)
        contents[i] = r.Content
        sourcePaths[i] = r.Metadata["source_path"]
        projectIDs[i] = r.Metadata["project_id"]
        appIDs[i] = r.Metadata["app_id"]
        vectors[i] = r.Vector
    }
    
    columns := []entity.Column{
        entity.NewColumnVarChar("id", ids),
        entity.NewColumnVarChar("document_id", documentIDs),
        entity.NewColumnInt32("chunk_index", chunkIndices),
        entity.NewColumnVarChar("content", contents),
        entity.NewColumnVarChar("source_path", sourcePaths),
        entity.NewColumnVarChar("project_id", projectIDs),
        entity.NewColumnVarChar("app_id", appIDs),
        entity.NewColumnFloatVector("vector", m.dimension, vectors),
    }
    
    _, err := m.client.Insert(ctx, m.collection, m.partition, columns...)
    if err != nil {
        return fmt.Errorf("insert vectors: %w", err)
    }
    
    return nil
}

// Search 向量搜索
func (m *MilvusClient) Search(ctx context.Context, vector []float32, topK int, filter string) ([]VectorRecord, error) {
    sp, err := entity.NewIndexIvfFlatSearchParam(16)
    if err != nil {
        return nil, err
    }
    
    results, err := m.client.Search(
        ctx,
        m.collection,
        []string{m.partition},
        filter,
        []string{"id", "document_id", "chunk_index", "content", "source_path"},
        []entity.Vector{entity.FloatVector(vector)},
        "vector",
        entity.L2,
        topK,
        sp,
    )
    if err != nil {
        return nil, fmt.Errorf("search: %w", err)
    }
    
    var records []VectorRecord
    for _, result := range results {
        for i := 0; i < result.ResultCount; i++ {
            record := VectorRecord{
                ID: result.IDs.(*entity.ColumnVarChar).Data()[i],
            }
            records = append(records, record)
        }
    }
    
    return records, nil
}

func (m *MilvusClient) Close() error {
    return m.client.Close()
}
```

#### 步骤 8：实现 VectorJobCtl 控制器

**新文件**: `pkg/apiserver/event/workflow/job/job_vectorize.go`

```go
package job

import (
    "context"
    "encoding/json"
    "fmt"
    "time"
    
    "github.com/google/uuid"
    "k8s.io/klog/v2"
    
    "kubemin-cli/pkg/apiserver/config"
    "kubemin-cli/pkg/apiserver/domain/model"
    "kubemin-cli/pkg/apiserver/domain/spec"
    "kubemin-cli/pkg/apiserver/infrastructure/datastore"
    "kubemin-cli/pkg/apiserver/infrastructure/storage"
    "kubemin-cli/pkg/apiserver/infrastructure/vectorize"
)

// VectorJobCtl 向量化 Job 控制器
type VectorJobCtl struct {
    job           *model.JobTask
    store         datastore.DataStore
    ack           func()
    
    // 向量化组件
    embedClient   vectorize.EmbeddingClient
    vectorStore   *storage.MilvusClient
    parser        *vectorize.DocumentParser
    chunker       *vectorize.TextChunker
    
    // 配置
    spec          *spec.VectorizeSpec
    
    // 状态
    stats         *model.VectorizeStats
}

func NewVectorJobCtl(job *model.JobTask, store datastore.DataStore, ack func()) *VectorJobCtl {
    return &VectorJobCtl{
        job:   job,
        store: store,
        ack:   ack,
        stats: &model.VectorizeStats{},
    }
}

func (c *VectorJobCtl) Run(ctx context.Context) error {
    logger := klog.FromContext(ctx).WithValues("jobName", c.job.Name, "job_type", c.job.JobType)
    
    c.job.Status = config.StatusRunning
    c.ack()
    
    // 1. 解析配置
    if err := c.parseSpec(); err != nil {
        return fmt.Errorf("parse vectorize spec: %w", err)
    }
    
    // 2. 初始化客户端
    if err := c.initClients(ctx); err != nil {
        return fmt.Errorf("init clients: %w", err)
    }
    defer c.cleanup()
    
    // 3. 获取文档列表
    documents, err := c.fetchDocuments(ctx)
    if err != nil {
        return fmt.Errorf("fetch documents: %w", err)
    }
    c.stats.TotalDocuments = len(documents)
    logger.Info("Fetched documents", "count", len(documents))
    
    // 4. 处理每个文档
    for i, doc := range documents {
        select {
        case <-ctx.Done():
            return NewStatusError(config.StatusCancelled, ctx.Err())
        default:
        }
        
        if err := c.processDocument(ctx, doc); err != nil {
            logger.Error(err, "Failed to process document", "path", doc.Path)
            c.stats.FailedDocs++
            
            if c.spec.Behavior.OnError == "fail" {
                return fmt.Errorf("process document %s: %w", doc.Path, err)
            }
            // skip or retry: 继续处理下一个
            continue
        }
        
        c.stats.ProcessedDocs++
        
        // 更新进度
        progress := int(float64(i+1) / float64(len(documents)) * 100)
        c.updateProgress(ctx, progress)
    }
    
    // 5. 保存统计信息
    logger.Info("Vectorization completed", 
        "processed", c.stats.ProcessedDocs,
        "failed", c.stats.FailedDocs,
        "vectors", c.stats.TotalVectors)
    
    c.job.Status = config.StatusCompleted
    return nil
}

func (c *VectorJobCtl) parseSpec() error {
    if c.job.JobInfo == nil {
        return fmt.Errorf("job info is nil")
    }
    
    specBytes, err := json.Marshal(c.job.JobInfo)
    if err != nil {
        return err
    }
    
    var vectorizeSpec spec.VectorizeSpec
    if err := json.Unmarshal(specBytes, &vectorizeSpec); err != nil {
        return err
    }
    
    c.spec = &vectorizeSpec
    return nil
}

func (c *VectorJobCtl) initClients(ctx context.Context) error {
    // 初始化嵌入客户端
    embedCfg := vectorize.EmbeddingClientConfig{
        Provider: c.spec.Embedding.Provider,
        Model:    c.spec.Embedding.Model,
        Endpoint: c.spec.Embedding.Endpoint,
    }
    
    embedClient, err := vectorize.NewEmbeddingClient(embedCfg)
    if err != nil {
        return fmt.Errorf("create embedding client: %w", err)
    }
    c.embedClient = embedClient
    
    // 初始化向量存储
    milvusCfg := storage.MilvusConfig{
        Host:       c.spec.Storage.Milvus.Host,
        Port:       c.spec.Storage.Milvus.Port,
        User:       c.spec.Storage.Milvus.User,
        Password:   c.spec.Storage.Milvus.Password,
        Collection: c.spec.Storage.Collection,
        Partition:  c.spec.Storage.Partition,
        Dimension:  embedClient.Dimension(),
        IndexType:  c.spec.Storage.IndexType,
        MetricType: c.spec.Storage.MetricType,
    }
    
    vectorStore, err := storage.NewMilvusClient(ctx, milvusCfg)
    if err != nil {
        return fmt.Errorf("create milvus client: %w", err)
    }
    c.vectorStore = vectorStore
    
    // 初始化文档解析器
    c.parser = vectorize.NewDocumentParser()
    
    // 初始化文本分块器
    c.chunker = vectorize.NewTextChunker(vectorize.ChunkerConfig{
        Strategy:    c.spec.Processing.ChunkStrategy,
        ChunkSize:   c.spec.Processing.ChunkSize,
        ChunkOverlap: c.spec.Processing.ChunkOverlap,
        Language:    c.spec.Processing.Language,
    })
    
    return nil
}

func (c *VectorJobCtl) fetchDocuments(ctx context.Context) ([]vectorize.Document, error) {
    switch c.spec.Source.Type {
    case "file":
        return c.parser.ParseFiles(ctx, c.spec.Source.Paths)
    case "s3":
        return c.parser.ParseS3(ctx, c.spec.Source.S3)
    case "http":
        return c.parser.ParseHTTP(ctx, c.spec.Source.HTTP)
    default:
        return nil, fmt.Errorf("unsupported source type: %s", c.spec.Source.Type)
    }
}

func (c *VectorJobCtl) processDocument(ctx context.Context, doc vectorize.Document) error {
    logger := klog.FromContext(ctx).WithValues("document", doc.Path)
    
    startTime := time.Now()
    
    // 1. 文本分块
    chunks := c.chunker.Chunk(doc.Content)
    c.stats.TotalChunks += len(chunks)
    logger.V(2).Info("Document chunked", "chunks", len(chunks))
    
    // 2. 批量嵌入
    batchSize := c.spec.Embedding.BatchSize
    if batchSize <= 0 {
        batchSize = 32
    }
    
    var records []storage.VectorRecord
    
    for i := 0; i < len(chunks); i += batchSize {
        end := i + batchSize
        if end > len(chunks) {
            end = len(chunks)
        }
        
        batch := chunks[i:end]
        
        embeddingStart := time.Now()
        vectors, err := c.embedClient.Embed(ctx, batch)
        if err != nil {
            return fmt.Errorf("embed batch: %w", err)
        }
        c.stats.EmbeddingTimeMs += time.Since(embeddingStart).Milliseconds()
        
        for j, vec := range vectors {
            record := storage.VectorRecord{
                ID:         uuid.New().String(),
                DocumentID: doc.ID,
                ChunkIndex: i + j,
                Content:    batch[j],
                Vector:     vec,
                Metadata: map[string]string{
                    "source_path": doc.Path,
                    "project_id":  c.job.ProjectID,
                    "app_id":      c.job.AppID,
                },
            }
            records = append(records, record)
        }
    }
    
    // 3. 批量写入向量数据库
    storageStart := time.Now()
    if err := c.vectorStore.Insert(ctx, records); err != nil {
        return fmt.Errorf("insert vectors: %w", err)
    }
    c.stats.StorageTimeMs += time.Since(storageStart).Milliseconds()
    c.stats.TotalVectors += len(records)
    
    c.stats.ProcessingTimeMs += time.Since(startTime).Milliseconds()
    
    return nil
}

func (c *VectorJobCtl) updateProgress(ctx context.Context, progress int) {
    // 可以通过回调或数据库更新进度
    klog.FromContext(ctx).V(2).Info("Progress updated", "progress", progress)
}

func (c *VectorJobCtl) cleanup() {
    if c.embedClient != nil {
        c.embedClient.Close()
    }
    if c.vectorStore != nil {
        c.vectorStore.Close()
    }
}

func (c *VectorJobCtl) Clean(ctx context.Context) {
    // 向量化 Job 的清理逻辑
    // 如果任务失败，可以选择删除已写入的向量
    logger := klog.FromContext(ctx).WithValues("jobName", c.job.Name)
    logger.Info("Cleaning up vectorize job resources")
    
    c.cleanup()
}

func (c *VectorJobCtl) SaveInfo(ctx context.Context) error {
    // 保存统计信息到数据库
    if c.stats == nil {
        return nil
    }
    
    // 这里可以保存 VectorizeTask 记录
    klog.FromContext(ctx).Info("Saving vectorize job stats",
        "totalDocs", c.stats.TotalDocuments,
        "processedDocs", c.stats.ProcessedDocs,
        "totalVectors", c.stats.TotalVectors)
    
    return nil
}
```

#### 步骤 9：扩展 Job 分发逻辑

**修改文件**: `pkg/apiserver/event/workflow/job/job.go`

在 `initJobCtl` 函数中添加：

```go
case string(config.JobVectorize):
    return NewVectorJobCtl(job, store, ack)
```

#### 步骤 10：扩展组件构建逻辑

**修改文件**: `pkg/apiserver/event/workflow/job_builder.go`

在 `buildJobsForComponent` 函数的 switch 语句中添加：

```go
case config.VectorJob:
    vectorTask := NewJobTask(
        component.Name,
        namespace,
        task.WorkflowID,
        task.ProjectID,
        task.AppID,
        task.TaskID,
        defaultJobTimeoutSeconds,
    )
    vectorTask.JobType = string(config.JobVectorize)
    vectorTask.JobInfo = parseVectorizeSpec(ctx, component.Properties)
    buckets[config.JobPriorityNormal] = append(buckets[config.JobPriorityNormal], vectorTask)
```

### 4.2 新增文件清单

| 文件路径 | 描述 |
|----------|------|
| `pkg/apiserver/domain/spec/vectorize.go` | 向量化配置 Spec 定义 |
| `pkg/apiserver/domain/model/vectorize.go` | 向量化任务数据模型 |
| `pkg/apiserver/infrastructure/vectorize/client.go` | 嵌入模型客户端接口 |
| `pkg/apiserver/infrastructure/vectorize/ollama.go` | Ollama 客户端实现 |
| `pkg/apiserver/infrastructure/vectorize/tei.go` | HuggingFace TEI 客户端实现 |
| `pkg/apiserver/infrastructure/vectorize/parser.go` | 文档解析器 |
| `pkg/apiserver/infrastructure/vectorize/chunker.go` | 文本分块器 |
| `pkg/apiserver/infrastructure/storage/milvus.go` | Milvus 向量存储客户端 |
| `pkg/apiserver/event/workflow/job/job_vectorize.go` | VectorJobCtl 控制器 |
| `pkg/apiserver/interfaces/api/vectorize.go` | 向量化 API 接口 |

### 4.3 修改文件清单

| 文件路径 | 修改内容 |
|----------|----------|
| `pkg/apiserver/config/consts.go` | 新增 `VectorJob` 和 `JobVectorize` 类型 |
| `pkg/apiserver/event/workflow/job/job.go` | `initJobCtl` 添加 `JobVectorize` case |
| `pkg/apiserver/event/workflow/job_builder.go` | `buildJobsForComponent` 添加 `VectorJob` case |

---

## 5. 文本分块策略

文本分块（Chunking）是向量化流程中非常关键的一步。将长文档切分为适当大小的文本块，直接影响向量检索的质量和效率。

### 5.1 分块策略对比

| 策略 | 描述 | 适用场景 | 优缺点 |
|------|------|----------|--------|
| **固定长度** | 按字符数切分 | 通用场景 | 简单但可能切断语义 |
| **句子分割** | 按句号、问号等切分 | 文章、问答 | 保持句子完整性 |
| **段落分割** | 按换行符切分 | 结构化文档 | 保持段落语义 |
| **正则分割** | 自定义模式切分 | 特定格式文档 | 灵活但需设计规则 |
| **递归分割** | 多级分隔符逐层切分 | 通用（推荐） | 兼顾语义和大小 |
| **语义分割** | AI 理解语义边界 | 高质量需求 | 效果好但成本高 |

### 5.2 分块大小权衡

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                          块大小权衡分析                                      │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│   太小 (<200字符)                    │         太大 (>2000字符)             │
│   ─────────────────────              │    ─────────────────────             │
│   ✓ 检索精确度高                     │    ✗ 检索精确度低                    │
│   ✗ 上下文信息不完整                 │    ✓ 上下文信息完整                  │
│   ✗ 向量数量多，存储成本高           │    ✓ 向量数量少，存储成本低          │
│   ✗ 可能丢失跨句语义                 │    ✗ 可能引入噪声信息                │
│                                                                             │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│   推荐块大小:                                                               │
│   • 中文文档: 300-800 字符                                                  │
│   • 英文文档: 500-1500 字符                                                 │
│   • 代码文档: 800-2000 字符                                                 │
│                                                                             │
│   推荐重叠比例: 10%-20% (避免边界信息丢失)                                  │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

### 5.3 常用正则模式参考

| 场景 | 正则表达式 | 说明 |
|------|-----------|------|
| Markdown 标题 | `(?m)^#{1,6}\s+` | 匹配 # 到 ###### 开头的行 |
| 中文句子 | `[。！？；]+` | 按中文标点分割 |
| 英文句子 | `(?<=[.!?])\s+` | 按英文句号后的空格分割 |
| 段落 | `\n\s*\n` | 双换行分割 |
| 无序列表 | `(?m)^[-*]\s+` | 匹配无序列表项 |
| 有序列表 | `(?m)^\d+\.\s+` | 匹配有序列表项 |
| 代码块 | `` ```[\s\S]*?``` `` | 匹配 Markdown 代码块 |
| 中文章节 | `(?m)^第[一二三四五六七八九十百\d]+[章节条]` | 匹配中文章节标题 |
| 函数定义 | `(?m)^func\s+\|^def\s+\|^function\s+` | 匹配函数定义边界 |
| HTML 标签 | `<\/?[a-z][^>]*>` | 匹配 HTML 标签边界 |

### 5.4 文本分块器实现

#### 5.4.1 核心接口定义

**文件**: `pkg/apiserver/infrastructure/vectorize/chunker.go`

```go
package vectorize

import (
    "regexp"
    "strings"
)

// ChunkerConfig 分块配置
type ChunkerConfig struct {
    // 分块策略: fixed, sentence, paragraph, regex, recursive
    Strategy string
    
    // 目标块大小（字符数）
    ChunkSize int
    
    // 重叠字符数（推荐 ChunkSize 的 10%-20%）
    ChunkOverlap int
    
    // 分隔符列表（按优先级排序，用于递归分割）
    Separators []string
    
    // 正则表达式模式（用于 regex 策略）
    RegexPattern string
    
    // 语言 (zh, en, auto)
    Language string
}

// TextChunker 文本分块器
type TextChunker struct {
    config ChunkerConfig
    regex  *regexp.Regexp
}

// NewTextChunker 创建文本分块器
func NewTextChunker(cfg ChunkerConfig) *TextChunker {
    tc := &TextChunker{config: cfg}
    
    // 编译正则表达式
    if cfg.RegexPattern != "" {
        tc.regex = regexp.MustCompile(cfg.RegexPattern)
    }
    
    // 设置默认分隔符
    if len(cfg.Separators) == 0 {
        if cfg.Language == "zh" {
            cfg.Separators = []string{
                "\n\n",     // 段落
                "\n",       // 换行
                "。",       // 句号
                "！",       // 感叹号
                "？",       // 问号
                "；",       // 分号
                "，",       // 逗号
                " ",        // 空格
            }
        } else {
            cfg.Separators = []string{
                "\n\n",     // 段落
                "\n",       // 换行
                ". ",       // 句号
                "! ",       // 感叹号
                "? ",       // 问号
                "; ",       // 分号
                ", ",       // 逗号
                " ",        // 空格
            }
        }
        tc.config = cfg
    }
    
    // 设置默认值
    if tc.config.ChunkSize <= 0 {
        tc.config.ChunkSize = 500
    }
    if tc.config.ChunkOverlap <= 0 {
        tc.config.ChunkOverlap = 50
    }
    
    return tc
}

// Chunk 根据配置策略分块
func (tc *TextChunker) Chunk(text string) []string {
    text = strings.TrimSpace(text)
    if text == "" {
        return nil
    }
    
    switch tc.config.Strategy {
    case "fixed":
        return tc.fixedChunk(text)
    case "sentence":
        return tc.sentenceChunk(text)
    case "paragraph":
        return tc.paragraphChunk(text)
    case "regex":
        return tc.regexChunk(text)
    case "recursive":
        return tc.recursiveChunk(text, tc.config.Separators)
    default:
        // 默认使用递归分割
        return tc.recursiveChunk(text, tc.config.Separators)
    }
}
```

#### 5.4.2 固定长度分割

```go
// fixedChunk 固定长度分割（支持重叠）
func (tc *TextChunker) fixedChunk(text string) []string {
    var chunks []string
    runes := []rune(text)
    size := tc.config.ChunkSize
    overlap := tc.config.ChunkOverlap
    
    for i := 0; i < len(runes); {
        end := i + size
        if end > len(runes) {
            end = len(runes)
        }
        
        chunk := string(runes[i:end])
        if strings.TrimSpace(chunk) != "" {
            chunks = append(chunks, strings.TrimSpace(chunk))
        }
        
        if end >= len(runes) {
            break
        }
        
        // 下一块起始位置（考虑重叠）
        i = end - overlap
        if i < 0 {
            i = end
        }
    }
    
    return chunks
}
```

#### 5.4.3 正则表达式分割

```go
// regexChunk 按正则表达式分割
func (tc *TextChunker) regexChunk(text string) []string {
    if tc.regex == nil {
        return []string{text}
    }
    
    // 查找所有匹配位置
    indices := tc.regex.FindAllStringIndex(text, -1)
    if len(indices) == 0 {
        return []string{text}
    }
    
    var rawChunks []string
    start := 0
    
    for _, idx := range indices {
        // 分隔符之前的内容
        chunk := strings.TrimSpace(text[start:idx[0]])
        if chunk != "" {
            rawChunks = append(rawChunks, chunk)
        }
        start = idx[1]
    }
    
    // 最后一段
    if start < len(text) {
        chunk := strings.TrimSpace(text[start:])
        if chunk != "" {
            rawChunks = append(rawChunks, chunk)
        }
    }
    
    // 合并过小的块
    return tc.mergeSmallChunks(rawChunks)
}
```

#### 5.4.4 递归分割（推荐）

```go
// recursiveChunk 递归分割（推荐策略）
// 按分隔符优先级逐层切分，保持语义完整性
func (tc *TextChunker) recursiveChunk(text string, separators []string) []string {
    // 如果没有更多分隔符，使用固定长度分割
    if len(separators) == 0 {
        return tc.fixedChunk(text)
    }
    
    separator := separators[0]
    remainingSeparators := separators[1:]
    
    // 按当前分隔符分割
    parts := strings.Split(text, separator)
    
    var chunks []string
    var currentChunk strings.Builder
    
    for _, part := range parts {
        part = strings.TrimSpace(part)
        if part == "" {
            continue
        }
        
        potentialSize := currentChunk.Len() + len(part) + len(separator)
        
        // 如果加上这部分仍在大小限制内
        if potentialSize <= tc.config.ChunkSize {
            if currentChunk.Len() > 0 {
                currentChunk.WriteString(separator)
            }
            currentChunk.WriteString(part)
        } else {
            // 保存当前块
            if currentChunk.Len() > 0 {
                chunks = append(chunks, currentChunk.String())
                currentChunk.Reset()
            }
            
            // 如果单个部分超过限制，用更细的分隔符继续分割
            if len(part) > tc.config.ChunkSize && len(remainingSeparators) > 0 {
                subChunks := tc.recursiveChunk(part, remainingSeparators)
                chunks = append(chunks, subChunks...)
            } else if len(part) > tc.config.ChunkSize {
                // 没有更细的分隔符了，使用固定长度分割
                subChunks := tc.fixedChunk(part)
                chunks = append(chunks, subChunks...)
            } else {
                currentChunk.WriteString(part)
            }
        }
    }
    
    // 保存最后一块
    if currentChunk.Len() > 0 {
        chunks = append(chunks, currentChunk.String())
    }
    
    return chunks
}
```

#### 5.4.5 句子和段落分割

```go
// sentenceChunk 按句子分割
func (tc *TextChunker) sentenceChunk(text string) []string {
    var pattern string
    if tc.config.Language == "zh" {
        pattern = `[。！？；\n]+`
    } else {
        pattern = `(?<=[.!?])\s+`
    }
    
    re := regexp.MustCompile(pattern)
    sentences := re.Split(text, -1)
    
    // 过滤空句子
    var validSentences []string
    for _, s := range sentences {
        s = strings.TrimSpace(s)
        if s != "" {
            validSentences = append(validSentences, s)
        }
    }
    
    // 合并短句到目标大小
    return tc.mergeToTargetSize(validSentences, " ")
}

// paragraphChunk 按段落分割
func (tc *TextChunker) paragraphChunk(text string) []string {
    paragraphs := regexp.MustCompile(`\n\s*\n`).Split(text, -1)
    
    var chunks []string
    for _, p := range paragraphs {
        p = strings.TrimSpace(p)
        if p == "" {
            continue
        }
        
        // 如果段落太长，进一步按句子分割
        if len([]rune(p)) > tc.config.ChunkSize {
            subChunks := tc.sentenceChunk(p)
            chunks = append(chunks, subChunks...)
        } else {
            chunks = append(chunks, p)
        }
    }
    
    return tc.mergeSmallChunks(chunks)
}
```

#### 5.4.6 辅助函数

```go
// mergeSmallChunks 合并过小的块
func (tc *TextChunker) mergeSmallChunks(chunks []string) []string {
    if len(chunks) == 0 {
        return chunks
    }
    
    minSize := tc.config.ChunkSize / 4 // 最小块大小为目标的 1/4
    var result []string
    var current strings.Builder
    
    for _, chunk := range chunks {
        if current.Len() == 0 {
            current.WriteString(chunk)
        } else if current.Len()+len(chunk)+1 <= tc.config.ChunkSize {
            current.WriteString("\n")
            current.WriteString(chunk)
        } else {
            result = append(result, current.String())
            current.Reset()
            current.WriteString(chunk)
        }
    }
    
    if current.Len() > 0 {
        // 最后一块太小就合并到前一块
        if current.Len() < minSize && len(result) > 0 {
            result[len(result)-1] += "\n" + current.String()
        } else {
            result = append(result, current.String())
        }
    }
    
    return result
}

// mergeToTargetSize 合并到目标大小
func (tc *TextChunker) mergeToTargetSize(parts []string, separator string) []string {
    var chunks []string
    var current strings.Builder
    
    for _, part := range parts {
        part = strings.TrimSpace(part)
        if part == "" {
            continue
        }
        
        potentialSize := current.Len() + len(part) + len(separator)
        
        if potentialSize <= tc.config.ChunkSize {
            if current.Len() > 0 {
                current.WriteString(separator)
            }
            current.WriteString(part)
        } else {
            if current.Len() > 0 {
                chunks = append(chunks, current.String())
                current.Reset()
            }
            current.WriteString(part)
        }
    }
    
    if current.Len() > 0 {
        chunks = append(chunks, current.String())
    }
    
    return chunks
}
```

### 5.5 不同文档类型的推荐配置

#### 5.5.1 Markdown 文档

```go
markdownConfig := ChunkerConfig{
    Strategy:     "recursive",
    ChunkSize:    500,
    ChunkOverlap: 50,
    Separators: []string{
        "\n## ",      // 二级标题
        "\n### ",     // 三级标题
        "\n#### ",    // 四级标题
        "\n\n",       // 段落
        "\n",         // 换行
        "。",         // 中文句号
        ". ",         // 英文句号
        " ",          // 空格
    },
    Language: "zh",
}
```

#### 5.5.2 代码文档

```go
codeConfig := ChunkerConfig{
    Strategy:     "regex",
    ChunkSize:    1000,
    ChunkOverlap: 100,
    RegexPattern: `(?m)^func\s+|^type\s+|^class\s+|^def\s+|^function\s+`,
    Language:     "en",
}
```

#### 5.5.3 FAQ 问答文档

```go
faqConfig := ChunkerConfig{
    Strategy:     "regex",
    ChunkSize:    400,
    ChunkOverlap: 0,  // FAQ 不需要重叠
    RegexPattern: `(?m)^Q[：:]\s*|^问[：:]\s*|^\d+\.\s*问[：:]`,
    Language:     "zh",
}
```

#### 5.5.4 法律/合同文档

```go
legalConfig := ChunkerConfig{
    Strategy:     "regex",
    ChunkSize:    800,
    ChunkOverlap: 80,
    RegexPattern: `(?m)^第[一二三四五六七八九十百零\d]+[章节条款项]`,
    Language:     "zh",
}
```

#### 5.5.5 技术文档（混合语言）

```go
techDocConfig := ChunkerConfig{
    Strategy:     "recursive",
    ChunkSize:    600,
    ChunkOverlap: 60,
    Separators: []string{
        "\n## ",      // Markdown 标题
        "\n### ",
        "\n```",      // 代码块边界
        "\n\n",       // 段落
        "\n",         // 换行
        "。",         // 中文句号
        ". ",         // 英文句号
        "；",         // 中文分号
        "; ",         // 英文分号
    },
    Language: "auto",
}
```

### 5.6 分块质量验证

#### 5.6.1 验证指标

| 指标 | 计算方式 | 推荐范围 |
|------|----------|----------|
| 平均块大小 | 总字符数 / 块数 | ChunkSize 的 70%-100% |
| 块大小标准差 | std(块大小列表) | < ChunkSize 的 30% |
| 最小块比例 | 小于 minSize 的块数 / 总块数 | < 5% |
| 语义完整性 | 人工抽样评估 | 句子完整率 > 95% |

#### 5.6.2 验证代码示例

```go
package main

import (
    "fmt"
    "math"
)

// ChunkStats 分块统计信息
type ChunkStats struct {
    TotalChunks   int
    TotalChars    int
    AvgChunkSize  float64
    MinChunkSize  int
    MaxChunkSize  int
    StdDev        float64
    SmallChunkPct float64  // 过小块占比
}

// AnalyzeChunks 分析分块质量
func AnalyzeChunks(chunks []string, minSizeThreshold int) ChunkStats {
    if len(chunks) == 0 {
        return ChunkStats{}
    }
    
    sizes := make([]int, len(chunks))
    totalChars := 0
    minSize := math.MaxInt
    maxSize := 0
    smallCount := 0
    
    for i, chunk := range chunks {
        size := len([]rune(chunk))
        sizes[i] = size
        totalChars += size
        
        if size < minSize {
            minSize = size
        }
        if size > maxSize {
            maxSize = size
        }
        if size < minSizeThreshold {
            smallCount++
        }
    }
    
    avgSize := float64(totalChars) / float64(len(chunks))
    
    // 计算标准差
    var variance float64
    for _, size := range sizes {
        diff := float64(size) - avgSize
        variance += diff * diff
    }
    variance /= float64(len(chunks))
    stdDev := math.Sqrt(variance)
    
    return ChunkStats{
        TotalChunks:   len(chunks),
        TotalChars:    totalChars,
        AvgChunkSize:  avgSize,
        MinChunkSize:  minSize,
        MaxChunkSize:  maxSize,
        StdDev:        stdDev,
        SmallChunkPct: float64(smallCount) / float64(len(chunks)) * 100,
    }
}

func main() {
    // 测试文档
    text := `
# 第一章 Kubernetes 介绍

Kubernetes 是一个开源的容器编排平台，用于自动化部署、扩展和管理容器化应用程序。

## 1.1 核心概念

Pod 是 Kubernetes 中最小的可部署单元。一个 Pod 可以包含一个或多个容器。

### 1.1.1 Pod 生命周期

Pod 从创建到终止经历多个阶段：Pending、Running、Succeeded、Failed、Unknown。

## 1.2 架构组件

Kubernetes 集群由控制平面和工作节点组成。控制平面包括 API Server、Scheduler、Controller Manager 和 etcd。

# 第二章 安装部署

本章介绍如何安装和配置 Kubernetes 集群。

## 2.1 环境要求

- 操作系统：Linux（推荐 Ubuntu 20.04 或 CentOS 7+）
- 内存：至少 2GB（推荐 4GB 以上）
- CPU：至少 2 核
- 网络：节点间网络互通
`

    // 创建分块器
    chunker := NewTextChunker(ChunkerConfig{
        Strategy:     "recursive",
        ChunkSize:    300,
        ChunkOverlap: 30,
        Language:     "zh",
    })
    
    // 执行分块
    chunks := chunker.Chunk(text)
    
    // 分析质量
    stats := AnalyzeChunks(chunks, 75) // 最小阈值 = ChunkSize / 4
    
    fmt.Println("=== 分块结果 ===")
    for i, chunk := range chunks {
        fmt.Printf("\n--- Chunk %d (%d 字符) ---\n%s\n", i+1, len([]rune(chunk)), chunk)
    }
    
    fmt.Println("\n=== 质量统计 ===")
    fmt.Printf("总块数: %d\n", stats.TotalChunks)
    fmt.Printf("总字符数: %d\n", stats.TotalChars)
    fmt.Printf("平均块大小: %.1f\n", stats.AvgChunkSize)
    fmt.Printf("最小/最大: %d / %d\n", stats.MinChunkSize, stats.MaxChunkSize)
    fmt.Printf("标准差: %.1f\n", stats.StdDev)
    fmt.Printf("过小块占比: %.1f%%\n", stats.SmallChunkPct)
}
```

### 5.7 分块与元数据

为了支持后续检索和溯源，建议在分块时保留元数据：

```go
// ChunkWithMetadata 带元数据的分块
type ChunkWithMetadata struct {
    // 分块内容
    Content string `json:"content"`
    
    // 分块索引
    Index int `json:"index"`
    
    // 在原文档中的起始位置
    StartPos int `json:"start_pos"`
    
    // 在原文档中的结束位置
    EndPos int `json:"end_pos"`
    
    // 源文档信息
    SourceDoc string `json:"source_doc"`
    
    // 所属章节/标题
    Section string `json:"section,omitempty"`
    
    // 自定义元数据
    Metadata map[string]string `json:"metadata,omitempty"`
}

// ChunkWithPositions 分块并保留位置信息
func (tc *TextChunker) ChunkWithPositions(text, sourceDoc string) []ChunkWithMetadata {
    chunks := tc.Chunk(text)
    result := make([]ChunkWithMetadata, len(chunks))
    
    pos := 0
    for i, chunk := range chunks {
        // 查找块在原文中的位置
        startPos := strings.Index(text[pos:], chunk)
        if startPos >= 0 {
            startPos += pos
        } else {
            startPos = pos
        }
        endPos := startPos + len(chunk)
        
        result[i] = ChunkWithMetadata{
            Content:   chunk,
            Index:     i,
            StartPos:  startPos,
            EndPos:    endPos,
            SourceDoc: sourceDoc,
        }
        
        pos = endPos
    }
    
    return result
}
```

---

## 6. 嵌入模型部署

### 6.1 模型选型决策树

```
                        ┌─────────────────────────┐
                        │   是否有 GPU 资源？      │
                        └───────────┬─────────────┘
                                    │
                    ┌───────────────┼───────────────┐
                    │ 是            │               │ 否
                    ▼               │               ▼
        ┌───────────────────┐       │   ┌───────────────────┐
        │ 推荐: TEI + BGE-M3 │       │   │ 推荐: Ollama +    │
        │ 维度: 1024        │       │   │ nomic-embed-text  │
        │ 性能: 高          │       │   │ 维度: 768         │
        └───────────────────┘       │   │ 性能: 中          │
                                    │   └───────────────────┘
                                    │
                    ┌───────────────┼───────────────┐
                    │               │               │
                    ▼               ▼               ▼
            ┌─────────────┐ ┌─────────────┐ ┌─────────────┐
            │ 中文为主？   │ │ 多语言？    │ │ 英文为主？  │
            └──────┬──────┘ └──────┬──────┘ └──────┬──────┘
                   │               │               │
                   ▼               ▼               ▼
            BGE-Large-zh    BGE-M3          all-MiniLM-L6
            (1024维)        (1024维)        (384维)
```

### 6.2 模型对比表

| 模型 | 提供者 | 维度 | 语言 | GPU 需求 | 推理速度 | 适用场景 |
|------|--------|------|------|----------|----------|----------|
| **BGE-M3** | BAAI | 1024 | 多语言 | 8GB+ | 快 | 生产环境首选 |
| **BGE-Large-zh** | BAAI | 1024 | 中文 | 4GB+ | 快 | 纯中文场景 |
| **nomic-embed-text** | Nomic AI | 768 | 英文 | CPU 可运行 | 中 | 资源受限环境 |
| **mxbai-embed-large** | Mixedbread | 1024 | 多语言 | 4GB+ | 中 | 平衡方案 |
| **all-MiniLM-L6-v2** | Sentence-Transformers | 384 | 英文 | CPU 可运行 | 快 | 轻量场景 |
| **text-embedding-3-small** | OpenAI | 1536 | 多语言 | API | 快 | 云端方案 |

### 6.3 Kubernetes GPU 部署方案

#### 5.3.1 HuggingFace TEI + BGE-M3 (推荐)

```yaml
# deploy/embedding/tei-bge-m3.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: embedding-tei
  namespace: kubemin-system
  labels:
    app: embedding-service
    model: bge-m3
spec:
  replicas: 1
  selector:
    matchLabels:
      app: embedding-service
  template:
    metadata:
      labels:
        app: embedding-service
    spec:
      containers:
      - name: tei
        image: ghcr.io/huggingface/text-embeddings-inference:1.2
        args:
          - "--model-id"
          - "BAAI/bge-m3"
          - "--port"
          - "8080"
          - "--max-batch-tokens"
          - "16384"
        ports:
        - containerPort: 8080
          name: http
        resources:
          requests:
            memory: "8Gi"
            cpu: "2"
            nvidia.com/gpu: 1
          limits:
            memory: "16Gi"
            cpu: "4"
            nvidia.com/gpu: 1
        livenessProbe:
          http_get:
            path: /health
            port: 8080
          initial_delay_seconds: 120
          period_seconds: 30
        readinessProbe:
          http_get:
            path: /health
            port: 8080
          initial_delay_seconds: 60
          period_seconds: 10
        volumeMounts:
        - name: model-cache
          mount_path: /data
        - name: shm
          mount_path: /dev/shm
      volumes:
      - name: model-cache
        persistentVolumeClaim:
          claim_name: embedding-model-cache
      - name: shm
        emptyDir:
          medium: Memory
          sizeLimit: "2Gi"
---
apiVersion: v1
kind: Service
metadata:
  name: embedding-service
  namespace: kubemin-system
spec:
  selector:
    app: embedding-service
  ports:
  - port: 8080
    targetPort: 8080
    name: http
  type: ClusterIP
---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: embedding-model-cache
  namespace: kubemin-system
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 20Gi
  storageClassName: standard
```

#### 5.3.2 Ollama 部署 (CPU 友好)

```yaml
# deploy/embedding/ollama.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: ollama
  namespace: kubemin-system
  labels:
    app: ollama
spec:
  replicas: 1
  selector:
    matchLabels:
      app: ollama
  template:
    metadata:
      labels:
        app: ollama
    spec:
      containers:
      - name: ollama
        image: ollama/ollama:latest
        ports:
        - containerPort: 11434
          name: http
        resources:
          requests:
            memory: "4Gi"
            cpu: "2"
          limits:
            memory: "8Gi"
            cpu: "4"
        volumeMounts:
        - name: ollama-data
          mount_path: /root/.ollama
        env:
        - name: OLLAMA_HOST
          value: "0.0.0.0"
        lifecycle:
          postStart:
            exec:
              command:
              - /bin/sh
              - -c
              - |
                sleep 10
                ollama pull nomic-embed-text
      volumes:
      - name: ollama-data
        persistentVolumeClaim:
          claim_name: ollama-data
---
apiVersion: v1
kind: Service
metadata:
  name: ollama
  namespace: kubemin-system
spec:
  selector:
    app: ollama
  ports:
  - port: 11434
    targetPort: 11434
    name: http
  type: ClusterIP
---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: ollama-data
  namespace: kubemin-system
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 10Gi
```

### 6.4 Milvus 部署

#### 5.4.1 Milvus Standalone (开发/测试)

```yaml
# deploy/vectordb/milvus-standalone.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: milvus
  namespace: kubemin-system
spec:
  replicas: 1
  selector:
    matchLabels:
      app: milvus
  template:
    metadata:
      labels:
        app: milvus
    spec:
      containers:
      - name: milvus
        image: milvusdb/milvus:v2.3.4
        command: ["milvus", "run", "standalone"]
        ports:
        - containerPort: 19530
          name: grpc
        - containerPort: 9091
          name: metrics
        env:
        - name: ETCD_USE_EMBED
          value: "true"
        - name: ETCD_DATA_DIR
          value: "/var/lib/milvus/etcd"
        - name: COMMON_STORAGETYPE
          value: "local"
        resources:
          requests:
            memory: "4Gi"
            cpu: "2"
          limits:
            memory: "8Gi"
            cpu: "4"
        volumeMounts:
        - name: milvus-data
          mount_path: /var/lib/milvus
      volumes:
      - name: milvus-data
        persistentVolumeClaim:
          claim_name: milvus-data
---
apiVersion: v1
kind: Service
metadata:
  name: milvus
  namespace: kubemin-system
spec:
  selector:
    app: milvus
  ports:
  - port: 19530
    targetPort: 19530
    name: grpc
  - port: 9091
    targetPort: 9091
    name: metrics
  type: ClusterIP
---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: milvus-data
  namespace: kubemin-system
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 50Gi
```

#### 5.4.2 Milvus Cluster (生产环境)

```bash
# 使用 Helm 部署 Milvus 集群
helm repo add milvus https://zilliztech.github.io/milvus-helm/
helm repo update

helm install milvus milvus/milvus \
  --namespace kubemin-system \
  --set cluster.enabled=true \
  --set etcd.replicaCount=3 \
  --set minio.mode=distributed \
  --set minio.replicas=4 \
  --set pulsar.enabled=true \
  --set queryNode.replicas=2 \
  --set indexNode.replicas=2 \
  --set dataNode.replicas=2
```

### 6.5 独立服务器部署

#### 5.5.1 Docker Compose 方案

```yaml
# docker-compose.yaml
version: '3.8'

services:
  ollama:
    image: ollama/ollama:latest
    ports:
      - "11434:11434"
    volumes:
      - ollama_data:/root/.ollama
    environment:
      - OLLAMA_HOST=0.0.0.0
    deploy:
      resources:
        limits:
          cpus: '4'
          memory: 8G
    command: >
      sh -c "ollama serve &
             sleep 10 &&
             ollama pull nomic-embed-text &&
             wait"

  milvus-etcd:
    image: quay.io/coreos/etcd:v3.5.5
    environment:
      - ETCD_AUTO_COMPACTION_MODE=revision
      - ETCD_AUTO_COMPACTION_RETENTION=1000
      - ETCD_QUOTA_BACKEND_BYTES=4294967296
    volumes:
      - etcd_data:/etcd
    command: etcd -advertise-client-urls=http://127.0.0.1:2379 -listen-client-urls http://0.0.0.0:2379

  milvus-minio:
    image: minio/minio:RELEASE.2023-03-20T20-16-18Z
    environment:
      MINIO_ACCESS_KEY: minioadmin
      MINIO_SECRET_KEY: minioadmin
    volumes:
      - minio_data:/minio_data
    command: minio server /minio_data
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:9000/minio/health/live"]
      interval: 30s
      timeout: 20s
      retries: 3

  milvus:
    image: milvusdb/milvus:v2.3.4
    command: ["milvus", "run", "standalone"]
    environment:
      ETCD_ENDPOINTS: milvus-etcd:2379
      MINIO_ADDRESS: milvus-minio:9000
    ports:
      - "19530:19530"
      - "9091:9091"
    volumes:
      - milvus_data:/var/lib/milvus
    depends_on:
      - milvus-etcd
      - milvus-minio

volumes:
  ollama_data:
  etcd_data:
  minio_data:
  milvus_data:
```

---

## 7. 工作流触发机制

### 7.1 向量化组件定义

向量化任务通过定义 `vectorize` 类型的组件来声明：

```json
{
  "name": "docs-vectorizer",
  "component_type": "vectorize",
  "properties": {
    "source": {
      "type": "s3",
      "s3": {
        "bucket": "knowledge-base",
        "prefix": "documents/",
        "secretRef": "s3-credentials"
      },
      "fileFilter": {
        "extensions": [".pdf", ".docx", ".md"]
      }
    },
    "processing": {
      "chunkStrategy": "semantic",
      "chunkSize": 512,
      "chunkOverlap": 50,
      "language": "zh",
      "enableOCR": false
    },
    "embedding": {
      "provider": "tei",
      "model": "BAAI/bge-m3",
      "endpoint": "http://embedding-service:8080",
      "batchSize": 32
    },
    "storage": {
      "type": "milvus",
      "milvus": {
        "host": "milvus",
        "port": 19530
      },
      "collection": "knowledge_base",
      "partition": "project_001",
      "indexType": "HNSW",
      "metricType": "COSINE"
    }
  }
}
```

### 7.2 工作流编排示例

```json
{
  "workflow": [
    {
      "name": "prepare-step",
      "mode": "StepByStep",
      "properties": [{"policies": ["s3-config"]}]
    },
    {
      "name": "vectorize-step",
      "mode": "DAG",
      "properties": [{"policies": ["docs-vectorizer", "faq-vectorizer"]}]
    },
    {
      "name": "notify-step",
      "mode": "StepByStep",
      "properties": [{"policies": ["webhook-notify"]}]
    }
  ]
}
```

### 7.3 执行流程时序图

```
用户                    API Server              Workflow Engine           VectorJobCtl
 │                          │                          │                        │
 │  POST /workflow/exec     │                          │                        │
 │─────────────────────────>│                          │                        │
 │                          │                          │                        │
 │                          │  创建 WorkflowQueue      │                        │
 │                          │  status = waiting        │                        │
 │                          │─────────────────────────>│                        │
 │                          │                          │                        │
 │  返回 task_id             │                          │                        │
 │<─────────────────────────│                          │                        │
 │                          │                          │                        │
 │                          │         Dispatcher       │                        │
 │                          │      ┌────────────────┐  │                        │
 │                          │      │ 发现 waiting   │  │                        │
 │                          │      │ 发布到 Kafka   │  │                        │
 │                          │      └───────┬────────┘  │                        │
 │                          │              │           │                        │
 │                          │              ▼           │                        │
 │                          │          Worker          │                        │
 │                          │      ┌────────────────┐  │                        │
 │                          │      │ 消费消息       │  │                        │
 │                          │      │ 生成 JobTasks  │──┼───────────────────────>│
 │                          │      └────────────────┘  │                        │
 │                          │                          │                        │
 │                          │                          │   1. 解析文档           │
 │                          │                          │   2. 文本分块           │
 │                          │                          │   3. 调用嵌入模型       │
 │                          │                          │   4. 写入 Milvus        │
 │                          │                          │<───────────────────────│
 │                          │                          │                        │
 │                          │  更新 status = completed │                        │
 │                          │<─────────────────────────│                        │
 │                          │                          │                        │
 │  GET /tasks/:id/status   │                          │                        │
 │─────────────────────────>│                          │                        │
 │                          │                          │                        │
 │  返回 completed + stats  │                          │                        │
 │<─────────────────────────│                          │                        │
```

### 7.4 进度查询与回调

#### 6.4.1 轮询查询

```bash
# 查询任务状态
GET /api/v1/vectorize/tasks/{task_id}/status

# 响应示例
{
  "task_id": "task-123",
  "status": "running",
  "progress": 45,
  "stats": {
    "totalDocuments": 100,
    "processedDocuments": 45,
    "failedDocuments": 2,
    "totalVectors": 4500
  },
  "start_time": "2025-01-15T10:00:00Z"
}
```

#### 6.4.2 Webhook 回调

在配置中指定 `progressCallback`：

```json
{
  "behavior": {
    "progressCallback": "https://your-service/webhook/vectorize"
  }
}
```

回调 Payload：

```json
{
  "task_id": "task-123",
  "event": "progress",  // progress, completed, failed
  "progress": 45,
  "stats": {...},
  "timestamp": "2025-01-15T10:05:00Z"
}
```

---

## 8. API 接口设计

### 8.1 接口列表

| 方法 | 路径 | 描述 |
|------|------|------|
| POST | `/api/v1/vectorize/tasks` | 创建向量化任务 |
| GET | `/api/v1/vectorize/tasks/:task_id` | 获取任务详情 |
| GET | `/api/v1/vectorize/tasks/:task_id/status` | 获取任务状态 |
| POST | `/api/v1/vectorize/tasks/:task_id/cancel` | 取消任务 |
| DELETE | `/api/v1/vectorize/tasks/:task_id` | 删除任务及其向量 |
| GET | `/api/v1/vectorize/tasks` | 列表查询任务 |

### 8.2 请求/响应示例

#### 创建任务

```bash
POST /api/v1/vectorize/tasks
Content-Type: application/json

{
  "name": "knowledge-base-sync",
  "project_id": "proj-001",
  "app_id": "app-001",
  "spec": {
    "source": {
      "type": "s3",
      "s3": {
        "bucket": "documents",
        "prefix": "kb/"
      }
    },
    "embedding": {
      "provider": "tei",
      "model": "BAAI/bge-m3",
      "endpoint": "http://embedding-service:8080"
    },
    "storage": {
      "type": "milvus",
      "collection": "knowledge_base",
      "milvus": {
        "host": "milvus",
        "port": 19530
      }
    }
  }
}

# 响应
{
  "task_id": "vec-task-20250115-abc123",
  "status": "waiting",
  "createdAt": "2025-01-15T10:00:00Z"
}
```

#### 查询状态

```bash
GET /api/v1/vectorize/tasks/vec-task-20250115-abc123/status

# 响应
{
  "task_id": "vec-task-20250115-abc123",
  "status": "running",
  "progress": 67,
  "stats": {
    "totalDocuments": 150,
    "processedDocuments": 100,
    "failedDocuments": 3,
    "totalChunks": 2500,
    "totalVectors": 2500,
    "processingTimeMs": 45000,
    "embeddingTimeMs": 30000,
    "storageTimeMs": 5000
  },
  "start_time": "2025-01-15T10:00:00Z"
}
```

---

## 9. 配置参考

### 9.1 向量化配置项

```yaml
# config.yaml
vectorize:
  # 嵌入模型默认配置
  embedding:
    provider: "tei"                    # ollama, tei, openai
    endpoint: "http://embedding-service:8080"
    model: "BAAI/bge-m3"
    batchSize: 32
    timeout: "60s"
  
  # 向量存储默认配置
  storage:
    type: "milvus"
    milvus:
      host: "milvus"
      port: 19530
    indexType: "HNSW"
    metricType: "COSINE"
  
  # 文档处理默认配置
  processing:
    chunkStrategy: "semantic"
    chunkSize: 512
    chunkOverlap: 50
    language: "auto"
    enableOCR: false
  
  # 任务行为默认配置
  behavior:
    onError: "skip"                    # fail, skip, retry
    maxRetries: 3
    concurrency: 4
```

### 9.2 命令行参数

```bash
# 向量化相关参数
--vectorize-embedding-provider=tei
--vectorize-embedding-endpoint=http://embedding-service:8080
--vectorize-embedding-model=BAAI/bge-m3
--vectorize-milvus-host=milvus
--vectorize-milvus-port=19530
--vectorize-chunk-size=512
--vectorize-batch-size=32
```

---

## 10. 部署清单

### 10.1 完整部署顺序

1. **部署 Milvus 向量数据库**
   ```bash
   kubectl apply -f deploy/vectordb/milvus-standalone.yaml
   ```

2. **部署嵌入模型服务**
   ```bash
   # GPU 环境
   kubectl apply -f deploy/embedding/tei-bge-m3.yaml
   
   # CPU 环境
   kubectl apply -f deploy/embedding/ollama.yaml
   ```

3. **更新 KubeMin-Cli 配置**
   ```bash
   kubectl edit configmap kubemin-config -n kubemin-system
   # 添加向量化配置
   ```

4. **重启 API Server**
   ```bash
   kubectl rollout restart deployment kubemin-apiserver -n kubemin-system
   ```

### 10.2 健康检查

```bash
# 检查嵌入服务
curl http://embedding-service:8080/health

# 检查 Milvus
curl http://milvus:9091/healthz

# 测试嵌入
curl -X POST http://embedding-service:8080/embed \
  -H "Content-Type: application/json" \
  -d '{"inputs": ["测试文本"]}'
```

### 10.3 监控指标

| 指标名称 | 类型 | 描述 |
|----------|------|------|
| `vectorize_tasks_total` | Counter | 向量化任务总数 |
| `vectorize_documents_processed` | Counter | 处理的文档数 |
| `vectorize_vectors_created` | Counter | 创建的向量数 |
| `vectorize_embedding_duration_seconds` | Histogram | 嵌入耗时 |
| `vectorize_storage_duration_seconds` | Histogram | 存储耗时 |
| `milvus_collection_vector_count` | Gauge | Collection 向量数量 |

---

## 附录

### A. 依赖库

```go
// go.mod 新增依赖
require (
    github.com/milvus-io/milvus-sdk-go/v2 v2.3.4
    github.com/ledongthuc/pdf v0.0.0-20220302134840-0c2507a12d80
    github.com/unidoc/unipdf/v3 v3.50.0  // PDF 解析
    github.com/nguyenthenguyen/docx v0.0.0-20230621112118-9c8e795a11d5  // DOCX 解析
    github.com/yuin/goldmark v1.6.0  // Markdown 解析
    golang.org/x/net v0.20.0  // HTML 解析
)
```

### B. 错误码定义

```go
// utils/bcode/003_vectorize.go
var (
    ErrVectorizeTaskNotFound     = NewBcode(50001, "vectorize task not found")
    ErrVectorizeTaskExists       = NewBcode(50002, "vectorize task already exists")
    ErrVectorizeSourceInvalid    = NewBcode(50003, "invalid source configuration")
    ErrVectorizeEmbeddingFailed  = NewBcode(50004, "embedding model request failed")
    ErrVectorizeStorageFailed    = NewBcode(50005, "vector storage operation failed")
    ErrVectorizeDocumentParseFailed = NewBcode(50006, "document parse failed")
)
```

### C. 相关文档

- [工作流架构详解](./workflow-architecture-guide.md)
- [Kafka 队列实现](./kafka-queue-implementation.md)
- [Milvus 官方文档](https://milvus.io/docs)
- [HuggingFace TEI 文档](https://huggingface.co/docs/text-embeddings-inference)

---

*文档版本：1.0.0*
*创建日期：2025-01*
*最后更新：2025-01*
