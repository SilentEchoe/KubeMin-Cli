package queue

import (
    "context"
    "errors"
    "time"

    "github.com/redis/go-redis/v9"
)

// RedisStreams implements Queue using Redis Streams + Consumer Groups.
type RedisStreams struct {
    cli   *redis.Client
    key   string
}

// NewRedisStreams creates a Redis Streams queue on key.
func NewRedisStreams(addr, username, password string, db int, key string) (*RedisStreams, error) {
    if addr == "" || key == "" {
        return nil, errors.New("redis streams requires addr and key")
    }
    cli := redis.NewClient(&redis.Options{Addr: addr, Username: username, Password: password, DB: db})
    ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
    defer cancel()
    if err := cli.Ping(ctx).Err(); err != nil {
        return nil, err
    }
    return &RedisStreams{cli: cli, key: key}, nil
}

func (r *RedisStreams) EnsureGroup(ctx context.Context) error {
    // Create default group lazily in server where group name is known; here no-op.
    return nil
}

func (r *RedisStreams) ensureGroup(ctx context.Context, group string) error {
    // XGroupCreateMkStream is idempotent if group exists; ignore BUSYGROUP error.
    return r.cli.XGroupCreateMkStream(ctx, r.key, group, "$").Err()
}

func (r *RedisStreams) Enqueue(ctx context.Context, payload []byte) (string, error) {
    args := &redis.XAddArgs{
        Stream: r.key,
        Values: map[string]interface{}{"p": payload},
    }
    return r.cli.XAdd(ctx, args).Result()
}

func (r *RedisStreams) ReadGroup(ctx context.Context, group, consumer string, count int, block time.Duration) ([]Message, error) {
    _ = r.ensureGroup(ctx, group) // best-effort ensure
    res, err := r.cli.XReadGroup(ctx, &redis.XReadGroupArgs{
        Group:    group,
        Consumer: consumer,
        Streams:  []string{r.key, ">"},
        Count:    int64(count),
        Block:    block,
        NoAck:    false,
    }).Result()
    if err != nil && err != redis.Nil {
        return nil, err
    }
    var msgs []Message
    for _, s := range res {
        for _, m := range s.Messages {
            // expect single field "p"
            if raw, ok := m.Values["p"]; ok {
                switch v := raw.(type) {
                case string:
                    msgs = append(msgs, Message{ID: m.ID, Payload: []byte(v)})
                case []byte:
                    msgs = append(msgs, Message{ID: m.ID, Payload: v})
                default:
                    // ignore malformed
                }
            }
        }
    }
    return msgs, nil
}

func (r *RedisStreams) Ack(ctx context.Context, group string, ids ...string) error {
    if len(ids) == 0 { return nil }
    return r.cli.XAck(ctx, r.key, group, ids...).Err()
}

