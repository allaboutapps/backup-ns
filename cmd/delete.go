package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

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
		namespace, err = getCurrentNamespace()
		if err != nil {
			fmt.Printf("Error getting current namespace: %v\n", err)
			os.Exit(1)
		}
	}

	// Get the VolumeSnapshotContent name
	vscName, err := getVolumeSnapshotContentName(namespace, volumeSnapshotName)
	if err != nil {
		fmt.Printf("Error getting VolumeSnapshotContent name: %v\n", err)
		os.Exit(1)
	}

	// Patch the VolumeSnapshotContent to set deletionPolicy to Delete
	if err := patchVolumeSnapshotContent(vscName); err != nil {
		fmt.Printf("Error patching VolumeSnapshotContent: %v\n", err)
		os.Exit(1)
	}

	// Delete the VolumeSnapshot
	if err := deleteVolumeSnapshot(namespace, volumeSnapshotName); err != nil {
		fmt.Printf("Error deleting VolumeSnapshot: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Successfully deleted VolumeSnapshot %s in namespace %s\n", volumeSnapshotName, namespace)
}

func getCurrentNamespace() (string, error) {
	cmd := exec.Command("kubectl", "config", "view", "--minify", "--output", "jsonpath={..namespace}")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("error getting current namespace: %w", err)
	}
	return string(output), nil
}

func getVolumeSnapshotContentName(namespace, volumeSnapshotName string) (string, error) {
	cmd := exec.Command("kubectl", "get", "volumesnapshot", volumeSnapshotName, "-n", namespace, "-o", "jsonpath={.status.boundVolumeSnapshotContentName}")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to get VolumeSnapshotContent name: %w, output: %s", err, output)
	}
	return strings.TrimSpace(string(output)), nil
}

func patchVolumeSnapshotContent(vscName string) error {
	patchCmd := exec.Command("kubectl", "patch", "volumesnapshotcontent", vscName, "--type", "merge", "-p", `{"spec":{"deletionPolicy":"Delete"}}`)
	output, err := patchCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to patch VolumeSnapshotContent: %w, output: %s", err, output)
	}
	fmt.Printf("Successfully patched VolumeSnapshotContent %s deletionPolicy to 'Delete'\n", vscName)
	return nil
}

func deleteVolumeSnapshot(namespace, volumeSnapshotName string) error {
	deleteCmd := exec.Command("kubectl", "delete", "volumesnapshot", volumeSnapshotName, "-n", namespace)
	output, err := deleteCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to delete VolumeSnapshot: %w, output: %s", err, output)
	}
	return nil
}
