package lib

import (
	_ "embed"
	"fmt"
	"log"
	"path/filepath"
	"text/template"
)

//go:embed templates/mysql_check.sh.tmpl
var mysqlCheckScript string

//go:embed templates/mysql_dump.sh.tmpl
var mysqlDumpScript string

func EnsureMySQLAvailable(namespace string, config MySQLConfig) error {
	log.Printf("Checking if MySQL is available in namespace '%s'...", namespace)

	tmpl, err := template.New("mysql_check").Parse(mysqlCheckScript)
	if err != nil {
		return fmt.Errorf("failed to parse MySQL check script template: %w", err)
	}

	return KubectlExecTemplate(namespace, config.ExecResource, config.ExecContainer, tmpl, config)
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

	tmpl, err := template.New("mysql_backup").Parse(mysqlDumpScript)
	if err != nil {
		return fmt.Errorf("failed to parse MySQL backup script template: %w", err)
	}

	return KubectlExecTemplate(namespace, config.ExecResource, config.ExecContainer, tmpl, data)
}
