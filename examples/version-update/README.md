# 版本更新 API 示例

本目录包含版本更新 API 的各种使用场景示例。

详细 API 文档请参考: [docs/version-update-api.md](../../docs/version-update-api.md)

## 快速开始

```bash
# 替换 {appID} 为实际的应用 ID
APP_ID="your-app-id"

# 简单镜像更新
curl -X POST "http://localhost:8080/api/v1/applications/${APP_ID}/version" \
  -H "Content-Type: application/json" \
  -d @01-simple-image-update.json
```

## 示例文件说明

详见 [EXAMPLES.md](./EXAMPLES.md)

