// Forked from github.com/k8sgpt-ai/k8sgpt
// Some parts of this file have been modified to make it functional in Zadig

package cache

import "github.com/redis/go-redis/v9"

// Cache defines the interface for cache operations.
// Implementations include MemCache (in-memory) and RedisCache (Redis-backed).
type Cache interface {
	Store(key string, data string) error
	Load(key string) (string, error)
	List() ([]string, error)
	Exists(key string) bool
	IsCacheDisabled() bool
	// GetRedisClient returns the underlying Redis client if available.
	// Returns nil for non-Redis implementations (e.g., MemCache).
	// This method enables dependency injection for components that need
	// direct Redis access (e.g., distributed locks, cancellation signals).
	GetRedisClient() *redis.Client
}

type CacheType string

var (
	CacheTypeRedis CacheType = "redis"
	CacheTypeMem   CacheType = "memory"
)

func New(noCache bool, cacheType CacheType) Cache {
	switch cacheType {
	case CacheTypeMem:
		return NewMemCache(noCache)
	case CacheTypeRedis:
		// Use global client if available; otherwise fallback to memory cache
		return NewRedisCacheWithClient(redisClient, noCache)

	default:
		return NewMemCache(noCache)
	}
}
