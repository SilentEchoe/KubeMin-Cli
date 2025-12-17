# 草案：Shared Bundle 与 Tmp 模板结合方案

本文是技术草案，描述如何将“命名空间共享资源包（Shared Bundle）”的 Create-Only 部署逻辑，与 `Tmp` 模板能力结合，支持在模板中声明“公共组件”。

> 目标场景：应用部署时某些组件是公共的（同一命名空间只应存在一套），默认只创建一次并静默运行，不随任一应用卸载而删除。

## 现状（已实现）

当前代码已支持通过 `traits.bundle` 将组件标记为 bundle，并实现：

- **Create-Only**：资源不存在则创建；已存在且属于同 bundle 则不更新；已存在但不属于该 bundle 则冲突失败。
- **Anchor Skip**：当锚点资源（anchor）存在且匹配 `kubemin.io/bundle=<bundle.name>` 时，该 bundle 下 Job 直接 `skipped`。
- **Retain**：应用清理逻辑会跳过带 `traits.bundle` 的组件，不删除其资源。
- **稳定命名**：bundle 组件的 Deployment/Service/StatefulSet 等命名不再追加 appID，改为“无 app 作用域”的稳定命名。

## 目标

将 Shared Bundle “作为模板的一部分”暴露给用户，但保持公共组件的核心语义：

- 模板可以声明公共组件（bundle member）。
- 多个应用实例（由同一模板或不同模板）可以共享同一个公共 bundle。
- 公共组件默认不升级（如需升级，用新的 bundle 名称创建一套新公共组件）。
- 公共组件不绑定到应用生命周期（retain）。

## 非目标（本草案不覆盖）

- 自动 reconcile/升级既有公共组件（“Always/Reconcile”）。
- 自动删除公共组件（uninstall）与依赖关系回收。
- “半安装修复（CreateMissing）”等运维修复能力（可作为后续扩展）。

## 设计原则

### 1) 公共组件必须具备稳定身份

模板中的公共组件必须能在命名空间内形成稳定、唯一的身份（identity），否则每个应用实例都会创建一套新资源，违背“共享”的定义。

推荐将 identity 定义为：

- `bundle.name`：公共组件集的逻辑名称（命名空间内唯一）
- `anchor`：锚点资源（推荐 ConfigMap）名称（命名空间内唯一）
- `component.name`：bundle 成员名（用于稳定生成 `deploy-<name>` / `svc-<name>` 等资源名）

### 2) 默认不升级：用新 bundle 名称演进

当需要“把某组件升级成公共组件的更高版本”，建议：

- 新建一个新的 `bundle.name`（例如 `shared-redis-v2`）
- 在模板/应用中切换依赖到新 bundle 的服务名/ConfigMap/Secret

这样避免对线上既有公共组件做隐式 reconcile，符合“只创建不打扰”的产品预期。

### 3) 依赖关系通过引用表达

应用模板中的业务组件与公共组件的关系，建议通过 Kubernetes 原生引用表达：

- 通过 Service DNS 访问：`svc-<bundleMemberName>.<ns>.svc`
- 通过 `envFrom/configMapKeyRef/secretKeyRef` 引用公共 ConfigMap/Secret
- 通过 RBAC/ServiceAccount 引用（谨慎，注意权限边界）

而不是让公共组件“成为应用的一部分”并随应用卸载删除。

## 结合方案（推荐路径）

### A. 直接在模板中声明 bundle 组件（最简单、与现实现完全匹配）

模板的组件列表中允许出现 bundle 组件，只需满足“稳定命名”约束：

- `component.name` 固定（不要按 appName/实例参数动态变化）
- `traits.bundle.name` 固定（或可控地按“公共组件版本”变化）
- `traits.bundle.anchor` 固定（推荐 `ConfigMap/kubemin-bundle-<bundle>`）

示例（JSON，伪结构）：

```json
{
  "component": [
    {
      "name": "redis",
      "type": "webservice",
      "namespace": "default",
      "image": "redis:7",
      "replicas": 1,
      "traits": {
        "bundle": {
          "name": "shared-redis",
          "anchor": { "kind": "ConfigMap", "name": "kubemin-bundle-shared-redis" }
        }
      }
    },
    {
      "name": "api",
      "type": "webservice",
      "namespace": "default",
      "image": "example/api:1.0.0",
      "replicas": 2,
      "properties": {
        "env": { "REDIS_ADDR": "svc-redis:6379" }
      }
    }
  ]
}
```

运行时行为：

- 第一次部署：bundle 组件资源被创建，最后创建 anchor ConfigMap。
- 后续部署（同命名空间）：发现 anchor 存在 → bundle 组件 Job 全部 `skipped`，不会更新/覆盖公共组件。

优点：

- 不引入新的 API/工作流概念，最少改动。
- 与当前实现完全一致。

风险/要求：

- 必须严格控制模板参数化：公共组件相关字段不可随应用实例变化。
- 公共组件创建失败会导致本次应用部署失败（这是“严格模式”，避免静默半安装）。

### B. 模板预装（Preinstall）公共 bundle（后续可选增强）

如果你不希望“应用工作流”承担公共组件的创建（例如希望公共组件作为集群/命名空间初始化步骤），可以引入“模板预装步骤”：

- 当用户选择某模板创建应用时：
  1) 先执行 `EnsureBundleInstalled(bundleSpec)`（独立流程）
  2) 再创建应用及其工作流（应用工作流中不包含 bundle 组件）

优点：

- 公共组件更像“命名空间基建”，不会掺入应用组件列表与工作流记录。

缺点：

- 需要新增 API/交互面（例如 UI 按钮、命令行子命令）与执行记录。
- 需要处理并发预装与观测（安装锁、状态上报）。

本草案建议优先落地方案 A，确认用户使用习惯后再评估方案 B。

## 参数化规则（模板作者指南）

为避免“每个实例都创建一套公共组件”，建议对模板参数化做约束：

**允许参数化：**

- 镜像/副本（仅在“新 bundle 名称”方案中用于版本升级演进）
- 业务组件对公共组件的引用地址（例如 `REDIS_ADDR`）

**禁止参数化：**

- `bundle.name`（除非是显式版本演进，例如 `shared-redis-v2`）
- `bundle.anchor.name`
- bundle 成员 `component.name`（它直接决定稳定资源名 `deploy-<name>` / `svc-<name>`）

## 冲突处理策略（模板与环境不一致时）

当发现资源已存在但不属于目标 bundle（label 不匹配）：

- 默认失败并返回冲突信息（资源名、命名空间、期望 bundle、现有 label）。
- 由用户手动确认是否：
  - 改用新的 bundle.name（新建一套公共组件），或
  - 手动清理/迁移既有资源

这与“只创建不打扰”的产品原则一致：不做隐式接管（adopt）。

## 观测与状态

由于 informer 过滤器依赖 `kube-min-cli-appId` label key 的存在，bundle 资源会设置：

- `kube-min-cli-appId=bundle:<bundle.name>`

同时避免写入 `kube-min-cli-componentId` / `kube-min-cli-componentName`，以免将公共组件状态错误写回某个应用组件记录。

## 后续扩展（可选）

- `bundle repair --create-missing`：仅补齐缺失资源，不更新已存在资源。
- `bundle uninstall <name>`：显式删除公共组件（可要求二次确认/force）。
- “兼容性校验”模式：anchor 存在时读取 `version/spec-hash`（仅用于诊断或阻止明显不兼容的依赖）。

