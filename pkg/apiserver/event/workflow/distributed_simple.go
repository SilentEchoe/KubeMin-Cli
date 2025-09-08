package workflow

import (
    "context"
    "encoding/json"
    "fmt"
    "os"
    "sync"
    "time"

    coordinationv1 "k8s.io/api/coordination/v1"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

    // k8s lease based heartbeat
    leaseNamespace    string
    leaseDurationSecs int32
    decided           bool
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

    // 初始化队列（处理未完成的任务）
    sdw.Workflow.InitQueue(ctx)

    // 启动 Lease 心跳（无论是否分布式，都用于统计实例数）
    if sdw.leaseNamespace == "" {
        sdw.leaseNamespace = "min-cli-system"
    }
    if sdw.leaseDurationSecs == 0 {
        sdw.leaseDurationSecs = 10
    }
    go sdw.heartbeatK8s(ctx)

    // 启动阶段：在一个观测窗口内统计副本数，决定是否开启分布式
    go sdw.bootstrap(ctx)
}

// bootstrap 决策是否启用分布式模式（只在启动时决定一次）
func (sdw *SimpleDistributedWorkflow) bootstrap(ctx context.Context) {
    window := 10 * time.Second
    ticker := time.NewTicker(2 * time.Second)
    defer ticker.Stop()
    deadline := time.Now().Add(window)
    maxCount := 1

    for time.Now().Before(deadline) {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            cnt, err := sdw.countNodesK8s(ctx)
            if err != nil {
                klog.Errorf("count nodes failed: %v", err)
                continue
            }
            if cnt > maxCount {
                maxCount = cnt
            }
        }
    }

    // 仅在第一次做决策
    sdw.mu.Lock()
    if sdw.decided {
        sdw.mu.Unlock()
        return
    }

    // 若副本数>=3 且 配置了Redis，尝试开启分布式
    if maxCount >= 3 && sdw.redisAddr != "" {
        if err := sdw.initRedis(); err != nil {
            klog.Errorf("Failed to init Redis, fallback to local mode: %v", err)
            sdw.enableDistributed = false
        } else {
            klog.Infof("Enable distributed workflow (replicas=%d, redis=%s)", maxCount, sdw.redisAddr)
            sdw.enableDistributed = true
        }
    } else {
        klog.Infof("Run in local workflow mode (maxReplicas=%d, redisConfigured=%v)", maxCount, sdw.redisAddr != "")
        sdw.enableDistributed = false
    }
    decided := sdw.enableDistributed
    sdw.decided = true
    sdw.mu.Unlock()

    // 根据决策启动发送与执行
    if decided {
        go sdw.distributedTaskSender(ctx)
        go sdw.distributedWorker(ctx)
    } else {
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
            // 如果是Leader且存在多节点，Leader只负责写入，不拉取执行任务
            if sdw.shouldSkipExecution(ctx) {
                klog.V(2).Infof("leader node %s skipping execution (multi-node)", sdw.nodeID)
                time.Sleep(time.Second)
                continue
            }
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

// heartbeatK8s 通过 Kubernetes Lease 上报心跳
func (sdw *SimpleDistributedWorkflow) heartbeatK8s(ctx context.Context) {
    // 确保 Lease 存在
    leaseName := fmt.Sprintf("kubemin-cli-node-%s", sdw.nodeID)
    if err := sdw.ensureLease(ctx, leaseName); err != nil {
        klog.Errorf("ensure lease failed: %v", err)
        return
    }
    ticker := time.NewTicker(3 * time.Second)
    defer ticker.Stop()
    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            now := metav1.MicroTime{Time: time.Now()}
            // 更新 renewTime
            lease, err := sdw.KubeClient.CoordinationV1().Leases(sdw.leaseNamespace).Get(ctx, leaseName, metav1.GetOptions{})
            if err != nil {
                klog.Errorf("get lease failed: %v", err)
                continue
            }
            lease.Spec.RenewTime = &now
            if lease.Spec.LeaseDurationSeconds == nil {
                d := sdw.leaseDurationSecs
                lease.Spec.LeaseDurationSeconds = &d
            }
            if _, err := sdw.KubeClient.CoordinationV1().Leases(sdw.leaseNamespace).Update(ctx, lease, metav1.UpdateOptions{}); err != nil {
                klog.Errorf("update lease failed: %v", err)
            }
        }
    }
}

func (sdw *SimpleDistributedWorkflow) ensureLease(ctx context.Context, name string) error {
    _, err := sdw.KubeClient.CoordinationV1().Leases(sdw.leaseNamespace).Get(ctx, name, metav1.GetOptions{})
    if err == nil {
        return nil
    }
    now := metav1.MicroTime{Time: time.Now()}
    d := sdw.leaseDurationSecs
    lease := &coordinationv1.Lease{
        ObjectMeta: metav1.ObjectMeta{
            Name:      name,
            Namespace: sdw.leaseNamespace,
            Labels: map[string]string{
                "app": "kubemin-cli-node",
            },
        },
        Spec: coordinationv1.LeaseSpec{
            HolderIdentity:       &sdw.nodeID,
            LeaseDurationSeconds: &d,
            AcquireTime:          &now,
            RenewTime:            &now,
        },
    }
    if _, err := sdw.KubeClient.CoordinationV1().Leases(sdw.leaseNamespace).Create(ctx, lease, metav1.CreateOptions{}); err != nil {
        return err
    }
    return nil
}

// shouldSkipExecution 判断本节点是否应跳过执行（Leader 且多节点）
func (sdw *SimpleDistributedWorkflow) shouldSkipExecution(ctx context.Context) bool {
    sdw.mu.RLock()
    leader := sdw.isLeader
    sdw.mu.RUnlock()
    if !leader {
        return false
    }
    cnt, err := sdw.countNodesK8s(ctx)
    if err != nil {
        // 无法统计节点数时，默认不跳过（保证可用性）
        return false
    }
    return cnt > 1
}

// countNodesK8s 统计有效 Lease 的数量
func (sdw *SimpleDistributedWorkflow) countNodesK8s(ctx context.Context) (int, error) {
    ls, err := sdw.KubeClient.CoordinationV1().Leases(sdw.leaseNamespace).List(ctx, metav1.ListOptions{LabelSelector: "app=kubemin-cli-node"})
    if err != nil {
        return 0, err
    }
    now := time.Now()
    var alive int
    for i := range ls.Items {
        l := ls.Items[i]
        if l.Spec.RenewTime == nil {
            continue
        }
    ttl := time.Duration(sdw.leaseDurationSecs) * time.Second
        if now.Sub(l.Spec.RenewTime.Time) <= ttl {
            alive++
        }
    }
    return alive, nil
}

// SetLeaseNamespace 配置 Lease 命名空间
func (sdw *SimpleDistributedWorkflow) SetLeaseNamespace(ns string) {
    sdw.mu.Lock()
    defer sdw.mu.Unlock()
    sdw.leaseNamespace = ns
}

// SetLeaseDurationSeconds 配置 Lease TTL（秒）
func (sdw *SimpleDistributedWorkflow) SetLeaseDurationSeconds(ttl int32) {
    sdw.mu.Lock()
    defer sdw.mu.Unlock()
    sdw.leaseDurationSecs = ttl
}
