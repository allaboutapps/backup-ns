package cmd

import (
	"github.com/spf13/cobra"
)

var controllerCmd = &cobra.Command{
	Use:   "controller",
	Short: "Controller related commands",
	Long:  `Commands that are typically run by the controller/operator.`,
}

func init() {
	rootCmd.AddCommand(controllerCmd)
}
