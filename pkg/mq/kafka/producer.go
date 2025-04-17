package kafka

import (
	"context"
	"fmt"
	"time"

	"github.com/segmentio/kafka-go"
)

const (
	brokerAddress = "localhost:9092"
	topic         = "test-topic"
)

func StartProducer(ctx context.Context) {
	writer := &kafka.Writer{
		Addr:     kafka.TCP(brokerAddress),
		Topic:    topic,
		Balancer: &kafka.LeastBytes{},
	}
	defer writer.Close()

	for i := 0; i < 10; i++ {
		msg := fmt.Sprintf("Message %d at %s", i, time.Now().Format(time.RFC3339))
		err := writer.WriteMessages(ctx, kafka.Message{Value: []byte(msg)})
		if err != nil {
			fmt.Printf("❌ Failed to produce: %v\n", err)
		} else {
			fmt.Printf("📤 Produced: %s\n", msg)
		}
		time.Sleep(1 * time.Second)
	}

	fmt.Println("✅ Producer done.")
}
