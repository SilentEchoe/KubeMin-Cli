package queue

import (
    "context"
    "time"
)

// Message represents a queue message with its ID and raw payload.
type Message struct {
    ID      string
    Payload []byte
}

// Queue abstracts a work queue with stream semantics (enqueue, group read, ack).
type Queue interface {
    // EnsureGroup ensures the underlying stream and consumer group exist.
    EnsureGroup(ctx context.Context) error
    // Enqueue pushes a payload to the stream and returns the message ID.
    Enqueue(ctx context.Context, payload []byte) (string, error)
    // ReadGroup reads messages for a consumer in a group.
    ReadGroup(ctx context.Context, group, consumer string, count int, block time.Duration) ([]Message, error)
    // Ack acknowledges a processed message by ID.
    Ack(ctx context.Context, group string, ids ...string) error
}

