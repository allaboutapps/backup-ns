package lib

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

func GenerateVSName(vsNameTemplate string, pvcName string, vsRand string) (string, error) {
	templ := template.Must(template.New("vsNameTemplate").Parse(vsNameTemplate))
	var buf bytes.Buffer

	err := templ.Execute(&buf, map[string]any{
		"pvcName":   pvcName,
		"timestamp": time.Now().Format("2006-01-02-150405"),
		"rand":      vsRand,
	})

	if err != nil {
		return "", fmt.Errorf("Error generating vsNameTemplate: %w", err)
	}

	return buf.String(), nil
}

func GenerateVSLabels(namespace, pvcName string, config LabelVSConfig) map[string]string {
	labels := map[string]string{
		"backup-ns.sh/pvc":  pvcName,
		"backup-ns.sh/type": config.Type,
	}
	if config.Pod != "" {
		labels["backup-ns.sh/pod"] = config.Pod
	}
	if config.Retain == "daily_weekly_monthly" {
		now := time.Now()
		labels["backup-ns.sh/retain"] = "daily_weekly_monthly"

		dailyLabel := now.Format("2006-01-02")

		_, week := time.Now().ISOWeek()
		weeklyLabel := now.Format("2006-") + fmt.Sprintf("w%02d", week)
		monthlyLabel := now.Format("2006-01")

		if !volumeSnapshotWithLabelValueExists(namespace, "backup-ns.sh/daily", dailyLabel) {
			labels["backup-ns.sh/daily"] = dailyLabel
		}
		if !volumeSnapshotWithLabelValueExists(namespace, "backup-ns.sh/weekly", weeklyLabel) {
			labels["backup-ns.sh/weekly"] = weeklyLabel
		}
		if !volumeSnapshotWithLabelValueExists(namespace, "backup-ns.sh/monthly", monthlyLabel) {
			labels["backup-ns.sh/monthly"] = monthlyLabel
		}
	} else if config.Retain == "days" {
		deleteAfter := time.Now().AddDate(0, 0, config.RetainDays).Format("2006-01-02")
		labels["backup-ns.sh/retain"] = "days"
		labels["backup-ns.sh/retain-days"] = strconv.Itoa(config.RetainDays)
		labels["backup-ns.sh/delete-after"] = deleteAfter
	}
	return labels
}

func volumeSnapshotWithLabelValueExists(namespace, labelKey, labelValue string) bool {
	// #nosec G204
	cmd := exec.Command("kubectl", "get", "volumesnapshot", "-n", namespace, "-l", fmt.Sprintf("%s=%s", labelKey, labelValue), "-o", "name")
	output, err := cmd.Output()
	if err != nil {
		log.Printf("Error checking for existing VolumeSnapshot: %v", err)
		return true // assume it exists to be safe, we don't want to delete existing snapshots by accident with the pruner!
	}
	return len(output) > 0
}

// typically we'll only save the used safe env vars inside the env-config annotation
func GenerateVSAnnotations(bakEnvVars map[string]string) map[string]string {
	var envConfigLines []string

	for key, value := range bakEnvVars {
		envConfigLines = append(envConfigLines, fmt.Sprintf("%s='%s'", key, value))
	}

	annotations := map[string]string{
		"backup-ns.sh/env-config": strings.Join(envConfigLines, "\n"),
	}
	return annotations
}

func GenerateVSObject(namespace, vsClassName, pvcName, vsName string, labels, annotations map[string]string) map[string]interface{} {
	manifest := map[string]interface{}{
		"apiVersion": "snapshot.storage.k8s.io/v1",
		"kind":       "VolumeSnapshot",
		"metadata": map[string]interface{}{
			"name":        vsName,
			"namespace":   namespace,
			"labels":      labels,
			"annotations": annotations,
		},
		"spec": map[string]interface{}{
			// "volumeSnapshotClassName": "", // optional
			"source": map[string]interface{}{
				"persistentVolumeClaimName": pvcName,
			},
		},
	}

	if vsClassName != "" {
		// else expect that the default VolumeSnapshotClass will automatically be chosen by the cluster
		manifest["spec"].(map[string]interface{})["volumeSnapshotClassName"] = vsClassName
	}

	return manifest
}

func CreateVolumeSnapshot(namespace string, dryRun bool, vsName string, vsObject map[string]interface{}, wait bool, waitTimeout string) error {
	stringifiedVSObject, err := json.MarshalIndent(vsObject, "", "  ")
	if err != nil {
		return fmt.Errorf("Error marshalIndent VolumeSnapshot object: %w", err)
	}

	log.Printf("Creating VolumeSnapshot '%s' in namespace '%s'...\n%s", vsName, namespace, string(stringifiedVSObject))

	if dryRun {
		log.Println("Skipping VolumeSnapshot creation - dry run mode is active")
		return nil
	}

	vsJSON, err := json.Marshal(vsObject)
	if err != nil {
		return fmt.Errorf("Error marshaling VolumeSnapshot object: %w", err)
	}

	// #nosec G204
	cmd := exec.Command("kubectl", "apply", "-f", "-", "-n", namespace)
	cmd.Stdin = bytes.NewReader(vsJSON)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("Error creating VolumeSnapshot: %w. Output:\n%s", err, string(output))
	}

	if wait {
		log.Printf("Waiting for VolumeSnapshot '%s' to be ready (timeout: %s)...", vsName, waitTimeout)

		// give kubectl some time to actually have a status field to wait for
		// https://github.com/kubernetes/kubectl/issues/1204
		// https://github.com/kubernetes/kubernetes/pull/109525
		time.Sleep(5 * time.Second)

		// #nosec G204
		cmd = exec.Command("kubectl", "wait", "--for=jsonpath={.status.readyToUse}=true", "--timeout", waitTimeout, "volumesnapshot/"+vsName, "-n", namespace)
		// log.Println(cmd.String())
		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("VolumeSnapshot '%s' did not become ready: %w. Output:\n%s", vsName, err, string(output))
		}
	}

	// #nosec G204
	cmd = exec.Command("kubectl", "get", "volumesnapshot/"+vsName, "-n", namespace)
	output, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("Error getting VolumeSnapshot details: %w. Output:\n%s", err, string(output))
	}

	log.Printf("VolumeSnapshot details:\n%s", string(output))
	return nil
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
	log.Printf("Successfully patched VolumeSnapshotContent %s deletionPolicy to 'Delete'\n", vscName)
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

// Dangerous!
// Delete a VolumeSnapshot, its associated VolumeSnapshotContent and the underlying storage!
// This is a destructive operation and should be used with caution!
// This function will set the deletionPolicy of the VolumeSnapshotContent to "Delete" before deleting the VolumeSnapshot, thus ensuring the underlying storage is also deleted.
func PruneVolumeSnapshot(namespace, volumeSnapshotName string) error {
	// Get the VolumeSnapshotContent name
	vscName, err := getVolumeSnapshotContentName(namespace, volumeSnapshotName)
	if err != nil {
		return err
	}

	// Patch the VolumeSnapshotContent to set deletionPolicy to Delete
	if err := patchVolumeSnapshotContent(vscName); err != nil {
		return err
	}

	// Delete the VolumeSnapshot
	if err := deleteVolumeSnapshot(namespace, volumeSnapshotName); err != nil {
		return err
	}

	return nil
}
