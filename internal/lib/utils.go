package lib

import (
	"fmt"
	"log"
	"os/exec"
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
