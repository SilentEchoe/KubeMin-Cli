package event

import (
    "KubeMin-Cli/pkg/apiserver/config"
    "KubeMin-Cli/pkg/apiserver/event/workflow"
    "k8s.io/klog/v2"
)

// InitEventWithOptions 根据配置初始化事件工作器
func InitEventWithOptions(cfg *config.Config) []interface{} {
    // 创建简化的分布式工作流：在启动窗口内自动判定是否启用分布式
    distributedWorkflow := &workflow.SimpleDistributedWorkflow{}
    distributedWorkflow.SetRedisAddr(cfg.Cache.CacheHost)
    distributedWorkflow.SetMaxWorkers(cfg.MaxWorkers)
    distributedWorkflow.SetLeaseNamespace(cfg.LeaseNamespace)
    distributedWorkflow.SetLeaseDurationSeconds(cfg.LeaseDurationSeconds)
    workers = append(workers, distributedWorkflow)
    klog.Infof("Initialized workflow worker (redis=%v, leaseNS=%s, leaseTTL=%ds)", cfg.Cache.CacheHost != "", cfg.LeaseNamespace, cfg.LeaseDurationSeconds)
    return []interface{}{distributedWorkflow}
}
