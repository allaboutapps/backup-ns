package cmd

import (
	"log"
	"path/filepath"

	"github.com/allaboutapps/backup-ns/internal/lib"
	"github.com/spf13/cobra"
)

// postgresDumpCmd represents the dump command
var postgresDumpCmd = &cobra.Command{
	Use:   "dump",
	Short: "Connects to the live postgres container and creates a database dump",
	// Long: `...`,
	Run: func(_ *cobra.Command, _ []string) {
		config := lib.LoadConfig()

		if config.DryRun {
			log.Println("Dry run mode is active, write operations are skipped!")
		}

		if !config.Postgres.Enabled {
			log.Fatal("BAK_DB_POSTGRES=true must be set.")
		}

		runPostgresDump(config)
	},
}

func init() {
	postgresCmd.AddCommand(postgresDumpCmd)
}

func runPostgresDump(config lib.Config) {
	if err := lib.EnsureResourceAvailable(config.Namespace, config.Postgres.ExecResource); err != nil {
		log.Fatal(err)
	}
	if err := lib.EnsurePostgresAvailable(config.Namespace, config.Postgres); err != nil {
		log.Fatal(err)
	}
	if err := lib.EnsureFreeSpace(config.Namespace, config.Postgres.ExecResource, config.Postgres.ExecContainer, filepath.Dir(config.Postgres.DumpFile), config.ThresholdSpaceUsedPercent); err != nil {
		log.Fatal(err)
	}

	if err := lib.DumpPostgres(config.Namespace, config.DryRun, config.Postgres); err != nil {
		log.Fatal(err)
	}

	log.Printf("Finished postgres dump in namespace='%s'!", config.Namespace)

}
