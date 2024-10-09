package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// deleteAfterMarkCmd represents the deleteAfterMark command
var deleteAfterMarkCmd = &cobra.Command{
	Use:   "deleteAfterMark",
	Short: "Marks all daily_weekly_monthly snapshots without daily/weeky/monthly label for deleteAfter today (to be deleted tomorrow)",
	// Long:  `...`,
	Run: func(_ *cobra.Command, _ []string) {
		fmt.Println("not implemented")
		os.Exit(1)
	},
}

func init() {
	rootCmd.AddCommand(deleteAfterMarkCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// deleteAfterMarkCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// deleteAfterMarkCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
