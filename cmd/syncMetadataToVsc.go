package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// syncMetadataToVscCmd represents the syncMetadataToVsc command
var syncMetadataToVscCmd = &cobra.Command{
	Use:   "syncMetadataToVsc",
	Short: "Synces vs label/annotations metadata to vsc",
	// Long:  `...`, // accidental namespace/vs deletion -> restore namespace...
	Run: func(_ *cobra.Command, _ []string) {
		fmt.Println("not implemented")
		os.Exit(1)
	},
}

func init() {
	rootCmd.AddCommand(syncMetadataToVscCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// syncMetadataToVscCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// syncMetadataToVscCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
