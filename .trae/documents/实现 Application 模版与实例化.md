## 背景与目标
- 目标：把现有“应用”抽象为可复用的“模版”，通过最少的参数（如实例名）快速创建多个相同应用实例（如 MySQL 5.7），自动生成组件、ConfigMap/Secret、PVC、Service、Ingress 等资源，并复用现有工作流与命名规则。
- 现有能力可直接复用：
  - 统一资源命名：`pkg/apiserver/workflow/naming/naming.go:17`、`pkg/apiserver/workflow/naming/naming.go:37`
  - PVC 模版转 VolumeClaimTemplates：`pkg/apiserver/workflow/traits/storage.go:81`、`pkg/apiserver/workflow/traits/processor.go:405`
  - 工作流任务生成与超时统一：`pkg/apiserver/event/workflow/job_builder.go:26`、`pkg/apiserver/event/workflow/job_builder.go:136`、`pkg/apiserver/event/workflow/controller.go:143`

## 模版数据结构
- 新增实体：`ApplicationTemplate`
  - 基础字段：`id`, `name`, `version`, `alias`, `description`, `icon`, `project`
  - 业务字段：`components[]`（组件蓝图）、`traits[]`（如存储/网络特性）、`parameters[]`（可注入参数定义与默认值）、`workflows[]`（默认工作流步骤）、`immutableFields[]`（实例化不可覆盖的字段）
  - 存储：沿用 `datastore.DataStore`（表名建议 `min_app_templates`，与 `min_applications` 一致的命名策略）

## 参数化与命名
- 占位符策略：在模版中允许 `${app.name}`, `${app.id}`, `${param.X}`, `${component.name}` 等占位符，实例化时进行无副作用替换。
- 命名复用：统一使用命名工具生成可复用的 RFC‑1123 名称（如 `deploy-<component>-<appID>`、`svc-...`、`ing-...`、`pvc-...`）：
  - `WebServiceName`: `pkg/apiserver/workflow/naming/naming.go:17`
  - `PVCName`: `pkg/apiserver/workflow/naming/naming.go:37`
- 唯一性与可读性：
  - 把“实例名”作为 `appID` 或参与资源名的组件段，以确保跨实例名称唯一（例如 `mysql-<instanceName>`）。
  - ConfigMap/Secret 按 `${app.id}` 后缀加盐，避免同名覆盖。

## API 设计
- 创建模版：`POST /api/v1/application-templates`
  - 请求：`CreateApplicationTemplateRequest{ name, version, components[], traits[], parameters[], workflows[] }`
  - 响应：`ApplicationTemplateBase{ id, name, version }`
- 列表/查询：`GET /api/v1/application-templates`、`GET /api/v1/application-templates/:id`
- 实例化应用：
  - `POST /api/v1/applications/from-template`
  - 请求：`InstantiateFromTemplateRequest{ templateId, instanceName, namespace, project, overrides{ parameters, env, configmapData, storageSize, imageTag } }`
  - 响应：`ApplicationBase{ id, name, version }` + 可选 `workflowId`
- 可选联动：支持 `POST /applications/:id/workflow/exec` 立即执行默认工作流（已有接口，路由见 `pkg/apiserver/interfaces/api/applications.go:30,83`）。

## 服务端实现步骤
- 解析模版：在 `ApplicationsService` 新增 `InstantiateFromTemplate(ctx, req)`
  - 读取模版 → 构造 `CreateApplicationsRequest`
  - 占位符渲染：把 `${...}` 替换为 `instanceName`, `namespace`, `overrides` 值
  - 资源命名：统一经 `naming` 生成（避免手写字符串）；PVC 创建路径复用 Traits 模型（见 `storage.go` 与 `processor.go` 引用）
- 写入与工作流：调用现有 `CreateApplications` 完成入库并返回 `ApplicationBase`；如请求指定执行，则生成并调用默认工作流（`GenerateJobTasks`/`RunJobs` 复用）。
- 幂等策略：若同名应用已存在，返回冲突；提供 `--replace` 模式时执行安全覆盖（校验不可变字段）。

## 校验与安全
- 名称合法性：全量经 `utils.ToRFC1123Name`；长度截断 63；空值回退策略与现有命名工具一致。
- 唯一性校验：`DataStore.IsExist` 按 `name+namespace+project` 检查应用名、ConfigMap/Secret 及 PVC 名冲突。
- 资源配额：校验 `storageSize`、副本数、端口占用与命名空间配额。
- 凭据治理：Password/RootSecret 强制从 Secret 引用，禁止明文；每实例独立 Secret 名。
- PVC 模版：对 StatefulSet 使用 `VolumeClaimTemplates`（见 `pkg/apiserver/workflow/traits/processor.go:405`）。

## MySQL 5.7 模版示例（最小可用）
- 参数：`instanceName`, `namespace`, `storageSizeGi`, `rootPasswordSecretRef`, `imageTag="5.7"`
- 组件：
  - StatefulSet：`deploy-mysql-${app.id}`，`replicas=1`，镜像 `mysql:${param.imageTag}`，挂载 `PVCName("data", ${app.id})`
  - Service：`svc-mysql-${app.id}`，端口 `3306`
  - PVC 模版：`pvc-data-${app.id}`（Traits 标注 `template`）
  - Secret：`secret-mysql-${app.id}`（引用 `rootPasswordSecretRef`）
- 默认工作流：步骤① 应用 StatefulSet/Service/Secret；步骤② 等待 Readiness；步骤③ 标记完成。

## 兼容性与迁移
- 与现有路由/服务解耦：新增 API 与 Service，不影响 `PUT /applications/:appID/workflow`。
- 命名与对象生成保持一致：完全复用 `naming` 与 `job_builder/controller` 的行为。
- 逐步引入：先支持“原生模板驱动”，后视需要扩展 Helm 驱动（与 `deploy/helm` 模板保持一致）。

## 验证与回滚策略
- 预检：解析后生成“渲染预览”，仅校验且不落库；通过后才实例化。
- 执行验证：创建后触发默认工作流并跟踪状态；失败自动回滚（删除已创建对象，保留审计日志）。
- 审计与可观测：为实例化过程打工作流与步骤级 Span；错误上报沿用 errgroup 与通道治理。

---
如认可该方案，我将按以上步骤实现：新增模版模型与 API、在 Service 层完成渲染与实例化、复用现有命名与 PVC 模版机制，并交付一个可直接创建多实例的 MySQL 5.7 模版。