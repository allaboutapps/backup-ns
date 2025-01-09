package cmd

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/allaboutapps/backup-ns/internal/lib"
	"github.com/spf13/cobra"
)

var forcePostgresRestore bool

// postgresRestoreCmd represents the dump command
var postgresRestoreCmd = &cobra.Command{
	Use:   "restore",
	Short: "Connects to the live postgres container and restores a preexisting database dump",
	// Long: `...`,
	Run: func(_ *cobra.Command, _ []string) {
		config := lib.LoadConfig()

		if config.DryRun {
			log.Println("Dry run mode is active, write operations are skipped!")
		}

		if !config.Postgres.Enabled {
			log.Fatal("BAK_DB_POSTGRES=true must be set.")
		}

		runPostgresRestore(config)
	},
}

func init() {
	postgresCmd.AddCommand(postgresRestoreCmd)
	postgresRestoreCmd.Flags().BoolVarP(&forcePostgresRestore, "force", "f", false, "Skip confirmation prompt")
}

func confirmRestorePostgres(namespace string) bool {
	reader := bufio.NewReader(os.Stdin)
	fmt.Printf("Are you sure you want to restore the postgres dump in namespace '%s'? [y/N]: ", namespace)

	response, err := reader.ReadString('\n')
	if err != nil {
		log.Fatal(err)
	}

	response = strings.ToLower(strings.TrimSpace(response))
	return response == "y" || response == "yes"
}

func runPostgresRestore(config lib.Config) {
	if err := lib.EnsureResourceAvailable(config.Namespace, config.Postgres.ExecResource); err != nil {
		log.Fatal(err)
	}
	if err := lib.EnsurePostgresAvailable(config.Namespace, config.Postgres); err != nil {
		log.Fatal(err)
	}

	if !config.DryRun && !forcePostgresRestore && !confirmRestorePostgres(config.Namespace) {
		log.Println("Restore cancelled by user.")
		return
	}

	if err := lib.RestorePostgres(config.Namespace, config.DryRun, config.Postgres); err != nil {
		log.Fatal(err)
	}

	log.Printf("Finished postgres restore in namespace='%s'!", config.Namespace)
}
