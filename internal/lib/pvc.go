package lib

import (
	"fmt"
	"log"
	"os/exec"
	"strconv"
	"strings"
)

func EnsurePVCAvailable(namespace, pvcName string) error {
	log.Printf("Checking if PVC '%s' exists in namespace '%s'...", pvcName, namespace)
	// #nosec G204
	cmd := exec.Command("kubectl", "get", "pvc", pvcName, "-n", namespace)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("PVC '%s' not found in namespace '%s': %w", pvcName, namespace, err)
	}
	log.Printf("PVC '%s' is available in namespace '%s'. Output:\n%s", pvcName, namespace, string(output))
	return nil
}

func EnsureFreeSpace(namespace, resource, container, dir string, thresholdSpaceUsedPercent int) error {
	log.Printf("Checking free space on %s in namespace '%s'...", dir, namespace)
	// #nosec G204
	cmd := exec.Command("kubectl", "exec", "-n", namespace, resource, "-c", container, "--", "df", "-h", dir)
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("Error checking free space: %w", err)
	}
	lines := strings.Split(string(output), "\n")
	if len(lines) < 2 {
		return fmt.Errorf("Unexpected df output: %s", string(output))
	}
	fields := strings.Fields(lines[1])
	if len(fields) < 5 {
		return fmt.Errorf("Unexpected df output: %s", string(output))
	}
	usedPercent, err := strconv.Atoi(strings.TrimRight(fields[4], "%"))
	if err != nil {
		return fmt.Errorf("Error parsing used percentage: %w", err)
	}
	if usedPercent >= thresholdSpaceUsedPercent {
		return fmt.Errorf("Not enough free space. Used: %d%%, Threshold: %d%%", usedPercent, thresholdSpaceUsedPercent)
	}

	log.Printf("Free space check succeeded. Used: %d%%, Threshold: %d%%. Output:\n%s", usedPercent, thresholdSpaceUsedPercent, string(output))
	return nil
}
