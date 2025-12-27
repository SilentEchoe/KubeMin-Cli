package cache

import (
	"runtime/debug"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"k8s.io/klog/v2"
)

type item struct {
	value     string
	expiresAt time.Time
}

type MemCache struct {
	noCache bool
	items   map[string]*item
	mu      sync.Mutex
	ttl     time.Duration
}

func (m *MemCache) Store(key string, data string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	// Upsert and refresh expiry
	expiresAt := time.Time{}
	if m.ttl > 0 {
		expiresAt = time.Now().Add(m.ttl)
	}
	m.items[key] = &item{
		value:     data,
		expiresAt: expiresAt,
	}
	return nil
}

func (m *MemCache) Load(key string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var result string
	if v, ok := m.items[key]; ok {
		// Honor expiration
		if !v.expired(time.Now()) {
			result = v.value
		} else {
			delete(m.items, key)
		}
	}
	return result, nil
}

func (m *MemCache) List() ([]string, error) {
	var ret []string
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, v := range m.items {
		ret = append(ret, v.value)
	}
	return ret, nil
}

func (m *MemCache) Exists(key string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if v, ok := m.items[key]; ok {
		if !v.expired(time.Now()) {
			return true
		}
		delete(m.items, key)
	}
	return false
}

func (m *MemCache) IsCacheDisabled() bool {
	return m.noCache
}

// GetRedisClient returns nil for MemCache since it doesn't use Redis.
// This method satisfies the Cache interface for dependency injection.
func (m *MemCache) GetRedisClient() *redis.Client {
	return nil
}

// expired 确定是否过期（ttl<=0 表示不过期）
func (i *item) expired(now time.Time) bool {
	if i.expiresAt.IsZero() {
		return false
	}
	return now.After(i.expiresAt)
}

func NewMemCache(noCache bool) Cache {
	c := &MemCache{
		noCache: noCache,
		items:   make(map[string]*item),
		ttl:     24 * time.Hour, // 默认设置过期时间为1天
	}
	go func() {
		defer func() {
			if err := recover(); err != nil {
				klog.Errorf("memcache cleaner panic: %v", err)
				debug.PrintStack()
			}
		}()

		t := time.NewTicker(time.Second)
		defer t.Stop()
		for {
			select {
			case <-t.C:
				c.mu.Lock()
				for k, v := range c.items {
					if v.expired(time.Now()) {
						delete(c.items, k)
					}
				}
				c.mu.Unlock()
			}
		}
	}()
	return c
}
