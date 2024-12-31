package app

import (
	"KubeMin-Cli/cmd/server/app/options"
	server "KubeMin-Cli/pkg/apiserver"
	"KubeMin-Cli/pkg/apiserver/utils/profiling"
	"context"
	"fmt"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"k8s.io/klog/v2"
	"os"
	"os/signal"
	"syscall"
)

// NewAPIServerCommand creates a *cobra.Command object with default parameters
func NewAPIServerCommand() *cobra.Command {
	s := options.NewServerRunOptions()
	cmd := &cobra.Command{
		Use:  "ApiServer",
		Long: `The KubeMin-CLI API service, which provides application deployment and Istio operations`,
		RunE: func(cmd *cobra.Command, args []string) error { //nolint:revive,unused
			if err := s.Validate(); err != nil {
				return err
			}
			return Run(s)
		},
		SilenceUsage: true,
	}

	fs := cmd.Flags()
	namedFlagSets := s.Flags()
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

	go profiling.StartProfilingServer(errChan)

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
	return nil
}

func run(ctx context.Context, s *options.ServerRunOptions, errChan chan error) error {
	//klog.Infof("KubeMin-Cli information: version: %v, gitRevision: %v", version.VelaVersion, version.GitRevision)
	server := server.New(*s.GenericServerRunOptions)

	return server.Run(ctx, errChan)
}
