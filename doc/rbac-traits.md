# RBAC Trait

RBAC Trait 允许组件声明其运行所需的 Kubernetes RBAC 对象。配置后，Kubemin 会在部署组件时自动创建以下资源：

- `ServiceAccount`
- 视权限范围生成 `Role` + `RoleBinding`（命名空间级）或 `ClusterRole` + `ClusterRoleBinding`（集群级）

## 配置示例

在组件配置中加入 `rbac` 块即可：

```json
{
  "rbac": [
    {
      "serviceAccount": "backend-sa",
      "roleName": "backend-reader",
      "bindingName": "backend-reader-binding",
      "rules": [
        {
          "apiGroups": [""],
          "resources": ["pods", "pods/log"],
          "verbs": ["get", "list"]
        }
      ],
      "serviceAccountLabels": {
        "app": "backend"
      },
      "roleLabels": {
        "role": "reader"
      },
      "bindingLabels": {
        "binding": "reader"
      }
    }
  ]
}
```

## 字段说明

| 字段 | 类型 | 说明 |
| ---- | ---- | ---- |
| `serviceAccount` | string（可选） | ServiceAccount 名称；未指定时默认 `<组件名>-sa`。 |
| `namespace` | string（可选） | 覆盖命名空间。未设置时使用组件所在命名空间。 |
| `clusterScope` | bool（可选） | `true` 时创建 ClusterRole/ClusterRoleBinding；默认创建命名空间范围的 Role/RoleBinding。 |
| `roleName` | string（可选） | Role 或 ClusterRole 名称；默认 `<serviceAccount>-role`。 |
| `bindingName` | string（可选） | RoleBinding 或 ClusterRoleBinding 名称；默认 `<serviceAccount>-binding`。 |
| `serviceAccountLabels` | map（可选） | ServiceAccount 附加标签。 |
| `serviceAccountAnnotations` | map（可选） | ServiceAccount 附加注解。 |
| `roleLabels` | map（可选） | Role/ClusterRole 标签。 |
| `bindingLabels` | map（可选） | RoleBinding/ClusterRoleBinding 标签。 |
| `automountServiceAccountToken` | bool（可选） | 控制 ServiceAccount 的 `automountServiceAccountToken`。 |
| `rules` | array（必填） | RBAC 权限规则，映射到 Kubernetes `PolicyRule`。 |

### PolicyRule 格式

```json
{
  "apiGroups": ["apps"],
  "resources": ["deployments"],
  "resourceNames": ["demo-deploy"],
  "nonResourceURLs": ["/metrics"],
  "verbs": ["get", "update"]
}
```

## 集群级权限示例

```json
{
  "rbac": [
    {
      "clusterScope": true,
      "rules": [
        {
          "apiGroups": ["batch"],
          "resources": ["jobs"],
          "verbs": ["create", "delete"]
        }
      ]
    }
  ]
}
```

自动生成的资源包括：

- `ServiceAccount <组件名>-sa`（位于组件命名空间）
- `ClusterRole <组件名>-sa-role`
- `ClusterRoleBinding <组件名>-sa-binding`

以上示例和字段均可按需覆盖，满足不同服务在 Kubernetes 中对 RBAC 的需求。
