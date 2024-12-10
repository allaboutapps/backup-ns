package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// mysqlCmd represents the db command
var mysqlCmd = &cobra.Command{
	Use:   "mysql <subcommand>",
	Short: "mysql/mariadb database related subcommands",
	Run: func(cmd *cobra.Command, _ []string /* args */) {
		if err := cmd.Help(); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		os.Exit(0)
	},
}

func init() {
	rootCmd.AddCommand(mysqlCmd)
}
