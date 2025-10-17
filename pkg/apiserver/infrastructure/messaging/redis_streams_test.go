package messaging

import (
	"context"
	"testing"

	"github.com/redis/go-redis/v9"
)

// fakeRedis implements redisCommander for testing without a real Redis.
type fakeRedis struct{ closed bool }

func (f *fakeRedis) Ping(ctx context.Context) *redis.StatusCmd {
	cmd := redis.NewStatusCmd(ctx)
	cmd.SetVal("PONG")
	return cmd
}

func (f *fakeRedis) XGroupCreateMkStream(ctx context.Context, stream, group, start string) *redis.StatusCmd {
	cmd := redis.NewStatusCmd(ctx)
	cmd.SetVal("OK")
	return cmd
}

func (f *fakeRedis) XAdd(ctx context.Context, a *redis.XAddArgs) *redis.StringCmd {
	cmd := redis.NewStringCmd(ctx)
	cmd.SetVal("1-0")
	return cmd
}

func (f *fakeRedis) XReadGroup(ctx context.Context, a *redis.XReadGroupArgs) *redis.XStreamSliceCmd {
	var cmd redis.XStreamSliceCmd
	return &cmd
}

func (f *fakeRedis) XAck(ctx context.Context, stream, group string, ids ...string) *redis.IntCmd {
	cmd := redis.NewIntCmd(ctx)
	cmd.SetVal(int64(len(ids)))
	return cmd
}

func (f *fakeRedis) XAutoClaim(ctx context.Context, a *redis.XAutoClaimArgs) *redis.XAutoClaimCmd {
	var cmd redis.XAutoClaimCmd
	return &cmd
}

func (f *fakeRedis) XLen(ctx context.Context, stream string) *redis.IntCmd {
	return redis.NewIntCmd(ctx)
}

func (f *fakeRedis) XPending(ctx context.Context, stream, group string) *redis.XPendingCmd {
	var cmd redis.XPendingCmd
	return &cmd
}

func (f *fakeRedis) Close() error { f.closed = true; return nil }

func TestNewRedisStreamsWithClient_Validation(t *testing.T) {
	// nil client
	if _, err := NewRedisStreamsWithClient(nil, "k", 0); err == nil {
		t.Fatalf("expected error for nil client")
	}
	// empty key
	rcli := redis.NewClient(&redis.Options{Addr: "127.0.0.1:0"})
	if _, err := NewRedisStreamsWithClient(rcli, "", 0); err == nil {
		t.Fatalf("expected error for empty key")
	}
}

func TestRedisStreams_WithCommanderBasic(t *testing.T) {
	f := &fakeRedis{}
	rs, err := NewRedisStreamsWithClient(f, "test-stream", 1000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	ctx := context.Background()
	// EnsureGroup
	if err := rs.EnsureGroup(ctx, "g"); err != nil {
		t.Fatalf("EnsureGroup error: %v", err)
	}
	// Enqueue
	if id, err := rs.Enqueue(ctx, []byte("hello")); err != nil || id == "" {
		t.Fatalf("Enqueue err=%v id=%q", err, id)
	}
	// Ack (no ids)
	if err := rs.Ack(ctx, "g"); err != nil {
		t.Fatalf("Ack empty ids should be nil, got %v", err)
	}
	// Close
	if err := rs.Close(ctx); err != nil {
		t.Fatalf("Close error: %v", err)
	}
	if !f.closed {
		t.Fatalf("expected fake client to be closed")
	}
}
