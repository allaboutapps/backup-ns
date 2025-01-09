package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// deleteAfterSweepCmd represents the deleteAfterSweep command
var deleteAfterSweepCmd = &cobra.Command{
	Use:   "deleteAfterSweep",
	Short: "Sweeps all snapshots with a deleteAfter label mark smaller then today (after having them marked yesterday)",
	// Long:  `...`,
	Run: func(_ *cobra.Command, _ []string) {
		fmt.Println("not implemented")
		os.Exit(1)
	},
}

func init() {
	controllerCmd.AddCommand(deleteAfterSweepCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// deleteAfterSweepCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// deleteAfterSweepCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
