package event

import (
	"KubeMin-Cli/pkg/apiserver/event/workflow"
	"k8s.io/klog/v2"
	"os"
)

// InitEventWithOptions 根据配置初始化事件工作器
func InitEventWithOptions(redisAddr string) []interface{} {
	// 如果没有提供Redis地址，尝试从环境变量读取
	if redisAddr == "" {
		redisAddr = os.Getenv("REDIS_ADDR")
	}

	if redisAddr != "" {
		klog.Infof("Initializing distributed workflow with Redis at %s", redisAddr)
		// 创建简化的分布式工作流
		distributedWorkflow := &workflow.SimpleDistributedWorkflow{}
		distributedWorkflow.SetRedisAddr(redisAddr)
		distributedWorkflow.SetMaxWorkers(10)
		workers = append(workers, distributedWorkflow)
		return []interface{}{distributedWorkflow}
	}

	// 使用原有的InitEvent逻辑
	return InitEvent()
}
