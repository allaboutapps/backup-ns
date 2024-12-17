package lib

import (
	"log"
	"path/filepath"
)

func EnsureMySQLAvailable(namespace string, config MySQLConfig) error {
	log.Printf("Checking if MySQL is available in namespace '%s'...", namespace)

	return KubectlExecTemplate(namespace, config.ExecResource, config.ExecContainer, GetTemplateAtlas().MySQLCheck, config)
}

func DumpMySQL(namespace string, dryRun bool, config MySQLConfig) error {
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

	return KubectlExecTemplate(namespace, config.ExecResource, config.ExecContainer, GetTemplateAtlas().MySQLDump, data)
}

func RestoreMySQL(namespace string, dryRun bool, config MySQLConfig) error {
	if dryRun {
		log.Println("Skipping MySQL restore - dry run mode is active")
		return nil
	}
	log.Printf("Restoring MySQL database '%s' in namespace '%s'...", config.DB, namespace)

	return KubectlExecTemplate(namespace, config.ExecResource, config.ExecContainer, GetTemplateAtlas().MySQLRestore, config)
}
