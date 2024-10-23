package cmd

import (
	"bytes"
	"fmt"
	"log"
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
			log.Fatalf("Error host requirements: %v\n", err)
		}

		fmt.Println("starting sync vs metadata to vsc matching label 'backup-ns.sh/type'")

		readySnapshots, err := getReadySnapshots()
		if err != nil {
			log.Fatalf("Error getting ready snapshots: %v\n", err)
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
			log.Fatalf("syncing metadata to vsc failed with %d errors.\n", fails)
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
	cmd := exec.Command("kubectl", "get", "volumesnapshot", name, "-n", namespace, "-o", "jsonpath={.status.boundVolumeSnapshotContentName}")
	vscNameBytes, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to get VolumeSnapshotContent name: %w", err)
	}
	vscName := strings.TrimSpace(string(vscNameBytes))

	if vscName == "" {
		return fmt.Errorf("volumeSnapshot %s in namespace %s not found or does not have a boundVolumeSnapshotContentName", name, namespace)
	}

	cmd = exec.Command("kubectl", "get", "volumesnapshot", name, "-n", namespace, "-o", "jsonpath={.metadata.labels}")
	vsLabelsBytes, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to get VolumeSnapshot labels: %w", err)
	}
	vsLabels := strings.TrimSpace(string(vsLabelsBytes))

	cmd = exec.Command("kubectl", "get", "volumesnapshotcontent", vscName, "-o", "jsonpath={.metadata.labels}")
	vscLabelsBytes, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to get VolumeSnapshotContent labels: %w", err)
	}
	vscLabels := strings.TrimSpace(string(vscLabelsBytes))

	vsLabelsMap := parseLabels(vsLabels)
	vscLabelsMap := parseLabels(vscLabels)

	labelDiff := getLabelDiff(vsLabelsMap, vscLabelsMap)

	log.Printf("namespace=%s vs=%s vsc=%s\nvsLabels=%v\nvscLabels=%v\nlabelDiff=%v\n", namespace, name, vscName, vsLabels, vscLabels, labelDiff)

	if len(labelDiff) == 0 {
		fmt.Printf("noop VolumeSnapshotContent %s of VolumeSnapshot %s already in sync.\n", vscName, name)
		return nil
	}

	labelDel := getLabelDel(vscLabelsMap)
	labelAdd := getLabelAdd(vsLabelsMap)

	if len(labelDel) > 0 {
		args := append([]string{"label", "volumesnapshotcontent", vscName}, labelDel...)
		cmd = exec.Command("kubectl", args...)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to delete labels from VolumeSnapshotContent %s of VolumeSnapshot %s: %w", vscName, name, err)
		}
	}

	args := append([]string{"label", "volumesnapshotcontent", vscName}, labelAdd...)
	cmd = exec.Command("kubectl", args...)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to apply labels to VolumeSnapshotContent %s of VolumeSnapshot %s: %w", vscName, name, err)
	}

	return nil
}

func parseLabels(labels string) map[string]string {
	labelMap := make(map[string]string)
	if labels == "" {
		return labelMap
	}
	labels = strings.Trim(labels, "{}")
	labels = strings.ReplaceAll(labels, "\"", "")
	pairs := strings.Split(labels, ",")
	for _, pair := range pairs {
		kv := strings.SplitN(pair, ":", 2)
		if len(kv) == 2 {
			labelMap[strings.TrimSpace(kv[0])] = strings.TrimSpace(kv[1])
		}
	}
	return labelMap
}

func getLabelDiff(vsLabels, vscLabels map[string]string) map[string]string {
	diff := make(map[string]string)
	for k, v := range vsLabels {
		if vscLabels[k] != v {
			diff[k] = v
		}
	}
	return diff
}

func getLabelDel(vscLabels map[string]string) []string {
	var del []string
	for k := range vscLabels {
		del = append(del, fmt.Sprintf("%s-", k))
	}
	return del
}

func getLabelAdd(vsLabels map[string]string) []string {
	var add []string
	for k, v := range vsLabels {
		add = append(add, fmt.Sprintf("%s=%s", k, v))
	}
	return add
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
