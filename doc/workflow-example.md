### 基础Deployment创建

```json
{
  "name": "Km3undHVwXZNCqMZTarC-nginx",
  "alias": "nginx",
  "version": "1.0.0",
  "project": "",
  "description": "Create Nginx",
  "component": [
    {
      "name": "nginx",
      "type": "webservice",
      "replicas": 1,
      "image": "nginx:latest",
      "properties": {"ports": [{"port": 80}],"env": {"MYSQL_DATABASE":"test"}}
    }
  ]
}
```

返回参数

```
{
    "id": "7mcor3r4su789r99jhpxyzat",
    "name": "fnlz2z1lxe85k3me66og-nginx",
    "alias": "nginx",
    "project": "",
    "description": "Create Nginx",
    "createTime": "2025-11-18T15:23:16.149212+08:00",
    "updateTime": "2025-11-18T15:23:16.149213+08:00",
    "icon": "",
    "workflow_id": "45yj4eopg7fl99fz0hzfzdg8"
}
```



#### 执行APP下的某一个工作流

```
curl http://{{127.0.0.1:8080}}/api/v1/applications/{{:APP_ID}}/workflow/exec

curl -X POST \
  http://127.0.0.1:8080/api/v1/applications/7mcor3r4su789r99jhpxyzat/workflow/exec \
  -H 'Content-Type: application/json' \
  -d '{"workflowId":"45yj4eopg7fl99fz0hzfzdg8"}'
```



```shell
> kubectl get deploy
NAME                                    READY   UP-TO-DATE   AVAILABLE   AGE
deploy-nginx-7mcor3r4su789r99jhpxyzat   1/1     1            1           5m31s

>kubectl get svc
svc-nginx-7mcor3r4su789r99jhpxyzat   ClusterIP   10.97.176.81   <none>        80/TCP    5m19s
```

