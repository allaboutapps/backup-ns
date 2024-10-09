package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// applyRetentionPolicyCmd represents the applyRetentionPolicy command
var applyRetentionPolicyCmd = &cobra.Command{
	Use:   "applyRetentionPolicy",
	Short: "Enforces that daily, weeky, monthly labels are only set for a specific number of snapshots",
	// Long:  `...`,
	Run: func(_ *cobra.Command, _ []string) {
		fmt.Println("not implemented")
		os.Exit(1)
	},
}

func init() {
	rootCmd.AddCommand(applyRetentionPolicyCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// applyRetentionPolicyCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// applyRetentionPolicyCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
