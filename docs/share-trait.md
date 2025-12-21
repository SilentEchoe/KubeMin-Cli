# Share Trait 策略说明

Share trait 用于在命名空间内复用组件资源。它只包含策略选择，不需要指定资源清单；组件生成的所有资源都会遵循同一策略。

## 策略选项

- `default`：命名空间内若已存在相同 share-name 的资源，则跳过该 Job；否则创建/更新资源。
- `ignore`：无论是否存在资源，直接在工作流中跳过该 Job。
- `force`：不进行共享判断，正常创建/更新资源（与未启用 share 的行为一致）。

未配置 `traits.share` 时，行为保持不变。

## 共享标识

共享资源通过固定 label 识别：

- `kubemin-share-name`：使用组件所属的 `namespace`，转换为 RFC1123 格式并裁剪至 63 字符。
- `kubemin-share-strategy`：策略值（`default`/`ignore`/`force`）。

这些 label 会被写入该组件产生的所有资源（含 trait 追加的 PVC、RBAC、Ingress 等）。

## 实现细节

### shareName 生成规则

1) 优先使用组件所属 `namespace` 作为 shareName。  
2) 对 `namespace` 执行 RFC1123 规范化（小写、非法字符替换为 `-`、截断至 63 字符）。  
3) 若 `namespace` 为空或规范化后为空，则回退到 `component.name` + `component.type` 组合。

> 这样 ClusterRole/ClusterRoleBinding 也能按命名空间共享，避免跨命名空间误跳过。

### 策略执行流程

- `default`
  1) Job 运行前读取资源标签中的 `kubemin-share-name` 与 `kubemin-share-strategy`。
  2) 通过 label selector（`kubemin-share-name=<shareName>`）进行 List 判断是否已存在共享资源。
  3) 若存在，Job 标记为 `skipped` 并返回；否则进入创建/更新流程。
- `ignore`
  - Job 直接标记为 `skipped`，不执行任何 K8s API 调用。
- `force`
  - 不做共享判断，行为与未启用 share 一致。

### 并发保护

`default` 策略的「label list 判断」新增并发保护，避免并发工作流同时 list 为空导致重复创建：

- 优先使用 Redis 分布式锁（`cache.AcquireLock`）。
- 若 Redis 未初始化，则回退到进程内锁。
- 锁 key 格式：`kubemin-share:<resourceKind>:<shareName>`，涵盖 Deployment/Service/ConfigMap/PVC/RBAC 等资源。

### JobInfo 落库

对预先标记为 `skipped` 的 Job（如 share `ignore` 或 share `default` 判定已存在），
也会写入 JobInfo，确保 workflow 的执行记录完整可追溯。

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
kubectl get deploy -n <namespace> -l kubemin-share-name=<namespace>
kubectl get svc -n <namespace> -l kubemin-share-name=<namespace>
kubectl get ingress -n <namespace> -l kubemin-share-name=<namespace>
```

将 `<namespace>` 替换为组件所属命名空间（已转为小写并规范化）。

## 测试用例

- `pkg/apiserver/event/workflow/job/job_run_test.go`
  - 验证 `skipped` 的 Job 仍会写入 JobInfo。
- `pkg/apiserver/event/workflow/job/shared_test.go`
  - 验证 `default` 策略使用 label selector 判断共享资源，并返回可释放锁。
- `pkg/apiserver/event/workflow/share_test.go`
  - 验证 shareName 优先使用 `namespace`，并在空 namespace 时回退为 `component.name + component.type`。
