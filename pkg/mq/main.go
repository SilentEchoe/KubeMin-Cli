package main

import (
	"KubeMin-Cli/pkg/mq/kafka"
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigchan := make(chan os.Signal, 1)
	signal.Notify(sigchan, syscall.SIGINT, syscall.SIGTERM)

	go kafka.StartConsumer(ctx)

	time.Sleep(2 * time.Second)

	go kafka.StartProducer(ctx)

	<-sigchan
	println("👋 Shutting down gracefully...")
}
