package lib

import (
	"embed"
	"log"
	"text/template"
)

//go:embed templates
var templates embed.FS

type TemplateAtlas struct {
	MySQLCheck      *template.Template
	MySQLDump       *template.Template
	MySQLRestore    *template.Template
	PostgresCheck   *template.Template
	PostgresDump    *template.Template
	PostgresRestore *template.Template
	TestTrap        *template.Template
}

var templateAtlas TemplateAtlas

func init() {
	tmpl, err := template.ParseFS(templates, "templates/*.sh.tmpl")

	if err != nil {
		log.Fatal("Failed to parse templates/*.sh.tmpl templates.")
	}

	templateAtlas = TemplateAtlas{
		MySQLCheck:      ensureChildTemplate(tmpl, "mysql_check.sh.tmpl"),
		MySQLDump:       ensureChildTemplate(tmpl, "mysql_dump.sh.tmpl"),
		MySQLRestore:    ensureChildTemplate(tmpl, "mysql_restore.sh.tmpl"),
		PostgresCheck:   ensureChildTemplate(tmpl, "postgres_check.sh.tmpl"),
		PostgresDump:    ensureChildTemplate(tmpl, "postgres_dump.sh.tmpl"),
		PostgresRestore: ensureChildTemplate(tmpl, "postgres_restore.sh.tmpl"),
		TestTrap:        ensureChildTemplate(tmpl, "test_trap.sh.tmpl"),
	}
}

func ensureChildTemplate(template *template.Template, name string) *template.Template {
	tmpl := template.Lookup(name)

	if tmpl == nil {
		log.Fatalf("Template '%s' not found.", name)
	}

	return tmpl
}

func GetTemplateAtlas() TemplateAtlas {
	return templateAtlas
}
