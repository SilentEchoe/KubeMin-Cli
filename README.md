KubeMin-Cli 是一个云原生命令工具，它以云原生服务应用为中心，基于Kubernetes，帮助开发人员在公有云/私有云集群中，部署中间件服务。

设计思路：
脱离交付和管理平台的思路，KubeMin-CLi 意在以一个极简的 Pipeline 构建 CI/CD
本项目初版不支持数据库，参照 Client-Go 实现双层缓存保持唯一的数据准确性。

Client-Go 是如何实现与 kube-apiserver 通信的？

Client-Go 中的 WorkQueue 一般使用延时队列实现，Resource Event Handlers 会将对象的 Key 放入 WorkQueue。

