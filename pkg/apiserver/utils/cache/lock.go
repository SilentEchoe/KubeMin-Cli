package cache

import (
	"fmt"
	"strings"
	"time"

	"github.com/go-redsync/redsync/v4"
	goredis "github.com/go-redsync/redsync/v4/redis/goredis/v9"
	"github.com/redis/go-redis/v9"
	"k8s.io/klog/v2"
)

var resync *redsync.Redsync

func getRedsync() (*redsync.Redsync, error) {
	if resync != nil {
		return resync, nil
	}
	if redisClient == nil {
		return nil, fmt.Errorf("redis lock client not initialized: call cache.SetGlobalRedisClient first")
	}
	// Uses the global redis client set via SetGlobalRedisClient
	resync = redsync.New(goredis.NewPool(redisClient))
	return resync, nil
}

type RedisLock struct {
	key   string
	mutex *redsync.Mutex
}

func NewRedisLock(key string) (*RedisLock, error) {
	rs, err := getRedsync()
	if err != nil {
		return nil, err
	}
	return &RedisLock{
		key:   key,
		mutex: rs.NewMutex(key, redsync.WithRetryDelay(time.Millisecond*500)),
	}, nil
}

func NewRedisLockWithExpiry(key string, expiry time.Duration) (*RedisLock, error) {
	rs, err := getRedsync()
	if err != nil {
		return nil, err
	}
	return &RedisLock{
		mutex: rs.NewMutex(key, redsync.WithRetryDelay(time.Millisecond*500), redsync.WithExpiry(expiry)),
	}, nil
}

func (lock *RedisLock) Lock() error {
	err := lock.mutex.Lock()
	if err != nil {
		if !strings.Contains(err.Error(), "lock already taken") {
			klog.Errorf("failed to acquire redis lock: %s, err: %s", lock.key, err)
		}
	}
	return err
}

func (lock *RedisLock) TryLock() error {
	err := lock.mutex.TryLock()
	if err != nil {
		if strings.Contains(err.Error(), "lock already taken") {
			klog.Errorf("failed to try acquire redis lock: %s, err: %s", lock.key, err)
		}
	}
	return err
}

func (lock *RedisLock) Unlock() error {
	_, err := lock.mutex.Unlock()
	return err
}

// Lock is a minimal interface for distributed or local locks.
type Lock interface {
	Lock() error
	TryLock() error
	Unlock() error
}

// NoopLock implements Lock but performs no locking; used as a safe fallback.
type NoopLock struct{}

func (NoopLock) Lock() error    { return nil }
func (NoopLock) TryLock() error { return nil }
func (NoopLock) Unlock() error  { return nil }

// AcquireLock returns a distributed Redis lock when a global client is available,
// otherwise returns a NoopLock. The second return indicates whether the lock is distributed.
func AcquireLock(key string) (Lock, bool, error) {
	rs, err := getRedsync()
	if err != nil {
		// Not initialized: fall back to no-op lock
		return NoopLock{}, false, nil
	}
	return &RedisLock{key: key, mutex: rs.NewMutex(key, redsync.WithRetryDelay(time.Millisecond*500))}, true, nil
}

// AcquireLockWithExpiry is like AcquireLock but sets an expiry when using Redis.
func AcquireLockWithExpiry(key string, expiry time.Duration) (Lock, bool, error) {
	rs, err := getRedsync()
	if err != nil {
		return NoopLock{}, false, nil
	}
	return &RedisLock{key: key, mutex: rs.NewMutex(key, redsync.WithRetryDelay(time.Millisecond*500), redsync.WithExpiry(expiry))}, true, nil
}

// ---- Dependency Injection Variants ----
// These functions accept a Redis client parameter instead of using global variables,
// making them suitable for unit testing with mock clients.

// getRedsyncWithClient creates a redsync instance from a provided Redis client.
func getRedsyncWithClient(cli *redis.Client) (*redsync.Redsync, error) {
	if cli == nil {
		return nil, fmt.Errorf("redis client is nil")
	}
	return redsync.New(goredis.NewPool(cli)), nil
}

// NewRedisLockWithClient creates a RedisLock using the provided Redis client.
// This variant enables dependency injection for testing.
func NewRedisLockWithClient(cli *redis.Client, key string) (*RedisLock, error) {
	rs, err := getRedsyncWithClient(cli)
	if err != nil {
		return nil, err
	}
	return &RedisLock{
		key:   key,
		mutex: rs.NewMutex(key, redsync.WithRetryDelay(time.Millisecond*500)),
	}, nil
}

// NewRedisLockWithClientAndExpiry creates a RedisLock with expiry using the provided Redis client.
func NewRedisLockWithClientAndExpiry(cli *redis.Client, key string, expiry time.Duration) (*RedisLock, error) {
	rs, err := getRedsyncWithClient(cli)
	if err != nil {
		return nil, err
	}
	return &RedisLock{
		key:   key,
		mutex: rs.NewMutex(key, redsync.WithRetryDelay(time.Millisecond*500), redsync.WithExpiry(expiry)),
	}, nil
}

// AcquireLockWithClient returns a distributed Redis lock using the provided client.
// Returns NoopLock if client is nil. The second return indicates whether the lock is distributed.
func AcquireLockWithClient(cli *redis.Client, key string) (Lock, bool, error) {
	if cli == nil {
		return NoopLock{}, false, nil
	}
	rs, err := getRedsyncWithClient(cli)
	if err != nil {
		return NoopLock{}, false, nil
	}
	return &RedisLock{key: key, mutex: rs.NewMutex(key, redsync.WithRetryDelay(time.Millisecond*500))}, true, nil
}

// AcquireLockWithClientAndExpiry is like AcquireLockWithClient but sets an expiry.
func AcquireLockWithClientAndExpiry(cli *redis.Client, key string, expiry time.Duration) (Lock, bool, error) {
	if cli == nil {
		return NoopLock{}, false, nil
	}
	rs, err := getRedsyncWithClient(cli)
	if err != nil {
		return NoopLock{}, false, nil
	}
	return &RedisLock{key: key, mutex: rs.NewMutex(key, redsync.WithRetryDelay(time.Millisecond*500), redsync.WithExpiry(expiry))}, true, nil
}
