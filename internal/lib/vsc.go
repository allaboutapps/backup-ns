package lib

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"os/exec"
	"strings"

	"github.com/google/uuid"
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
func GetVolumeSnapshotContentObject(vscName string) (map[string]interface{}, error) {
	cmd := exec.Command("kubectl", "get", "volumesnapshotcontent", vscName, "-o", "json")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get VolumeSnapshotContent object: %w", err)
	}

	var vscObject map[string]interface{}
	err = json.Unmarshal(output, &vscObject)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal VolumeSnapshotContent object: %w", err)
	}

	return vscObject, nil
}

func CreatePreProvisionedVSC(vscObject map[string]interface{}, postfix string) (map[string]interface{}, error) {
	// Create a new VSC object for the pre-provisioned VSC
	preProvisionedVSC := make(map[string]interface{})

	// Copy and modify the metadata
	metadata := make(map[string]interface{})
	originalMetadata := vscObject["metadata"].(map[string]interface{})

	newVSCName := "restoredvsc-" + uuid.New().String()

	metadata["name"] = newVSCName
	if labels, ok := originalMetadata["labels"]; ok {
		metadata["labels"] = labels
	}
	preProvisionedVSC["metadata"] = metadata

	// Copy and modify the spec
	spec := make(map[string]interface{})
	originalSpec := vscObject["spec"].(map[string]interface{})

	// Set deletionPolicy to Retain
	spec["deletionPolicy"] = "Retain"

	// Copy volumeSnapshotClassName if it exists
	if vsClassName, ok := originalSpec["volumeSnapshotClassName"]; ok {
		spec["volumeSnapshotClassName"] = vsClassName
	}

	// Set the source with the correct snapshotHandle
	source := make(map[string]interface{})
	if status, ok := vscObject["status"].(map[string]interface{}); ok {
		if snapshotHandle, ok := status["snapshotHandle"]; ok {
			source["snapshotHandle"] = snapshotHandle
		} else {
			return nil, fmt.Errorf("status.snapshotHandle not found in the original VSC")
		}
	} else {
		return nil, fmt.Errorf("status field not found in the original VSC")
	}
	spec["source"] = source

	// Set the required driver field
	if driver, ok := originalSpec["driver"]; ok {
		spec["driver"] = driver
	} else {
		return nil, fmt.Errorf("spec.driver not found in the original VSC")
	}

	// Set volumeSnapshotRef using the original values
	originalVolumeSnapshotRef := originalSpec["volumeSnapshotRef"].(map[string]interface{})

	// Get original name and remove any existing postfix (everything after last dash)
	originalName := originalVolumeSnapshotRef["name"].(string)
	if lastDashIndex := strings.LastIndex(originalName, "-"); lastDashIndex != -1 {
		originalName = originalName[:lastDashIndex]
	}
	newVSName := originalName + "-" + postfix

	spec["volumeSnapshotRef"] = map[string]interface{}{
		"name":      newVSName,
		"namespace": originalVolumeSnapshotRef["namespace"],
	}

	preProvisionedVSC["spec"] = spec

	// Set the correct apiVersion and kind
	preProvisionedVSC["apiVersion"] = "snapshot.storage.k8s.io/v1"
	preProvisionedVSC["kind"] = "VolumeSnapshotContent"

	stringifiedVSC, err := json.MarshalIndent(preProvisionedVSC, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshalIndent pre-provisioned VSC: %w", err)
	}

	log.Printf("Creating pre-provisioned VSC '%s' targeting VS '%s' in namespace=%s...\n%s", newVSCName, newVSName, originalVolumeSnapshotRef["namespace"], string(stringifiedVSC))

	// Create the pre-provisioned VSC
	vscJSON, err := json.Marshal(preProvisionedVSC)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal pre-provisioned VSC: %w", err)
	}

	cmd := exec.Command("kubectl", "create", "-f", "-")
	cmd.Stdin = bytes.NewReader(vscJSON)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to create pre-provisioned VSC: %w, output: %s", err, output)
	}

	// Fetch the created pre-provisioned VSC
	createdVSC, err := GetVolumeSnapshotContentObject(metadata["name"].(string))
	if err != nil {
		return nil, fmt.Errorf("failed to get created pre-provisioned VSC: %w", err)
	}

	return createdVSC, nil
}

func DeleteVolumeSnapshotContent(volumeSnapshotContentName string) error {
	deleteCmd := exec.Command("kubectl", "delete", "volumesnapshotcontent", volumeSnapshotContentName)
	output, err := deleteCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to delete VolumeSnapshotContent: %w, output: %s", err, output)
	}
	return nil
}

// func deepCopy(src, dst map[string]interface{}) {
// 	for k, v := range src {
// 		switch v := v.(type) {
// 		case map[string]interface{}:
// 			dst[k] = make(map[string]interface{})
// 			deepCopy(v, dst[k].(map[string]interface{}))
// 		case []interface{}:
// 			dst[k] = make([]interface{}, len(v))
// 			copy(dst[k].([]interface{}), v)
// 		default:
// 			dst[k] = v
// 		}
// 	}
// }
