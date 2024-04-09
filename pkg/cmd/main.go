package main

import (
	"KubeMin-Cli/pkg/cmd/server/app"
	"context"
	"fmt"
	"k8s.io/klog/v2"
	"log"
	"os"
	"os/signal"
	"syscall"
)

func Run() error {
	errChan := make(chan error)
	ctx, cancel := context.WithCancel(context.Background())

	defer cancel()

	go func() {
		if err := run(ctx, errChan); err != nil {
			errChan <- fmt.Errorf("failed to run apiserver: %w", err)
		}
	}()

	var term = make(chan os.Signal, 1)
	signal.Notify(term, os.Interrupt, syscall.SIGTERM)

	select {
	case <-term:
		klog.Infof("Received SIGTERM, exiting gracefully...")
	case err := <-errChan:
		klog.Errorf("Received an error: %s, exiting gracefully...", err.Error())
		return err
	}

	return nil
}

func run(ctx context.Context, errChan chan error) error {
	klog.Infof("KubeMin-CLI Start ……")

	cmd := app.NewAPIServerCommand()
	if err := cmd.Execute(); err != nil {
		log.Fatalln(err)
	}
	return nil
}
