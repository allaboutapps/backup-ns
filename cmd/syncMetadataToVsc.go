package cmd

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
)

// syncMetadataToVscCmd represents the syncMetadataToVsc command
var syncMetadataToVscCmd = &cobra.Command{
	Use:   "syncMetadataToVsc",
	Short: "Synces vs label/annotations metadata to vsc",
	// Long:  `...`, // accidental namespace/vs deletion -> restore namespace...
	Run: func(_ *cobra.Command, _ []string) {
		if err := checkHostRequirements(); err != nil {
			fmt.Println("Error:", err)
			os.Exit(1)
		}

		fmt.Println("starting sync vs metadata to vsc matching label 'backup-ns.sh/type'")

		readySnapshots, err := getReadySnapshots()
		if err != nil {
			fmt.Println("Error getting ready snapshots:", err)
			os.Exit(1)
		}

		fails := 0

		for _, line := range readySnapshots {
			parts := strings.Fields(line)
			if len(parts) < 2 {
				continue
			}
			vsName := parts[0]
			vsNamespace := parts[1]

			if err := syncLabelsToVsc(vsNamespace, vsName); err != nil {
				fails++
				fmt.Printf("fail#%d syncing metadata to vsc failed for vs_name='%s' in ns='%s'.\n", fails, vsName, vsNamespace)
			}
		}

		if fails > 0 {
			fmt.Printf("syncing metadata to vsc failed with %d errors.\n", fails)
			os.Exit(1)
		}

		fmt.Println("syncing metadata to vsc done with", fails, "errors.")
	},
}

func checkHostRequirements() error {
	_, err := exec.LookPath("jq")
	if err != nil {
		return fmt.Errorf("jq is not installed")
	}
	return nil
}

func getReadySnapshots() ([]string, error) {
	cmd := exec.Command("kubectl", "get", "volumesnapshot", "--all-namespaces", "-lbackup-ns.sh/type", "-o=jsonpath={range .items[*]}{.metadata.name} {.metadata.namespace}{\"\\n\"}{end}")
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		return nil, err
	}
	return strings.Split(strings.TrimSpace(out.String()), "\n"), nil
}

func syncLabelsToVsc(namespace, name string) error {
	// Implement the vs_sync_labels_to_vsc logic here
	// This is a placeholder for the actual implementation
	return nil
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
