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

func GenerateVSName(vsNameTemplate string, pvcName string, vsRand string) string {
	templ := template.Must(template.New("vsNameTemplate").Parse(vsNameTemplate))
	var buf bytes.Buffer

	err := templ.Execute(&buf, map[string]any{
		"pvcName":   pvcName,
		"timestamp": time.Now().Format("2006-01-02-150405"),
		"rand":      vsRand,
	})

	if err != nil {
		log.Fatalf("Error generating vsNameTemplate: %v", err)
	}

	return buf.String()
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
		return false
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
			// "snapshotClassName": "", // optional
			"source": map[string]interface{}{
				"persistentVolumeClaimName": pvcName,
			},
		},
	}

	if vsClassName != "" {
		// else expect that the default VolumeSnapshotClass will automatically be chosen by the cluster
		manifest["spec"].(map[string]interface{})["snapshotClassName"] = vsClassName
	}

	return manifest
}

func CreateVolumeSnapshot(namespace string, dryRun bool, vsName string, vsObject map[string]interface{}, wait bool, waitTimeout string) {
	stringifiedVSObject, err := json.MarshalIndent(vsObject, "", "  ")
	if err != nil {
		log.Fatalf("Error marshaling VolumeSnapshot object: %v", err)
	}

	log.Printf("Creating VolumeSnapshot '%s' in namespace '%s'...\n%s", vsName, namespace, string(stringifiedVSObject))

	if dryRun {
		log.Println("Skipping VolumeSnapshot creation - dry run mode is active")
		return
	}

	vsJSON, err := json.Marshal(vsObject)
	if err != nil {
		log.Fatalf("Error marshaling VolumeSnapshot object: %v", err)
	}

	// #nosec G204
	cmd := exec.Command("kubectl", "apply", "-f", "-", "-n", namespace)
	cmd.Stdin = bytes.NewReader(vsJSON)
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Fatalf("Error creating VolumeSnapshot: %v. Output:\n%s", err, string(output))
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
			log.Printf("Warning: VolumeSnapshot '%s' may not be ready: %v. Output:\n%s", vsName, err, string(output))
		}
	}

	// #nosec G204
	cmd = exec.Command("kubectl", "get", "volumesnapshot/"+vsName, "-n", namespace)
	output, err = cmd.CombinedOutput()
	if err != nil {
		log.Printf("Warning: Error getting VolumeSnapshot details: %v. Output:\n%s", err, string(output))
	} else {
		log.Printf("VolumeSnapshot details:\n%s", string(output))
	}
}
