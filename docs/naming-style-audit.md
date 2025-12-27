# 命名与风格修正记录

## 范围
审查 Go 源码中的命名与风格使用情况，重点关注 Go 命名规范、API/JSON 标签一致性以及项目日志规范。

## 修正要点
1) API JSON 统一为 snake_case。
示例：`create_time`、`workflow_id`、`tmp_enable` 等字段保持一致的命名风格。

2) 结构体字段命名统一为 Go 惯例。
示例：将 `NameSpace` 统一为 `Namespace`，JSON key 统一为 `namespace`。

3) 修复 JSON 标签拼写与字段含义不一致问题。
示例：`template`、`product_id`；将 `Info` 更名为 `ServiceType` 以匹配 `service_type`。

4) ID 相关 JSON key 统一为 snake_case，label key 保持不变。
示例：`app_id`、`workflow_id`、`project_id`；`kube-min-cli-appId` label key 保持原样。

5) 接口命名移除 `I` 前缀。
示例：`ICache` -> `Cache`，`RedisICache` -> `RedisCache`。

6) 指针辅助函数命名与语义对齐。
示例：`ParseInt64` -> `Int64Ptr`。

7) 常量命名统一为 CamelCase，并修复拼写。
示例：`Redis`、`MySQL`、`DBNameKubeMinCLI`、`SystemNamespace`，以及 `DeleteTimeout`、`JobNameRegex`、`WorkflowRegex`。

8) 枚举值统一为 snake_case。
示例：`StatusNotRun = "not_run"`。

9) 主入口日志统一使用 `k8s.io/klog/v2`。

10) 模块路径统一为小写。
示例：`module kubemin-cli`。
