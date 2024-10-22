package cmd

import (
	"log"

	"github.com/allaboutapps/backup-ns/internal/lib"
	"github.com/spf13/cobra"
)

var namespace string

// deleteCmd represents the delete command
var deleteCmd = &cobra.Command{
	Use:   "delete <volumesnapshot>",
	Short: "Deletes an application-aware snapshot (autopatches the vsc to deletionPolicy=delete first)",
	Long: `This command deletes a VolumeSnapshot and its associated VolumeSnapshotContent,
ensuring that the underlying storage is also deleted. It first patches the
VolumeSnapshotContent's deletionPolicy to "Delete" before deleting the VolumeSnapshot.

CAUTION: This is a destructive operation and should be used with care!`,
	Args: cobra.ExactArgs(1),
	Run:  runDelete,
}

func init() {
	rootCmd.AddCommand(deleteCmd)
	deleteCmd.Flags().StringVarP(&namespace, "namespace", "n", "", "Namespace of the VolumeSnapshot (defaults to the current namespace in the context)")
}

func runDelete(_ *cobra.Command, args []string) {
	volumeSnapshotName := args[0]

	if namespace == "" {
		var err error
		namespace, err = lib.GetCurrentNamespace()
		if err != nil {
			log.Fatalf("Error getting current namespace from context: %v\n", err)
		}
	}

	log.Printf("Using namespace '%s'.\n", namespace)

	if err := lib.PruneVolumeSnapshot(namespace, volumeSnapshotName); err != nil {
		log.Fatalf("Error deleting VolumeSnapshot: %v\n", err)
	}

	log.Printf("Successfully deleted VolumeSnapshot %s in namespace %s\n", volumeSnapshotName, namespace)
}
