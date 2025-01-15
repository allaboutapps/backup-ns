package cmd

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/allaboutapps/backup-ns/internal/lib"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"
)

var (
	pvcName      string
	storageClass string
	wait         bool
	timeout      string
	outputFormat string
)

// restoreCmd represents the restore command
var restoreCmd = &cobra.Command{
	Use:   "restore SNAPSHOT_NAME",
	Short: "Restore a volume snapshot to a new PVC",
	Long: `Restore a volume snapshot to a new PVC.
The snapshot name is provided as a positional argument.

When using --output/-o flag, the PVC manifest will only be printed in the specified format
without being applied to the cluster.`,
	Example: `  # Create new PVC from snapshot
  backup-ns restore my-snapshot --pvc new-pvc
  backup-ns restore my-snapshot --pvc new-pvc --storage-class standard
  
  # Only print manifest without applying
  backup-ns restore my-snapshot --pvc new-pvc -o yaml
  backup-ns restore my-snapshot --pvc new-pvc -o json
  
  # Print manifest without applying (alternative)
  backup-ns restore my-snapshot --pvc new-pvc --dry-run`,
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
	restoreCmd.Flags().StringVarP(&namespace, "namespace", "n", "", "Namespace of the VolumeSnapshot (defaults to the current namespace in the context)")
	restoreCmd.Flags().StringVarP(&outputFormat, "output", "o", "", "Output format (json or yaml)")
	restoreCmd.Flags().StringVar(&storageClass, "storage-class", "", "Storage class to use for the new PVC (optional)")
	restoreCmd.Flags().BoolVar(&wait, "wait", false, "Wait for restore operation to complete")
	restoreCmd.Flags().StringVar(&timeout, "timeout", "30s", "Timeout for wait operation")
}

func runRestore(_ *cobra.Command, args []string) {
	snapshotName := args[0]

	if namespace == "" {
		var err error
		namespace, err = lib.GetCurrentNamespace()
		if err != nil {
			log.Fatalf("Error getting current namespace from context: %v\n", err)
		}
	}

	// Create PVC manifest
	pvcObject, err := lib.CreatePVCManifestFromVolumeSnapshot(
		namespace,
		snapshotName,
		pvcName,
		storageClass,
	)
	if err != nil {
		log.Fatalf("Failed to create PVC manifest: %v", err)
	}

	// Handle output format
	switch outputFormat {
	case "json":
		jsonData, err := json.MarshalIndent(pvcObject, "", "  ")
		if err != nil {
			log.Fatalf("Failed to marshal JSON: %v", err)
		}
		fmt.Println(string(jsonData))
		return
	case "yaml":
		yamlData, err := yaml.Marshal(pvcObject)
		if err != nil {
			log.Fatalf("Failed to marshal YAML: %v", err)
		}
		fmt.Println(string(yamlData))
		return
	case "":
		// Normal restore operation
		err = lib.RestoreVolumeSnapshot(
			namespace,
			snapshotName,
			pvcName,
			storageClass,
			wait,
			timeout,
		)
		if err != nil {
			log.Fatalf("Failed to restore snapshot: %v", err)
		}
		log.Printf("Successfully restored snapshot '%s' to PVC '%s'", snapshotName, pvcName)
	default:
		log.Fatalf("Invalid output format: %s (must be json or yaml)", outputFormat)
	}
}
