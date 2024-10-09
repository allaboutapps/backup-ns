package lib

import (
	"fmt"
	"log"
	"os/exec"
	"path/filepath"
)

func EnsureMySQLAvailable(config Config) {
	log.Printf("Checking if MySQL is available in namespace '%s'...", config.Namespace)

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
	`, config.DBMySQLPassword, config.DBMySQLHost, config.DBMySQLUser, config.DBMySQLDB)

	// #nosec G204
	cmd := exec.Command("kubectl", "exec", "-n", config.Namespace, config.DBMySQLExecResource, "-c", config.DBMySQLExecContainer, "--", "bash", "-c", script)
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("Error checking MySQL availability: %v\nOutput: %s", err, string(output))
		log.Fatalf("MySQL not available in namespace '%s'", config.Namespace)
	}
	log.Printf("MySQL is available in namespace '%s'. Output:\n%s", config.Namespace, string(output))
}

func BackupMySQL(config Config) {
	if config.DryRun {
		log.Println("Skipping MySQL backup - dry run mode is active")
		return
	}
	log.Printf("Backing up MySQL database '%s' in namespace '%s'...", config.DBMySQLDB, config.Namespace)

	script := fmt.Sprintf(`
		# inject default MYSQL_PWD into current env (before cmds are visible in logs)
		export MYSQL_PWD=%s

		set -Eeox pipefail
		
        # setup trap in case of dump failure to disk (typically due to disk space issues)
        # we will automatically remove the dump file in case of failure!
		trap 'exit_code=$?; [ $exit_code -ne 0 ] && echo "TRAP!" && rm -f %s && df -h %s; exit $exit_code' EXIT
		
		# create dump and pipe to gzip archive (default password injected via above MYSQL_PWD)
		mysqldump \
            --host%s \
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
	`, config.DBMySQLPassword, config.DBMySQLDumpFile, filepath.Dir(config.DBMySQLDumpFile),
		config.DBMySQLHost, config.DBMySQLUser, config.DBMySQLDB, config.DBMySQLDumpFile,
		config.DBMySQLDumpFile, config.DBMySQLDumpFile, filepath.Dir(config.DBMySQLDumpFile))

	// #nosec G204
	cmd := exec.Command("kubectl", "exec", "-n", config.Namespace, config.DBMySQLExecResource, "-c", config.DBMySQLExecContainer, "--", "bash", "-c", script)
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("Error backing up MySQL: %v\nOutput: %s", err, string(output))
		log.Fatal("MySQL backup failed")
	}
	log.Printf("MySQL backup completed. Output:\n%s", string(output))
}
