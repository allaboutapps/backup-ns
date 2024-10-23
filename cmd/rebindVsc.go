package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// rebindVscCmd represents the rebindVsc command
var rebindVscCmd = &cobra.Command{
	Use:   "rebindVsc",
	Short: "A brief description of your command",
	// Long: `...`,
	Run: func(_ *cobra.Command, _ []string) {
		fmt.Println("rebindVsc called")
	},
}

func init() {
	rootCmd.AddCommand(rebindVscCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// rebindVscCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// rebindVscCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
