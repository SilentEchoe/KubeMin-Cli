// Forked from github.com/k8sgpt-ai/k8sgpt
// Some parts of this file have been modified to make it functional in Zadig

package cache

type ICache interface {
	Store(key string, data string) error
	Load(key string) (string, error)
	List() ([]string, error)
	Exists(key string) bool
	IsCacheDisabled() bool
}

type CacheType string

var (
	CacheTypeRedis CacheType = "redis"
	CacheTypeMem   CacheType = "memory"
)

func New(noCache bool, cacheType CacheType) ICache {
	switch cacheType {
	case CacheTypeMem:
		return NewMemCache(noCache)
	case CacheTypeRedis:
		// Use global client if available; otherwise fallback to memory cache
		return NewRedisICacheWithClient(redisClient, noCache)

	default:
		return NewMemCache(noCache)
	}
}
