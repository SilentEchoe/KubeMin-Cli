package messaging

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/segmentio/kafka-go"
	"k8s.io/klog/v2"
)

// KafkaConfig holds Kafka-specific configuration options.
type KafkaConfig struct {
	Brokers         []string
	Topic           string
	GroupID         string
	AutoOffsetReset string // "earliest" or "latest"
}

// KafkaQueue implements Queue using Kafka Consumer Groups.
// It uses kafka-go library for both producing and consuming messages.
type KafkaQueue struct {
	cfg    KafkaConfig
	writer *kafka.Writer

	// reader is lazily initialized when EnsureGroup is called
	mu     sync.RWMutex
	reader *kafka.Reader

	// pendingMessages tracks messages that have been read but not yet acknowledged.
	// Key is the message ID (partition:offset), value is the kafka message for commit.
	pendingMu       sync.Mutex
	pendingMessages map[string]kafka.Message
}

// NewKafkaQueue creates a new KafkaQueue with the given configuration.
// The writer is initialized immediately, but the reader is created lazily
// when EnsureGroup is called to set up the consumer group.
func NewKafkaQueue(cfg KafkaConfig) (*KafkaQueue, error) {
	if len(cfg.Brokers) == 0 {
		return nil, errors.New("kafka brokers cannot be empty")
	}
	if cfg.Topic == "" {
		return nil, errors.New("kafka topic cannot be empty")
	}
	if cfg.GroupID == "" {
		cfg.GroupID = "kubemin-workflow-workers"
	}
	if cfg.AutoOffsetReset == "" {
		cfg.AutoOffsetReset = "earliest"
	}

	writer := &kafka.Writer{
		Addr:         kafka.TCP(cfg.Brokers...),
		Topic:        cfg.Topic,
		Balancer:     &kafka.LeastBytes{},
		BatchSize:    1, // Send immediately for low latency
		BatchTimeout: 10 * time.Millisecond,
		RequiredAcks: kafka.RequireOne,
	}

	return &KafkaQueue{
		cfg:             cfg,
		writer:          writer,
		pendingMessages: make(map[string]kafka.Message),
	}, nil
}

// EnsureGroup ensures the consumer group exists and initializes the reader.
// In Kafka, consumer groups are created automatically when a consumer joins,
// so this method primarily initializes the reader with the specified group.
func (k *KafkaQueue) EnsureGroup(ctx context.Context, group string) error {
	k.mu.Lock()
	defer k.mu.Unlock()

	// If reader already exists, nothing to do
	if k.reader != nil {
		return nil
	}

	// Determine start offset based on configuration
	startOffset := kafka.FirstOffset
	if k.cfg.AutoOffsetReset == "latest" {
		startOffset = kafka.LastOffset
	}

	// Create the reader with consumer group
	k.reader = kafka.NewReader(kafka.ReaderConfig{
		Brokers:        k.cfg.Brokers,
		Topic:          k.cfg.Topic,
		GroupID:        k.cfg.GroupID,
		StartOffset:    startOffset,
		MinBytes:       1,
		MaxBytes:       10e6, // 10MB
		MaxWait:        500 * time.Millisecond,
		CommitInterval: 0, // Disable auto-commit, we commit manually on Ack
	})

	klog.V(2).Infof("kafka reader initialized for topic=%s group=%s", k.cfg.Topic, k.cfg.GroupID)
	return nil
}

// Enqueue pushes a payload to the Kafka topic and returns the message offset as ID.
func (k *KafkaQueue) Enqueue(ctx context.Context, payload []byte) (string, error) {
	msg := kafka.Message{
		Value: payload,
	}

	if err := k.writer.WriteMessages(ctx, msg); err != nil {
		return "", err
	}

	// Kafka doesn't return the offset on write in kafka-go's WriteMessages.
	// We return a placeholder ID. The actual message ID will be assigned
	// when the message is read.
	return "pending", nil
}

// ReadGroup reads messages for a consumer in a group.
// The consumer parameter is ignored as Kafka manages consumers within groups automatically.
func (k *KafkaQueue) ReadGroup(ctx context.Context, group, consumer string, count int, block time.Duration) ([]Message, error) {
	k.mu.RLock()
	reader := k.reader
	k.mu.RUnlock()

	if reader == nil {
		return nil, errors.New("kafka reader not initialized, call EnsureGroup first")
	}

	// Create a context with timeout for blocking
	readCtx, cancel := context.WithTimeout(ctx, block)
	defer cancel()

	var messages []Message
	for i := 0; i < count; i++ {
		msg, err := reader.FetchMessage(readCtx)
		if err != nil {
			if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
				// Timeout or cancellation, return what we have
				break
			}
			// For other errors, if we have some messages, return them
			if len(messages) > 0 {
				klog.V(4).Infof("kafka read partial: got %d messages before error: %v", len(messages), err)
				break
			}
			return nil, err
		}

		// Generate a unique ID from partition and offset
		msgID := k.messageID(msg)

		// Store message for later acknowledgment
		k.pendingMu.Lock()
		k.pendingMessages[msgID] = msg
		k.pendingMu.Unlock()

		messages = append(messages, Message{
			ID:      msgID,
			Payload: msg.Value,
		})
	}

	return messages, nil
}

// Ack acknowledges processed messages by committing their offsets.
func (k *KafkaQueue) Ack(ctx context.Context, group string, ids ...string) error {
	if len(ids) == 0 {
		return nil
	}

	k.mu.RLock()
	reader := k.reader
	k.mu.RUnlock()

	if reader == nil {
		return errors.New("kafka reader not initialized")
	}

	k.pendingMu.Lock()
	defer k.pendingMu.Unlock()

	for _, id := range ids {
		msg, ok := k.pendingMessages[id]
		if !ok {
			klog.V(4).Infof("kafka ack: message %s not found in pending, may be already acked", id)
			continue
		}

		if err := reader.CommitMessages(ctx, msg); err != nil {
			return err
		}

		delete(k.pendingMessages, id)
	}

	return nil
}

// AutoClaim returns empty as Kafka relies on its native rebalance mechanism.
// When a consumer fails or leaves the group, Kafka automatically rebalances
// and assigns the partitions to other consumers in the group.
func (k *KafkaQueue) AutoClaim(ctx context.Context, group, consumer string, minIdle time.Duration, count int) ([]Message, error) {
	// Kafka handles message redelivery through its rebalance mechanism.
	// When a consumer fails to commit offsets before leaving/crashing,
	// the messages will be redelivered to another consumer after rebalance.
	return nil, nil
}

// Close releases the Kafka writer and reader resources.
func (k *KafkaQueue) Close(ctx context.Context) error {
	var errs []error

	if k.writer != nil {
		if err := k.writer.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	k.mu.Lock()
	if k.reader != nil {
		if err := k.reader.Close(); err != nil {
			errs = append(errs, err)
		}
		k.reader = nil
	}
	k.mu.Unlock()

	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

// Stats returns the current lag statistics for the consumer group.
// backlog: total number of messages in the topic (approximate)
// pending: number of messages read but not yet acknowledged
func (k *KafkaQueue) Stats(ctx context.Context, group string) (backlog int64, pending int64, err error) {
	k.mu.RLock()
	reader := k.reader
	k.mu.RUnlock()

	if reader == nil {
		return 0, 0, nil
	}

	// Get reader stats
	stats := reader.Stats()
	backlog = stats.Lag

	// Count pending messages
	k.pendingMu.Lock()
	pending = int64(len(k.pendingMessages))
	k.pendingMu.Unlock()

	return backlog, pending, nil
}

// messageID generates a unique message ID from partition and offset.
func (k *KafkaQueue) messageID(msg kafka.Message) string {
	return fmt.Sprintf("%d:%d", msg.Partition, msg.Offset)
}
