package cmd

import (
	"fmt"
	"log"
	"os"
	"os/exec"

	"github.com/allaboutapps/backup-ns/internal/lib"
	"github.com/spf13/cobra"
)

// mysqlShellCmd represents the shell command
var mysqlShellCmd = &cobra.Command{
	Use:   "shell",
	Short: "Opens an interactive mysql shell within the running database container",
	Run: func(_ *cobra.Command, _ []string) {
		config := lib.LoadConfig()

		if !config.MySQL.Enabled {
			log.Fatal("BAK_DB_MYSQL=true must be set.")
		}

		runMySQLShell(config)
	},
}

func init() {
	mysqlCmd.AddCommand(mysqlShellCmd)
}

func runMySQLShell(config lib.Config) {
	// Construct mysql command with proper quoting for bash -c
	mysqlCmd := fmt.Sprintf(`mysql --host=%s --port=%s --user=%s --password="%s" --default-character-set=%s %s`,
		config.MySQL.Host,
		config.MySQL.Port,
		config.MySQL.User,
		config.MySQL.Password, // may contain ${MYSQL_ROOT_PASSWORD}
		config.MySQL.DefaultCharacterSet,
		config.MySQL.DB, // may contain ${MYSQL_DATABASE}
	)

	// Create interactive kubectl exec command wrapped in bash -c
	// #nosec G204
	cmd := exec.Command("kubectl", "exec",
		"-it",
		"-n", config.Namespace,
		config.MySQL.ExecResource,
		"-c", config.MySQL.ExecContainer,
		"--",
		"bash", "-c",
		mysqlCmd,
	)

	// Connect stdin/stdout/stderr for interactive session
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		log.Fatal(err)
	}
}
