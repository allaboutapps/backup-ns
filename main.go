package main

import (
	"log"
	"path/filepath"

	"github.com/allaboutapps/backup-ns/internal/lib"
)

func main() {
	config := lib.LoadConfig()

	if config.Debug {
		log.Println("Config:", config)
	}

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

	vsLabels := lib.GenerateVSLabels(config)
	vsAnnotations := lib.GenerateVSAnnotations(config)

	vsObject := lib.GenerateVSObject(config, vsName, vsLabels, vsAnnotations)

	lib.CreateVolumeSnapshot(config, vsName, vsObject)

	log.Printf("Finished backup vs_name='%s' in namespace='%s'!", vsName, config.Namespace)
}
