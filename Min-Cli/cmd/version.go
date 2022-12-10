package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(versionCmd)
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "v0.1",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Kube-mincli version v0.1 -- HEAD")
	},
}
