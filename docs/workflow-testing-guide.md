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

**测试项 TC001: 基础Deployment创建**

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

  

**测试项 TC002: 基础Store组件创建**

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
        "ports": [
          {
            "port": 3306,
            "expose": true
          }
        ],
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
   curl http://127.0.0.1:8080/api/v1/tasks/${TASK_ID}
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



**测试项 TC003: 配置组件创建**

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

  

**测试项 TC004: 密钥组件创建**

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
**验证点:**
- [ ] Secret成功创建
- [ ] 数据正确编码
- [ ] 可以被其他组件引用
- [ ] 工作流状态为completed

#### 1.2 组件更新测试

**测试项 TC005: Deployment镜像更新**
```yaml
# 初始状态
组件: nginx-deployment
镜像: nginx:1.21

# 更新操作
新镜像: nginx:1.22
```
**验证点:**
- [ ] 滚动更新成功执行
- [ ] 新版本Pod正常启动
- [ ] 旧版本Pod平滑下线
- [ ] 服务不中断
- [ ] 更新状态为completed

**测试项 TC006: 环境变量更新**
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
**验证点:**
- [ ] 环境变量正确更新
- [ ] Pod重新创建
- [ ] 新配置生效
- [ ] 更新状态为completed

### 2. Trait系统测试 (Trait System Tests)

#### 2.1 存储Trait测试

**测试项 TC007: PVC挂载测试**
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
**验证点:**
- [ ] PVC自动创建
- [ ] Volume正确挂载
- [ ] 挂载路径正确
- [ ] 权限设置正确
- [ ] 数据持久化验证

**测试项 TC008: ConfigMap挂载测试**
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
**验证点:**
- [ ] ConfigMap正确挂载
- [ ] 文件路径正确
- [ ] 文件权限正确
- [ ] 内容只读属性
- [ ] 热更新支持

**测试项 TC009: Secret挂载测试**
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
**验证点:**
- [ ] Secret正确挂载
- [ ] 文件路径正确
- [ ] 文件权限正确(600)
- [ ] 内容自动解码
- [ ] 安全性验证

#### 2.2 网络Trait测试

**测试项 TC010: Service配置测试**
```yaml
# 测试用例
组件: app-deployment
traits:
  - 类型: service
    属性:
      ports:
        - 名称: http
        端口: 8080
        目标端口: 80
      type: ClusterIP
```
**验证点:**
- [ ] Service成功创建
- [ ] 端口映射正确
- [ ] 端点正确关联
- [ ] DNS解析正常
- [ ] 网络连通性验证

**测试项 TC011: Ingress配置测试**
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
**验证点:**
- [ ] Ingress成功创建
- [ ] 路由规则正确
- [ ] TLS配置正确
- [ ] 外部访问正常
- [ ] 证书验证通过

#### 2.3 RBAC Trait测试

**测试项 TC012: ServiceAccount配置测试**
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
**验证点:**
- [ ] ServiceAccount创建
- [ ] Role/ClusterRole创建
- [ ] RoleBinding创建
- [ ] 权限正确绑定
- [ ] Pod使用正确的SA

### 3. 多组件工作流测试 (Multi-Component Workflow Tests)

#### 3.1 依赖关系测试

**测试项 TC013: 顺序依赖测试**
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
**验证点:**
- [ ] 按依赖顺序执行
- [ ] 等待依赖就绪
- [ ] 错误时正确回滚
- [ ] 状态正确传播
- [ ] 并发控制有效

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
**验证点:**
- [ ] 并行执行正确
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
**验证点:**
- [ ] 所有组件创建成功
- [ ] 网络配置正确
- [ ] 服务发现正常
- [ ] 数据存储正确
- [ ] 整体性能达标

### 4. 错误处理测试 (Error Handling Tests)

#### 4.1 组件创建失败测试

**测试项 TC016: 镜像拉取失败处理**
```yaml
# 测试用例
组件: bad-image-deployment
镜像: nonexistent/image:latest
```
**验证点:**
- [ ] 错误正确捕获
- [ ] 状态更新为failed
- [ ] 资源清理执行
- [ ] 错误信息详细
- [ ] 重试机制有效

**测试项 TC017: 资源不足处理**
```yaml
# 测试用例
组件: high-resource-app
资源需求:
  cpu: 1000核
  memory: 1Ti
```
**验证点:**
- [ ] 调度失败检测
- [ ] 错误信息准确
- [ ] 回滚操作执行
- [ ] 状态正确更新
- [ ] 资源释放验证

#### 4.2 更新失败测试

**测试项 TC018: 冲突更新处理**
```yaml
# 测试场景
并发更新同一组件的不同字段
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
**验证点:**
- [ ] 已创建组件清理
- [ ] 依赖关系处理
- [ ] 状态正确回退
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
**验证点:**
- [ ] 性能不降级
- [ ] 内存使用合理
- [ ] 并发控制有效
- [ ] 状态管理正确
- [ ] 资源清理完整

**测试项 TC021: 大容量配置测试**
```yaml
# 测试用例
配置大小: 1MB+
环境变量: 1000+
卷挂载: 50+
```
**验证点:**
- [ ] 配置正确应用
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
**验证点:**
- [ ] 命名规范转换
- [ ] Kubernetes兼容性
- [ ] 引用关系正确
- [ ] 错误处理适当
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



### 7.版本更新测试

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
