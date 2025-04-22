package cmd

import (
	"fmt"
	"log"
	"os"
	"os/exec"

	"github.com/allaboutapps/backup-ns/internal/lib"
	"github.com/spf13/cobra"
)

var postgresShellCmd = &cobra.Command{
	Use:   "shell",
	Short: "Opens an interactive psql shell within the running database container",
	Run: func(_ *cobra.Command, _ []string) {
		config := lib.LoadConfig()

		if !config.Postgres.Enabled {
			log.Fatal("BAK_DB_POSTGRES=true must be set.")
		}

		runPostgresShell(config)
	},
}

func init() {
	postgresCmd.AddCommand(postgresShellCmd)
}

func runPostgresShell(config lib.Config) {
	// Construct psql command with proper quoting for bash -c
	psqlCmd := fmt.Sprintf(`psql --host=%s --port=%s --username=%s --dbname=%s`,
		config.Postgres.Host,
		config.Postgres.Port,
		config.Postgres.User,
		config.Postgres.DB,
	)

	// Create interactive kubectl exec command wrapped in bash -c
	// Environment variable PGPASSWORD is used instead of --password flag
	// #nosec G204
	cmd := exec.Command("kubectl", "exec",
		"-it",
		"-n", config.Namespace,
		config.Postgres.ExecResource,
		"-c", config.Postgres.ExecContainer,
		"--",
		"bash", "-c",
		fmt.Sprintf("PGPASSWORD='%s' %s", config.Postgres.Password, psqlCmd),
	)

	// Connect stdin/stdout/stderr for interactive session
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		log.Fatal(err)
	}
}
