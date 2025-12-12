# 版本更新 API 示例文件说明

## 示例文件列表

| 文件 | 说明 |
|------|------|
| `01-simple-image-update.json` | 简单镜像更新 |
| `02-scale-replicas.json` | 扩容副本数 |
| `03-add-component.json` | 新增组件 |
| `04-remove-component.json` | 删除组件 |
| `05-mixed-operations.json` | 混合操作 |
| `06-canary-release.json` | 金丝雀发布 |
| `07-update-env.json` | 更新环境变量 |
| `08-version-bump-only.json` | 仅更新版本号 |

## 使用方法

```bash
# 替换 APP_ID 为实际的应用 ID
APP_ID="your-app-id"

# 使用示例文件发送请求
curl -X POST "http://localhost:8080/api/v1/applications/${APP_ID}/version" \
  -H "Content-Type: application/json" \
  -d @01-simple-image-update.json
```

## 注意事项

- `version` 字段是必填项
- 组件名称会自动转换为小写
- `autoExec` 默认为 `true`，会自动触发工作流执行











