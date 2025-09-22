package messaging

import (
	"context"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
	"k8s.io/klog/v2"
)

// RedisStreams implements Queue using Redis Streams + Consumer Groups.
// redisCommander abstracts the subset of go-redis client used by this queue.
// It allows tests to inject a fake implementation without a real Redis server.
type redisCommander interface {
	Ping(ctx context.Context) *redis.StatusCmd
	XGroupCreateMkStream(ctx context.Context, stream, group, start string) *redis.StatusCmd
	XAdd(ctx context.Context, a *redis.XAddArgs) *redis.StringCmd
	XReadGroup(ctx context.Context, a *redis.XReadGroupArgs) *redis.XStreamSliceCmd
	XAck(ctx context.Context, stream, group string, ids ...string) *redis.IntCmd
	XAutoClaim(ctx context.Context, a *redis.XAutoClaimArgs) *redis.XAutoClaimCmd
	XLen(ctx context.Context, stream string) *redis.IntCmd
	XPending(ctx context.Context, stream, group string) *redis.XPendingCmd
	Close() error
}

type RedisStreams struct {
	cli redisCommander
	key string
	// maxLen limits the stream length via XADD MAXLEN to avoid unbounded growth.
	// When <= 0, no trimming is applied.
	maxLen int64
}

// NewRedisStreamsWithClient builds a RedisStreams using a shared go-redis client (or any compatible implementation).
// The caller owns the client's lifecycle (creation and Close). This avoids creating multiple clients/pools per stream
// and is the recommended way to construct queues.
func NewRedisStreamsWithClient(cli redisCommander, key string, maxLen int64) (*RedisStreams, error) {
	if cli == nil {
		return nil, errors.New("redis client is nil")
	}
	if key == "" {
		return nil, errors.New("redis streams requires key")
	}
	return &RedisStreams{cli: cli, key: key, maxLen: maxLen}, nil
}

func (r *RedisStreams) EnsureGroup(ctx context.Context, group string) error {
	// XGroupCreateMkStream is idempotent if group exists; ignore BUSYGROUP error.
	return r.cli.XGroupCreateMkStream(ctx, r.key, group, "$").Err()
}

func (r *RedisStreams) Enqueue(ctx context.Context, payload []byte) (string, error) {
	args := &redis.XAddArgs{
		Stream: r.key,
		Values: map[string]interface{}{"p": payload},
	}
	if r.maxLen > 0 {
		args.MaxLen = r.maxLen
	}
	return r.cli.XAdd(ctx, args).Result()
}

func (r *RedisStreams) ReadGroup(ctx context.Context, group, consumer string, count int, block time.Duration) ([]Message, error) {
	res, err := r.cli.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    group,
		Consumer: consumer,
		Streams:  []string{r.key, ">"},
		Count:    int64(count),
		Block:    block,
		NoAck:    false,
	}).Result()
	if err != nil && !errors.Is(redis.Nil, err) {
		return nil, err
	}
	var mags []Message
	for _, s := range res {
		for _, m := range s.Messages {
			// expect single field "p"
			if raw, ok := m.Values["p"]; ok {
				switch v := raw.(type) {
				case string:
					mags = append(mags, Message{ID: m.ID, Payload: []byte(v)})
				case []byte:
					mags = append(mags, Message{ID: m.ID, Payload: v})
				default:
					klog.Warningf("redis stream malformed payload type id=%s type=%T", m.ID, v)
				}
			} else {
				klog.Warningf("redis stream message missing payload field 'p' id=%s", m.ID)
			}
		}
	}
	return mags, nil
}

func (r *RedisStreams) Ack(ctx context.Context, group string, ids ...string) error {
	if len(ids) == 0 {
		return nil
	}
	return r.cli.XAck(ctx, r.key, group, ids...).Err()
}

func (r *RedisStreams) AutoClaim(ctx context.Context, group, consumer string, minIdle time.Duration, count int) ([]Message, error) {
	// Use XAutoClaim to claim stale messages. Start from 0-0 each time for simplicity.
	start := "0-0"
	res, _, err := r.cli.XAutoClaim(ctx, &redis.XAutoClaimArgs{
		Stream:   r.key,
		Group:    group,
		Consumer: consumer,
		MinIdle:  minIdle,
		Start:    start,
		Count:    int64(count),
	}).Result()
	if err != nil && !errors.Is(err, redis.Nil) {
		return nil, err
	}
	var msgs []Message
	for _, m := range res {
		if raw, ok := m.Values["p"]; ok {
			switch v := raw.(type) {
			case string:
				msgs = append(msgs, Message{ID: m.ID, Payload: []byte(v)})
			case []byte:
				msgs = append(msgs, Message{ID: m.ID, Payload: v})
			default:
				klog.Warningf("redis stream malformed claimed payload type id=%s type=%T", m.ID, v)
			}
		} else {
			klog.Warningf("redis stream claimed message missing payload field 'p' id=%s", m.ID)
		}
	}
	return msgs, nil
}

func (r *RedisStreams) Close(ctx context.Context) error { return r.cli.Close() }

func (r *RedisStreams) Stats(ctx context.Context, group string) (int64, int64, error) {
	xl, err1 := r.cli.XLen(ctx, r.key).Result()
	xp, err2 := r.cli.XPending(ctx, r.key, group).Result()
	var cnt int64
	if err2 == nil && xp != nil {
		cnt = xp.Count
	}
	if err1 != nil {
		return 0, cnt, err1
	}
	if err2 != nil && err2 != redis.Nil {
		return xl, 0, err2
	}
	return xl, cnt, nil
}
