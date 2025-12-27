# 基于 template.id 的应用模板实例化设计

## 背景与目标
- 支持用户在请求体中提供 `template:{id:{{app_id}}}`，以数据库 `min_applications` 中已有应用作为模板，快速创建多个相同形态的新应用（如多套 MySQL）。
- 在实例化过程中，用用户传入的组件 `name`（例：`fnlz2z1lxe85k3me66og`）覆盖模板中的组件名称及相关子字段（如存储名），并生成新的应用 `name`/`alias` 等元数据。
- 实例化结果写入数据库，新增的应用可选择成为“标准模板”：`min_applications` 表新增列 `tmp_enable`（bool，默认 false），用来标记该应用是否允许被其他请求作为模板引用。
- 保证生成的资源可用、无冲突、可追溯，且操作幂等。

## 输入示例
```json
{
  "name": "fnlz2z1lxe85k3me66og-mysql",
  "alias": "mysql",
  "version": "1.0.0",
  "project": "",
  "description": "Create template Mysql",
  "component": [
    {
      "name": "fnlz2z1lxe85k3me66og",
      "template": { "id": "4tbupjg43ln3yj249l0v0fv8", "target": "mysql" }
    }
  ]
}
```

## 处理流程（草案）
1. 校验顶层字段：`name/alias/version` 必填；`component` 非空；`template.id` 必填且格式合法。
2. 查询模板：按 `template.id` 读取 `min_applications` 及其组件；校验模板状态可用且 `tmp_enable=true` 时才允许被引用（若为 false，则拒绝作为模板来源）。
3. 克隆模板：
   - 复制模板应用的结构（组件、traits、workload 配置等）。
   - 应用字段替换规则（见下）。
   - 对需要唯一性的字段生成新值（ID、端口、PVC 名、Service 名等）。
4. 幂等性检查：以顶层 `name`（或显式幂等 token）做幂等键；若已存在同名实例则返回已存在/幂等结果。
5. 冲突检测：检查命名冲突（组件名、PVC/Service 名）、端口占用、项目/命名空间下资源配额。
6. 写入/创建：
   - 在事务内将“新应用”落库到 `min_applications`，默认 `tmp_enable=false`，除非用户/系统指定该实例应作为标准模板。
   - 同时写入新组件及关联资源，并触发后续创建流程；失败则回滚或标记可重试状态。
7. 返回结果：返回新应用 ID、组件列表、来源模板信息（`templateId`、版本），以及新应用的 `tmp_enable` 状态。

## 字段替换规则
- 保留模板：镜像、配置 schema、必需的运行参数、trait 类型等模板定义本身。
- 使用用户应用元数据覆盖：应用 `name`/`alias`/`description`/`version`/`project`。
- 组件级覆盖（按传入组件 `name`）：
  - `component.name` 统一替换模板组件的 `name`。
  - 特征中的资源名（例：PVC/Storage 名、Service 名、Deployment 名称前缀）以新组件名为前缀/整体替换，保持命名约定。
  - 持久化存储命名：当模板存储名是通用名（如 `data`）或包含模板组件名时，实例化会用请求组件名或顶层应用名作为前缀重写（如 `tenant-a-mysql-app-data`），并同步到 init/sidecar 中同名存储；`claim_name`/`source_name` 仅在显式提供时按映射规则重写，不自动填充默认值。
  - 存储创建策略：`create=true` 视为使用 volumeClaimTemplates 风格创建 PVC（按重写后的存储名生成 PVC，并在主容器/init/sidecar 以同名卷挂载，`claim_name` 可留空）；`create=false` 表示引用已有 PVC，实例化后依赖重写后的 `name`/显式 `claim_name` 绑定现有 PVC，不会创建新卷。
  - 若模板包含副本数/计算资源等可调参数，可允许用户覆盖；未提供则沿用模板默认。
  - 支持显式覆盖：`properties.env` 可用用户输入覆盖模板同名环境变量；`properties.secret` 仅对 `type=secret` 的组件生效，用于覆盖模板的 Secret 数据。
  - 精确匹配：`template.target`（模板组件名）用于指定要覆盖的模板组件；未提供时按类型优先匹配，可能存在不确定性。
- 必须重生成：
  - 组件/trait 唯一 ID、内部 UID。
  - 端口号若有冲突需重新分配（保留模板端口偏好，冲突时寻找可用端口）。
- 禁止直接复制：
  - Secret/密码类字段，必须使用占位符或从密钥管理加载。
  - 与运行时绑定的标识（如主机名、PVC 绑定 UID）。
- 默认不重写：
  - RBAC 类特征（ServiceAccount/Role/Binding 名）保持模板定义；命名空间对齐组件命名空间（为空则使用默认命名空间）。

## 标签与审计
- 创建时为应用与组件添加标签/注解：
  - `templateId=<template.id>`
  - `templateVersion=<模板版本>`
  - `origin=templated`
  - `createdBy=<user>`、`createdAt=<ts>`
- 审计日志记录模板 ID、请求体、生成的资源名/端口分配。

## 错误与返回码示例
- 404：`template.id` 对应模板不存在或不可用。
- 409：命名/端口/配额冲突；重复的幂等请求。
- 400：请求体缺失必填字段或校验失败。
- 500：数据库或下游创建失败（需包含可重试/不可重试标识）。

## 数据库与迁移
- 在 `min_applications` 表新增列 `tmp_enable`（bool，默认 false），表示该应用是否允许作为模板被引用。
- 迁移要求：
  - 默认值为 false，老数据不自动成为模板。
  - 对现有“官方模板”可通过离线脚本或后台管理界面批量设置 `tmp_enable=true`。
  - 索引/查询：若模板查询频繁，可在 `tmp_enable` 与 `id` 上建立组合索引以加速过滤。

## 测试要点
- 正常路径：基于模板成功创建应用，组件名称和存储名被正确替换。
- 存储重写：持久化存储在主组件及 init/sidecar 中的名称保持一致的重写结果，`claim_name`/`source_name` 仅在声明时跟随重写，不自动补全。
- 存储创建策略：`create=true` 的模板实例化后应生成新 PVC 并挂载同名卷；`create=false` 的模板实例化后应引用已存在的 PVC（按重写后的 `name`/显式 `claim_name`），否则挂载会失败。
- 幂等：相同 `name` 或幂等 token 的重复请求只创建一次。
- 冲突：端口/名称冲突时返回 409，不产生脏资源。
- 敏感信息：模板中含 Secret 占位符时，实例化要求用户提供或从配置加载；拒绝直接复制明文。
- 事务性：中途失败时数据库和已创建资源回滚/清理。
- 标签追踪：新实例包含来源模板标签/注解。

## 测试示例与验证步骤
- 创建模板应用：调用 `/api/v1/applications` 创建基础模板，设置 `tmp_enable=true`，组件 traits 含存储/Ingress/RBAC 等资源命名，确保模板组件具备镜像与必需字段。
  ```json
  {
    "name": "tmpl-mysql",
    "alias": "mysql-template",
    "version": "1.0.0",
    "project": "demo",
    "description": "mysql base template",
    "component": [
      {
        "name": "mysql",
        "type": "store",
        "image": "mysql:8.0",
        "namespace": "default",
        "replicas": 1,
        "properties": { "ports": [ { "port": 3306, "expose": true } ], "env": { "MYSQL_ROOT_PASSWORD": "changeme" } },
        "traits": {
          "storage": [ { "name": "mysql", "type": "persistent", "create": true, "size": "5Gi" } ],
          "rbac": [ { "service_account": "mysql", "role_name": "mysql", "binding_name": "mysql" } ]
        }
      }
    ],
    "tmp_enable": true
  }
  ```
- 克隆创建新应用（成功）：请求体中仅提供组件名和 `template.id`。期望组件名及存储/Ingress/RBAC/EnvFrom/init/sidecar 中的资源名统一替换为新组件名。
  ```json
  {
    "name": "tenant-a-mysql-app",
    "alias": "tenant-a-mysql",
    "version": "1.0.1",
    "description": "mysql cloned from template",
    "component": [
      { "name": "tenant-a-mysql", "type": "store", "template": { "id": "tmpl-mysql-id" } }
    ]
  }
  ```
  验证：调用 `/api/v1/applications/{appID}/components`，检查组件名、traits.storage 的 `name`（以及已声明的 `claim_name`/`source_name`）被重写，Ingress backend 的 `service_name`、RBAC 的 `service_account/role_name/binding_name` 等按规则替换为新组件名。
- 覆盖规则（单模板多条目不重复克隆）：如果同一个 `template.id` 在请求中出现多次，只克隆一套模板组件，后续同模板条目仅用于覆盖匹配的组件（按类型优先匹配），支持：
  - 组件重命名：同模板多个条目可为不同组件指定新名称。
  - 环境变量覆盖：`properties.env` 覆盖模板 env。
  - Secret 覆盖：仅 `type=secret` 组件的 `properties.secret` 会覆盖模板 Secret 数据。
  - 精确匹配：推荐在 `template` 中提供 `target`（模板组件名），确保覆盖应用到指定组件；缺失 target 时按类型匹配并存在不确定性。
  未匹配的模板组件按 baseName（若提供）或模板组件名生成新名称。
- 模板未启用错误：当模板 `tmp_enable=false` 时，同样的克隆请求应返回 400，消息 `template application is not enabled`。
- 模板缺失或 ID 为空：`template.id` 为空应返回 400（`template id is required`）；不存在的 ID 返回 404（`application name is not exist`）。
- 多组件模板命名：模板包含多个组件（如 `api`、`worker`）时，请求组件名为 `foo-app`，预期实例化后组件名为 `foo-app-api` 与 `foo-app-worker`，并同步重写相关资源名。
- 幂等/冲突：重复提交相同顶层 `name`（或幂等键）应返回已存在；模板组件缺少必需镜像应报 `the image of the component has not been set..`。
- 数据库检查：确认 `min_applications.tmp_enable` 新列存在；模板行应为 `true`，克隆出的应用默认为 `false`（除非请求显式设置）。
- 自动化单测参考：运行 `go test ./pkg/apiserver/domain/service -run Template -count=1` 覆盖模板校验与克隆逻辑（需具备写 Go build 缓存权限）。

## 待决问题
- 模板升级策略：模板更新后是否影响已实例化的应用（建议默认不回溯，只记录来源版本）。
- 覆盖字段白名单：哪些模板字段允许用户覆盖，是否提供显式的 override 列表。
- 端口分配策略：端口冲突时的自动分配规则与可配置范围。
- 幂等键：仅使用应用 `name` 还是允许显式 `idempotencyKey`。
- 项目/命名空间绑定：顶层 `project` 为空时的默认归属策略。
- 优化提案（待定）：为 `component.template` 增加 `overrideMode`（如 `merge|replace|none`）按组件粒度控制合并/替换/禁用覆盖。需明确：
  - 覆盖范围：`merge` 合并 env/secret/config 等 map，`replace` 整体替换这些字段，`none` 仅克隆模板。
  - 匹配规则：同一模板多条目按组件类型或显式 target 匹配，避免重复克隆。
  - 限制校验：合并后对 ConfigMap/Secret 等大小限制做校验，超限时返回错误并提示改用 `replace`，保持行为可预期。
  - 组件定位：考虑在 `component.template` 增加 `target`（指向模板组件名/唯一键），请求侧校验 target 存在且类型匹配；若未提供 target，则保留当前类型匹配策略，但需告知顺序不确定或直接报错。
