package cache

import (
	"fmt"
	"runtime/debug"
	"sync"
	"time"
)

type item struct {
	value   string
	expires int64
}

type MemCache struct {
	noCache bool
	items   map[string]*item
	mu      sync.Mutex
	expires int64
}

func (m *MemCache) Store(key string, data string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.items[key]; !ok {
		m.items[key] = &item{
			value:   data,
			expires: m.expires,
		}
	}
	return nil
}

func (m *MemCache) Load(key string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var result string
	if v, ok := m.items[key]; ok {
		result = v.value
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
	if _, ok := m.items[key]; ok {
		return true
	}
	return false
}

func (m *MemCache) IsCacheDisabled() bool {
	return m.noCache
}

// Expired 确定是否过期
func (i *item) Expired(time int64) bool {
	if i.expires == 0 {
		return true
	}
	return time > i.expires
}

func NewMemCache(noCache bool) ICache {
	c := &MemCache{
		noCache: noCache,
		items:   make(map[string]*item),
		expires: int64(time.Minute * 60 * 24), //默认设置过期时间为1天
	}
	go func() {
		defer func() {
			if err := recover(); err != nil {
				fmt.Println(err)
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
					if v.Expired(time.Now().UnixNano()) {
						delete(c.items, k)
					}
				}
				c.mu.Unlock()
			}
		}
	}()
	return c
}
