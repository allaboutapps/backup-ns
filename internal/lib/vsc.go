package lib

import (
	"fmt"
	"log"
	"os/exec"
	"strings"
)

func GetVolumeSnapshotContentName(namespace, volumeSnapshotName string) (string, error) {
	cmd := exec.Command("kubectl", "get", "volumesnapshot", volumeSnapshotName, "-n", namespace, "-o", "jsonpath={.status.boundVolumeSnapshotContentName}")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to get VolumeSnapshotContent name: %w, output: %s", err, output)
	}
	return strings.TrimSpace(string(output)), nil
}

func patchVolumeSnapshotContentDeletionPolicy(vscName string) error {
	patchCmd := exec.Command("kubectl", "patch", "volumesnapshotcontent", vscName, "--type", "merge", "-p", `{"spec":{"deletionPolicy":"Delete"}}`)
	output, err := patchCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to patch VolumeSnapshotContent: %w, output: %s", err, output)
	}
	log.Printf("Successfully patched VolumeSnapshotContent %s deletionPolicy to 'Delete'\n", vscName)
	return nil
}

func SyncVSLabelsToVsc(namespace, vsName string) error {
	cmd := exec.Command("kubectl", "get", "volumesnapshot", vsName, "-n", namespace, "-o", "jsonpath={.status.boundVolumeSnapshotContentName}")
	vscNameBytes, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to get VolumeSnapshotContent name: %w", err)
	}
	vscName := strings.TrimSpace(string(vscNameBytes))

	if vscName == "" {
		return fmt.Errorf("volumeSnapshot %s in namespace %s not found or does not have a boundVolumeSnapshotContentName", vsName, namespace)
	}

	vsLabelsMap, err := GetBackupNsLabelMap(namespace, "volumesnapshot", vsName)
	if err != nil {
		return fmt.Errorf("failed to get VolumeSnapshot labels: %w", err)
	}

	vscLabelsMap, err := GetBackupNsLabelMap(namespace, "volumesnapshotcontent", vscName)
	if err != nil {
		return fmt.Errorf("failed to get VolumeSnapshotContent labels: %w", err)
	}

	labelDiff := getLabelDiff(vsLabelsMap, vscLabelsMap)

	log.Printf("namespace=%s vs=%s vsc=%s\nvsLabels=%v\nvscLabels=%v\nlabelDiff=%v\n", namespace, vsName, vscName, vsLabelsMap, vscLabelsMap, labelDiff)

	if len(labelDiff) == 0 {
		log.Printf("noop namespace=%s vs=%s vsc=%s already in sync.\n", namespace, vscName, vsName)
		return nil
	}

	labelDel := getLabelDel(vscLabelsMap)
	labelAdd := getLabelAdd(vsLabelsMap)

	if len(labelDel) > 0 {
		args := append([]string{"label", "volumesnapshotcontent", vscName}, labelDel...)
		cmd = exec.Command("kubectl", args...)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to delete labels from VolumeSnapshotContent %s of VolumeSnapshot %s: %w", vscName, vsName, err)
		}
	}

	args := append([]string{"label", "volumesnapshotcontent", vscName}, labelAdd...)
	cmd = exec.Command("kubectl", args...)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to apply labels to VolumeSnapshotContent %s of VolumeSnapshot %s: %w", vscName, vsName, err)
	}

	return nil
}

func GetBackupNsLabelMap(namespace, kind, name string) (map[string]string, error) {
	cmd := exec.Command("kubectl", "get", kind, name, "-n", namespace, "-o", "jsonpath={.metadata.labels}")
	labelsBytes, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get ns=%s %s/%s labels: %w", namespace, kind, name, err)
	}
	labels := strings.TrimSpace(string(labelsBytes))
	labelsMap := parseBackupNsLabels(labels)

	return labelsMap, nil
}

func parseBackupNsLabels(labels string) map[string]string {
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

			key := strings.TrimSpace(kv[0])
			val := strings.TrimSpace(kv[1])

			if strings.Index(key, "backup-ns.sh/") != 0 {
				continue // filter out.
			}

			labelMap[key] = val
		}
	}
	return labelMap
}

func getLabelDiff(target, current map[string]string) map[string]string {
	diff := make(map[string]string)
	for k, v := range target {
		if current[k] != v {
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
