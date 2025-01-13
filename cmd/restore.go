package cmd

import (
	"log"

	"github.com/allaboutapps/backup-ns/internal/lib"
	"github.com/spf13/cobra"
)

var (
	pvcName      string
	storageClass string
)

// restoreCmd represents the restore command
var restoreCmd = &cobra.Command{
	Use:   "restore SNAPSHOT_NAME",
	Short: "Restore a volume snapshot to a new PVC",
	Long: `Restore a volume snapshot to a new PVC.
The snapshot name is provided as a positional argument.`,
	Example: `  backup-ns restore my-snapshot --pvc new-pvc
  backup-ns restore my-snapshot --pvc new-pvc --storage-class standard
  backup-ns restore --pvc new-pvc my-snapshot`,
	Args: cobra.ExactArgs(1),
	Run:  runRestore,
}

func init() {
	rootCmd.AddCommand(restoreCmd)

	// Required flags
	restoreCmd.Flags().StringVarP(&pvcName, "pvc", "p", "", "Name of the new PVC to create")
	if err := restoreCmd.MarkFlagRequired("pvc"); err != nil {
		log.Fatalf("Failed to mark 'pvc' flag as required: %v", err)
	}

	// Optional flags
	restoreCmd.Flags().StringVar(&storageClass, "storage-class", "", "Storage class to use for the new PVC (optional)")
}

func runRestore(_ *cobra.Command, args []string) {
	config := lib.LoadConfig()
	snapshotName := args[0]

	lib.PrintTimeZone()
	lib.PrintConfig(config)

	if config.DryRun {
		log.Println("Dry run mode is active, write operations are skipped!")
	}

	log.Printf("Restoring snapshot '%s' to PVC '%s' in namespace '%s'...",
		snapshotName,
		pvcName,
		config.Namespace,
	)

	err := lib.RestoreVolumeSnapshot(
		config.Namespace,
		config.DryRun,
		snapshotName,
		pvcName,
		storageClass,
		true, // wait
		"5m", // timeout
	)

	if err != nil {
		log.Fatalf("Failed to restore snapshot: %v", err)
	}

	log.Printf("Successfully restored snapshot '%s' to PVC '%s'", snapshotName, pvcName)
}
