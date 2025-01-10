package lib

import (
	"bytes"
	"cmp"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"math"
	"os/exec"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"
)

type NamespacedK8sObject struct {
	Namespace string
	Name      string
}

// We expect all VS that we manage to have to "backup-ns.sh/type" label key.
func GetManagedVolumeSnapshots() ([]NamespacedK8sObject, error) {
	cmd := exec.Command("kubectl", "get", "volumesnapshot", "--all-namespaces", "-lbackup-ns.sh/type", "-o=jsonpath={range .items[*]}{.metadata.namespace} {.metadata.name}{\"\\n\"}{end}")
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		return nil, err
	}
	vsLines := strings.Split(strings.TrimSpace(out.String()), "\n")

	vss := make([]NamespacedK8sObject, 0, len(vsLines))

	for _, line := range vsLines {
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}

		vss = append(vss, NamespacedK8sObject{
			Namespace: parts[0],
			Name:      parts[1],
		})
	}

	slices.SortFunc(vss, func(a, b NamespacedK8sObject) int {
		return strings.Compare(strings.ToLower(a.Namespace+a.Name), strings.ToLower(b.Namespace+b.Name))
	})

	return vss, nil
}

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

func GenerateVSLabels(namespace, pvcName string, config LabelVSConfig, now time.Time) map[string]string {
	labels := map[string]string{
		"backup-ns.sh/pvc":  pvcName,
		"backup-ns.sh/type": config.Type,
	}
	if config.Pod != "" {
		labels["backup-ns.sh/pod"] = config.Pod
	}
	if config.Retain == "daily_weekly_monthly" {
		now := now
		labels["backup-ns.sh/retain"] = "daily_weekly_monthly"

		dailyLabel := now.Format("2006-01-02")

		_, week := now.ISOWeek()
		weeklyLabel := fmt.Sprintf("w%02d", week)
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
		deleteAfter := now.AddDate(0, 0, config.RetainDays).Format("2006-01-02")
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

	for _, key := range sortedKeys(bakEnvVars) {
		envConfigLines = append(envConfigLines, fmt.Sprintf("%s='%s'", key, bakEnvVars[key]))
	}

	annotations := map[string]string{
		"backup-ns.sh/env-config": strings.Join(envConfigLines, "\n"),
	}
	return annotations
}

// https://stackoverflow.com/questions/18342784/how-to-iterate-through-a-map-in-golang-in-order
func sortedKeys[K cmp.Ordered, V any](m map[K]V) []K {
	keys := make([]K, len(m))
	i := 0
	for k := range m {
		keys[i] = k
		i++
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	return keys
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

func GenerateVSObjectFromVSC(vscName string, vscObject map[string]interface{}) (map[string]interface{}, error) {
	vsName, ok := vscObject["spec"].(map[string]interface{})["volumeSnapshotRef"].(map[string]interface{})["name"].(string)
	if !ok {
		return nil, fmt.Errorf("failed to get VolumeSnapshot name from VolumeSnapshotContent")
	}

	namespace, ok := vscObject["spec"].(map[string]interface{})["volumeSnapshotRef"].(map[string]interface{})["namespace"].(string)
	if !ok {
		return nil, fmt.Errorf("failed to get namespace from VolumeSnapshotContent")
	}

	labels, ok := vscObject["metadata"].(map[string]interface{})["labels"].(map[string]interface{})
	if !ok {
		labels = make(map[string]interface{})
	}

	// pvcName, ok := labels["backup-ns.sh/pvc"].(string)
	// if !ok {
	// 	return nil, fmt.Errorf("failed to get PVC name from VolumeSnapshotContent labels")
	// }

	vsClassName, ok := vscObject["spec"].(map[string]interface{})["volumeSnapshotClassName"].(string)
	if !ok {
		return nil, fmt.Errorf("failed to get vsClassName from VolumeSnapshotContent")
	}

	manifest := map[string]interface{}{
		"apiVersion": "snapshot.storage.k8s.io/v1",
		"kind":       "VolumeSnapshot",
		"metadata": map[string]interface{}{
			"name":      vsName,
			"namespace": namespace,
			"labels":    labels,
		},
		"spec": map[string]interface{}{
			"source": map[string]interface{}{
				"volumeSnapshotContentName": vscName,
			},
			"volumeSnapshotClassName": vsClassName,
		},
	}

	return manifest, nil
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
		// time.Sleep(5 * time.Second)

		maxRetries := 3
		baseDelay := time.Second
		maxDelay := 5 * time.Second

		for attempt := 0; attempt < maxRetries; attempt++ {
			// #nosec G204
			cmd = exec.Command("kubectl", "wait", "--for=jsonpath={.status.readyToUse}=true", "--timeout", waitTimeout, "volumesnapshot/"+vsName, "-n", namespace)
			output, err := cmd.CombinedOutput()
			if err == nil {
				// Success, break the retry loop
				break
			}

			if attempt == maxRetries-1 {
				// Last attempt failed
				return fmt.Errorf("VolumeSnapshot '%s' did not become ready after %d attempts: %w. Output:\n%s", vsName, maxRetries, err, string(output))
			}

			// Calculate delay with exponential backoff
			delay := time.Duration(math.Pow(2, float64(attempt))) * baseDelay
			if delay > maxDelay {
				delay = maxDelay
			}

			log.Printf("Attempt %d failed. Retrying in %v...", attempt+1, delay)
			time.Sleep(delay)
		}

		// // #nosec G204
		// cmd = exec.Command("kubectl", "wait", "--for=jsonpath={.status.readyToUse}=true", "--timeout", waitTimeout, "volumesnapshot/"+vsName, "-n", namespace)
		// // log.Println(cmd.String())
		// output, err := cmd.CombinedOutput()
		// if err != nil {
		// 	return fmt.Errorf("VolumeSnapshot '%s' did not become ready: %w. Output:\n%s", vsName, err, string(output))
		// }
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

func deleteVolumeSnapshot(namespace, volumeSnapshotName string, wait bool) error {
	args := []string{"delete", "volumesnapshot", volumeSnapshotName, "-n", namespace}
	if !wait {
		args = append(args, "--wait=false")
	}

	deleteCmd := exec.Command("kubectl", args...)
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
func PruneVolumeSnapshot(namespace, volumeSnapshotName string, wait bool) error {
	// Get the VolumeSnapshotContent name
	vscName, err := GetVolumeSnapshotContentName(namespace, volumeSnapshotName)
	if err != nil {
		return err
	}

	// Patch the VolumeSnapshotContent to set deletionPolicy to Delete
	if err := patchVolumeSnapshotContentDeletionPolicy(vscName); err != nil {
		return err
	}

	// Delete the VolumeSnapshot
	if err := deleteVolumeSnapshot(namespace, volumeSnapshotName, wait); err != nil {
		return err
	}

	return nil
}

// RestoreVolumeSnapshot creates a new PVC from a VolumeSnapshot
func RestoreVolumeSnapshot(namespace string, dryRun bool, vsName string, pvcName string, storageClass string, wait bool, waitTimeout string) error {
	// Create PVC manifest
	pvcObject := map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "PersistentVolumeClaim",
		"metadata": map[string]interface{}{
			"name":      pvcName,
			"namespace": namespace,
		},
		"spec": map[string]interface{}{
			"dataSource": map[string]interface{}{
				"name":     vsName,
				"kind":     "VolumeSnapshot",
				"apiGroup": "snapshot.storage.k8s.io",
			},
			"accessModes": []string{"ReadWriteOnce"},
		},
	}

	if storageClass != "" {
		pvcObject["spec"].(map[string]interface{})["storageClassName"] = storageClass
	}

	// Get the VolumeSnapshot to copy its storage request
	// #nosec G204
	cmd := exec.Command("kubectl", "get", "volumesnapshot", vsName, "-n", namespace, "-o", "json")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("error getting VolumeSnapshot details: %w. Output:\n%s", err, string(output))
	}

	var vs map[string]interface{}
	if err := json.Unmarshal(output, &vs); err != nil {
		return fmt.Errorf("error unmarshaling VolumeSnapshot: %w", err)
	}

	// Get the storage size from the VolumeSnapshot status
	if status, ok := vs["status"].(map[string]interface{}); ok {
		if restoreSize, ok := status["restoreSize"].(string); ok {
			pvcObject["spec"].(map[string]interface{})["resources"] = map[string]interface{}{
				"requests": map[string]interface{}{
					"storage": restoreSize,
				},
			}
		}
	}

	stringifiedPVCObject, err := json.MarshalIndent(pvcObject, "", "  ")
	if err != nil {
		return fmt.Errorf("error marshaling PVC object: %w", err)
	}

	log.Printf("Creating PVC '%s' in namespace '%s' from VolumeSnapshot '%s'...\n%s",
		pvcName, namespace, vsName, string(stringifiedPVCObject))

	if dryRun {
		log.Println("Skipping PVC creation - dry run mode is active")
		return nil
	}

	pvcJSON, err := json.Marshal(pvcObject)
	if err != nil {
		return fmt.Errorf("error marshaling PVC object: %w", err)
	}

	cmd = exec.Command("kubectl", "apply", "-f", "-", "-n", namespace)
	cmd.Stdin = bytes.NewReader(pvcJSON)
	output, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("error creating PVC: %w. Output:\n%s", err, string(output))
	}

	if wait {
		log.Printf("Waiting for PVC '%s' to be bound (timeout: %s)...", pvcName, waitTimeout)
		// #nosec G204
		cmd = exec.Command("kubectl", "wait", "--for=jsonpath={.status.phase}=Bound", "--timeout", waitTimeout,
			"pvc/"+pvcName, "-n", namespace)
		log.Println(cmd.Args)
		output, err = cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("PVC '%s' did not become bound: %w. Output:\n%s",
				pvcName, err, string(output))
		}
	}
	// #nosec G204
	cmd = exec.Command("kubectl", "get", "pvc/"+pvcName, "-n", namespace)
	output, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("error getting PVC details: %w. Output:\n%s", err, string(output))
	}

	log.Printf("PVC details:\n%s", string(output))
	return nil
}
