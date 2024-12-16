package cmd

import (
	"log"

	"github.com/allaboutapps/backup-ns/internal/lib"
	"github.com/spf13/cobra"
)

// postgresRestoreCmd represents the dump command
var postgresRestoreCmd = &cobra.Command{
	Use:   "restore",
	Short: "Connects to the live postgres container and restores the database dump",
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
}

func runPostgresRestore(config lib.Config) {
	if err := lib.EnsureResourceAvailable(config.Namespace, config.Postgres.ExecResource); err != nil {
		log.Fatal(err)
	}
	if err := lib.EnsurePostgresAvailable(config.Namespace, config.Postgres); err != nil {
		log.Fatal(err)
	}

	if err := lib.RestorePostgres(config.Namespace, config.DryRun, config.Postgres); err != nil {
		log.Fatal(err)
	}

	log.Printf("Finished postgres restore in namespace='%s'!", config.Namespace)

}
