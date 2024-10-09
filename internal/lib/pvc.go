package lib

import (
	"log"
	"os/exec"
	"strconv"
	"strings"
)

func EnsurePVCAvailable(config Config) {
	log.Printf("Checking if PVC '%s' exists in namespace '%s'...", config.PVCName, config.Namespace)
	// #nosec G204
	cmd := exec.Command("kubectl", "get", "pvc", config.PVCName, "-n", config.Namespace)
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Fatalf("PVC '%s' not found in namespace '%s'", config.PVCName, config.Namespace)
	}
	log.Printf("PVC '%s' is available in namespace '%s'. Output:\n%s", config.PVCName, config.Namespace, string(output))
}

func EnsureFreeSpace(config Config, resource, container, dir string) {
	log.Printf("Checking free space on %s in namespace '%s'...", dir, config.Namespace)
	// #nosec G204
	cmd := exec.Command("kubectl", "exec", "-n", config.Namespace, resource, "-c", container, "--", "df", "-h", dir)
	output, err := cmd.Output()
	if err != nil {
		log.Fatalf("Error checking free space: %v", err)
	}
	lines := strings.Split(string(output), "\n")
	if len(lines) < 2 {
		log.Fatalf("Unexpected df output: %s", string(output))
	}
	fields := strings.Fields(lines[1])
	if len(fields) < 5 {
		log.Fatalf("Unexpected df output: %s", string(output))
	}
	usedPercent, err := strconv.Atoi(strings.TrimRight(fields[4], "%"))
	if err != nil {
		log.Fatalf("Error parsing used percentage: %v", err)
	}
	if usedPercent >= config.ThresholdSpaceUsedPercent {
		log.Fatalf("Not enough free space. Used: %d%%, Threshold: %d%%", usedPercent, config.ThresholdSpaceUsedPercent)
	}

	log.Printf("Free space check succeeded. Used: %d%%, Threshold: %d%%. Output:\n%s", usedPercent, config.ThresholdSpaceUsedPercent, string(output))
}
