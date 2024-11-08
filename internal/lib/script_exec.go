package lib

import (
	"bytes"
	"fmt"
	"log"
	"os/exec"
	"text/template"
)

func KubectlExecTemplate(namespace, execResource, execContainer string, tmpl *template.Template, templateData any) error {

	tmplName := tmpl.Name()

	var script bytes.Buffer
	if err := tmpl.Execute(&script, templateData); err != nil {
		return fmt.Errorf("Failed to populate data in templated script '%s': %w", tmplName, err)
	}

	// #nosec G204
	cmd := exec.Command("kubectl", "exec", "-i", "-n", namespace, execResource, "-c", execContainer, "--", "bash", "-s")
	cmd.Stdin = bytes.NewBufferString(script.String() + "\n")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("Error running templated script '%s': %w\nOutput: %s", tmplName, err, string(output))
	}
	log.Printf("Templated script '%s' completed. Output:\n%s", tmplName, string(output))
	return nil
}

func KubectlExecCommand(namespace, execResource, execContainer, command string) error {
	cmd := exec.Command("kubectl", "exec", "-n", namespace, execResource, "-c", execContainer, "--", "bash", "-c", command)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("Error executing command '%s': %w\nOutput: %s", command, err, string(output))
	}

	log.Printf("ExecCommand completed. Output:\n%s", string(output))
	return nil
}
