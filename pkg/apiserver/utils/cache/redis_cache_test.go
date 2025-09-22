package cache

import (
    "context"
    "testing"
    "time"

    miniredis "github.com/alicebob/miniredis/v2"
    "github.com/redis/go-redis/v9"
    "github.com/stretchr/testify/require"
)

func newTestRedisClient(t *testing.T) (*miniredis.Miniredis, *redis.Client) {
    t.Helper()
    s, err := miniredis.Run()
    require.NoError(t, err)
    cli := redis.NewClient(&redis.Options{Addr: s.Addr()})
    return s, cli
}

func TestRedisICache_Basic(t *testing.T) {
    s, cli := newTestRedisClient(t)
    defer s.Close()

    c := NewRedisICacheWithClient(cli, false)
    require.NoError(t, c.Store("k", "v"))

    require.True(t, c.Exists("k"))
    got, err := c.Load("k")
    require.NoError(t, err)
    require.Equal(t, "v", got)
}

func TestRedisICache_List(t *testing.T) {
    s, cli := newTestRedisClient(t)
    defer s.Close()

    c := NewRedisICacheWithClient(cli, false)
    require.NoError(t, c.Store("k1", "v1"))
    require.NoError(t, c.Store("k2", "v2"))

    vals, err := c.List()
    require.NoError(t, err)
    require.Len(t, vals, 2)
    // Order is not guaranteed; verify set-wise
    m := map[string]bool{"v1": false, "v2": false}
    for _, v := range vals { m[v] = true }
    require.True(t, m["v1"] && m["v2"])
}

func TestRedisICache_NoCacheFlag(t *testing.T) {
    s, cli := newTestRedisClient(t)
    defer s.Close()
    c := NewRedisICacheWithClient(cli, true)
    require.True(t, c.IsCacheDisabled())
}

func TestRedisICache_CustomTTLAndPrefix(t *testing.T) {
    s, cli := newTestRedisClient(t)
    defer s.Close()
    ttl := 2 * time.Second
    prefix := "t:"
    c := NewRedisICache(cli, false, ttl, prefix).(*RedisICache)
    require.NoError(t, c.Store("kk", "vv"))

    // Key should exist with prefix
    ctx := context.Background()
    keys, _, err := cli.Scan(ctx, 0, prefix+"*", 100).Result()
    require.NoError(t, err)
    require.Len(t, keys, 1)
    require.Equal(t, prefix+"kk", keys[0])

    // TTL should be close to requested
    dur, err := cli.TTL(ctx, keys[0]).Result()
    require.NoError(t, err)
    require.Greater(t, int64(dur), int64(0))
    require.LessOrEqual(t, dur, ttl)
}
