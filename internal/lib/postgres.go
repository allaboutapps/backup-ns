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

//go:embed templates/postgres_check.sh.tmpl
var postgresCheckScript string

//go:embed templates/postgres_backup.sh.tmpl
var postgresBackupScript string

func EnsurePostgresAvailable(namespace string, config PostgresConfig) error {
	log.Printf("Checking if Postgres is available in namespace '%s'...", namespace)

	tmpl, err := template.New("postgres_check").Parse(postgresCheckScript)
	if err != nil {
		return fmt.Errorf("failed to parse Postgres check script template: %w", err)
	}

	var script bytes.Buffer
	if err := tmpl.Execute(&script, config); err != nil {
		return fmt.Errorf("failed to execute Postgres check script template: %w", err)
	}

	// #nosec G204
	cmd := exec.Command("kubectl", "exec", "-n", namespace, config.ExecResource, "-c", config.ExecContainer, "--", "bash", "-c", script.String())
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("Error checking Postgres availability: %w\nOutput: %s", err, string(output))
	}
	log.Printf("Postgres is available in namespace '%s'. Output:\n%s", namespace, string(output))
	return nil
}

// TODO: find a way to kill the remote process (e.g. pgdump / mysqldump) the exec command started
// in the case if origin process on the host terminates (or if we lose connection?)
func BackupPostgres(namespace string, dryRun bool, config PostgresConfig) error {
	if dryRun {
		log.Println("Skipping Postgres backup - dry run mode is active")
		return nil
	}
	log.Printf("Backing up Postgres database '%s' in namespace '%s'...", config.DB, namespace)

	// Create template data with computed fields
	type templateData struct {
		PostgresConfig
		DumpFileDir string
	}
	data := templateData{
		PostgresConfig: config,
		DumpFileDir:    filepath.Dir(config.DumpFile),
	}

	tmpl, err := template.New("postgres_backup").Parse(postgresBackupScript)
	if err != nil {
		return fmt.Errorf("failed to parse Postgres backup script template: %w", err)
	}

	var script bytes.Buffer
	if err := tmpl.Execute(&script, data); err != nil {
		return fmt.Errorf("failed to execute Postgres backup script template: %w", err)
	}

	// #nosec G204
	cmd := exec.Command("kubectl", "exec", "-n", namespace, config.ExecResource, "-c", config.ExecContainer, "--", "bash", "-c", script.String())
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("Error backing up Postgres: %w\nOutput: %s", err, string(output))
	}
	log.Printf("Postgres backup completed. Output:\n%s", string(output))
	return nil
}
