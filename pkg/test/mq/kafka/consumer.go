package kafka

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/segmentio/kafka-go"
)

func StartConsumer(ctx context.Context) {
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:     []string{brokerAddress},
		GroupID:     "go-consumer-group",
		Topic:       topic,
		StartOffset: kafka.FirstOffset,
	})
	defer reader.Close()

	fmt.Println("🔄 Consumer started.")

	for {
		msg, err := reader.FetchMessage(ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				fmt.Println("🛑 Consumer stopped.")
				return
			}
			fmt.Printf("❌ Error fetching: %v\n", err)
			time.Sleep(1 * time.Second)
			continue
		}

		fmt.Printf("📥 Consumed: %s (offset: %d)\n", string(msg.Value), msg.Offset)

		if err := reader.CommitMessages(ctx, msg); err != nil {
			fmt.Printf("⚠️ Commit failed: %v\n", err)
		} else {
			fmt.Println("✅ Offset committed.")
		}
	}
}
