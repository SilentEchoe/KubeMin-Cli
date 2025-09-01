package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"KubeMin-Cli/pkg/apiserver/config"
	"KubeMin-Cli/pkg/apiserver/domain/model"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"k8s.io/klog/v2"
)

// SimpleDistributedWorkflow 简化的分布式工作流实现
type SimpleDistributedWorkflow struct {
	// 嵌入原有的Workflow以复用其方法
	Workflow

	// 分布式相关
	redisClient *redis.Client
	isLeader    bool
	nodeID      string
	mu          sync.RWMutex

	// 配置
	enableDistributed bool
	redisAddr         string
	maxWorkers        int
}

// Start 启动分布式工作流
func (sdw *SimpleDistributedWorkflow) Start(ctx context.Context, errChan chan error) {
	// 设置节点ID
	if sdw.nodeID == "" {
		hostname, _ := os.Hostname()
		sdw.nodeID = fmt.Sprintf("%s-%s", hostname, uuid.New().String()[:8])
	}

	// 尝试从环境变量获取Redis地址
	if sdw.redisAddr == "" {
		sdw.redisAddr = os.Getenv("REDIS_ADDR")
	}

	// 如果配置了Redis，启用分布式模式
	if sdw.redisAddr != "" {
		if err := sdw.initRedis(); err != nil {
			klog.Errorf("Failed to init Redis, falling back to local mode: %v", err)
			sdw.enableDistributed = false
		} else {
			sdw.enableDistributed = true
			klog.Infof("Distributed mode enabled with Redis at %s", sdw.redisAddr)
		}
	}

	// 初始化队列（处理未完成的任务）
	sdw.Workflow.InitQueue(ctx)

	if sdw.enableDistributed {
		// 分布式模式
		go sdw.distributedTaskSender(ctx)
		go sdw.distributedWorker(ctx)
	} else {
		// 本地模式
		go sdw.Workflow.WorkflowTaskSender()
	}
}

// initRedis 初始化Redis连接
func (sdw *SimpleDistributedWorkflow) initRedis() error {
	sdw.redisClient = redis.NewClient(&redis.Options{
		Addr:         sdw.redisAddr,
		PoolSize:     10,
		MinIdleConns: 5,
		MaxRetries:   3,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return sdw.redisClient.Ping(ctx).Err()
}

// distributedTaskSender 分布式任务发送器（只有Leader执行）
func (sdw *SimpleDistributedWorkflow) distributedTaskSender(ctx context.Context) {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// 只有Leader才能调度新任务
			if !sdw.isLeader {
				continue
			}
			// 获取等待的任务
			waitingTasks, err := sdw.WorkflowService.WaitingTasks(ctx)
			if err != nil || len(waitingTasks) == 0 {
				continue
			}

			for _, task := range waitingTasks {
				if err := sdw.publishTask(ctx, task); err != nil {
					klog.Errorf("Failed to publish task %s: %v", task.TaskID, err)
				}
			}
		}
	}
}

// distributedWorker 分布式工作节点
func (sdw *SimpleDistributedWorkflow) distributedWorker(ctx context.Context) {
	// 启动多个worker
	for i := 0; i < sdw.maxWorkers; i++ {
		go sdw.workerLoop(ctx, i)
	}
}

// workerLoop 单个worker循环
func (sdw *SimpleDistributedWorkflow) workerLoop(ctx context.Context, workerID int) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			// 从Redis队列获取任务
			task, err := sdw.fetchTask(ctx)
			if err != nil {
				time.Sleep(time.Second)
				continue
			}

			if task != nil {
				sdw.executeTask(ctx, task, workerID)
			}
		}
	}
}

// publishTask 发布任务到Redis
func (sdw *SimpleDistributedWorkflow) publishTask(ctx context.Context, task *model.WorkflowQueue) error {
	// 更新任务状态
	task.Status = config.StatusQueued
	if !sdw.WorkflowService.UpdateTask(ctx, task) {
		return fmt.Errorf("failed to update task status")
	}

	// 序列化任务
	taskData, err := json.Marshal(task)
	if err != nil {
		return err
	}

	// 发布到Redis队列
	return sdw.redisClient.RPush(ctx, "workflow:queue", taskData).Err()
}

// fetchTask 从Redis获取任务
func (sdw *SimpleDistributedWorkflow) fetchTask(ctx context.Context) (*model.WorkflowQueue, error) {
	// 从队列获取任务
	result, err := sdw.redisClient.BLPop(ctx, time.Second, "workflow:queue").Result()
	if err != nil {
		return nil, err
	}

	if len(result) > 1 {
		var task model.WorkflowQueue
		if err := json.Unmarshal([]byte(result[1]), &task); err != nil {
			return nil, err
		}
		return &task, nil
	}

	return nil, nil
}

// executeTask 执行任务
func (sdw *SimpleDistributedWorkflow) executeTask(ctx context.Context, task *model.WorkflowQueue, workerID int) {
	klog.Infof("Worker %d executing task %s", workerID, task.TaskID)

	// 使用分布式锁避免重复执行
	lockKey := fmt.Sprintf("lock:%s", task.TaskID)
	locked := sdw.acquireLock(ctx, lockKey, 30*time.Second)
	if !locked {
		// 任务已被其他worker处理
		return
	}
	defer sdw.releaseLock(ctx, lockKey)

	// 执行任务
	controller := NewWorkflowController(task, sdw.KubeClient, sdw.Store)
	controller.Run(ctx, 1)
}

// acquireLock 获取分布式锁
func (sdw *SimpleDistributedWorkflow) acquireLock(ctx context.Context, key string, ttl time.Duration) bool {
	result, err := sdw.redisClient.SetNX(ctx, key, sdw.nodeID, ttl).Result()
	if err != nil {
		klog.Errorf("Failed to acquire lock: %v", err)
		return false
	}
	return result
}

// releaseLock 释放分布式锁
func (sdw *SimpleDistributedWorkflow) releaseLock(ctx context.Context, key string) {
	sdw.redisClient.Del(ctx, key)
}

// SetAsLeader 设置是否为Leader
func (sdw *SimpleDistributedWorkflow) SetAsLeader(isLeader bool) {
	sdw.mu.Lock()
	defer sdw.mu.Unlock()
	sdw.isLeader = isLeader

	if isLeader {
		klog.Infof("Node %s became leader", sdw.nodeID)
	} else {
		klog.Infof("Node %s is follower", sdw.nodeID)
	}
}

// SetMaxWorkers 设置最大Worker数
func (sdw *SimpleDistributedWorkflow) SetMaxWorkers(count int) {
	sdw.mu.Lock()
	defer sdw.mu.Unlock()
	sdw.maxWorkers = count
}

// SetRedisAddr 设置Redis地址
func (sdw *SimpleDistributedWorkflow) SetRedisAddr(addr string) {
	sdw.mu.Lock()
	defer sdw.mu.Unlock()
	sdw.redisAddr = addr
}
