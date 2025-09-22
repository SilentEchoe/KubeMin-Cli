package clients

import (
    "context"
    "fmt"
    "sync"
    "time"

    "KubeMin-Cli/pkg/apiserver/config"
    "github.com/redis/go-redis/v9"
)

var (
    redisMu   sync.Mutex
    redisOnce sync.Once // reserved if we later need one-time wiring
    rcli      *redis.Client
)

// EnsureRedis returns a process-wide redis client built from cfg if not yet initialized.
// Subsequent calls reuse the same client instance.
func EnsureRedis(cfg config.RedisCacheConfig) (*redis.Client, error) {
    if rcli != nil {
        return rcli, nil
    }
    redisMu.Lock()
    defer redisMu.Unlock()
    if rcli != nil {
        return rcli, nil
    }
    addr := fmt.Sprintf("%s:%d", cfg.CacheHost, cfg.CacheProt)
    cli := redis.NewClient(&redis.Options{
        Addr:     addr,
        Username: cfg.UserName,
        Password: cfg.Password,
        DB:       int(cfg.CacheDB),
    })
    ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
    defer cancel()
    if err := cli.Ping(ctx).Err(); err != nil {
        _ = cli.Close()
        return nil, err
    }
    rcli = cli
    return rcli, nil
}

// GetRedis returns the initialized redis client or nil if not initialized.
func GetRedis() *redis.Client { return rcli }

