package lib_test

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/allaboutapps/backup-ns/internal/lib"
)

func TestBackupPostgres(t *testing.T) {
	vsName := fmt.Sprintf("test-backup-postgres-%s", lib.GenerateRandomString(6))
	namespace := "postgres-test"

	postgresConfig := lib.PostgresConfig{
		Enabled:       true,
		ExecResource:  "deployment/database",
		ExecContainer: "postgres",
		DumpFile:      "/var/lib/postgresql/data/dump.sql.gz",
		User:          "${POSTGRES_USER}",     // read inside container
		Password:      "${POSTGRES_PASSWORD}", // read inside container
		DB:            "${POSTGRES_DB}",       // read inside container
	}

	labelVSConfig := lib.LabelVSConfig{
		Type:       "adhoc",
		Pod:        "",
		Retain:     "days",
		RetainDays: 1,
	}

	lib.EnsurePVCAvailable("postgres-test", "data")

	lib.EnsureResourceAvailable(namespace, "deployment/database")
	lib.EnsurePostgresAvailable(namespace, postgresConfig)
	lib.EnsureFreeSpace(namespace, postgresConfig.ExecResource,
		postgresConfig.ExecContainer, filepath.Dir(postgresConfig.DumpFile), 90)
	lib.BackupPostgres(namespace, false, postgresConfig)

	vsLabels := lib.GenerateVSLabels(namespace, "data", labelVSConfig)
	vsAnnotations := lib.GenerateVSAnnotations(lib.GetBAKEnvVars())

	vsObject := lib.GenerateVSObject(namespace, "csi-hostpath-snapclass", "data", vsName, vsLabels, vsAnnotations)

	lib.CreateVolumeSnapshot(namespace, false, vsName, vsObject, false, "15m")

	cmd := exec.Command("kubectl", "get", "vs", vsName, "-n", namespace)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatal("get vs failed: ", err, string(output))
	}
}
