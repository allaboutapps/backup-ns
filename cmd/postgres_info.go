package cmd

import (
	"log"

	"github.com/allaboutapps/backup-ns/internal/lib"
	"github.com/spf13/cobra"
)

var postgresInfoCmd = &cobra.Command{
	Use:   "info",
	Short: "Shows information about the postgres database backup state",
	Run: func(_ *cobra.Command, _ []string) {
		config := lib.LoadConfig()

		if !config.Postgres.Enabled {
			log.Fatal("BAK_DB_POSTGRES=true must be set.")
		}

		runPostgresInfo(config)
	},
}

func init() {
	postgresCmd.AddCommand(postgresInfoCmd)
}

func runPostgresInfo(config lib.Config) {
	if err := lib.EnsureResourceAvailable(config.Namespace, config.Postgres.ExecResource); err != nil {
		log.Fatal(err)
	}
	if err := lib.EnsurePostgresAvailable(config.Namespace, config.Postgres); err != nil {
		log.Fatal(err)
	}

	timestamp, err := lib.GetRemoteFileTimestamp(config.Namespace, config.Postgres.ExecResource, config.Postgres.ExecContainer, config.Postgres.DumpFile)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("Last postgres dump in namespace='%s' was created at %s", config.Namespace, timestamp.UTC().Format("2006-01-02 15:04:05 MST"))
}
