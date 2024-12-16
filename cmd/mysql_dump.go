package cmd

import (
	"log"
	"path/filepath"

	"github.com/allaboutapps/backup-ns/internal/lib"
	"github.com/spf13/cobra"
)

// mysqlDumpCmd represents the dump command
var mysqlDumpCmd = &cobra.Command{
	Use:   "dump",
	Short: "Connects to the live mysql/mariadb container and creates a database dump",
	// Long: `...`,
	Run: func(_ *cobra.Command, _ []string) {
		config := lib.LoadConfig()

		if config.DryRun {
			log.Println("Dry run mode is active, write operations are skipped!")
		}

		if !config.MySQL.Enabled {
			log.Fatal("BAK_DB_MYSQL=true must be set.")
		}

		runMySQLDump(config)
	},
}

func init() {
	mysqlCmd.AddCommand(mysqlDumpCmd)
}

func runMySQLDump(config lib.Config) {
	if err := lib.EnsureResourceAvailable(config.Namespace, config.MySQL.ExecResource); err != nil {
		log.Fatal(err)
	}
	if err := lib.EnsureMySQLAvailable(config.Namespace, config.MySQL); err != nil {
		log.Fatal(err)
	}
	if err := lib.EnsureFreeSpace(config.Namespace, config.MySQL.ExecResource, config.MySQL.ExecContainer, filepath.Dir(config.MySQL.DumpFile), config.ThresholdSpaceUsedPercent); err != nil {
		log.Fatal(err)
	}

	if err := lib.DumpMySQL(config.Namespace, config.DryRun, config.MySQL); err != nil {
		log.Fatal(err)
	}

	log.Printf("Finished mysql dump in namespace='%s'!", config.Namespace)
}
