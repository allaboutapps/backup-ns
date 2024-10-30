package cmd

import (
	"log"

	"github.com/allaboutapps/backup-ns/internal/lib"
	"github.com/spf13/cobra"
)

// rebindVscCmd represents the rebindVsc command
var rebindVscCmd = &cobra.Command{
	Use:   "rebindVsc <vsc-name>",
	Short: "Rebind a VolumeSnapshotContent to a new VolumeSnapshot",
	Long: `This command takes a VolumeSnapshotContent name as an argument and creates a new VolumeSnapshot
based on the information in the VolumeSnapshotContent. It effectively restores the VolumeSnapshot
from the VolumeSnapshotContent.`,
	Args: cobra.ExactArgs(1),
	Run: func(_ *cobra.Command, args []string) {
		vscName := args[0]

		config := lib.LoadConfig()

		if err := lib.RebindVsc(vscName, config.VSRand, config.VSWaitUntilReady, config.VSWaitUntilReadyTimeout); err != nil {
			log.Fatalf("Error rebinding VSC: %v", err)
		}
	},
}

func init() {
	rootCmd.AddCommand(rebindVscCmd)
}
