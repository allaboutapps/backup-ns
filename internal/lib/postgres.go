package lib

import (
	"log"
	"path/filepath"
)

func EnsurePostgresAvailable(namespace string, config PostgresConfig) error {
	log.Printf("Checking if Postgres is available in namespace '%s'...", namespace)

	return KubectlExecTemplate(namespace, config.ExecResource, config.ExecContainer, GetTemplateAtlas().PostgresCheck, config)
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

	return KubectlExecTemplate(namespace, config.ExecResource, config.ExecContainer, GetTemplateAtlas().PostgresDump, data)
}

func RestorePostgres(namespace string, dryRun bool, config PostgresConfig) error {
	if dryRun {
		log.Println("Skipping Postgres restore - dry run mode is active")
		return nil
	}
	log.Printf("Restoring Postgres database '%s' in namespace '%s'...", config.DB, namespace)

	return KubectlExecTemplate(namespace, config.ExecResource, config.ExecContainer, GetTemplateAtlas().PostgresRestore, config)
}
