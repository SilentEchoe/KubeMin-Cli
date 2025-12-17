# Shared Bundle（命名空间内公共资源包）部署策略

本文描述 KubeMin 在“同一命名空间内只应存在一套公共组件（多资源集合）”场景下的部署策略设计与实现。

## 背景与目标

在应用部署时，某些组件可能是“命名空间级共享”的，例如一组固定名称的 `Deployment`、`Service`、`ConfigMap`、`RBAC` 等资源：

- 同一命名空间只允许存在一套（single instance）。
- 默认只“创建一次”后静默运行，不希望每次应用部署都对其做 reconcile。
- 该公共组件不应绑定在任一应用的卸载生命周期中（retain），避免误删影响其他应用。

## 核心抽象：Bundle Trait

通过 `traits.bundle` 将组件标记为某个共享资源包（bundle）的成员。Bundle 的关键点是：

- **已安装判定**：通过一个 **锚点资源（anchor）** 判定 bundle 是否已安装。
- **默认策略**：`CreateIfAbsent`（只创建，不接管，不升级）。

### Trait 结构

`traits.bundle` 结构如下：

```json
{
  "bundle": {
    "name": "shared-redis",
    "anchor": {
      "kind": "ConfigMap",
      "name": "kubemin-bundle-shared-redis"
    }
  }
}
```

字段说明：

- `bundle.name`：bundle 标识（同一命名空间内应唯一）。
- `bundle.anchor.kind`：锚点资源类型，目前支持：
  - `ConfigMap`（推荐，作为安装标记）
  - `Deployment`（将某个固定名称 Deployment 作为判定锚点）
- `bundle.anchor.name`：锚点资源名称。
  - 当 `kind=ConfigMap` 且 `name` 为空时，系统默认使用 `kubemin-bundle-<bundle.name>`（会做 RFC1123 规范化）。

## 行为语义

### 1) 是否已安装（Bundle Skip）

当锚点资源存在且带有 label `kubemin.io/bundle=<bundle.name>`：

- 该 bundle 下的所有 Job 直接标记为 `skipped`，不会对任何资源进行 create/update/patch。

如果锚点资源存在但 label 不匹配：

- 视为冲突，Job 失败（避免误把其他系统资源当作 bundle）。

### 2) Create-Only（不升级、不接管）

当锚点不存在时：

- 对 bundle 成员资源执行“创建优先”逻辑：
  - 资源不存在：创建
  - 资源已存在且带有 `kubemin.io/bundle=<bundle.name>`：**不更新**（跳过）
  - 资源已存在但不属于该 bundle：冲突失败

### 3) Anchor ConfigMap 自动创建（推荐路径）

当 `anchor.kind=ConfigMap` 时，工作流会在低优先级阶段追加一个“锚点 ConfigMap 创建 Job”：

- 该 ConfigMap 仅作为 bundle 安装标记，不承载业务配置。
- 该 Job 在 bundle 内其他资源 Job 成功执行后再创建，用于后续部署的快速跳过。

### 4) Retain（不绑定应用卸载）

当组件声明了 `traits.bundle`：

- 应用资源清理（`CleanupApplicationResources`）会跳过该组件的所有资源删除逻辑。

## 命名与标签

### 命名

bundle 成员的工作负载命名不再追加 appID，改为“无 app 作用域”的稳定命名：

- Deployment：`deploy-<componentName>`
- Service：`svc-<componentName>`
- StatefulSet：`store-<componentName>`
- Ingress：`ing-<name>`

（会进行 RFC1123 规范化与截断）

### 标签

bundle 成员资源会带上以下标签（用于判定/冲突检测/selector）：

- `kubemin.io/bundle=<bundle.name>`
- `kubemin.io/bundle-member=<component.name>`
- `kube-min-cli-appId=bundle:<bundle.name>`（用于满足 informer 过滤器 “label key 存在” 的要求）

注意：bundle 资源默认不写入 `kube-min-cli-componentId` / `kube-min-cli-componentName`，避免状态同步误绑定到某个具体应用的组件记录。

## 使用建议

1. 优先使用 `anchor.kind=ConfigMap`，并让系统自动创建锚点 ConfigMap。
2. 如果需要“升级为公共组件”，建议新建一个新的 bundle（新的 `bundle.name` + 新的固定命名），而不是对既有 bundle 做 reconcile。
3. 如需支持“半安装修复（CreateMissing）/显式 uninstall”等运维能力，可在此基础上扩展，但默认策略保持简单。

