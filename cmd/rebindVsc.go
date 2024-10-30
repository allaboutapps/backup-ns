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

func rebindVsc(oldVSCName string) error {
	config := lib.LoadConfig()

	// Get the VolumeSnapshotContent object
	oldVSCObject, err := lib.GetVolumeSnapshotContentObject(oldVSCName)
	if err != nil {
		return fmt.Errorf("failed to get VolumeSnapshotContent object: %w", err)
	}

	// Create a restored VSC from the existing VSC
	restoredVSC, err := lib.CreatePreProvisionedVSC(oldVSCObject, config.VSRand)
	if err != nil {
		return fmt.Errorf("failed to create restored VolumeSnapshotContent: %w", err)
	}

	// Generate the VolumeSnapshot object from the restored VolumeSnapshotContent
	vsObject, err := lib.GenerateVSObjectFromVSC(restoredVSC["metadata"].(map[string]interface{})["name"].(string), restoredVSC)
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
	err = lib.CreateVolumeSnapshot(namespace, false, vsName, vsObject, config.VSWaitUntilReady, config.VSWaitUntilReadyTimeout)
	if err != nil {
		return fmt.Errorf("failed to create VolumeSnapshot: %w", err)
	}

	fmt.Printf("Successfully rebound old VolumeSnapshotContent '%s' to new VolumeSnapshot '%s' in namespace '%s'\n", oldVSCName, vsName, namespace)

	// Delete the old VolumeSnapshotContent after making sure its deletionPolicy is 'Retain'
	fmt.Printf("Attempting to delete old VolumeSnapshotContent '%s'...\n", oldVSCName)

	deletionPolicy, ok := oldVSCObject["spec"].(map[string]interface{})["deletionPolicy"].(string)
	if !ok {
		return fmt.Errorf("deletionPolicy not found in old VolumeSnapshotContent")
	}

	if deletionPolicy != "Retain" {
		return fmt.Errorf("deletionPolicy is not 'Retain' in old VolumeSnapshotContent, refusing to delete")
	}

	fmt.Printf("Old VolumeSnapshotContent '%s' has deletionPolicy 'Retain' set and is thus safe to delete! Deleting...\n", oldVSCName)

	err = lib.DeleteVolumeSnapshotContent(oldVSCName)
	if err != nil {
		return fmt.Errorf("failed to delete old VolumeSnapshotContent: %w", err)
	}

	fmt.Printf("Successfully deleted old VolumeSnapshotContent '%s'\n", oldVSCName)

	return nil
}
