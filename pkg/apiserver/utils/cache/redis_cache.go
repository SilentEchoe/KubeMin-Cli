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

// Low-level helper preserved for lock initialization to access the global client.
type RedisCache struct {
	redisClient *redis.Client
}

// NewRedisCache returns a handle to the global client (used by lock.go).
func NewRedisCache(_ int) *RedisCache {
	return &RedisCache{redisClient: redisClient}
}

// RedisICache implements ICache using Redis as backend with a default TTL.
type RedisICache struct {
	cli       *redis.Client
	noCache   bool
	ttl       time.Duration
	keyPrefix string
}

const defaultTTL = 24 * time.Hour
const defaultKeyPrefix = "kubemin:cache:"

// NewRedisICacheWithClient creates an ICache backed by the provided client.
// If cli is nil, falls back to in-memory cache to remain functional.
func NewRedisICacheWithClient(cli *redis.Client, noCache bool) ICache {
	if cli == nil {
		return NewMemCache(noCache)
	}
	return &RedisICache{cli: cli, noCache: noCache, ttl: defaultTTL, keyPrefix: defaultKeyPrefix}
}

// NewRedisICache creates an ICache with custom ttl and prefix.
func NewRedisICache(cli *redis.Client, noCache bool, ttl time.Duration, prefix string) ICache {
	if cli == nil {
		return NewMemCache(noCache)
	}
	if ttl <= 0 {
		ttl = defaultTTL
	}
	if prefix == "" {
		prefix = defaultKeyPrefix
	}
	return &RedisICache{cli: cli, noCache: noCache, ttl: ttl, keyPrefix: prefix}
}

func (c *RedisICache) key(k string) string { return c.keyPrefix + k }

func (c *RedisICache) Store(key string, data string) error {
	return c.cli.Set(context.Background(), c.key(key), data, c.ttl).Err()
}

func (c *RedisICache) Load(key string) (string, error) {
	val, err := c.cli.Get(context.Background(), c.key(key)).Result()
	if err == redis.Nil {
		return "", nil
	}
	return val, err
}

// List returns the cached values for keys under the prefix.
// For performance, this uses SCAN; if keys are many, this can be expensive.
func (c *RedisICache) List() ([]string, error) {
	var (
		cursor uint64
		out    []string
	)
	pattern := c.keyPrefix + "*"
	ctx := context.Background()
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

func (c *RedisICache) Exists(key string) bool {
	n, err := c.cli.Exists(context.Background(), c.key(key)).Result()
	return err == nil && n == 1
}

func (c *RedisICache) IsCacheDisabled() bool { return c.noCache }
