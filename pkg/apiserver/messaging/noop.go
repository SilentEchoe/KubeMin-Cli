package messaging

import "context"

// NoopBroker is a stub implementation that drops messages.
type NoopBroker struct{}

func (n *NoopBroker) Publish(ctx context.Context, topic string, payload []byte) error { return nil }
func (n *NoopBroker) Subscribe(ctx context.Context, topic string) (Subscription, error) {
	ch := make(chan []byte)
	close(ch)
	return &noopSub{out: ch}, nil
}
func (n *NoopBroker) Close(ctx context.Context) error { return nil }

type noopSub struct{ out chan []byte }

func (s *noopSub) C() <-chan []byte                      { return s.out }
func (s *noopSub) Err() <-chan error                     { return nil }
func (s *noopSub) Unsubscribe(ctx context.Context) error { return nil }
