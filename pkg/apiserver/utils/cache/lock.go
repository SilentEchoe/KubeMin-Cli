package cache

import (
	"KubeMin-Cli/pkg/apiserver/config"
	"github.com/go-redsync/redsync/v4"
	goredis "github.com/go-redsync/redsync/v4/redis/goredis/v9"
	"k8s.io/klog/v2"
	"strings"
	"time"
)

var resync *redsync.Redsync

func init() {
	resync = redsync.New(goredis.NewPool(NewRedisCache(config.CacheDB).redisClient))
}

type RedisLock struct {
	key   string
	mutex *redsync.Mutex
}

func NewRedisLock(key string) *RedisLock {
	return &RedisLock{
		key:   key,
		mutex: resync.NewMutex(key, redsync.WithRetryDelay(time.Millisecond*500)),
	}
}

func NewRedisLockWithExpiry(key string, expiry time.Duration) *RedisLock {
	return &RedisLock{
		mutex: resync.NewMutex(key, redsync.WithRetryDelay(time.Millisecond*500), redsync.WithExpiry(expiry)),
	}
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
