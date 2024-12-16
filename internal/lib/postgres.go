package lib

import (
	_ "embed"
	"fmt"
	"log"
	"path/filepath"
	"text/template"
)

//go:embed templates/postgres_check.sh.tmpl
var postgresCheckScript string

//go:embed templates/postgres_dump.sh.tmpl
var postgresDumpScript string

//go:embed templates/postgres_restore.sh.tmpl
var postgresRestoreScript string

func EnsurePostgresAvailable(namespace string, config PostgresConfig) error {
	log.Printf("Checking if Postgres is available in namespace '%s'...", namespace)

	tmpl, err := template.New("postgres_check").Parse(postgresCheckScript)
	if err != nil {
		return fmt.Errorf("failed to parse Postgres check script template: %w", err)
	}

	return KubectlExecTemplate(namespace, config.ExecResource, config.ExecContainer, tmpl, config)
}

func DumpPostgres(namespace string, dryRun bool, config PostgresConfig) error {
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

	tmpl, err := template.New("postgres_backup").Parse(postgresDumpScript)
	if err != nil {
		return fmt.Errorf("failed to parse Postgres backup script template: %w", err)
	}

	return KubectlExecTemplate(namespace, config.ExecResource, config.ExecContainer, tmpl, data)
}

func RestorePostgres(namespace string, dryRun bool, config PostgresConfig) error {
	if dryRun {
		log.Println("Skipping Postgres restore - dry run mode is active")
		return nil
	}
	log.Printf("Restoring Postgres database '%s' in namespace '%s'...", config.DB, namespace)

	tmpl, err := template.New("postgres_restore").Parse(postgresRestoreScript)
	if err != nil {
		return fmt.Errorf("failed to parse postgres restore script template: %w", err)
	}

	return KubectlExecTemplate(namespace, config.ExecResource, config.ExecContainer, tmpl, config)
}
