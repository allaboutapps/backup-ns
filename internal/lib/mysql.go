package lib

import (
	"bytes"
	_ "embed"
	"fmt"
	"log"
	"os/exec"
	"path/filepath"
	"text/template"
)

//go:embed templates/mysql_check.sh.tmpl
var mysqlCheckScript string

//go:embed templates/mysql_backup.sh.tmpl
var mysqlBackupScript string

func EnsureMySQLAvailable(namespace string, config MySQLConfig) error {
	log.Printf("Checking if MySQL is available in namespace '%s'...", namespace)

	tmpl, err := template.New("mysql_check").Parse(mysqlCheckScript)
	if err != nil {
		return fmt.Errorf("failed to parse MySQL check script template: %w", err)
	}

	var script bytes.Buffer
	if err := tmpl.Execute(&script, config); err != nil {
		return fmt.Errorf("failed to execute MySQL check script template: %w", err)
	}

	// #nosec G204
	cmd := exec.Command("kubectl", "exec", "-n", namespace, config.ExecResource, "-c", config.ExecContainer, "--", "bash", "-c", script.String())
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("Error checking MySQL availability: %w\nOutput: %s", err, string(output))
	}
	log.Printf("MySQL is available in namespace '%s'. Output:\n%s", namespace, string(output))
	return nil
}

func BackupMySQL(namespace string, dryRun bool, config MySQLConfig) error {
	if dryRun {
		log.Println("Skipping MySQL backup - dry run mode is active")
		return nil
	}
	log.Printf("Backing up MySQL database '%s' in namespace '%s'...", config.DB, namespace)

	// Create template data with computed fields
	type templateData struct {
		MySQLConfig
		DumpFileDir string
	}
	data := templateData{
		MySQLConfig: config,
		DumpFileDir: filepath.Dir(config.DumpFile),
	}

	tmpl, err := template.New("mysql_backup").Parse(mysqlBackupScript)
	if err != nil {
		return fmt.Errorf("failed to parse MySQL backup script template: %w", err)
	}

	var script bytes.Buffer
	if err := tmpl.Execute(&script, data); err != nil {
		return fmt.Errorf("failed to execute MySQL backup script template: %w", err)
	}

	// #nosec G204
	cmd := exec.Command("kubectl", "exec", "-n", namespace, config.ExecResource, "-c", config.ExecContainer, "--", "bash", "-c", script.String())
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("Error backing up MySQL: %w\nOutput: %s", err, string(output))
	}
	log.Printf("MySQL backup completed. Output:\n%s", string(output))
	return nil
}
