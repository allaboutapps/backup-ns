package lib

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

func GenerateVSName(config Config) string {
	vsName := config.VSNameTemplate
	vsName = strings.ReplaceAll(vsName, "${BAK_PVC_NAME}", config.PVCName)
	vsName = strings.ReplaceAll(vsName, "${BAK_VS_RAND}", config.VSRand)
	vsName = strings.ReplaceAll(vsName, "$(date +\"%Y-%m-%d-%H%M%S\")", time.Now().Format("2006-01-02-150405"))
	return vsName
}

func GenerateVSLabels(config Config) map[string]string {
	labels := map[string]string{
		"backup-ns.sh/pvc":  config.PVCName,
		"backup-ns.sh/type": config.LabelVSType,
	}
	if config.LabelVSPod != "" {
		labels["backup-ns.sh/pod"] = config.LabelVSPod
	}
	if config.LabelVSRetain == "daily_weekly_monthly" {
		now := time.Now()
		labels["backup-ns.sh/retain"] = "daily_weekly_monthly"

		dailyLabel := now.Format("2006-01-02")

		_, week := time.Now().ISOWeek()
		weeklyLabel := now.Format("2006-") + fmt.Sprintf("w%02d", week)
		monthlyLabel := now.Format("2006-01")

		if !volumeSnapshotWithLabelValueExists(config.Namespace, "backup-ns.sh/daily", dailyLabel) {
			labels["backup-ns.sh/daily"] = dailyLabel
		}
		if !volumeSnapshotWithLabelValueExists(config.Namespace, "backup-ns.sh/weekly", weeklyLabel) {
			labels["backup-ns.sh/weekly"] = weeklyLabel
		}
		if !volumeSnapshotWithLabelValueExists(config.Namespace, "backup-ns.sh/monthly", monthlyLabel) {
			labels["backup-ns.sh/monthly"] = monthlyLabel
		}
	} else if config.LabelVSRetain == "days" {
		deleteAfter := time.Now().AddDate(0, 0, config.LabelVSRetainDays).Format("2006-01-02")
		labels["backup-ns.sh/retain"] = "days"
		labels["backup-ns.sh/retain-days"] = strconv.Itoa(config.LabelVSRetainDays)
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

func GenerateVSAnnotations(config Config) map[string]string {
	annotations := map[string]string{
		"backup-ns.sh/env-config": fmt.Sprintf(`BAK_DB_POSTGRES=%t
BAK_DB_POSTGRES_EXEC_RESOURCE=%s
BAK_DB_POSTGRES_EXEC_CONTAINER=%s
BAK_DB_POSTGRES_DUMP_FILE=%s
BAK_DB_POSTGRES_USER=%s
BAK_DB_POSTGRES_DB=%s
BAK_DB_MYSQL=%t
BAK_DB_MYSQL_EXEC_RESOURCE=%s
BAK_DB_MYSQL_EXEC_CONTAINER=%s
BAK_DB_MYSQL_DUMP_FILE=%s
BAK_DB_MYSQL_HOST=%s
BAK_DB_MYSQL_USER=%s
BAK_DB_MYSQL_DB=%s`,
			config.DBPostgres, config.DBPostgresExecResource, config.DBPostgresExecContainer, config.DBPostgresDumpFile, config.DBPostgresUser, config.DBPostgresDB,
			config.DBMySQL, config.DBMySQLExecResource, config.DBMySQLExecContainer, config.DBMySQLDumpFile, config.DBMySQLHost, config.DBMySQLUser, config.DBMySQLDB),
	}
	return annotations
}

func GenerateVSObject(config Config, vsName string, labels, annotations map[string]string) map[string]interface{} {
	return map[string]interface{}{
		"apiVersion": "snapshot.storage.k8s.io/v1",
		"kind":       "VolumeSnapshot",
		"metadata": map[string]interface{}{
			"name":        vsName,
			"namespace":   config.Namespace,
			"labels":      labels,
			"annotations": annotations,
		},
		"spec": map[string]interface{}{
			"volumeSnapshotClassName": config.VSClassName,
			"source": map[string]interface{}{
				"persistentVolumeClaimName": config.PVCName,
			},
		},
	}
}

func CreateVolumeSnapshot(config Config, vsName string, vsObject map[string]interface{}) {
	stringifiedVSObject, err := json.MarshalIndent(vsObject, "", "  ")
	if err != nil {
		log.Fatalf("Error marshaling VolumeSnapshot object: %v", err)
	}

	log.Printf("Creating VolumeSnapshot '%s' in namespace '%s'...\n%s", vsName, config.Namespace, string(stringifiedVSObject))

	if config.DryRun {
		log.Println("Skipping VolumeSnapshot creation - dry run mode is active")
		return
	}

	vsJSON, err := json.Marshal(vsObject)
	if err != nil {
		log.Fatalf("Error marshaling VolumeSnapshot object: %v", err)
	}

	// #nosec G204
	cmd := exec.Command("kubectl", "apply", "-f", "-", "-n", config.Namespace)
	cmd.Stdin = bytes.NewReader(vsJSON)
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Fatalf("Error creating VolumeSnapshot: %v. Output:\n%s", err, string(output))
	}

	if config.VSWaitUntilReady {
		log.Printf("Waiting for VolumeSnapshot '%s' to be ready (timeout: %s)...", vsName, config.VSWaitUntilReadyTimeout)

		// give kubectl some time to actually have a status field to wait for
		// https://github.com/kubernetes/kubectl/issues/1204
		// https://github.com/kubernetes/kubernetes/pull/109525
		time.Sleep(5 * time.Second)

		// #nosec G204
		cmd = exec.Command("kubectl", "wait", "--for=jsonpath={.status.readyToUse}=true", "--timeout", config.VSWaitUntilReadyTimeout, "volumesnapshot/"+vsName, "-n", config.Namespace)
		// log.Println(cmd.String())
		output, err := cmd.CombinedOutput()
		if err != nil {
			log.Printf("Warning: VolumeSnapshot '%s' may not be ready: %v. Output:\n%s", vsName, err, string(output))
		}
	}

	// #nosec G204
	cmd = exec.Command("kubectl", "get", "volumesnapshot/"+vsName, "-n", config.Namespace)
	output, err = cmd.CombinedOutput()
	if err != nil {
		log.Printf("Warning: Error getting VolumeSnapshot details: %v. Output:\n%s", err, string(output))
	} else {
		log.Printf("VolumeSnapshot details:\n%s", string(output))
	}
}
