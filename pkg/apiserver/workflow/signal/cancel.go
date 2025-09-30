package signal

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"k8s.io/klog/v2"

	"KubeMin-Cli/pkg/apiserver/utils/cache"
)

const (
	// cancelKeyPrefix is the Redis prefix for workflow cancellation keys.
	cancelKeyPrefix = "kubemin:workflow:cancel:"
	// defaultExpiry defines how long the cancellation key should live before expiring.
	defaultExpiry = 45 * time.Second
	// extendInterval controls how frequently we renew the key TTL and verify ownership.
	extendInterval = 10 * time.Second
)

// CancelWatcher coordinates redis-backed cancellation signalling for a workflow task.
type CancelWatcher struct {
	cli    *redis.Client
	key    string
	token  string
	stopCh chan struct{}
	once   sync.Once
	wg     sync.WaitGroup
	state  *cancelState
}

type cancelState struct {
	mu     sync.RWMutex
	reason string
}

func (c *cancelState) set(reason string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.reason = reason
}

func (c *cancelState) get() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.reason
}

// Watch establishes a cancellation watcher for the given workflow task. When Redis
// is not configured, the context is returned unchanged and cleanup becomes a no-op.
func Watch(ctx context.Context, taskID string) (*CancelWatcher, context.Context, context.CancelFunc, error) {
	cli := cache.GetGlobalRedisClient()
	if cli == nil {
		// No redis available; fall back to the provided context without cancellation propagation.
		emptyState := &cancelState{}
		ctxWithState := context.WithValue(ctx, cancelStateKey{}, emptyState)
		return &CancelWatcher{state: emptyState}, ctxWithState, func() {}, nil
	}

	key := cancelKeyPrefix + taskID
	token := uuid.NewString()
	watcher := &CancelWatcher{cli: cli, key: key, token: token, stopCh: make(chan struct{}), state: &cancelState{}}

	// Attempt to claim the key for this task execution.
	ok, err := cli.SetNX(ctx, key, token, defaultExpiry).Result()
	if err != nil {
		return nil, ctx, nil, fmt.Errorf("acquire cancel watcher lock: %w", err)
	}
	if !ok {
		// If key already exists but holds a cancel marker, treat as immediate cancellation.
		existing, err := cli.Get(ctx, key).Result()
		if err != nil && err != redis.Nil {
			return nil, ctx, nil, fmt.Errorf("inspect existing cancel key: %w", err)
		}
		if isCancelledToken(existing) {
			watcher.state.set(extractCancelReason(existing))
			derivedCtx, cancelFn := context.WithCancel(ctx)
			cancelFn()
			derivedCtx = context.WithValue(derivedCtx, cancelStateKey{}, watcher.state)
			return watcher, derivedCtx, func() {}, nil
		}
		return nil, ctx, nil, fmt.Errorf("task %s already running", taskID)
	}

	derivedCtx, cancelFn := context.WithCancel(ctx)
	derivedCtx = context.WithValue(derivedCtx, cancelStateKey{}, watcher.state)
	watcher.wg.Add(1)
	go watcher.maintain(derivedCtx, cancelFn)

	return watcher, derivedCtx, cancelFn, nil
}

// Cancel marks the workflow task as cancelled. Running watchers will detect the
// marker and cancel their contexts.
func Cancel(ctx context.Context, taskID, reason string) error {
	cli := cache.GetGlobalRedisClient()
	if cli == nil {
		return fmt.Errorf("redis client not configured")
	}
	value := cancelMarker(reason)
	return cli.Set(ctx, cancelKeyPrefix+taskID, value, defaultExpiry).Err()
}

// Stop releases the cancellation key when the workflow finishes.
func (w *CancelWatcher) Stop(ctx context.Context) {
	if w == nil {
		return
	}
	w.once.Do(func() {
		close(w.stopCh)
		w.wg.Wait()
		if w.cli == nil {
			return
		}
		val, err := w.cli.Get(ctx, w.key).Result()
		if err != nil && err != redis.Nil {
			klog.Warningf("get cancel key %s failed during release: %v", w.key, err)
			return
		}
		if val == w.token {
			if err := w.cli.Del(ctx, w.key).Err(); err != nil {
				klog.Warningf("failed to delete cancel key %s: %v", w.key, err)
			}
		}
	})
}

// Reason returns the cancellation reason observed by the watcher, if any.
func (w *CancelWatcher) Reason() string {
	if w == nil || w.state == nil {
		return ""
	}
	return w.state.get()
}

func (w *CancelWatcher) maintain(ctx context.Context, cancelFn context.CancelFunc) {
	defer w.wg.Done()
	ticker := time.NewTicker(extendInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-w.stopCh:
			return
		case <-ticker.C:
			w.step(ctx, cancelFn)
		}
	}
}

func (w *CancelWatcher) step(ctx context.Context, cancelFn context.CancelFunc) {
	if w.cli == nil {
		return
	}
	val, err := w.cli.Get(ctx, w.key).Result()
	if err == redis.Nil {
		cancelFn()
		return
	}
	if err != nil {
		klog.Warningf("cancel watcher get key %s failed: %v", w.key, err)
		return
	}
	if val != w.token {
		w.state.set(extractCancelReason(val))
		cancelFn()
		return
	}
	if err := w.cli.Expire(ctx, w.key, defaultExpiry).Err(); err != nil {
		klog.Warningf("failed to extend cancel key %s: %v", w.key, err)
	}
}

func cancelMarker(reason string) string {
	trimmed := strings.TrimSpace(reason)
	if trimmed == "" {
		trimmed = "cancelled"
	}
	return "cancelled:" + trimmed
}

func isCancelledToken(val string) bool {
	return strings.HasPrefix(val, "cancelled:")
}

func extractCancelReason(val string) string {
	if !isCancelledToken(val) {
		return "cancelled"
	}
	parts := strings.SplitN(val, ":", 2)
	if len(parts) != 2 || parts[1] == "" {
		return "cancelled"
	}
	return parts[1]
}

type cancelStateKey struct{}

// ReasonFromContext retrieves the cancellation reason set by the watcher.
func ReasonFromContext(ctx context.Context) string {
	raw := ctx.Value(cancelStateKey{})
	if raw == nil {
		return ""
	}
	if state, ok := raw.(*cancelState); ok {
		return state.get()
	}
	return ""
}
