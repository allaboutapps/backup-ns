package cmd

import (
	"fmt"
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
		if err := rebindVsc(vscName); err != nil {
			log.Fatalf("Error rebinding VSC: %v", err)
		}
	},
}

func init() {
	rootCmd.AddCommand(rebindVscCmd)
}

func rebindVsc(vscName string) error {
	// Get the VolumeSnapshotContent object
	vscObject, err := lib.GetVolumeSnapshotContentObject(vscName)
	if err != nil {
		return fmt.Errorf("failed to get VolumeSnapshotContent object: %w", err)
	}

	// Generate the VolumeSnapshot object from the VolumeSnapshotContent
	vsObject, err := lib.GenerateVSObjectFromVSC(vscName, vscObject)
	if err != nil {
		return fmt.Errorf("failed to generate VolumeSnapshot object: %w", err)
	}

	// Extract necessary information from the generated VS object
	metadata, ok := vsObject["metadata"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid metadata in generated VolumeSnapshot object")
	}

	namespace, ok := metadata["namespace"].(string)
	if !ok {
		return fmt.Errorf("invalid namespace in generated VolumeSnapshot object")
	}

	vsName, ok := metadata["name"].(string)
	if !ok {
		return fmt.Errorf("invalid name in generated VolumeSnapshot object")
	}

	// Create the VolumeSnapshot
	err = lib.CreateVolumeSnapshot(namespace, false, vsName, vsObject, true, "5m")
	if err != nil {
		return fmt.Errorf("failed to create VolumeSnapshot: %w", err)
	}

	fmt.Printf("Successfully rebound VolumeSnapshotContent '%s' to new VolumeSnapshot '%s' in namespace '%s'\n", vscName, vsName, namespace)
	return nil
}
