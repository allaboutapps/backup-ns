package lib

import (
	"encoding/json"
	"fmt"
	"log"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

func EnsureResourceAvailable(namespace, resource string) error {
	log.Printf("Checking if resource '%s' exists in namespace '%s'...", resource, namespace)

	cmd := exec.Command("kubectl", "get", "-n", namespace, resource, "-o", "wide")
	output, err := cmd.CombinedOutput()

	if err != nil {
		return fmt.Errorf("Error checking resource availability: %w\nOutput: %s", err, string(output))
	}
	log.Printf("Resource '%s' is available in namespace '%s'. Output:\n%s", resource, namespace, string(output))
	return nil
}

func GetCurrentNamespace() (string, error) {
	cmd := exec.Command("kubectl", "config", "view", "--minify", "--output", "jsonpath={..namespace}")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("error getting current namespace: %w", err)
	}
	return string(output), nil
}

func GetRemoteFileTimestamp(namespace, execResource, execContainer, absolutePathToFile string) (time.Time, error) {
	cmd := exec.Command("kubectl",
		"exec",
		"-n", namespace,
		"-c", execContainer,
		execResource,
		"--",
		"stat",
		"-c", "%Y",
		absolutePathToFile,
	)

	output, err := cmd.Output()
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to get file timestamp: %w", err)
	}

	unixTimestamp, err := strconv.ParseInt(strings.TrimSpace(string(output)), 10, 64)
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to parse timestamp: %w", err)
	}

	return time.Unix(unixTimestamp, 0), nil
}

// Returns a --selector compatible string (e.g. app=postgres) from a resource in the format kind/name
func GetSelectorFromResource(namespace, resource string) (string, error) {
	parts := strings.Split(resource, "/")
	if len(parts) != 2 {
		return "", fmt.Errorf("invalid resource format, expected kind/name, got: %s", resource)
	}

	// Get selector from resource
	cmd := exec.Command("kubectl",
		"get",
		"-n", namespace,
		resource,
		"-o", "jsonpath={.spec.selector.matchLabels}")

	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get resource selector: %w", err)
	}

	// Parse JSON map
	var labels map[string]string
	if err := json.Unmarshal(output, &labels); err != nil {
		return "", fmt.Errorf("failed to parse selector labels: %w", err)
	}

	// Convert to key=value format
	var selectorParts []string
	for k, v := range labels {
		selectorParts = append(selectorParts, fmt.Sprintf("%s=%s", k, v))
	}

	return strings.Join(selectorParts, ","), nil
}

func GetPodFromResource(namespace, resource string) (string, error) {
	selector, err := GetSelectorFromResource(namespace, resource)

	if err != nil {
		return "", err
	}

	// Get first pod using selector
	podCmd := exec.Command("kubectl",
		"get",
		"pods",
		"-n", namespace,
		"--selector", selector,
		"-o", "jsonpath={.items[0].metadata.name}")

	podOutput, err := podCmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get pod name: %w", err)
	}

	podName := strings.TrimSpace(string(podOutput))
	if podName == "" {
		return "", fmt.Errorf("no pod found for %s in namespace %s", resource, namespace)
	}

	return podName, nil
}
