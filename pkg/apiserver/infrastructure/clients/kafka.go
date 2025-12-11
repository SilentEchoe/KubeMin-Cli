package clients

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/segmentio/kafka-go"
	"k8s.io/klog/v2"
)

var (
	kafkaMu     sync.Mutex
	kafkaDialer *kafka.Dialer
	kafkaConns  map[string]*kafka.Conn // broker address -> connection
)

func init() {
	kafkaConns = make(map[string]*kafka.Conn)
}

// KafkaConfig holds the configuration for Kafka client initialization.
type KafkaConfig struct {
	Brokers []string
}

// EnsureKafka validates the Kafka brokers connectivity and returns the dialer.
// It performs a health check by connecting to one of the brokers.
// The connection is cached for reuse.
func EnsureKafka(cfg KafkaConfig) (*kafka.Dialer, error) {
	if kafkaDialer != nil {
		return kafkaDialer, nil
	}
	kafkaMu.Lock()
	defer kafkaMu.Unlock()
	if kafkaDialer != nil {
		return kafkaDialer, nil
	}

	if len(cfg.Brokers) == 0 {
		return nil, fmt.Errorf("kafka brokers cannot be empty")
	}

	// Create a dialer with reasonable defaults
	dialer := &kafka.Dialer{
		Timeout:   10 * time.Second,
		DualStack: true,
	}

	// Validate connectivity by connecting to the first available broker
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var lastErr error
	for _, broker := range cfg.Brokers {
		conn, err := dialer.DialContext(ctx, "tcp", broker)
		if err != nil {
			lastErr = err
			klog.V(4).Infof("failed to connect to kafka broker %s: %v", broker, err)
			continue
		}

		// Successfully connected, cache the connection
		kafkaConns[broker] = conn
		kafkaDialer = dialer
		klog.V(2).Infof("kafka dialer initialized, connected to broker: %s", broker)
		return dialer, nil
	}

	return nil, fmt.Errorf("failed to connect to any kafka broker: %w", lastErr)
}

// GetKafkaDialer returns the initialized Kafka dialer or nil if not initialized.
func GetKafkaDialer() *kafka.Dialer {
	return kafkaDialer
}

// CloseKafkaConnections closes all cached Kafka connections.
// This should be called during graceful shutdown.
func CloseKafkaConnections() {
	kafkaMu.Lock()
	defer kafkaMu.Unlock()

	for addr, conn := range kafkaConns {
		if err := conn.Close(); err != nil {
			klog.Warningf("failed to close kafka connection to %s: %v", addr, err)
		}
	}
	kafkaConns = make(map[string]*kafka.Conn)
	kafkaDialer = nil
}

// CheckKafkaHealth performs a health check on the Kafka cluster
// by attempting to fetch cluster metadata.
func CheckKafkaHealth(ctx context.Context, brokers []string) error {
	if len(brokers) == 0 {
		return fmt.Errorf("no brokers configured")
	}

	dialer := &kafka.Dialer{
		Timeout: 5 * time.Second,
	}

	for _, broker := range brokers {
		conn, err := dialer.DialContext(ctx, "tcp", broker)
		if err != nil {
			continue
		}
		defer conn.Close()

		// Try to get cluster controller to verify connectivity
		_, err = conn.Controller()
		if err != nil {
			continue
		}
		return nil // Successfully connected
	}

	return fmt.Errorf("unable to connect to any kafka broker")
}
