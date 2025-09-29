package signal

import (
	"context"
	"testing"
	"time"

	miniredis "github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"KubeMin-Cli/pkg/apiserver/utils/cache"
)

func TestCancelWatcherReceivesSignal(t *testing.T) {
	server, err := miniredis.Run()
	if err != nil {
		t.Fatalf("start miniredis: %v", err)
	}
	defer server.Close()

	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	cache.SetGlobalRedisClient(client)
	defer cache.SetGlobalRedisClient(nil)

	watcher, jobCtx, cancelFn, err := Watch(context.Background(), "task-cancel-test")
	if err != nil {
		t.Fatalf("watcher setup failed: %v", err)
	}
	defer cancelFn()

	done := make(chan struct{})
	go func() {
		<-jobCtx.Done()
		close(done)
	}()

	if err := Cancel(context.Background(), "task-cancel-test", "manual stop"); err != nil {
		t.Fatalf("send cancel signal: %v", err)
	}

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatalf("expected cancel signal to close context")
	}

	if reason := watcher.Reason(); reason != "manual stop" {
		t.Fatalf("unexpected cancel reason: %s", reason)
	}

	watcher.Stop(context.Background())
	if exists, _ := client.Exists(context.Background(), cancelKeyPrefix+"task-cancel-test").Result(); exists != 0 {
		t.Fatalf("expected cancel key to be removed, got %d", exists)
	}
}
