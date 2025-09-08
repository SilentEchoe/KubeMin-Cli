package messaging

import (
	"context"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisBroker implements Broker using Redis Pub/Sub.
type RedisBroker struct {
	cli *redis.Client
}

func NewRedisBroker(addr, username, password string, db int) (*RedisBroker, error) {
	if addr == "" {
		return nil, errors.New("redis addr is required")
	}
	cli := redis.NewClient(&redis.Options{Addr: addr, Username: username, Password: password, DB: db})
	// ping with short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := cli.Ping(ctx).Err(); err != nil {
		return nil, err
	}
	return &RedisBroker{cli: cli}, nil
}

func (b *RedisBroker) Publish(ctx context.Context, topic string, payload []byte) error {
	return b.cli.Publish(ctx, topic, payload).Err()
}

func (b *RedisBroker) Subscribe(ctx context.Context, topic string) (Subscription, error) {
	sub := b.cli.Subscribe(ctx, topic)
	ch := sub.Channel() // receives *redis.Message
	out := make(chan []byte, 128)
	errCh := make(chan error, 1)

	go func() {
		defer close(out)
		for {
			select {
			case <-ctx.Done():
				return
			case msg, ok := <-ch:
				if !ok {
					return
				}
				if msg == nil {
					continue
				}
				out <- []byte(msg.Payload)
			}
		}
	}()

	return &redisSub{sub: sub, out: out, errCh: errCh}, nil
}

func (b *RedisBroker) Close(ctx context.Context) error {
	return b.cli.Close()
}

type redisSub struct {
	sub   *redis.PubSub
	out   chan []byte
	errCh chan error
}

func (s *redisSub) C() <-chan []byte                      { return s.out }
func (s *redisSub) Err() <-chan error                     { return s.errCh }
func (s *redisSub) Unsubscribe(ctx context.Context) error { return s.sub.Close() }
