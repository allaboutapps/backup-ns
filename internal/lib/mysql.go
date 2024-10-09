package lib

import (
	"fmt"
	"log"
	"os/exec"
	"path/filepath"
)

type MySQLConfig struct {
	Enabled       bool
	ExecResource  string
	ExecContainer string
	DumpFile      string
	Host          string
	User          string
	Password      string `json:"-"` // sensitive
	DB            string
}

func EnsureMySQLAvailable(namespace string, config MySQLConfig) {
	log.Printf("Checking if MySQL is available in namespace '%s'...", namespace)

	script := fmt.Sprintf(`
		# inject default MYSQL_PWD into current env (before cmds are visible in logs)
		export MYSQL_PWD=%s

		set -Eeox pipefail

		# check clis are available
		command -v gzip
		mysql --version
		mysqldump --version

		# check db is accessible (default password injected via above MYSQL_PWD)
		mysql \
			--host %s \
			--user %s \
			--default-character-set=utf8 \
			%s \
			-e "SELECT 1;" >/dev/null
	`, config.Password, config.Host, config.User, config.DB)

	// #nosec G204
	cmd := exec.Command("kubectl", "exec", "-n", namespace, config.ExecResource, "-c", config.ExecContainer, "--", "bash", "-c", script)
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("Error checking MySQL availability: %v\nOutput: %s", err, string(output))
		log.Fatalf("MySQL not available in namespace '%s'", namespace)
	}
	log.Printf("MySQL is available in namespace '%s'. Output:\n%s", namespace, string(output))
}

func BackupMySQL(namespace string, dryRun bool, config MySQLConfig) {
	if dryRun {
		log.Println("Skipping MySQL backup - dry run mode is active")
		return
	}
	log.Printf("Backing up MySQL database '%s' in namespace '%s'...", config.DB, namespace)

	script := fmt.Sprintf(`
		# inject default MYSQL_PWD into current env (before cmds are visible in logs)
		export MYSQL_PWD=%s

		set -Eeox pipefail
		
        # setup trap in case of dump failure to disk (typically due to disk space issues)
        # we will automatically remove the dump file in case of failure!
		trap 'exit_code=$?; [ $exit_code -ne 0 ] && echo "TRAP!" && rm -f %s && df -h %s; exit $exit_code' EXIT
		
		# create dump and pipe to gzip archive (default password injected via above MYSQL_PWD)
		mysqldump \
            --host %s \
            --user %s \
            --default-character-set=utf8 \
            --add-locks \
            --set-charset \
            --compact \
            --create-options \
            --add-drop-table \
            --lock-tables \
            %s \
            | gzip -c > %s
		
		# print dump file info
		ls -lha %s
		
		# ensure generated file is bigger than 0 bytes
		[ -s %s ] || exit 1
		
		# print mounted disk space
		df -h %s
	`, config.Password, config.DumpFile, filepath.Dir(config.DumpFile),
		config.Host, config.User, config.DB, config.DumpFile,
		config.DumpFile, config.DumpFile, filepath.Dir(config.DumpFile))

	// #nosec G204
	cmd := exec.Command("kubectl", "exec", "-n", namespace, config.ExecResource, "-c", config.ExecContainer, "--", "bash", "-c", script)
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("Error backing up MySQL: %v\nOutput: %s", err, string(output))
		log.Fatal("MySQL backup failed")
	}
	log.Printf("MySQL backup completed. Output:\n%s", string(output))
}
