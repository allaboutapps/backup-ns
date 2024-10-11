package cmd

import (
	"github.com/allaboutapps/backup-ns/internal/lib"
	"github.com/spf13/cobra"
)

// envCmd represents the env command
var envCmd = &cobra.Command{
	Use:   "env",
	Short: "Prints the timeZone and BAK_ environment variables that are set in the current shell.",
	// Long: `...`,
	Run: func(_ *cobra.Command, _ []string) {
		config := lib.LoadConfig()
		lib.PrintTimeZone()
		lib.PrintConfig(config)
	},
}

func init() {
	rootCmd.AddCommand(envCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// envCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// envCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
