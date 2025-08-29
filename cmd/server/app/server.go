package app

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"k8s.io/klog/v2"

	"KubeMin-Cli/cmd/server/app/options"
	server "KubeMin-Cli/pkg/apiserver"
	"KubeMin-Cli/pkg/apiserver/utils"
	"KubeMin-Cli/pkg/apiserver/utils/profiling"
	"KubeMin-Cli/pkg/tracing"
	"KubeMin-Cli/version"
)

// NewAPIServerCommand creates a *cobra.Command object with default parameters
func NewAPIServerCommand() *cobra.Command {
	s := options.NewServerRunOptions()

	// Initialize klog flags
	klog.InitFlags(nil)

	cmd := &cobra.Command{
		Use:  "ApiServer",
		Long: `The KubeMin-CLI API service, which provides application deployment and Istio operations`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := s.Validate(); err != nil {
				return err
			}
			return Run(s)
		},
		SilenceUsage: true,
	}

	fs := cmd.Flags()
	namedFlagSets := s.Flags()
	// Add klog flags to the command's flag set
	namedFlagSets.FlagSet("klog").AddGoFlagSet(flag.CommandLine)

	for _, set := range namedFlagSets.FlagSets {
		fs.AddFlagSet(set)
	}

	return cmd
}

// Run runs the specified APIServer. This should never exit.
func Run(s *options.ServerRunOptions) error {
	// The server is not terminal, there is no color default.
	// Force set to false, this is useful for the dry-run API.
	color.NoColor = false

	errChan := make(chan error)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	//开启分析服务
	go profiling.StartProfilingServer(errChan)

	// Start log cleanup service
	logDir := flag.Lookup("log_dir").Value.String()

	// Ensure the log directory exists before starting services that log to files.
	if logDir != "" {
		if err := os.MkdirAll(logDir, 0755); err != nil {
			klog.Fatalf("Failed to create log directory %s: %v", logDir, err)
		}
	}

	go utils.StartLogCleanup(logDir, 7*24*time.Hour)

	go func() {
		if err := run(ctx, s, errChan); err != nil {
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
	klog.Infof("See you next time!")
	klog.Flush()
	return nil
}

func run(ctx context.Context, s *options.ServerRunOptions, errChan chan error) error {
	klog.Infof("KubeMin-Cli information: version: %v", version.KubeMinCliVersion)

	if s.GenericServerRunOptions.EnableTracing {
		klog.InfoS("Distributed tracing enabled", "jaegerEndpoint", s.GenericServerRunOptions.JaegerEndpoint)
		shutdown, err := tracing.InitTracerProvider("kubemin-cli", s.GenericServerRunOptions.JaegerEndpoint)
		if err != nil {
			return fmt.Errorf("failed to init tracer provider: %w", err)
		}
		defer func() {
			if err := shutdown(context.Background()); err != nil {
				klog.ErrorS(err, "Failed to shutdown tracer provider")
			}
		}()
	}

	apiServer := server.New(*s.GenericServerRunOptions)
	return apiServer.Run(ctx, errChan)
}
