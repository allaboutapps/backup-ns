package lib

import (
	"fmt"
	"log"
	"os/exec"
	"path/filepath"
)

func EnsurePostgresAvailable(config Config) {
	log.Printf("Checking if Postgres is available in namespace '%s'...", config.Namespace)

	script := fmt.Sprintf(`
		# inject default PGPASSWORD into current env (before cmds are visible in logs)
		export PGPASSWORD=%s
		
		set -Eeox pipefail

		# check clis are available
		command -v gzip
		psql --version
		pg_dump --version

		# check db is accessible
		psql --username=%s %s -c "SELECT 1;" >/dev/null
	`, config.DBPostgresPassword, config.DBPostgresUser, config.DBPostgresDB)

	// #nosec G204
	cmd := exec.Command("kubectl", "exec", "-n", config.Namespace, config.DBPostgresExecResource, "-c", config.DBPostgresExecContainer, "--", "bash", "-c", script)
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("Error checking Postgres availability: %v\nOutput: %s", err, string(output))
		log.Fatalf("Postgres not available in namespace '%s'", config.Namespace)
	}
	log.Printf("Postgres is available in namespace '%s'. Output:\n%s", config.Namespace, string(output))
}

// TODO: find a way to kill the remote process (e.g. pgdump / mysqldump) the exec command started
// in the case if origin process on the host terminates (or if we lose connection?)
func BackupPostgres(config Config) {
	if config.DryRun {
		log.Println("Skipping Postgres backup - dry run mode is active")
		return
	}
	log.Printf("Backing up Postgres database '%s' in namespace '%s'...", config.DBPostgresDB, config.Namespace)

	script := fmt.Sprintf(`
		# inject default PGPASSWORD into current env (before cmds are visible in logs)
		export PGPASSWORD=%s

		set -Eeox pipefail

		# setup trap in case of dump failure to disk (typically due to disk space issues)
        # we will automatically remove the dump file in case of failure!
		trap 'exit_code=$?; [ $exit_code -ne 0 ] && echo "TRAP!" && rm -f %s && df -h %s; exit $exit_code' EXIT
		
		# create dump and pipe to gzip archive
		pg_dump --username=%s --format=p --clean --if-exists %s | gzip -c > %s
		
		# print dump file info
		ls -lha %s
		
		# ensure generated file is bigger than 0 bytes
		[ -s %s ] || exit 1
		
		# print mounted disk space
		df -h %s
	`, config.DBPostgresPassword, config.DBPostgresDumpFile, filepath.Dir(config.DBPostgresDumpFile),
		config.DBPostgresUser, config.DBPostgresDB, config.DBPostgresDumpFile,
		config.DBPostgresDumpFile, config.DBPostgresDumpFile, filepath.Dir(config.DBPostgresDumpFile))

	// #nosec G204
	cmd := exec.Command("kubectl", "exec", "-n", config.Namespace, config.DBPostgresExecResource, "-c", config.DBPostgresExecContainer, "--", "bash", "-c", script)
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("Error backing up Postgres: %v\nOutput: %s", err, string(output))
		log.Fatal("Postgres backup failed")
	}
	log.Printf("Postgres backup completed. Output:\n%s", string(output))
}
