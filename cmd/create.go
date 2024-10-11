package cmd

import (
	"log"
	"path/filepath"

	"github.com/allaboutapps/backup-ns/internal/lib"
	"github.com/spf13/cobra"
)

// createCmd represents the create command
var createCmd = &cobra.Command{
	Use:   "create",
	Short: "(DEFAULT) Creates an application-aware snapshot",
	// 	Long: `...`,
	Run: runCreate,
}

func init() {
	rootCmd.AddCommand(createCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// createCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// createCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

func runCreate(_ *cobra.Command, _ []string) {
	config := lib.LoadConfig()

	lib.PrintTimeZone()
	lib.PrintConfig(config)

	if config.DryRun {
		log.Println("Dry run mode is active, write operations are skipped!")
	}

	if !config.Postgres.Enabled && !config.MySQL.Enabled && !config.DBSkip {
		log.Fatal("Either BAK_DB_POSTGRES=true or BAK_DB_MYSQL=true or BAK_DB_SKIP=true must be set.")
	}

	if config.Flock.Enabled {
		lockFile := lib.FlockShuffleLockFile(config.Flock.Dir, config.Flock.Count)
		log.Printf("Using lock_file='%s'...", lockFile)

		unlock := lib.FlockLock(lockFile, config.Flock.TimeoutSec, config.DryRun)
		defer unlock()
	}

	vsName := lib.GenerateVSName(config.VSNameTemplate, config.PVCName, config.VSRand)
	log.Println("VS Name:", vsName)

	lib.EnsurePVCAvailable(config.Namespace, config.PVCName)

	if config.Postgres.Enabled {
		lib.EnsureResourceAvailable(config.Namespace, config.Postgres.ExecResource)
		lib.EnsurePostgresAvailable(config.Namespace, config.Postgres)
		lib.EnsureFreeSpace(config.Namespace, config.Postgres.ExecResource,
			config.Postgres.ExecContainer, filepath.Dir(config.Postgres.DumpFile), config.ThresholdSpaceUsedPercent)
		lib.BackupPostgres(config.Namespace, config.DryRun, config.Postgres)
	}

	if config.MySQL.Enabled {
		lib.EnsureResourceAvailable(config.Namespace, config.MySQL.ExecResource)
		lib.EnsureMySQLAvailable(config.Namespace, config.MySQL)
		lib.EnsureFreeSpace(config.Namespace, config.MySQL.ExecResource,
			config.MySQL.ExecContainer, filepath.Dir(config.MySQL.DumpFile), config.ThresholdSpaceUsedPercent)
		lib.BackupMySQL(config.Namespace, config.DryRun, config.MySQL)
	}

	vsLabels := lib.GenerateVSLabels(config.Namespace, config.PVCName, config.LabelVS)
	vsAnnotations := lib.GenerateVSAnnotations(lib.GetBAKEnvVars())

	vsObject := lib.GenerateVSObject(config.Namespace, config.VSClassName, config.PVCName, vsName, vsLabels, vsAnnotations)

	lib.CreateVolumeSnapshot(config.Namespace, config.DryRun, vsName, vsObject, config.VSWaitUntilReady, config.VSWaitUntilReadyTimeout)

	log.Printf("Finished backup vs_name='%s' in namespace='%s'!", vsName, config.Namespace)
}
