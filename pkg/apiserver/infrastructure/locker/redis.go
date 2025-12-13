/*
Copyright 2024 The KubeMin Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package locker

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/go-redsync/redsync/v4"
	goredis "github.com/go-redsync/redsync/v4/redis/goredis/v9"
	"github.com/redis/go-redis/v9"
	"k8s.io/klog/v2"
)

// RedisLocker implements Locker using Redis as the backend.
// It uses the redsync library for distributed locking.
type RedisLocker struct {
	mu      sync.RWMutex
	client  *redis.Client
	redsync *redsync.Redsync
	prefix  string
	closed  bool
}

// NewRedisLocker creates a new Redis-based distributed locker.
func NewRedisLocker(client *redis.Client, prefix string) (*RedisLocker, error) {
	if client == nil {
		return nil, fmt.Errorf("redis client is nil")
	}

	pool := goredis.NewPool(client)
	rs := redsync.New(pool)

	return &RedisLocker{
		client:  client,
		redsync: rs,
		prefix:  prefix,
	}, nil
}

// NewMutex creates a new distributed mutex for the given key.
func (l *RedisLocker) NewMutex(key string, opts ...Option) Mutex {
	l.mu.RLock()
	defer l.mu.RUnlock()

	if l.closed {
		// Return a degraded mutex that will fail on operations
		return &degradedMutex{key: l.prefixedKey(key), err: ErrLockerClosed}
	}

	options := ApplyOptions(opts...)
	fullKey := l.prefixedKey(key)

	// Build redsync options
	rsOpts := []redsync.Option{
		redsync.WithExpiry(options.TTL),
		redsync.WithRetryDelay(options.RetryDelay),
	}

	// Configure retry behavior
	if options.RetryCount >= 0 {
		rsOpts = append(rsOpts, redsync.WithTries(options.RetryCount+1))
	} else {
		// Infinite retries: use a very large number
		// Actual cancellation is handled by context
		rsOpts = append(rsOpts, redsync.WithTries(1000000))
	}

	mutex := l.redsync.NewMutex(fullKey, rsOpts...)

	return &RedisMutex{
		key:     fullKey,
		mutex:   mutex,
		options: options,
	}
}

// Close releases resources. The Redis client is not closed as it may be shared.
func (l *RedisLocker) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.closed = true
	return nil
}

func (l *RedisLocker) prefixedKey(key string) string {
	if l.prefix == "" {
		return key
	}
	return l.prefix + ":" + key
}

// RedisMutex implements Mutex using Redis/redsync.
type RedisMutex struct {
	key     string
	mutex   *redsync.Mutex
	options *Options
}

// Lock acquires the lock, blocking until available or ctx is cancelled.
func (m *RedisMutex) Lock(ctx context.Context) error {
	err := m.mutex.LockContext(ctx)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if strings.Contains(err.Error(), "lock already taken") {
			return ErrLockAcquireFailed
		}
		klog.V(4).Infof("failed to acquire redis lock %s: %v", m.key, err)
		return fmt.Errorf("acquire lock %s: %w", m.key, err)
	}
	return nil
}

// TryLock attempts to acquire the lock without blocking.
func (m *RedisMutex) TryLock(ctx context.Context) error {
	err := m.mutex.TryLockContext(ctx)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if strings.Contains(err.Error(), "lock already taken") {
			return ErrLockAcquireFailed
		}
		return fmt.Errorf("try lock %s: %w", m.key, err)
	}
	return nil
}

// Unlock releases the lock.
func (m *RedisMutex) Unlock(ctx context.Context) error {
	ok, err := m.mutex.UnlockContext(ctx)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("unlock %s: %w", m.key, err)
	}
	if !ok {
		return ErrLockNotHeld
	}
	return nil
}

// Extend extends the lock's TTL.
func (m *RedisMutex) Extend(ctx context.Context) error {
	ok, err := m.mutex.ExtendContext(ctx)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("extend lock %s: %w", m.key, err)
	}
	if !ok {
		return ErrLockNotHeld
	}
	return nil
}

// Key returns the key of this mutex.
func (m *RedisMutex) Key() string {
	return m.key
}

// degradedMutex is returned when the locker is closed or unavailable.
type degradedMutex struct {
	key string
	err error
}

func (m *degradedMutex) Lock(ctx context.Context) error    { return m.err }
func (m *degradedMutex) TryLock(ctx context.Context) error { return m.err }
func (m *degradedMutex) Unlock(ctx context.Context) error  { return m.err }
func (m *degradedMutex) Extend(ctx context.Context) error  { return m.err }
func (m *degradedMutex) Key() string                       { return m.key }

// Compile-time interface checks
var (
	_ Locker = (*RedisLocker)(nil)
	_ Mutex  = (*RedisMutex)(nil)
	_ Mutex  = (*degradedMutex)(nil)
)

// ---- Helper Functions ----

// NewRedisLockerFromConfig creates a RedisLocker from a Config.
// This is a convenience function that extracts the Redis client from the config.
func NewRedisLockerFromConfig(cfg Config) (*RedisLocker, error) {
	client, ok := cfg.RedisClient.(*redis.Client)
	if !ok || client == nil {
		return nil, fmt.Errorf("invalid redis client in config")
	}
	return NewRedisLocker(client, cfg.Prefix)
}

// AutoExtend starts a goroutine that periodically extends the lock's TTL.
// It stops when ctx is cancelled or the returned cancel function is called.
// This is useful for long-running operations that need to hold the lock.
func AutoExtend(ctx context.Context, mutex Mutex, interval time.Duration) (cancel func()) {
	if interval <= 0 {
		interval = DefaultTTL / 3
	}

	extendCtx, cancelFunc := context.WithCancel(ctx)

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-extendCtx.Done():
				return
			case <-ticker.C:
				if err := mutex.Extend(extendCtx); err != nil {
					if extendCtx.Err() != nil {
						return
					}
					klog.Warningf("failed to extend lock %s: %v", mutex.Key(), err)
					return
				}
			}
		}
	}()

	return cancelFunc
}
