package lib

import (
	"log"
	"os/exec"
)

func EnsureResourceAvailable(namespace string, resource string) {
	log.Printf("Checking if resource '%s' exists in namespace '%s'...", resource, namespace)

	cmd := exec.Command("kubectl", "get", "-n", namespace, resource, "-o", "wide")
	output, err := cmd.CombinedOutput()

	if err != nil {
		log.Printf("Error checking resource availability: %v\nOutput: %s", err, string(output))
		log.Fatalf("Resource '%s' not available in namespace '%s'", resource, namespace)
	}
	log.Printf("Resource '%s' is available in namespace '%s'. Output:\n%s", resource, namespace, string(output))
}
