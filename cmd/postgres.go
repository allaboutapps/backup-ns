package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// postgresCmd represents the postgres command
var postgresCmd = &cobra.Command{
	Use:   "postgres <subcommand>",
	Short: "postgres database related subcommands",
	Run: func(cmd *cobra.Command, _ []string /* args */) {
		if err := cmd.Help(); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		os.Exit(0)
	},
}

func init() {
	rootCmd.AddCommand(postgresCmd)
}
