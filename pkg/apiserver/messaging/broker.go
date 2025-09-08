package messaging

import (
    "context"
)

// Broker is a pub/sub abstraction to decouple the workflow dispatcher from the underlying messaging system.
// Implementations: Redis, Kafka (future), Noop.
type Broker interface {
    // Publish sends a message to a topic.
    Publish(ctx context.Context, topic string, payload []byte) error
    // Subscribe subscribes to a topic and returns a subscription.
    Subscribe(ctx context.Context, topic string) (Subscription, error)
    // Close releases any underlying resources.
    Close(ctx context.Context) error
}

// Subscription wraps a streaming subscription to a topic.
type Subscription interface {
    // C returns a channel that yields message payloads.
    C() <-chan []byte
    // Err returns a channel delivering terminal errors (optional; may be nil).
    Err() <-chan error
    // Unsubscribe cancels the subscription and frees resources.
    Unsubscribe(ctx context.Context) error
}

// MessageCodec defines encode/decode for strongly-typed messages.
// Workflow can use a shared codec for task dispatch messages.
type MessageCodec[T any] interface {
    Encode(T) ([]byte, error)
    Decode([]byte) (T, error)
}

