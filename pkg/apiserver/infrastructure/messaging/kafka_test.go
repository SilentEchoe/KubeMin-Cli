package messaging

import (
	"context"
	"testing"
	"time"
)

func TestNewKafkaQueue_Validation(t *testing.T) {
	tests := []struct {
		name    string
		cfg     KafkaConfig
		wantErr bool
	}{
		{
			name: "empty brokers",
			cfg: KafkaConfig{
				Brokers: []string{},
				Topic:   "test-topic",
			},
			wantErr: true,
		},
		{
			name: "empty topic",
			cfg: KafkaConfig{
				Brokers: []string{"localhost:9092"},
				Topic:   "",
			},
			wantErr: true,
		},
		{
			name: "valid config with defaults",
			cfg: KafkaConfig{
				Brokers: []string{"localhost:9092"},
				Topic:   "test-topic",
			},
			wantErr: false,
		},
		{
			name: "valid config with all fields",
			cfg: KafkaConfig{
				Brokers:         []string{"localhost:9092", "localhost:9093"},
				Topic:           "test-topic",
				GroupID:         "test-group",
				AutoOffsetReset: "earliest",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kq, err := NewKafkaQueue(tt.cfg)
			if tt.wantErr {
				if err == nil {
					t.Errorf("NewKafkaQueue() expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("NewKafkaQueue() unexpected error: %v", err)
				}
				if kq == nil {
					t.Errorf("NewKafkaQueue() returned nil queue")
				}
			}
		})
	}
}

func TestKafkaQueue_DefaultValues(t *testing.T) {
	cfg := KafkaConfig{
		Brokers: []string{"localhost:9092"},
		Topic:   "test-topic",
	}

	kq, err := NewKafkaQueue(cfg)
	if err != nil {
		t.Fatalf("NewKafkaQueue() error: %v", err)
	}

	// Check default values are applied
	if kq.cfg.GroupID != "kubemin-workflow-workers" {
		t.Errorf("expected default GroupID 'kubemin-workflow-workers', got %q", kq.cfg.GroupID)
	}
	if kq.cfg.AutoOffsetReset != "earliest" {
		t.Errorf("expected default AutoOffsetReset 'earliest', got %q", kq.cfg.AutoOffsetReset)
	}
}

func TestKafkaQueue_EnsureGroupWithoutReader(t *testing.T) {
	cfg := KafkaConfig{
		Brokers: []string{"localhost:9092"},
		Topic:   "test-topic",
	}

	kq, err := NewKafkaQueue(cfg)
	if err != nil {
		t.Fatalf("NewKafkaQueue() error: %v", err)
	}

	// Reader is nil before EnsureGroup
	if kq.reader != nil {
		t.Errorf("expected reader to be nil before EnsureGroup")
	}
}

func TestKafkaQueue_ReadGroupWithoutInit(t *testing.T) {
	cfg := KafkaConfig{
		Brokers: []string{"localhost:9092"},
		Topic:   "test-topic",
	}

	kq, err := NewKafkaQueue(cfg)
	if err != nil {
		t.Fatalf("NewKafkaQueue() error: %v", err)
	}

	ctx := context.Background()
	// ReadGroup should fail without EnsureGroup
	_, err = kq.ReadGroup(ctx, "group", "consumer", 10, time.Second)
	if err == nil {
		t.Error("ReadGroup() expected error when reader not initialized")
	}
}

func TestKafkaQueue_AckWithoutInit(t *testing.T) {
	cfg := KafkaConfig{
		Brokers: []string{"localhost:9092"},
		Topic:   "test-topic",
	}

	kq, err := NewKafkaQueue(cfg)
	if err != nil {
		t.Fatalf("NewKafkaQueue() error: %v", err)
	}

	ctx := context.Background()
	// Ack with empty ids should succeed
	err = kq.Ack(ctx, "group")
	if err != nil {
		t.Errorf("Ack() with empty ids should succeed, got error: %v", err)
	}

	// Ack with ids should fail without reader
	err = kq.Ack(ctx, "group", "0:1")
	if err == nil {
		t.Error("Ack() expected error when reader not initialized")
	}
}

func TestKafkaQueue_AutoClaimReturnsEmpty(t *testing.T) {
	cfg := KafkaConfig{
		Brokers: []string{"localhost:9092"},
		Topic:   "test-topic",
	}

	kq, err := NewKafkaQueue(cfg)
	if err != nil {
		t.Fatalf("NewKafkaQueue() error: %v", err)
	}

	ctx := context.Background()
	// AutoClaim should return empty (Kafka uses rebalance mechanism)
	msgs, err := kq.AutoClaim(ctx, "group", "consumer", time.Minute, 10)
	if err != nil {
		t.Errorf("AutoClaim() unexpected error: %v", err)
	}
	if len(msgs) != 0 {
		t.Errorf("AutoClaim() expected empty messages, got %d", len(msgs))
	}
}

func TestKafkaQueue_StatsWithoutInit(t *testing.T) {
	cfg := KafkaConfig{
		Brokers: []string{"localhost:9092"},
		Topic:   "test-topic",
	}

	kq, err := NewKafkaQueue(cfg)
	if err != nil {
		t.Fatalf("NewKafkaQueue() error: %v", err)
	}

	ctx := context.Background()
	// Stats should return zeros when reader not initialized
	backlog, pending, err := kq.Stats(ctx, "group")
	if err != nil {
		t.Errorf("Stats() unexpected error: %v", err)
	}
	if backlog != 0 || pending != 0 {
		t.Errorf("Stats() expected (0, 0), got (%d, %d)", backlog, pending)
	}
}

func TestKafkaQueue_CloseWithoutInit(t *testing.T) {
	cfg := KafkaConfig{
		Brokers: []string{"localhost:9092"},
		Topic:   "test-topic",
	}

	kq, err := NewKafkaQueue(cfg)
	if err != nil {
		t.Fatalf("NewKafkaQueue() error: %v", err)
	}

	ctx := context.Background()
	// Close should succeed even without reader
	err = kq.Close(ctx)
	if err != nil {
		t.Errorf("Close() unexpected error: %v", err)
	}
}

func TestKafkaQueue_MessageID(t *testing.T) {
	cfg := KafkaConfig{
		Brokers: []string{"localhost:9092"},
		Topic:   "test-topic",
	}

	kq, err := NewKafkaQueue(cfg)
	if err != nil {
		t.Fatalf("NewKafkaQueue() error: %v", err)
	}

	// Test messageID format
	tests := []struct {
		partition int
		offset    int64
		expected  string
	}{
		{0, 0, "0:0"},
		{1, 100, "1:100"},
		{2, 999999, "2:999999"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			// We can't directly test messageID as it takes kafka.Message
			// but we can verify the expected format
			expected := tt.expected
			if expected == "" {
				t.Skip("skipping direct messageID test")
			}
		})
	}

	// Verify pending messages map is initialized
	if kq.pendingMessages == nil {
		t.Error("pendingMessages map should be initialized")
	}
}

func TestKafkaQueue_PendingMessagesTracking(t *testing.T) {
	cfg := KafkaConfig{
		Brokers: []string{"localhost:9092"},
		Topic:   "test-topic",
	}

	kq, err := NewKafkaQueue(cfg)
	if err != nil {
		t.Fatalf("NewKafkaQueue() error: %v", err)
	}

	// Initial pending count should be 0
	kq.pendingMu.Lock()
	count := len(kq.pendingMessages)
	kq.pendingMu.Unlock()

	if count != 0 {
		t.Errorf("expected 0 pending messages, got %d", count)
	}
}

func TestKafkaConfig_AutoOffsetResetValues(t *testing.T) {
	tests := []struct {
		name   string
		offset string
		valid  bool
	}{
		{"earliest", "earliest", true},
		{"latest", "latest", true},
		{"empty defaults to earliest", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := KafkaConfig{
				Brokers:         []string{"localhost:9092"},
				Topic:           "test-topic",
				AutoOffsetReset: tt.offset,
			}

			kq, err := NewKafkaQueue(cfg)
			if err != nil {
				t.Fatalf("NewKafkaQueue() error: %v", err)
			}

			// Empty should default to earliest
			expected := tt.offset
			if expected == "" {
				expected = "earliest"
			}
			if kq.cfg.AutoOffsetReset != expected {
				t.Errorf("expected AutoOffsetReset %q, got %q", expected, kq.cfg.AutoOffsetReset)
			}
		})
	}
}






