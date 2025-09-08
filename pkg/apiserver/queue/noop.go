package queue

import (
    "context"
    "time"
)

// NoopQueue provides a minimal in-memory queue-like behavior for local mode.
type NoopQueue struct{}

func (n *NoopQueue) EnsureGroup(ctx context.Context) error { return nil }
func (n *NoopQueue) Enqueue(ctx context.Context, payload []byte) (string, error) { return "", nil }
func (n *NoopQueue) ReadGroup(ctx context.Context, group, consumer string, count int, block time.Duration) ([]Message, error) {
    // No messages in noop mode.
    select {
    case <-ctx.Done():
        return nil, ctx.Err()
    case <-time.After(block):
        return nil, nil
    }
}
func (n *NoopQueue) Ack(ctx context.Context, group string, ids ...string) error { return nil }

