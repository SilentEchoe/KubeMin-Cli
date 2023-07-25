package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
)

//子命令

var getCmd = &cobra.Command{
	Use:   "get",
	Short: "Get Kubernetes resource",
	Long:  `Get Kubernetes resource`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("kubeMin-cli get")
	},
}

func init() {
	rootCmd.AddCommand(getCmd)
}
