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

	if !config.DBPostgres && !config.DBMySQL && !config.DBSkip {
		log.Fatal("Either BAK_DB_POSTGRES=true or BAK_DB_MYSQL=true or BAK_DB_SKIP=true must be set.")
	}

	if config.Flock {
		lockFile := lib.FlockShuffleLockFile(config.FlockDir, config.FlockCount)
		log.Printf("Using lock_file='%s'...", lockFile)

		unlock := lib.FlockLock(lockFile, config.FlockTimeoutSec, config.DryRun)
		defer unlock()
	}

	vsName := lib.GenerateVSName(config)
	log.Println("VS Name:", vsName)

	lib.EnsurePVCAvailable(config)

	if config.DBPostgres {
		lib.EnsureResourceAvailable(config.Namespace, config.DBPostgresExecResource)
		lib.EnsurePostgresAvailable(config)
		lib.EnsureFreeSpace(config, config.DBPostgresExecResource, config.DBPostgresExecContainer, filepath.Dir(config.DBPostgresDumpFile))
		lib.BackupPostgres(config)
	}

	if config.DBMySQL {
		lib.EnsureResourceAvailable(config.Namespace, config.DBMySQLExecResource)
		lib.EnsureMySQLAvailable(config)
		lib.EnsureFreeSpace(config, config.DBMySQLExecResource, config.DBMySQLExecContainer, filepath.Dir(config.DBMySQLDumpFile))
		lib.BackupMySQL(config)
	}

	vsLabels := lib.GenerateVSLabels(config)
	vsAnnotations := lib.GenerateVSAnnotations(config)

	vsObject := lib.GenerateVSObject(config, vsName, vsLabels, vsAnnotations)

	if config.Debug {
		log.Println("VS Object:", vsObject)
	}

	lib.CreateVolumeSnapshot(config, vsName, vsObject)

	log.Printf("Finished backup vs_name='%s' in namespace='%s'!", vsName, config.Namespace)
}
