package app

import (
	"KubeMin-Cli/pkg/cmd/server/app/options"
	"github.com/spf13/cobra"
)

// NewAPIServerCommand creates a *cobra.Command object with default parameters
func NewAPIServerCommand() *cobra.Command {
	_ = options.NewServerRunOptions()
	cmd := &cobra.Command{}

	return cmd
}
