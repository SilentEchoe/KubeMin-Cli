package cache

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

// A process-wide redis client for cache/locks that can be set by the app.
var redisClient *redis.Client

// SetGlobalRedisClient configures the shared redis client for cache/locks.
func SetGlobalRedisClient(cli *redis.Client) {
	redisClient = cli
}

// GetGlobalRedisClient returns the shared redis client if set.
func GetGlobalRedisClient() *redis.Client { return redisClient }

// RedisCache implements Cache using Redis as backend with a default TTL.
type RedisCache struct {
	cli       *redis.Client
	noCache   bool
	ttl       time.Duration
	keyPrefix string
}

const defaultTTL = 24 * time.Hour
const defaultKeyPrefix = "kubemin:cache:"

// NewRedisCacheWithClient creates an Cache backed by the provided client.
// If cli is nil, falls back to in-memory cache to remain functional.
func NewRedisCacheWithClient(cli *redis.Client, noCache bool) Cache {
	if cli == nil {
		return NewMemCache(noCache)
	}
	return &RedisCache{cli: cli, noCache: noCache, ttl: defaultTTL, keyPrefix: defaultKeyPrefix}
}

// NewRedisCache creates an Cache with custom ttl and prefix.
func NewRedisCache(cli *redis.Client, noCache bool, ttl time.Duration, prefix string) Cache {
	if cli == nil {
		return NewMemCache(noCache)
	}
	if ttl <= 0 {
		ttl = defaultTTL
	}
	if prefix == "" {
		prefix = defaultKeyPrefix
	}
	return &RedisCache{cli: cli, noCache: noCache, ttl: ttl, keyPrefix: prefix}
}

// defaultOpTimeout is the default timeout for Redis cache operations.
const defaultOpTimeout = 5 * time.Second

func (c *RedisCache) key(k string) string { return c.keyPrefix + k }

func (c *RedisCache) Store(key string, data string) error {
	ctx, cancel := context.WithTimeout(context.Background(), defaultOpTimeout)
	defer cancel()
	return c.cli.Set(ctx, c.key(key), data, c.ttl).Err()
}

func (c *RedisCache) Load(key string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultOpTimeout)
	defer cancel()
	val, err := c.cli.Get(ctx, c.key(key)).Result()
	if err == redis.Nil {
		return "", nil
	}
	return val, err
}

// List returns the cached values for keys under the prefix.
// For performance, this uses SCAN; if keys are many, this can be expensive.
func (c *RedisCache) List() ([]string, error) {
	var (
		cursor uint64
		out    []string
	)
	pattern := c.keyPrefix + "*"
	// Use a longer timeout for List as it may involve multiple SCAN iterations
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	for {
		keys, next, err := c.cli.Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			return out, err
		}
		cursor = next
		if len(keys) > 0 {
			vals, err := c.cli.MGet(ctx, keys...).Result()
			if err != nil {
				return out, err
			}
			for _, v := range vals {
				if v == nil {
					continue
				}
				if s, ok := v.(string); ok {
					out = append(out, s)
				}
			}
		}
		if cursor == 0 {
			break
		}
	}
	return out, nil
}

func (c *RedisCache) Exists(key string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), defaultOpTimeout)
	defer cancel()
	n, err := c.cli.Exists(ctx, c.key(key)).Result()
	return err == nil && n == 1
}

func (c *RedisCache) IsCacheDisabled() bool { return c.noCache }

// GetRedisClient returns the underlying Redis client for dependency injection.
// This allows components like distributed locks and cancellation signals to
// obtain the Redis client through the Cache interface instead of global variables.
func (c *RedisCache) GetRedisClient() *redis.Client { return c.cli }
