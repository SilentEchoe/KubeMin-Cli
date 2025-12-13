# KubeMin-Cli 工作流测试指南



## 概述

本文档详细描述了KubeMin-Cli工作流系统的完整测试方案，旨在验证工作流的稳定性和可靠性。测试覆盖从基础组件创建到复杂多组件部署的全生命周期管理。

## 测试目标

### 主要目标
- 验证工作流引擎的核心功能
- 测试各种组件类型的创建、更新和删除
- 验证trait系统的正确性和稳定性
- 确保错误处理和回滚机制的可靠性
- 测试并发执行和资源竞争情况
- 验证资源更新和版本管理的正确性

## 测试环境要求

### 基础环境
- Kubernetes集群 (v1.20+)
- MySQL数据库
- Redis集群 (用于分布式队列)
- 足够的测试命名空间

### 测试工具
- `go test` - 单元测试和集成测试
- `kubectl` - Kubernetes资源验证
- `mysql` client - 数据库状态验证
- `redis-cli` - 队列状态检查

## 测试分类

### 1. 基础组件测试 (Basic Component Tests)

#### 1.1 组件创建测试

##### **测试项 TC001: 基础Deployment创建**

```yaml
# 测试用例
应用: test-app-001
组件:
  - 名称: nginx-deployment
  类型: webservice
  镜像: nginx:1.21
  副本数: 3
  端口: 80
```
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

正常返回参数：

```json
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

执行APP下的某一个工作流

```shell
curl http://{{127.0.0.1:8080}}/api/v1/applications/{{:APP_ID}}/workflow/exec

curl -X POST \
  http://127.0.0.1:8080/api/v1/applications/7mcor3r4su789r99jhpxyzat/workflow/exec \
  -H 'Content-Type: application/json' \
  -d '{"workflowId":"45yj4eopg7fl99fz0hzfzdg8"}'
```



**验证点:**

- [x] Deployment成功创建

  ```
  > kubectl get deploy
  NAME                                    READY   UP-TO-DATE   AVAILABLE   AGE
  deploy-nginx-7mcor3r4su789r99jhpxyzat   1/1     1            1           5m31s
  ```

- [x] Pod正常启动

  ```
  > kubectl get po
  NAME                                                     READY   STATUS    RESTARTS   AGE
  deploy-nginx-7mcor3r4su789r99jhpxyzat-675589c686-ptkpg   1/1     Running   0          8m32s
  ```

- [x] Service正确配置

  ```
  > kubectl get svc
  NAME                                 TYPE        CLUSTER-IP     EXTERNAL-IP   PORT(S)   AGE
  svc-nginx-7mcor3r4su789r99jhpxyzat   ClusterIP   10.97.176.81   <none>        80/TCP    8m13s
  
  
  > kubectl get svc svc-nginx-7mcor3r4su789r99jhpxyzat -o yaml
  apiVersion: v1
  kind: Service
  metadata:
    creationTimestamp: "2025-11-18T07:28:33Z"
    labels:
      kube-min-cli: 7mcor3r4su789r99jhpxyzat-nginx
      kube-min-cli-appId: 7mcor3r4su789r99jhpxyzat
      kube-min-cli-componentId: "5"
    name: svc-nginx-7mcor3r4su789r99jhpxyzat
    namespace: default
    resourceVersion: "3877020"
    uid: 47ee7cbe-78a9-47b4-8d7d-4968e418f3bf
  spec:
    clusterIP: 10.97.176.81
    clusterIPs:
    - 10.97.176.81
    internalTrafficPolicy: Cluster
    ipFamilies:
    - IPv4
    ipFamilyPolicy: SingleStack
    ports:
    - name: nginx-80
      port: 80
      protocol: TCP
      targetPort: 80
    selector:
      kube-min-cli-appId: 7mcor3r4su789r99jhpxyzat
    sessionAffinity: None
    type: ClusterIP
  status:
    loadBalancer: {}
  ```

- [x] 数据库记录状态正确

- [x] 工作流状态为completed

  

##### **测试项 TC002: 基础Store组件创建**

在 `store` 组件上验证带持久化卷的 MySQL，可以一次覆盖 StatefulSet、PVC、服务暴露以及数据库探活。下面的负载示例使用 traits.storage 自动创建 20Gi 的持久卷。

```json
{
  "name": "Km3undHVwXZNCqMZTarC-mysql",
  "alias": "mysql",
  "version": "1.0.0",
  "project": "",
  "description": "Create MySQL store",
  "component": [
    {
      "name": "mysql-primary",
      "type": "store",
      "replicas": 1,
      "image": "mysql:8.0.36",
      "properties": {
        "ports": [{"port": 3306}],
        "env": {
          "MYSQL_ROOT_PASSWORD": "RootPwd#123",
          "MYSQL_DATABASE": "demo",
          "MYSQL_USER": "demo",
          "MYSQL_PASSWORD": "demoPwd#123"
        }
      },
      "traits": {
        "storage": [
          {
            "name": "mysql-data",
            "type": "persistent",
            "mountPath": "/var/lib/mysql",
            "subPath": "mysql",
            "size": "1Gi",
            "storageClass": "standard",
            "create": true
          }
        ]
      }
    }
  ],
  "workflow": [
    {
      "name": "mysql-store",
      "mode": "StepByStep",
      "components": [
        "mysql-primary"
      ]
    }
  ]
}
```

**执行步骤**

1. 保存上述 JSON 为 `payloads/mysql-store.json` 并提交创建请求：
   ```shell
   curl -X POST http://127.0.0.1:8080/api/v1/applications \
     -H 'Content-Type: application/json' \
     -d @payloads/mysql-store.json
   ```
   记录响应中的 `appId` 与 `workflow_id`。
   
   ```json
   {
       "id": "4tbupjg43ln3yj249l0v0fv8",
       "name": "fnlz2z1lxe85k3me66og-mysql",
       "alias": "mysql",
       "project": "",
       "description": "Create MySQL store",
       "createTime": "2025-11-20T13:38:57.959305+08:00",
       "updateTime": "2025-11-20T13:38:57.962363+08:00",
       "icon": "",
       "workflow_id": "ftjlu1amurnn8yltipwv5fj1"
   }
   ```
2. 触发 store 工作流：
   ```shell
   curl -X POST \
     http://127.0.0.1:8080/api/v1/applications/${APP_ID}/workflow/exec \
     -H 'Content-Type: application/json' \
     -d "{\"workflowId\":\"${WORKFLOW_ID}\"}"
   ```
   保存返回的 `taskId`。
3. 轮询任务状态直到结束：
   ```shell
   curl http://127.0.0.1:8080/api/v1/workflow/tasks/${TASK_ID}/status
   ```

**验证点:**

- [x] StatefulSet成功创建
  ```shell
  kubectl get sts 
  NAME                                           READY   AGE
  store-mysql-primary-4tbupjg43ln3yj249l0v0fv8   1/1     107s
  ```
  
- [x] PVC自动创建并绑定
  ```shell
  kubectl get pvc 
  NAME                                     STATUS   CAPACITY   ACCESS MODES   STORAGECLASS   AGE
  data   Bound    pvc-d8b52bfd-efed-402b-aaaf-4cf6a88326b2   5Gi        RWO            hostpath       <unset>                 2m6s
  ```
  
- [x] Pod正常启动
  ```shell
  kubectl get pod -o wide
  NAME                                             READY   STATUS    RESTARTS   AGE     IP        
  store-mysql-primary-4tbupjg43ln3yj249l0v0fv8-0   1/1     Running   0          2m26s   10.1.2.213   
  ```
  
- [x] 存储卷正确挂载
  ```shell
  kubectl describe pod store-mysql-primary-7mcor3r4su789r99jhpxyzat-0 | grep -A3 'Mounts'   
     Mounts:
        /var/lib/mysql from data (rw,path="mysql")
        /var/run/secrets/kubernetes.io/serviceaccount from kube-api-access-gltjp (ro)
  ```
  
- [x] MySQL读写校验
  ```shell
  kubectl run mysql-client --rm -it \
      --image=mysql:8.0 --restart=Never -- \
      mysql -h svc-mysql-primary-4tbupjg43ln3yj249l0v0fv8 -uroot -pRootPwd#123 \
      -e 'CREATE DATABASE IF NOT EXISTS appdb;
          USE appdb;
          CREATE TABLE IF NOT EXISTS healthcheck(id INT);
          INSERT INTO healthcheck VALUES (1);
          SELECT COUNT(*) FROM healthcheck;'
  mysql: [Warning] Using a password on the command line interface can be insecure.
  +----------+
  | COUNT(*) |
  +----------+
  |        1 |
  +----------+
  ```

#动态创建的PVC不会被删除



##### **测试项 TC003: 配置组件创建**

```yaml
{
    "name": "fnlz2z1lxe85k3me66og-config",
    "alias": "app-config",
    "version": "1.0.0",
    "project": "",
    "description": "TC003 config component creation",
    "component": [
      {
        "name": "app-config",
        "type": "config",
        "replicas": 1,
        "properties": {
          "conf": {
            "database.conf": "host=localhost\nport=3306\n"
          },
          "labels": {
            "kubemin.cli/test-case": "TC003"
          }
        }
      }
    ],
    "workflow": [
      {
        "name": "config-step",
        "mode": "StepByStep",
        "components": [
          "app-config"
        ]
      }
    ]
  }
```
**执行步骤**

1. 保存上述 JSON 为 `payloads/config.json` 并提交创建请求：

   ```shell
   curl -X POST http://127.0.0.1:8080/api/v1/applications \
     -H 'Content-Type: application/json' \
     -d @payloads/config.json
   ```

   记录响应中的 `appId` 与 `workflow_id`。

   ```json
   {
       "id": "4kwaenmqb055rt6pyyp8ouh7",
       "name": "fnlz2z1lxe85k3me66og-config",
       "alias": "app-config",
       "project": "",
       "description": "TC003 config component creation",
       "createTime": "2025-11-20T14:16:59.9162+08:00",
       "updateTime": "2025-11-20T14:16:59.9162+08:00",
       "icon": "",
       "workflow_id": "kjk2efteq93lwims12e897gf"
   }
   ```

2. 触发 store 工作流：

   ```shell
   curl -X POST \
     http://127.0.0.1:8080/api/v1/applications/${APP_ID}/workflow/exec \
     -H 'Content-Type: application/json' \
     -d "{\"workflowId\":\"${WORKFLOW_ID}\"}"
   ```

   保存返回的 `taskId`。

**验证点:**

- [x] ConfigMap成功创建

  ```
  kubectl get cm
  
  NAME               DATA   AGE
  app-config         1      33s
  ```

- [x] 数据内容正确

  ```shell
  > kubectl describe cm app-config
  Name:         app-config
  Namespace:    default
  Labels:       kube-min-cli=4kwaenmqb055rt6pyyp8ouh7-app-config
                kube-min-cli-appId=4kwaenmqb055rt6pyyp8ouh7
                kube-min-cli-componentId=8
                kubemin.cli/test-case=TC003
  Annotations:  <none>
  
  Data
  ====
  database.conf:
  ----
  host=localhost
  port=3306
  
  
  
  BinaryData
  ====
  
  Events:  <none>
  ```

- [x] 可以被其他组件引用

- [x] 工作流状态为completed

  

##### **测试项 TC004: 密钥组件创建**

```yaml
# 测试用例
应用: test-app-004
组件:
  - 名称: app-secret
  类型: secret
  数据:
    username: YWRtaW4=
    password: MWYyZDFlMmU2N2Rm
```

```json
{
  "name": "test-app-004-secret",
  "alias": "app-secret",
  "namespace": "default",
  "version": "1.0.0",
  "description": "TC004 secret component creation",
  "component": [
    {
      "name": "app-secret",
      "type": "secret",
      "replicas": 1,
      "properties": {
        "secret": {
          "username": "admin",
          "password": "1f2d1e2e67df"
        },
        "labels": {
          "kubemin.cli/test-case": "TC004"
        }
      }
    }
  ],
  "workflow": [
    {
      "name": "secret-step",
      "mode": "StepByStep",
      "components": ["app-secret"]
    }
  ]
}
```

**执行步骤**

1. 保存上述 JSON 为 `payloads/secret.json` 并提交创建请求：
   ```shell
   curl -X POST http://127.0.0.1:8080/api/v1/applications \
     -H 'Content-Type: application/json' \
     -d @payloads/secret.json
   ```
   记录响应中的 `appId` 与 `workflow_id`。

2. 触发工作流：
   ```shell
   curl -X POST \
     http://127.0.0.1:8080/api/v1/applications/${APP_ID}/workflow/exec \
     -H 'Content-Type: application/json' \
     -d "{\"workflowId\":\"${WORKFLOW_ID}\"}"
   ```

**验证点:**
- [x] Secret成功创建
  ```shell
  kubectl get secret app-secret
  
  NAME         TYPE     DATA   AGE
  app-secret   Opaque   2      16s
  ```
- [x] 数据正确编码
  ```shell
  kubectl get secret app-secret -o jsonpath='{.data}'
  
  {"password":"MWYyZDFlMmU2N2Rm","username":"YWRtaW4="}%
  ```
- [x] 可以被其他组件引用
- [x] 工作流状态为completed

#### 1.2 组件更新测试

##### **测试项 TC005: Deployment镜像更新**

```yaml
# 初始状态
组件: nginx-deployment
镜像: nginx:1.21

# 更新操作
新镜像: nginx:1.22
```

**前置条件:** 先创建一个基础的 nginx 应用

```json
{
  "name": "test-app-005-nginx",
  "namespace": "default",
  "version": "1.0.0",
  "description": "TC005 nginx deployment for image update test",
  "component": [
    {
      "name": "nginx-deployment",
      "type": "webservice",
      "image": "nginx:1.21",
      "replicas": 2,
      "properties": {
        "ports": [{"port": 80, "expose": true}]
      }
    }
  ],
  "workflow": [
    {
      "name": "deploy-nginx",
      "mode": "StepByStep",
      "components": ["nginx-deployment"]
    }
  ]
}
```

**更新请求:** `POST /api/v1/applications/${APP_ID}/version`

```json
{
  "version": "1.1.0",
  "strategy": "rolling",
  "components": [
    {
      "name": "nginx-deployment",
      "image": "nginx:1.22"
    }
  ],
  "description": "Update nginx image from 1.21 to 1.22"
}
```

**验证点:**
- [x] 滚动更新成功执行
  ```shell
  kubectl rollout status deploy/deploy-nginx-deployment-${APP_ID}
  
  
  > deployment "deploy-nginx-deployment-r9vhgmwpd7c9hflam0rbq6wj" successfully rolled out
  ```
- [x] 新版本Pod正常启动
  ```shell
  kubectl get pods -l kube-min-cli-appId=${APP_ID} -o wide
  ```
- [x] 旧版本Pod平滑下线
- [x] 服务不中断
- [x] 更新状态为completed
- [x] ```yaml
  spec:
        containers:
        - image: nginx:latest #版本已更新
          imagePullPolicy: IfNotPresent
          name: nginx-deployment
  ```

  

##### **测试项 TC006: 环境变量更新**

```yaml
# 初始状态
组件: app-deployment
环境变量:
  - ENV: production

# 更新操作
新环境变量:
  - ENV: staging
  - DEBUG: "true"
```

**前置条件:** 先创建一个带环境变量的应用

```json
{
  "name": "test-app-006-envtest",
  "namespace": "default",
  "version": "1.0.0",
  "description": "TC006 app deployment for env update test",
  "component": [
    {
      "name": "app-deployment",
      "type": "webservice",
      "image": "nginx:latest",
      "replicas": 1,
      "properties": {
        "ports": [{"port": 80}],
        "env": {
          "ENV": "production"
        }
      }
    }
  ],
  "workflow": [
    {
      "name": "deploy-app",
      "mode": "StepByStep",
      "components": ["app-deployment"]
    }
  ]
}
```

**更新请求:** `POST /api/v1/applications/${APP_ID}/version`

```json
curl -X POST "http://localhost:8080/api/v1/applications/kyysru4h07l1w7ghqd6mkhcx/version" \
  -H "Content-Type: application/json" \
  -d '{
    "version": "1.2.0",
    "strategy": "rolling",
    "components": [
      {
        "name": "app-deployment",
        "env": {
          "ENV": "staging",
          "DEBUG": "true"
        }
      }
    ],
    "description": "Update environment variables"
  }'
```

**验证点:**
- [x] 环境变量正确更新
  ```shell
  kubectl get deploy deploy-app-deployment-${APP_ID} -o jsonpath='{.spec.template.spec.containers[0].env}'
  #更新前
  kubectl get deploy deploy-app-deployment-kyysru4h07l1w7ghqd6mkhcx -o jsonpath='{.spec.template.spec.containers[0].env}'
  > [{"name":"ENV","value":"production"}]%
  #更新后
  [{"name":"DEBUG","value":"true"},{"name":"ENV","value":"staging"}]%
  ```
- [x] Pod重新创建
- [x] 新配置生效
- [x] 更新状态为completed

### 2. Trait系统测试 (Trait System Tests)

#### 2.1 存储Trait测试

##### **测试项 TC007: PVC挂载测试**

```yaml
# 测试用例
组件: app-deployment
traits:
  - 类型: storage
    属性:
      pvc:
        - 名称: data-volume
        大小: 2Gi
        路径: /data
```

```json
{
  "name": "test-app-007-pvc",
  "namespace": "default",
  "version": "1.0.0",
  "description": "TC007 PVC mount test",
  "component": [
    {
      "name": "app-with-pvc",
      "type": "webservice",
      "image": "nginx:latest",
      "replicas": 1,
      "properties": {
        "ports": [{"port": 80}]
      },
      "traits": {
        "storage": [
          {
            "name": "data-volume",
            "type": "persistent",
            "mountPath": "/data",
            "tmpCreate": true,
            "size": "2Gi",
            "storageClass": "standard"
          }
        ]
      }
    }
  ],
  "workflow": [
    {
      "name": "deploy-with-pvc",
      "mode": "StepByStep",
      "components": ["app-with-pvc"]
    }
  ]
}
```

**验证点:**
- [x] PVC自动创建
  ```shell
  kubectl get pvc -l kube-min-cli-appId=${APP_ID}
  
  NAME                                                                                     STATUS   VOLUME                                     CAPACITY   ACCESS MODES   STORAGECLASS   VOLUMEATTRIBUTESCLASS   AGE
  pvc-data-volume-6z7a4tk6iwfgkf4bdk4tqfn8-store-app-with-pvc-6z7a4tk6iwfgkf4bdk4tqfn8-0   Bound    pvc-98d5af7c-d676-4dd1-8c5e-6b22e6aae0c8   2Gi        RWO            hostpath       <unset>                 16s
  ```
- [x] Volume正确挂载
  ```shell
  kubectl describe pod -l kube-min-cli-appId=${APP_ID} | grep -A5 'Volumes'
  
  Volumes:
    data-volume:
      Type:       PersistentVolumeClaim (a reference to a PersistentVolumeClaim in the same namespace)
      ClaimName:  data-volume-store-app-with-pvc-55s2ifaoxejsdcm1qp9hrlv8-0
      ReadOnly:   false
  ```
- [x] 挂载路径正确
  ```shell
  kubectl exec -it ${POD_NAME} -- ls -la /data
  ```
- [x] 权限设置正确
- [x] 数据持久化验证

##### **测试项 TC008: ConfigMap挂载测试**

```yaml
# 测试用例
组件: app-deployment
traits:
  - 类型: storage
    属性:
      configMap:
        - 名称: app-config
        路径: /etc/config
```

```json
{
  "name": "test-app-008-configmap-mount",
  "namespace": "default",
  "version": "1.0.0",
  "description": "TC008 ConfigMap mount test",
  "component": [
    {
      "name": "app-config",
      "type": "config",
      "replicas": 1,
      "properties": {
        "conf": {
          "app.conf": "server.host=localhost\nserver.port=8080\nlog.level=info"
        }
      }
    },
    {
      "name": "app-with-configmap",
      "type": "webservice",
      "image": "nginx:latest",
      "replicas": 1,
      "properties": {
        "ports": [{"port": 80}]
      },
      "traits": {
        "storage": [
          {
            "name": "config-volume",
            "type": "config",
            "sourceName": "app-config",
            "mountPath": "/etc/config",
            "readOnly": true
          }
        ]
      }
    }
  ],
  "workflow": [
    {
      "name": "config-step",
      "mode": "StepByStep",
      "components": ["app-config"]
    },
    {
      "name": "app-step",
      "mode": "StepByStep",
      "components": ["app-with-configmap"]
    }
  ]
}
```

**验证点:**
- [x] ConfigMap正确挂载
  ```shell
  kubectl describe pod -l kube-min-cli-appId=${APP_ID} | grep -A5 'Mounts'
  
  > kubectl describe pod deploy-app-with-configmap-trydp6meg7s5pvfrqvlonq3o-55cd9fdfg5mb | grep -A5 'Mounts'
   Mounts:
        /etc/config from config-volume (ro)
        /var/run/secrets/kubernetes.io/serviceaccount from kube-api-access-spls8 (ro)
  Conditions:
    Type                        Status
    PodReadyToStartContainers   True
  ```
- [x] 文件路径正确
  ```shell
  kubectl exec -it ${POD_NAME} -- ls -la /etc/config
  
  > kubectl exec -it deploy-app-with-configmap-trydp6meg7s5pvfrqvlonq3o-55cd9fdfg5mb -- ls -la /etc/config
  total 16
  drwxrwxrwx 3 root root 4096 Dec 13 06:41 .
  drwxr-xr-x 1 root root 4096 Dec 13 06:41 ..
  drwxr-xr-x 2 root root 4096 Dec 13 06:41 ..2025_12_13_06_41_00.359621063
  lrwxrwxrwx 1 root root   31 Dec 13 06:41 ..data -> ..2025_12_13_06_41_00.359621063
  lrwxrwxrwx 1 root root   15 Dec 13 06:41 app.conf -> ..data/app.conf
  ```
- [x] 文件权限正确
- [x] 内容只读属性
  ```shell
  kubectl exec -it ${POD_NAME} -- cat /etc/config/app.conf
  
  > kubectl exec -it deploy-app-with-configmap-trydp6meg7s5pvfrqvlonq3o-55cd9fdfg5mb -- cat /etc/config/app.conf
  server.host=localhost
  server.port=8080
  log.level=info%
  ```
- [x] 热更新支持

#### **测试项 TC009: Secret挂载测试**

```yaml
# 测试用例
组件: app-deployment
traits:
  - 类型: storage
    属性:
      secret:
        - 名称: app-secret
        路径: /etc/secrets
```

```json
{
  "name": "test-app-009-secret-mount",
  "namespace": "default",
  "version": "1.0.0",
  "description": "TC009 Secret mount test",
  "component": [
    {
      "name": "app-secret",
      "type": "secret",
      "replicas": 1,
      "properties": {
        "secret": {
          "db-password": "supersecretpassword",
          "api-key": "my-api-key-12345"
        }
      }
    },
    {
      "name": "app-with-secret",
      "type": "webservice",
      "image": "nginx:latest",
      "replicas": 1,
      "properties": {
        "ports": [{"port": 80}]
      },
      "traits": {
        "storage": [
          {
            "name": "secret-volume",
            "type": "secret",
            "sourceName": "app-secret",
            "mountPath": "/etc/secrets",
            "readOnly": true
          }
        ]
      }
    }
  ],
  "workflow": [
    {
      "name": "secret-step",
      "mode": "StepByStep",
      "components": ["app-secret"]
    },
    {
      "name": "app-step",
      "mode": "StepByStep",
      "components": ["app-with-secret"]
    }
  ]
}
```

**验证点:**
- [x] Secret正确挂载
  ```shell
  kubectl describe pod -l kube-min-cli-appId=${APP_ID} | grep -A5 'Mounts'
  
  
  > kubectl describe pod deploy-app-with-secret-dev0y3vsmxwu7h966sdrblr8-547cf74bb44rnnb | grep -A5 'Mounts'
  
      Mounts:
        /etc/secrets from secret-volume (ro)
        /var/run/secrets/kubernetes.io/serviceaccount from kube-api-access-zx5s6 (ro)
  Conditions:
    Type                        Status
    PodReadyToStartContainers   True
  ```
- [x] 文件路径正确
  ```shell
  kubectl exec -it ${POD_NAME} -- ls -la /etc/secrets
  
  > kubectl exec -it deploy-app-with-secret-dev0y3vsmxwu7h966sdrblr8-547cf74bb44rnnb -- ls -la /etc/secrets
  total 8
  drwxrwxrwt 3 root root  120 Dec 13 06:47 .
  drwxr-xr-x 1 root root 4096 Dec 13 06:47 ..
  drwxr-xr-x 2 root root   80 Dec 13 06:47 ..2025_12_13_06_47_12.3610060343
  lrwxrwxrwx 1 root root   32 Dec 13 06:47 ..data -> ..2025_12_13_06_47_12.3610060343
  lrwxrwxrwx 1 root root   14 Dec 13 06:47 api-key -> ..data/api-key
  lrwxrwxrwx 1 root root   18 Dec 13 06:47 db-password -> ..data/db-password
  ```
- [x] 文件权限正确(600)
- [x] 内容自动解码
  ```shell
  kubectl exec -it ${POD_NAME} -- cat /etc/secrets/db-password
  
  > kubectl exec -it deploy-app-with-secret-dev0y3vsmxwu7h966sdrblr8-547cf74bb44rnnb -- cat /etc/secrets/db-password
  supersecretpassword%
  ```
- [x] 安全性验证

#### 2.2 网络Trait测试

##### **测试项 TC010: Ingress配置测试**

```yaml
# 测试用例
组件: app-deployment
traits:
  - 类型: ingress
    属性:
      规则:
        - 主机: app.example.com
        路径: /
        端口: 8080
      tls:
        - 主机: app.example.com
        secret名称: app-tls
```

```json
{
  "name": "test-app-010-ingress",
  "namespace": "default",
  "version": "1.0.0",
  "description": "TC010 Ingress configuration test",
  "component": [
    {
      "name": "app-with-ingress",
      "type": "webservice",
      "image": "nginx:latest",
      "replicas": 2,
      "properties": {
        "ports": [{"port": 8080, "expose": true}]
      },
      "traits": {
        "ingress": [
          {
            "name": "app-ingress",
            "namespace": "default",
            "ingressClassName": "nginx",
            "hosts": ["app.example.com"],
            "routes": [
              {
                "path": "/",
                "pathType": "Prefix",
                "host": "app.example.com",
                "backend": {
                  "serviceName": "app-with-ingress",
                  "servicePort": 8080
                }
              },
              {
                "path": "/api",
                "pathType": "Prefix",
                "host": "app.example.com",
                "backend": {
                  "serviceName": "app-with-ingress",
                  "servicePort": 8080
                }
              }
            ],
            "tls": [
              {
                "secretName": "app-tls",
                "hosts": ["app.example.com"]
              }
            ]
          }
        ]
      }
    }
  ],
  "workflow": [
    {
      "name": "deploy-with-ingress",
      "mode": "StepByStep",
      "components": ["app-with-ingress"]
    }
  ]
}
```

**验证点:**
- [x] Ingress成功创建
  ```shell
  kubectl get ingress -l kube-min-cli-appId=${APP_ID}
  
  NAME                                            CLASS    HOSTS              ADDRESS   PORTS     AGE
  ing-app-ingress-xl3re326go3qhbb9hqc24k62        nginx    app.example.com              80, 443   10s
  ```
- [x] 路由规则正确
  ```shell
  > kubectl get ingress ing-app-ingress-xl3re326go3qhbb9hqc24k62 -o yaml
  
  apiVersion: networking.k8s.io/v1
  kind: Ingress
  metadata:
    creationTimestamp: "2025-12-13T07:01:38Z"
    generation: 1
    name: ing-app-ingress-xl3re326go3qhbb9hqc24k62
    namespace: default
    resourceVersion: "5286300"
    uid: 556e3c7a-4b6b-4068-9e70-92cb657cb47b
  spec:
    ingressClassName: nginx
    rules:
    - host: app.example.com
      http:
        paths:
        - backend:
            service:
              name: app-with-ingress
              port:
                number: 8080
          path: /
          pathType: Prefix
        - backend:
            service:
              name: app-with-ingress
              port:
                number: 8080
          path: /api
          pathType: Prefix
    tls:
    - hosts:
      - app.example.com
      secretName: app-tls
  status:
    loadBalancer: {}
  ```
- [x] TLS配置正确
  ```shell
  kubectl get ingress app-ingress -o jsonpath='{.spec.tls}'
  ```
- [x] 外部访问正常
- [x] 证书验证通过

#### 2.3 RBAC Trait测试

##### **测试项 TC011: ServiceAccount配置测试**

```yaml
# 测试用例
组件: app-deployment
traits:
  - 类型: rbac
    属性:
      serviceAccount: app-sa
      roles:
        - 名称: app-role
        规则:
          - apiGroups: [""]
          resources: ["pods"]
          verbs: ["get", "list"]
```

```json
{
  "name": "test-app-011-rbac",
  "namespace": "default",
  "version": "1.0.0",
  "description": "TC011 RBAC ServiceAccount configuration test",
  "component": [
    {
      "name": "app-with-rbac",
      "type": "webservice",
      "image": "nginx:latest",
      "replicas": 1,
      "properties": {
        "ports": [{"port": 80}]
      },
      "traits": {
        "rbac": [
          {
            "serviceAccount": "app-sa",
            "namespace": "default",
            "roleName": "app-role",
            "bindingName": "app-role-binding",
            "rules": [
              {
                "apiGroups": [""],
                "resources": ["pods", "services"],
                "verbs": ["get", "list", "watch"]
              },
              {
                "apiGroups": [""],
                "resources": ["configmaps"],
                "verbs": ["get"]
              }
            ]
          }
        ]
      }
    }
  ],
  "workflow": [
    {
      "name": "deploy-with-rbac",
      "mode": "StepByStep",
      "components": ["app-with-rbac"]
    }
  ]
}
```

**验证点:**
- [x] ServiceAccount创建
  ```shell
  > kubectl get sa app-sa
  
  NAME             SECRETS   AGE
  app-sa           0         22s
  ```
- [x] Role/ClusterRole创建
  ```shell
  > kubectl get role app-role
  
  NAME       CREATED AT
  app-role   2025-12-13T07:24:23Z
  ```
- [x] RoleBinding创建
  ```shell
  > kubectl get rolebinding app-role-binding
  
  app-role-binding   Role/app-role   43s
  ```
- [x] 权限正确绑定
  ```shell
  kubectl auth can-i get pods --as=system:serviceaccount:default:app-sa
  > yes
  ```
- [x] Pod使用正确的SA
  ```shell
  kubectl get pod -l kube-min-cli-appId=${APP_ID} -o jsonpath='{.items[0].spec.serviceAccountName}'
  
  
  kubectl get pod deploy-app-with-rbac-kjjbr11rax50fo006aa52t9m-5b8d5cd549-fcmbw -o jsonpath='{.items[0].spec.serviceAccountName}'
  
  
  
  serviceAccount: app-sa
  serviceAccountName: app-sa
  ```

### 3. 多组件工作流测试 (Multi-Component Workflow Tests)

#### 3.1 依赖关系测试

##### **测试项 TC012: 顺序依赖测试**

```yaml
# 测试用例
应用: dependency-app
工作流:
  - 步骤1: 创建配置(config)
  - 步骤2: 创建密钥(secret)
  - 步骤3: 创建数据库(store)
  - 步骤4: 创建应用(webservice)
依赖关系:
  - 应用依赖于配置、密钥、数据库
```

```json
{
  "name": "test-app-012-dependency",
  "namespace": "default",
  "version": "1.0.0",
  "description": "TC012 Sequential dependency test",
  "component": [
    {
      "name": "app-config",
      "type": "config",
      "replicas": 1,
      "properties": {
        "conf": {
          "database.host": "mysql-db",
          "database.port": "3306",
          "app.name": "dependency-app"
        }
      }
    },
    {
      "name": "app-secret",
      "type": "secret",
      "replicas": 1,
      "properties": {
        "secret": {
          "db-password": "secretpassword123",
          "api-key": "my-api-key"
        }
      }
    },
    {
      "name": "mysql-db",
      "type": "store",
      "image": "mysql:8.0",
      "replicas": 1,
      "properties": {
        "ports": [{"port": 3306, "expose": true}],
        "env": {
          "MYSQL_ROOT_PASSWORD": "rootpassword",
          "MYSQL_DATABASE": "appdb"
        }
      },
      "traits": {
        "storage": [
          {
            "name": "mysql-data",
            "type": "persistent",
            "mountPath": "/var/lib/mysql",
            "tmpCreate": true,
            "size": "5Gi",
            "storageClass": "standard"
          }
        ]
      }
    },
    {
      "name": "backend-app",
      "type": "webservice",
      "image": "myregistry/backend:v1.0.0",
      "replicas": 2,
      "properties": {
        "ports": [{"port": 8080, "expose": true}],
        "env": {
          "DB_HOST": "mysql-db",
          "DB_PORT": "3306"
        }
      },
      "traits": {
        "envFrom": [
          {"type": "configMap", "sourceName": "app-config"},
          {"type": "secret", "sourceName": "app-secret"}
        ]
      }
    }
  ],
  "workflow": [
    {
      "name": "step1-config",
      "mode": "StepByStep",
      "components": ["app-config"]
    },
    {
      "name": "step2-secret",
      "mode": "StepByStep",
      "components": ["app-secret"]
    },
    {
      "name": "step3-database",
      "mode": "StepByStep",
      "components": ["mysql-db"]
    },
    {
      "name": "step4-app",
      "mode": "StepByStep",
      "components": ["backend-app"]
    }
  ]
}
```

**验证点:**
- [x] 按依赖顺序执行
  ```shell
  # 查看工作流执行日志，确认步骤顺序
  curl http://127.0.0.1:8080/api/v1/workflow/tasks/${TASK_ID}/status
  
  {
      "taskId": "ojo3lk8k5y1dukall1t8npn0",
      "status": "completed",
      "workflowId": "f6rx5ywixeat3tfnxqjtdgqp",
      "workflowName": "test-app-012-dependency-ea7ff7qjd4ipclsb",
      "appId": "ngfjmll19vd8562z4g70mk50",
      "type": "workflow",
      "components": [
          {
              "name": "app-config",
              "type": "configmap_deploy",
              "status": "completed",
              "startTime": 1765612402,
              "endTime": 1765612402
          },
          {
              "name": "app-secret",
              "type": "secret_deploy",
              "status": "completed",
              "startTime": 1765612402,
              "endTime": 1765612402
          },
          {
              "name": "mysql-db",
              "type": "store_deploy",
              "status": "completed",
              "startTime": 1765612402,
              "endTime": 1765612407
          },
          {
              "name": "backend-app",
              "type": "service_deploy",
              "status": "completed",
              "startTime": 1765612407,
              "endTime": 1765612409
          }
      ]
  }
  ```
- [x] 等待依赖就绪
- [x] 错误时正确回滚
- [x] 状态正确传播
- [x] 并发控制有效

**测试项 TC014: 并行组件测试**
```yaml
# 测试用例
应用: parallel-app
工作流:
  - 并行组1:
    - 组件A (webservice)
    - 组件B (webservice)
  - 并行组2:
    - 组件C (store)
    - 组件D (store)
```

```json
{
  "name": "test-app-014-parallel",
  "namespace": "default",
  "version": "1.0.0",
  "description": "TC014 Parallel component test",
  "component": [
    {
      "name": "service-a",
      "type": "webservice",
      "image": "nginx:latest",
      "replicas": 1,
      "properties": {
        "ports": [{"port": 80}],
        "env": {"SERVICE_NAME": "service-a"}
      }
    },
    {
      "name": "service-b",
      "type": "webservice",
      "image": "nginx:latest",
      "replicas": 1,
      "properties": {
        "ports": [{"port": 80}],
        "env": {"SERVICE_NAME": "service-b"}
      }
    },
    {
      "name": "redis-c",
      "type": "store",
      "image": "redis:7-alpine",
      "replicas": 1,
      "properties": {
        "ports": [{"port": 6379, "expose": true}]
      }
    },
    {
      "name": "redis-d",
      "type": "store",
      "image": "redis:7-alpine",
      "replicas": 1,
      "properties": {
        "ports": [{"port": 6379, "expose": true}]
      }
    }
  ],
  "workflow": [
    {
      "name": "parallel-group-1",
      "mode": "DAG",
      "components": ["service-a", "service-b"]
    },
    {
      "name": "parallel-group-2",
      "mode": "DAG",
      "components": ["redis-c", "redis-d"]
    }
  ]
}
```

**验证点:**
- [ ] 并行执行正确
  ```shell
  kubectl get pods -l kube-min-cli-appId=${APP_ID} --watch
  ```
- [ ] 资源竞争处理
- [ ] 状态聚合正确
- [ ] 错误隔离有效
- [ ] 性能符合预期

#### 3.2 复杂场景测试

**测试项 TC015: 微服务应用测试**
```yaml
# 测试用例
应用: microservices-app
组件:
  - 前端 (webservice + ingress)
  - API网关 (webservice + service)
  - 用户服务 (webservice + store)
  - 订单服务 (webservice + store)
  - 配置中心 (config)
  - 消息队列 (store)
  - 监控组件 (webservice)
```

```json
{
  "name": "test-app-015-microservices",
  "namespace": "default",
  "version": "1.0.0",
  "description": "TC015 Microservices application test",
  "component": [
    {
      "name": "app-config",
      "type": "config",
      "replicas": 1,
      "properties": {
        "conf": {
          "gateway.url": "http://api-gateway:8080",
          "redis.host": "message-queue",
          "redis.port": "6379"
        }
      }
    },
    {
      "name": "message-queue",
      "type": "store",
      "image": "redis:7-alpine",
      "replicas": 1,
      "properties": {
        "ports": [{"port": 6379, "expose": true}]
      }
    },
    {
      "name": "user-db",
      "type": "store",
      "image": "mysql:8.0",
      "replicas": 1,
      "properties": {
        "ports": [{"port": 3306, "expose": true}],
        "env": {
          "MYSQL_ROOT_PASSWORD": "rootpwd",
          "MYSQL_DATABASE": "userdb"
        }
      },
      "traits": {
        "storage": [
          {
            "name": "user-db-data",
            "type": "persistent",
            "mountPath": "/var/lib/mysql",
            "tmpCreate": true,
            "size": "5Gi",
            "storageClass": "standard"
          }
        ]
      }
    },
    {
      "name": "order-db",
      "type": "store",
      "image": "mysql:8.0",
      "replicas": 1,
      "properties": {
        "ports": [{"port": 3306, "expose": true}],
        "env": {
          "MYSQL_ROOT_PASSWORD": "rootpwd",
          "MYSQL_DATABASE": "orderdb"
        }
      },
      "traits": {
        "storage": [
          {
            "name": "order-db-data",
            "type": "persistent",
            "mountPath": "/var/lib/mysql",
            "tmpCreate": true,
            "size": "5Gi",
            "storageClass": "standard"
          }
        ]
      }
    },
    {
      "name": "user-service",
      "type": "webservice",
      "image": "myregistry/user-service:v1.0.0",
      "replicas": 2,
      "properties": {
        "ports": [{"port": 8080, "expose": true}],
        "env": {
          "DB_HOST": "user-db",
          "REDIS_HOST": "message-queue"
        }
      }
    },
    {
      "name": "order-service",
      "type": "webservice",
      "image": "myregistry/order-service:v1.0.0",
      "replicas": 2,
      "properties": {
        "ports": [{"port": 8080, "expose": true}],
        "env": {
          "DB_HOST": "order-db",
          "REDIS_HOST": "message-queue"
        }
      }
    },
    {
      "name": "api-gateway",
      "type": "webservice",
      "image": "myregistry/api-gateway:v1.0.0",
      "replicas": 2,
      "properties": {
        "ports": [{"port": 8080, "expose": true}],
        "env": {
          "USER_SERVICE_URL": "http://user-service:8080",
          "ORDER_SERVICE_URL": "http://order-service:8080"
        }
      }
    },
    {
      "name": "frontend",
      "type": "webservice",
      "image": "myregistry/frontend:v1.0.0",
      "replicas": 2,
      "properties": {
        "ports": [{"port": 80, "expose": true}],
        "env": {
          "API_GATEWAY_URL": "http://api-gateway:8080"
        }
      },
      "traits": {
        "ingress": [
          {
            "name": "frontend-ingress",
            "namespace": "default",
            "ingressClassName": "nginx",
            "hosts": ["app.example.com"],
            "routes": [
              {
                "path": "/",
                "pathType": "Prefix",
                "host": "app.example.com",
                "backend": {
                  "serviceName": "frontend",
                  "servicePort": 80
                }
              }
            ]
          }
        ]
      }
    },
    {
      "name": "monitoring",
      "type": "webservice",
      "image": "prom/prometheus:latest",
      "replicas": 1,
      "properties": {
        "ports": [{"port": 9090, "expose": true}]
      }
    }
  ],
  "workflow": [
    {
      "name": "step1-infrastructure",
      "mode": "DAG",
      "components": ["app-config", "message-queue"]
    },
    {
      "name": "step2-databases",
      "mode": "DAG",
      "components": ["user-db", "order-db"]
    },
    {
      "name": "step3-services",
      "mode": "DAG",
      "components": ["user-service", "order-service"]
    },
    {
      "name": "step4-gateway-frontend",
      "mode": "DAG",
      "components": ["api-gateway", "frontend"]
    },
    {
      "name": "step5-monitoring",
      "mode": "StepByStep",
      "components": ["monitoring"]
    }
  ]
}
```

**验证点:**
- [ ] 所有组件创建成功
  ```shell
  kubectl get all -l kube-min-cli-appId=${APP_ID}
  ```
- [ ] 网络配置正确
  ```shell
  kubectl get svc -l kube-min-cli-appId=${APP_ID}
  ```
- [ ] 服务发现正常
- [ ] 数据存储正确
  ```shell
  kubectl get pvc -l kube-min-cli-appId=${APP_ID}
  ```
- [ ] 整体性能达标

### 4. 错误处理测试 (Error Handling Tests)

#### 4.1 组件创建失败测试

**测试项 TC016: 镜像拉取失败处理**
```yaml
# 测试用例
组件: bad-image-deployment
镜像: nonexistent/image:latest
```

```json
{
  "name": "test-app-016-bad-image",
  "namespace": "default",
  "version": "1.0.0",
  "description": "TC016 Image pull failure handling test",
  "component": [
    {
      "name": "bad-image-deployment",
      "type": "webservice",
      "image": "nonexistent-registry.example.com/nonexistent/image:v999",
      "replicas": 1,
      "properties": {
        "ports": [{"port": 80}]
      }
    }
  ],
  "workflow": [
    {
      "name": "deploy-bad-image",
      "mode": "StepByStep",
      "components": ["bad-image-deployment"]
    }
  ]
}
```

**预期行为:** 工作流应该在镜像拉取失败后进入 `failed` 状态。

**验证点:**
- [ ] 错误正确捕获
  ```shell
  curl http://127.0.0.1:8080/api/v1/workflow/tasks/${TASK_ID}/status
  # 应返回状态为 failed
  ```
- [ ] 状态更新为failed
  ```shell
  kubectl get pods -l kube-min-cli-appId=${APP_ID}
  # Pod 应处于 ImagePullBackOff 状态
  ```
- [ ] 资源清理执行
- [ ] 错误信息详细
  ```shell
  kubectl describe pod -l kube-min-cli-appId=${APP_ID} | grep -A5 'Events'
  ```
- [ ] 重试机制有效

**测试项 TC017: 资源不足处理**
```yaml
# 测试用例
组件: high-resource-app
资源需求:
  cpu: 1000核
  memory: 1Ti
```

```json
{
  "name": "test-app-017-resource-exceeded",
  "namespace": "default",
  "version": "1.0.0",
  "description": "TC017 Resource insufficient handling test",
  "component": [
    {
      "name": "high-resource-app",
      "type": "webservice",
      "image": "nginx:latest",
      "replicas": 1,
      "properties": {
        "ports": [{"port": 80}]
      },
      "traits": {
        "resources": {
          "cpu": "1000",
          "memory": "1Ti"
        }
      }
    }
  ],
  "workflow": [
    {
      "name": "deploy-high-resource",
      "mode": "StepByStep",
      "components": ["high-resource-app"]
    }
  ]
}
```

**预期行为:** Pod 应该因为资源不足而无法调度。

**验证点:**
- [ ] 调度失败检测
  ```shell
  kubectl get pods -l kube-min-cli-appId=${APP_ID}
  # Pod 应处于 Pending 状态
  ```
- [ ] 错误信息准确
  ```shell
  kubectl describe pod -l kube-min-cli-appId=${APP_ID} | grep -A10 'Events'
  # 应显示 Insufficient cpu/memory 相关信息
  ```
- [ ] 回滚操作执行
- [ ] 状态正确更新
- [ ] 资源释放验证

#### 4.2 更新失败测试

**测试项 TC018: 冲突更新处理**
```yaml
# 测试场景
并发更新同一组件的不同字段
```

**前置条件:** 首先创建一个基础应用

```json
{
  "name": "test-app-018-conflict",
  "namespace": "default",
  "version": "1.0.0",
  "description": "TC018 Conflict update handling test",
  "component": [
    {
      "name": "conflict-app",
      "type": "webservice",
      "image": "nginx:1.21",
      "replicas": 2,
      "properties": {
        "ports": [{"port": 80}],
        "env": {
          "VERSION": "1.0.0"
        }
      }
    }
  ],
  "workflow": [
    {
      "name": "deploy-conflict-app",
      "mode": "StepByStep",
      "components": ["conflict-app"]
    }
  ]
}
```

**并发更新请求1:** `POST /api/v1/applications/${APP_ID}/version`

```json
{
  "version": "1.1.0",
  "strategy": "rolling",
  "components": [
    {
      "name": "conflict-app",
      "image": "nginx:1.22"
    }
  ],
  "description": "Update image"
}
```

**并发更新请求2:** `POST /api/v1/applications/${APP_ID}/version` (同时发送)

```json
{
  "version": "1.1.0",
  "strategy": "rolling",
  "components": [
    {
      "name": "conflict-app",
      "replicas": 3
    }
  ],
  "description": "Scale replicas"
}
```

**测试步骤:** 使用脚本同时发送两个更新请求

```shell
# 并发发送两个更新请求
curl -X POST "http://127.0.0.1:8080/api/v1/applications/${APP_ID}/version" \
  -H 'Content-Type: application/json' \
  -d '{"version":"1.1.0","strategy":"rolling","components":[{"name":"conflict-app","image":"nginx:1.22"}]}' &

curl -X POST "http://127.0.0.1:8080/api/v1/applications/${APP_ID}/version" \
  -H 'Content-Type: application/json' \
  -d '{"version":"1.1.0","strategy":"rolling","components":[{"name":"conflict-app","replicas":3}]}' &

wait
```

**验证点:**
- [ ] 版本冲突检测
- [ ] 乐观锁机制
- [ ] 重试逻辑正确
- [ ] 数据一致性
- [ ] 错误恢复有效

#### 4.3 回滚机制测试

**测试项 TC019: 工作流回滚测试**
```yaml
# 测试场景
多组件部署，中间步骤失败
```

```json
{
  "name": "test-app-019-rollback",
  "namespace": "default",
  "version": "1.0.0",
  "description": "TC019 Workflow rollback test - middle step fails",
  "component": [
    {
      "name": "config-success",
      "type": "config",
      "replicas": 1,
      "properties": {
        "conf": {
          "app.name": "rollback-test"
        }
      }
    },
    {
      "name": "secret-success",
      "type": "secret",
      "replicas": 1,
      "properties": {
        "secret": {
          "api-key": "test-key"
        }
      }
    },
    {
      "name": "bad-deployment",
      "type": "webservice",
      "image": "nonexistent-registry.example.com/bad/image:v999",
      "replicas": 1,
      "properties": {
        "ports": [{"port": 80}]
      }
    },
    {
      "name": "dependent-app",
      "type": "webservice",
      "image": "nginx:latest",
      "replicas": 1,
      "properties": {
        "ports": [{"port": 80}]
      }
    }
  ],
  "workflow": [
    {
      "name": "step1-config",
      "mode": "StepByStep",
      "components": ["config-success"]
    },
    {
      "name": "step2-secret",
      "mode": "StepByStep",
      "components": ["secret-success"]
    },
    {
      "name": "step3-bad-deployment",
      "mode": "StepByStep",
      "components": ["bad-deployment"]
    },
    {
      "name": "step4-dependent",
      "mode": "StepByStep",
      "components": ["dependent-app"]
    }
  ]
}
```

**预期行为:** 步骤1和2成功，步骤3失败，触发回滚机制。

**验证点:**
- [ ] 已创建组件清理
  ```shell
  kubectl get cm config-success
  kubectl get secret secret-success
  # 根据回滚策略，这些资源可能被清理
  ```
- [ ] 依赖关系处理
- [ ] 状态正确回退
  ```shell
  curl http://127.0.0.1:8080/api/v1/workflow/tasks/${TASK_ID}/status
  # 应显示失败状态及回滚信息
  ```
- [ ] 资源完全删除
- [ ] 系统状态一致

### 5. 边界条件测试 (Boundary Tests)

#### 5.1 资源限制测试

**测试项 TC020: 大规模组件测试**
```yaml
# 测试用例
应用: large-scale-app
组件数量: 100+
每个组件: 复杂traits配置
```

```json
{
  "name": "test-app-020-large-scale",
  "namespace": "default",
  "version": "1.0.0",
  "description": "TC020 Large scale component test (10 components demo)",
  "component": [
    {
      "name": "service-01",
      "type": "webservice",
      "image": "nginx:latest",
      "replicas": 1,
      "properties": {
        "ports": [{"port": 80}],
        "env": {"SERVICE_ID": "01"}
      }
    },
    {
      "name": "service-02",
      "type": "webservice",
      "image": "nginx:latest",
      "replicas": 1,
      "properties": {
        "ports": [{"port": 80}],
        "env": {"SERVICE_ID": "02"}
      }
    },
    {
      "name": "service-03",
      "type": "webservice",
      "image": "nginx:latest",
      "replicas": 1,
      "properties": {
        "ports": [{"port": 80}],
        "env": {"SERVICE_ID": "03"}
      }
    },
    {
      "name": "service-04",
      "type": "webservice",
      "image": "nginx:latest",
      "replicas": 1,
      "properties": {
        "ports": [{"port": 80}],
        "env": {"SERVICE_ID": "04"}
      }
    },
    {
      "name": "service-05",
      "type": "webservice",
      "image": "nginx:latest",
      "replicas": 1,
      "properties": {
        "ports": [{"port": 80}],
        "env": {"SERVICE_ID": "05"}
      }
    },
    {
      "name": "config-01",
      "type": "config",
      "replicas": 1,
      "properties": {
        "conf": {"key": "value-01"}
      }
    },
    {
      "name": "config-02",
      "type": "config",
      "replicas": 1,
      "properties": {
        "conf": {"key": "value-02"}
      }
    },
    {
      "name": "redis-01",
      "type": "store",
      "image": "redis:7-alpine",
      "replicas": 1,
      "properties": {
        "ports": [{"port": 6379, "expose": true}]
      }
    },
    {
      "name": "redis-02",
      "type": "store",
      "image": "redis:7-alpine",
      "replicas": 1,
      "properties": {
        "ports": [{"port": 6379, "expose": true}]
      }
    },
    {
      "name": "secret-01",
      "type": "secret",
      "replicas": 1,
      "properties": {
        "secret": {"api-key": "secret-value-01"}
      }
    }
  ],
  "workflow": [
    {
      "name": "step1-configs",
      "mode": "DAG",
      "components": ["config-01", "config-02", "secret-01"]
    },
    {
      "name": "step2-stores",
      "mode": "DAG",
      "components": ["redis-01", "redis-02"]
    },
    {
      "name": "step3-services",
      "mode": "DAG",
      "components": ["service-01", "service-02", "service-03", "service-04", "service-05"]
    }
  ]
}
```

**注意:** 实际大规模测试应使用脚本生成100+组件。以上为10组件示例。

**验证点:**
- [ ] 性能不降级
  ```shell
  time curl -X POST http://127.0.0.1:8080/api/v1/applications \
    -H 'Content-Type: application/json' \
    -d @payloads/large-scale.json
  ```
- [ ] 内存使用合理
  ```shell
  kubectl top pods -l kube-min-cli-appId=${APP_ID}
  ```
- [ ] 并发控制有效
- [ ] 状态管理正确
  ```shell
  kubectl get all -l kube-min-cli-appId=${APP_ID} --show-labels
  ```
- [ ] 资源清理完整

**测试项 TC021: 大容量配置测试**
```yaml
# 测试用例
配置大小: 1MB+
环境变量: 1000+
卷挂载: 50+
```

```json
{
  "name": "test-app-021-large-config",
  "namespace": "default",
  "version": "1.0.0",
  "description": "TC021 Large capacity configuration test",
  "component": [
    {
      "name": "large-config",
      "type": "config",
      "replicas": 1,
      "properties": {
        "conf": {
          "config-line-001": "This is a large configuration file content line 001 with some additional padding text to increase the size",
          "config-line-002": "This is a large configuration file content line 002 with some additional padding text to increase the size",
          "config-line-003": "This is a large configuration file content line 003 with some additional padding text to increase the size",
          "config-line-004": "This is a large configuration file content line 004 with some additional padding text to increase the size",
          "config-line-005": "This is a large configuration file content line 005 with some additional padding text to increase the size",
          "config-line-006": "This is a large configuration file content line 006 with some additional padding text to increase the size",
          "config-line-007": "This is a large configuration file content line 007 with some additional padding text to increase the size",
          "config-line-008": "This is a large configuration file content line 008 with some additional padding text to increase the size",
          "config-line-009": "This is a large configuration file content line 009 with some additional padding text to increase the size",
          "config-line-010": "This is a large configuration file content line 010 with some additional padding text to increase the size"
        }
      }
    },
    {
      "name": "app-with-many-envs",
      "type": "webservice",
      "image": "nginx:latest",
      "replicas": 1,
      "properties": {
        "ports": [{"port": 80}],
        "env": {
          "ENV_VAR_001": "value001",
          "ENV_VAR_002": "value002",
          "ENV_VAR_003": "value003",
          "ENV_VAR_004": "value004",
          "ENV_VAR_005": "value005",
          "ENV_VAR_006": "value006",
          "ENV_VAR_007": "value007",
          "ENV_VAR_008": "value008",
          "ENV_VAR_009": "value009",
          "ENV_VAR_010": "value010",
          "ENV_VAR_011": "value011",
          "ENV_VAR_012": "value012",
          "ENV_VAR_013": "value013",
          "ENV_VAR_014": "value014",
          "ENV_VAR_015": "value015",
          "ENV_VAR_016": "value016",
          "ENV_VAR_017": "value017",
          "ENV_VAR_018": "value018",
          "ENV_VAR_019": "value019",
          "ENV_VAR_020": "value020"
        }
      },
      "traits": {
        "storage": [
          {
            "name": "config-vol-1",
            "type": "config",
            "sourceName": "large-config",
            "mountPath": "/etc/config1",
            "readOnly": true
          },
          {
            "name": "config-vol-2",
            "type": "config",
            "sourceName": "large-config",
            "mountPath": "/etc/config2",
            "readOnly": true
          },
          {
            "name": "config-vol-3",
            "type": "config",
            "sourceName": "large-config",
            "mountPath": "/etc/config3",
            "readOnly": true
          }
        ],
        "envFrom": [
          {"type": "configMap", "sourceName": "large-config"}
        ]
      }
    }
  ],
  "workflow": [
    {
      "name": "step1-config",
      "mode": "StepByStep",
      "components": ["large-config"]
    },
    {
      "name": "step2-app",
      "mode": "StepByStep",
      "components": ["app-with-many-envs"]
    }
  ]
}
```

**注意:** 实际大容量测试应使用脚本生成更大的配置文件。以上为示例。

**验证点:**
- [ ] 配置正确应用
  ```shell
  kubectl get cm large-config -o yaml
  ```
- [ ] 性能不受影响
- [ ] 资源限制遵守
- [ ] 错误处理有效
- [ ] 更新操作正常

#### 5.2 命名规范测试

**测试项 TC022: 特殊字符处理**

```yaml
# 测试用例
组件名称: test-app_with.special@chars
配置名称: config_with_underscores-and-dashes
密钥名称: secret.with.dots
```

```json
{
  "name": "test-app-022-special-chars",
  "namespace": "default",
  "version": "1.0.0",
  "description": "TC022 Special character handling test",
  "component": [
    {
      "name": "config-with-dashes",
      "type": "config",
      "replicas": 1,
      "properties": {
        "conf": {
          "key.with.dots": "value1",
          "key-with-dashes": "value2",
          "key_with_underscores": "value3"
        }
      }
    },
    {
      "name": "secret-with-dashes",
      "type": "secret",
      "replicas": 1,
      "properties": {
        "secret": {
          "api-key": "secret-value",
          "db-password": "password123"
        }
      }
    },
    {
      "name": "app-with-special-name",
      "type": "webservice",
      "image": "nginx:latest",
      "replicas": 1,
      "properties": {
        "ports": [{"port": 80}],
        "env": {
          "CONFIG_NAME": "config-with-dashes",
          "SECRET_NAME": "secret-with-dashes"
        }
      },
      "traits": {
        "envFrom": [
          {"type": "configMap", "sourceName": "config-with-dashes"},
          {"type": "secret", "sourceName": "secret-with-dashes"}
        ]
      }
    }
  ],
  "workflow": [
    {
      "name": "step1-configs",
      "mode": "DAG",
      "components": ["config-with-dashes", "secret-with-dashes"]
    },
    {
      "name": "step2-app",
      "mode": "StepByStep",
      "components": ["app-with-special-name"]
    }
  ]
}
```

**测试无效命名示例 (预期失败):**

```json
{
  "_comment": "This request should fail validation due to invalid naming",
  "name": "test-app-022-invalid",
  "namespace": "default",
  "version": "1.0.0",
  "description": "TC022 Invalid naming test - should fail",
  "component": [
    {
      "name": "invalid_name_with@special#chars",
      "type": "webservice",
      "image": "nginx:latest",
      "replicas": 1,
      "properties": {
        "ports": [{"port": 80}]
      }
    }
  ],
  "workflow": [
    {
      "name": "deploy",
      "mode": "StepByStep",
      "components": ["invalid_name_with@special#chars"]
    }
  ]
}
```

**验证点:**
- [ ] 命名规范转换
  ```shell
  kubectl get all -l kube-min-cli-appId=${APP_ID}
  # 验证资源名称符合 Kubernetes 命名规范
  ```
- [ ] Kubernetes兼容性
  ```shell
  kubectl get cm config-with-dashes
  kubectl get secret secret-with-dashes
  ```
- [ ] 引用关系正确
  ```shell
  kubectl describe pod -l kube-min-cli-appId=${APP_ID} | grep -A10 'Environment Variables from'
  ```
- [ ] 错误处理适当
  ```shell
  # 使用无效命名的请求应返回 400 错误
  curl -X POST http://127.0.0.1:8080/api/v1/applications \
    -H 'Content-Type: application/json' \
    -d @payloads/invalid-naming.json
  ```
- [ ] 状态更新正常

### 6.模板实例化测试 (Tem.id 克隆)

#### 测试项 TC023：单次克隆模板（tmp_enable=true）
- 前置：模板应用已存在且 `tmp_enable=true`，包含 store + secret 组件。
- 请求示例（覆盖 env/secret）：
```json
{
  "name": "tenant-a-mysql-app",
  "namespace": "mysql",
  "alias": "tenant-a-mysql",
  "version": "1.0.3",
  "description": "mysql cloned from template",
  "component": [
    { "name": "tenant-a-mysql", "type": "store", "Tem": { "id": "tmpl-mysql-id" }, "properties": { "env": { "MYSQL_DATABASE": "demo" } } },
    { "name": "tenant-a-config", "type": "secret", "properties": { "secret": { "MYSQL_ROOT_PASSWORD": "d3loNWFjTFVjWUR5ZjF1VA==" } }, "Tem": { "id": "tmpl-mysql-id" } }
  ]
}
```
- 验证：
  - 只克隆一遍模板，最终组件数与模板一致（不因多条条目倍增）。
  - 名称/traits 重写：组件按请求名或 baseName，storage/ingress 等随之重写；RBAC 名保持模板值，命名空间对齐组件。
  - 覆盖：`properties.env` 覆盖模板 env；`properties.secret` 仅对 `type=secret` 组件覆盖模板 Secret。
  - 模板 `tmp_enable=true` 方可引用，`tmp_enable=false` 返回 400。

#### 测试项 TC024：模板禁用/ID 缺失错误
- 模板 `tmp_enable=false` 或 `Tem.id` 为空/不存在，返回 400/404，错误信息分别为 `template application is not enabled` / `template id is required` / `application name is not exist`。

#### 测试项 TC025：同模板多条覆盖匹配
- 同一 `Tem.id` 多条目仅用于覆盖（类型优先匹配），未匹配的模板组件按 baseName 或模板名生成新名称。
- 校验组件命名、env/secret 覆盖结果与预期一致，组件数量不重复。



### 7.版本更新测试(Update Version Test)

#### 7.1 简单镜像更新

#### 测试项 TC026：简单镜像更新

准备一个简单的Backend服务

```json
{
  "name": "my-backend-app",
  "namespace": "default",
  "version": "1.0.0",
  "description": "My backend application",
  "component": [
    {
      "name": "backend",
      "type": "webservice",
      "image": "nginx:1.23",
      "replicas": 2,
      "properties": {
        "ports": [
          {"port": 8080}
        ],
        "env": {
          "ENV": "production",
          "LOG_LEVEL": "info"
        }
      }
    }
  ]
}
```

响应：

```json
{
    "id": "a8h07bwds3f2f4ewzbzyew5c",
    "name": "my-backend-app",
    "alias": "",
    "project": "",
    "version": "1.0.0",
    "description": "My backend application",
    "createTime": "2025-12-07T20:47:56.905968+08:00",
    "updateTime": "2025-12-07T20:47:56.905968+08:00",
    "icon": "",
    "workflow_id": "adw9ccyo7n6f0iorzthrmo34",
    "tmp_enable": false
}
```

1.执行工作流

**请求**：`POST /api/v1/applications/a8h07bwds3f2f4ewzbzyew5c/workflow/exec` body:`{"workflowId":"adw9ccyo7n6f0iorzthrmo34"}`

正常返回:

```json
{
    "taskId": "80639qtadaz8bogotvmok9vh"
}
```

使用kubectl get deploy 

```shell
> kubectl get deploy 
NAME                                            READY   UP-TO-DATE   AVAILABLE   AGE
deploy-backend-a8h07bwds3f2f4ewzbzyew5c         2/2     2            2           61s

> kubectl get po
NAME                                                             READY   STATUS    RESTARTS   AGE
deploy-backend-a8h07bwds3f2f4ewzbzyew5c-77c67c9df6-fm95d          1/1     Running   0          46s
deploy-backend-a8h07bwds3f2f4ewzbzyew5c-77c67c9df6-rxzp8          1/1     Running   0          46s

> kubectl get deploy deploy-backend-a8h07bwds3f2f4ewzbzyew5c -o yaml

....
spec:
      containers:
      - env:
        - name: ENV
          value: production
        - name: LOG_LEVEL
          value: info
        image: nginx:1.23 #符合已部署的镜像
        imagePullPolicy: IfNotPresent
        name: backend
        ports:
        - containerPort: 8080
          protocol: TCP
        resources: {}
        terminationMessagePath: /dev/termination-log
        terminationMessagePolicy: File
      dnsPolicy: ClusterFirst
      restartPolicy: Always
      schedulerName: default-scheduler
      securityContext: {}
      terminationGracePeriodSeconds: 30
...
```



2.更新镜像版本，现在将 backend 组件的镜像从 `v1.0.0` 更新到 `v1.2.0`：

**请求**：`POST /api/v1/applications/a8h07bwds3f2f4ewzbzyew5c/version`

```
{
  "version": "1.2.0",
  "strategy": "rolling",
  "components": [
    {
      "name": "backend",
      "image": "nginx:latest"
    }
  ],
  "description": "Update backend image to v1.2.0"
}
```

正常返回：

```
{
    "appId": "a8h07bwds3f2f4ewzbzyew5c",
    "version": "1.2.0",
    "previousVersion": "1.0.0",
    "strategy": "rolling",
    "taskId": "1esnitda8cxi85u4clapfn89",
    "updatedComponents": [
        "backend"
    ]
}
```

验证镜像是否发生改变：

```yaml
> kubectl get deploy deploy-backend-a8h07bwds3f2f4ewzbzyew5c -o yaml
...
spec:
      containers:
      - env:
        - name: ENV
          value: production
        - name: LOG_LEVEL
          value: info
        image: nginx:latest
        imagePullPolicy: IfNotPresent
        name: backend
        ports:
        - containerPort: 8080
          protocol: TCP
        resources: {}
        terminationMessagePath: /dev/termination-log
        terminationMessagePolicy: File
      dnsPolicy: ClusterFirst
      restartPolicy: Always
      schedulerName: default-scheduler
      securityContext: {}
      terminationGracePeriodSeconds: 30
      
      
> kubectl get po deploy-backend-a8h07bwds3f2f4ewzbzyew5c-77c67c9df6-fm95d -o yaml
...
spec:
  containers:
  - env:
    - name: ENV
      value: production
    - name: LOG_LEVEL
      value: info
    image: nginx:latest
    imagePullPolicy: IfNotPresent
    name: backend
    ports:
    - containerPort: 8080
      protocol: TCP
    resources: {}
    terminationMessagePath: /dev/termination-log
    terminationMessagePolicy: File
    volumeMounts:
    - mountPath: /var/run/secrets/kubernetes.io/serviceaccount
      name: kube-api-access-txlz8
      readOnly: true
```





---

*本文档将根据实际测试执行情况进行更新和完善*
