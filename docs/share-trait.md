# Share Trait 策略说明

Share trait 用于在命名空间内复用组件资源。它只包含策略选择，不需要指定资源清单；组件生成的所有资源都会遵循同一策略。

## 策略选项

- `default`：命名空间内若已存在相同 share-name 的资源，则跳过该 Job；否则创建/更新资源。
- `ignore`：无论是否存在资源，直接在工作流中跳过该 Job。
- `force`：不进行共享判断，正常创建/更新资源（与未启用 share 的行为一致）。

未配置 `traits.share` 时，行为保持不变。

## 共享标识

共享资源通过固定 label 识别：

- `kubemin-share-name`：由 `component.name` 与 `component.type` 组合，转换为 RFC1123 格式并裁剪至 63 字符。
- `kubemin-share-strategy`：策略值（`default`/`ignore`/`force`）。

这些 label 会被写入该组件产生的所有资源（含 trait 追加的 PVC、RBAC、Ingress 等）。

## 配置示例

```yaml
traits:
  share:
    strategy: default
```

```yaml
traits:
  share:
    strategy: ignore
```

JSON 请求示例：

```json
{
  "name": "proxy",
  "componentType": "webservice",
  "namespace": "default",
  "image": "example/proxy:1.0.0",
  "replicas": 1,
  "traits": {
    "share": {
      "strategy": "default"
    }
  }
}
```

## 验证示例

1) 首次执行工作流：资源正常创建。
2) 再次执行：
   - `default`：对应 Job 状态为 `skipped`。
   - `ignore`：Job 直接为 `skipped`。
   - `force`：Job 正常执行。

也可以通过 label 验证：

```bash
kubectl get deploy -n <namespace> -l kubemin-share-name=<component-name>-<component-type>
kubectl get svc -n <namespace> -l kubemin-share-name=<component-name>-<component-type>
kubectl get ingress -n <namespace> -l kubemin-share-name=<component-name>-<component-type>
```

将 `<component-name>` 与 `<component-type>` 替换为组件信息（已转为小写并规范化）。
