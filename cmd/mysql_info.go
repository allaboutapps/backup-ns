package cmd

import (
	"log"

	"github.com/allaboutapps/backup-ns/internal/lib"
	"github.com/spf13/cobra"
)

var mysqlInfoCmd = &cobra.Command{
	Use:   "info",
	Short: "Shows information about the mysql database backup state",
	Run: func(_ *cobra.Command, _ []string) {
		config := lib.LoadConfig()

		if !config.MySQL.Enabled {
			log.Fatal("BAK_DB_MYSQL=true must be set.")
		}

		runMySQLInfo(config)
	},
}

func init() {
	mysqlCmd.AddCommand(mysqlInfoCmd)
}

func runMySQLInfo(config lib.Config) {
	if err := lib.EnsureResourceAvailable(config.Namespace, config.MySQL.ExecResource); err != nil {
		log.Fatal(err)
	}
	if err := lib.EnsureMySQLAvailable(config.Namespace, config.MySQL); err != nil {
		log.Fatal(err)
	}

	timestamp, err := lib.GetRemoteFileTimestamp(config.Namespace, config.MySQL.ExecResource, config.MySQL.ExecContainer, config.MySQL.DumpFile)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("Last mysql dump in namespace='%s' was created at %s", config.Namespace, timestamp.UTC().Format("2006-01-02 15:04:05 MST"))
}
