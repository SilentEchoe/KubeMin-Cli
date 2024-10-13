package app

import (
	"github.com/spf13/cobra"
)

// NewAPIServerCommand creates a *cobra.Command object with default parameters
func NewAPIServerCommand() *cobra.Command {
	//s := options.NewServerRunOptions()
	cmd := &cobra.Command{
		Use:  "ApiServer",
		Long: `The KubeMin-CLI API service, which provides application deployment and Istio operations`,
	}

	return cmd
}
