package cmd

import (
	"log"

	"github.com/allaboutapps/backup-ns/internal/lib"
	"github.com/spf13/cobra"
)

// mysqlRestoreCmd represents the restore command
var mysqlRestoreCmd = &cobra.Command{
	Use:   "restore",
	Short: "Connects to the live mysql/mariadb container and restores a preexisting database dump",
	// Long: `...`,
	Run: func(_ *cobra.Command, _ []string) {
		config := lib.LoadConfig()

		if config.DryRun {
			log.Println("Dry run mode is active, write operations are skipped!")
		}

		if !config.MySQL.Enabled {
			log.Fatal("BAK_DB_MYSQL=true must be set.")
		}

		runMySQLRestore(config)
	},
}

func init() {
	mysqlCmd.AddCommand(mysqlRestoreCmd)
}

func runMySQLRestore(config lib.Config) {
	if err := lib.EnsureResourceAvailable(config.Namespace, config.MySQL.ExecResource); err != nil {
		log.Fatal(err)
	}
	if err := lib.EnsureMySQLAvailable(config.Namespace, config.MySQL); err != nil {
		log.Fatal(err)
	}

	if err := lib.RestoreMySQL(config.Namespace, config.DryRun, config.MySQL); err != nil {
		log.Fatal(err)
	}

	log.Printf("Finished mysql restore in namespace='%s'!", config.Namespace)
}
