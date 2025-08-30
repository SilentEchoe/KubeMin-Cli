package workflow

import (
	"context"
	"os"
	"testing"
	"time"
)

// TestSimpleDistributedWorkflow 测试分布式工作流
func TestSimpleDistributedWorkflow(t *testing.T) {
	// 设置测试环境变量
	os.Setenv("REDIS_ADDR", "localhost:6379")
	defer os.Unsetenv("REDIS_ADDR")
	
	// 创建分布式工作流
	sdw := &SimpleDistributedWorkflow{}
	sdw.SetRedisAddr("localhost:6379")
	sdw.SetMaxWorkers(2)
	
	// 创建测试上下文
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	// 启动工作流（注意：需要依赖注入完成后才能真正工作）
	errChan := make(chan error, 1)
	go sdw.Start(ctx, errChan)
	
	// 等待一段时间
	time.Sleep(1 * time.Second)
	
	// 检查是否有错误
	select {
	case err := <-errChan:
		t.Fatalf("Workflow error: %v", err)
	default:
		t.Log("Workflow started successfully")
	}
}

// TestDistributedModeDetection 测试分布式模式检测
func TestDistributedModeDetection(t *testing.T) {
	tests := []struct {
		name      string
		redisAddr string
		wantDist  bool
	}{
		{
			name:      "With Redis address",
			redisAddr: "localhost:6379",
			wantDist:  true,
		},
		{
			name:      "Without Redis address",
			redisAddr: "",
			wantDist:  false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sdw := &SimpleDistributedWorkflow{}
			sdw.SetRedisAddr(tt.redisAddr)
			
			// 检查是否启用分布式
			if tt.wantDist && sdw.redisAddr == "" {
				t.Error("Expected distributed mode to be enabled")
			}
			if !tt.wantDist && sdw.redisAddr != "" {
				t.Error("Expected distributed mode to be disabled")
			}
		})
	}
}

// TestLeaderElection 测试Leader选举集成
func TestLeaderElection(t *testing.T) {
	sdw := &SimpleDistributedWorkflow{}
	
	// 测试设置为Leader
	sdw.SetAsLeader(true)
	if !sdw.isLeader {
		t.Error("Expected to be leader")
	}
	
	// 测试设置为Follower
	sdw.SetAsLeader(false)
	if sdw.isLeader {
		t.Error("Expected to be follower")
	}
}