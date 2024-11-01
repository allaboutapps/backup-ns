package lib

import (
	"fmt"
	"log"
	"os/exec"
	"path/filepath"
)

func EnsurePostgresAvailable(namespace string, config PostgresConfig) error {
	log.Printf("Checking if Postgres is available in namespace '%s'...", namespace)

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
	`, config.Password, config.User, config.DB)

	// #nosec G204
	cmd := exec.Command("kubectl", "exec", "-n", namespace, config.ExecResource, "-c", config.ExecContainer, "--", "bash", "-c", script)
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
	`, config.Password, config.DumpFile, filepath.Dir(config.DumpFile),
		config.User, config.DB, config.DumpFile,
		config.DumpFile, config.DumpFile, filepath.Dir(config.DumpFile))

	// #nosec G204
	cmd := exec.Command("kubectl", "exec", "-n", namespace, config.ExecResource, "-c", config.ExecContainer, "--", "bash", "-c", script)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("Error backing up Postgres: %w\nOutput: %s", err, string(output))
	}
	log.Printf("Postgres backup completed. Output:\n%s", string(output))
	return nil
}
