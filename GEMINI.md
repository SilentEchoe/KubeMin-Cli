# GEMINI.md — KubeMin-Cli 项目级指南

## 1) 项目概览
- 本项目是一个云原生 PaaS CLI，基于 Go 和 Kubernetes，目标是让业务团队通过简单 JSON 描述快速部署服务（Web、Stateful、AI 应用、中间件等）。
- 架构核心：组件（component）+ traits（env/storage/sidecar/probe 等）+ workflow job 执行器。
- 平台抽象：屏蔽 PVC、ConfigMap、Secret 等底层细节，通过统一 trait 描述。

## 2) 协作规则
- **子目录 GEMINI.md 优先于本文件**，如果有冲突，执行更具体的规则。
- 所有输出必须带解释：说明为什么这样做、对现有 trait/组件的影响。
- 不要擅自引入外部依赖或打破 trait 抽象；如果必要，先产出“影响分析”。

## 3) 构建与运行`
- 本地运行 CLI：`go run ./cmd/main.go`

## 4) Go 代码规范
- 错误必须向上返回并加上下文：`fmt.Errorf("create pvc: %w", err)`
- 日志统一使用 `klog`，禁止使用 `fmt.Println`
- 并发必须考虑取消与超时：`context.Context` 必须传入所有 goroutine
- 使用 `errgroup` 或 `WaitGroup` 控制并发，禁止裸 goroutine

## 5) Kubernetes 约束
- 清单统一放在 `/deploy` 下，命名规则：`<app>-<component>`
- **存储 trait**：
    - `persistent` → PVC（自动生成）
    - `ephemeral` → emptyDir
    - `config` / `secret` → 仅挂载文件，不直接做 env 注入
- **Sidecar trait**：允许作为日志收集、代理等扩展，必须显式声明
- 所有 Deployment/StatefulSet 必须成对设置 requests/limits
- 不使用 `:latest` 镜像，必须指定 semver 或 commit hash

## 6) Workflow/Job 规则
- JobCtl 接口：`Run / Clean / SaveInfo`，每个实现必须保证幂等
- Workflow 执行顺序：
    1. 生成资源（PVC/ConfigMap/Secret 等）
    2. 部署组件（Deployment/StatefulSet）
    3. 挂载/Sidecar/探针配置
    4. 状态校验（等待 Available/Active）
- 日志与状态：
    - 每个 JobTask 在运行时写入数据库，失败原因必须保存
    - 允许并发执行，但同一 component 下的 Job 串行执行

## 7) 提问与请求格式
- **修复请求**：
    - “定位并修复 X，给出 diff，解释影响与回滚方案。”
- **生成请求**：
    - “在 `/pkg/job` 新增 `StatefulSetJob`，满足 `JobCtl` 接口，附带单元测试覆盖失败场景。”
- **配置请求**：
    - “生成 MySQL StatefulSet（1 副本，30Gi 存储），必须支持 initContainer 初始化。”

## 8) 安全与合规
- 不要在输出中暴露实际 secret，统一用 `<SECRET_NAME>` 占位
- 涉及外部云服务（NAS/ECS/Kafka）时，优先给出 mock/本地可运行示例
- 配置文件中禁止硬编码 StorageClass / Secret 名称，必须可配置

## 9) 优先级与冲突
- 根目录 GEMINI.md = 全局规范
- 子目录 GEMINI.md > 根目录 GEMINI.md
- 冲突时优先更具体的规则，并在输出中标注“依据 X 规则”

## 10) 常用命令
- 生成 CRD 与 deepcopy：`make generate`
- 执行全部单元测试：`make test`
- 启动 Kafka/Redis/MySQL 本地环境：`docker-compose up`
- 查看 Workflow 日志：`kubemin-cli logs --workflow <id>`