package lib_test

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/allaboutapps/backup-ns/internal/lib"
	"github.com/allaboutapps/backup-ns/internal/test"
)

func TestBackupPostgres(t *testing.T) {
	vsName := fmt.Sprintf("test-backup-postgres-%s", lib.GenerateRandomStringOrPanic(6))
	namespace := "postgres-test"

	postgresConfig := lib.PostgresConfig{
		Enabled:       true,
		ExecResource:  "deployment/postgres",
		ExecContainer: "postgres",
		DumpFile:      "/var/lib/postgresql/data/dump.sql.gz",
		User:          "${POSTGRES_USER}",     // read inside container
		Password:      "${POSTGRES_PASSWORD}", // read inside container
		DB:            "${POSTGRES_DB}",       // read inside container
	}

	labelVSConfig := lib.LabelVSConfig{
		Type:       "adhoc",
		Pod:        "gotest",
		Retain:     "days",
		RetainDays: 1,
	}

	lib.EnsurePVCAvailable("postgres-test", "data")

	if err := lib.EnsureResourceAvailable(namespace, postgresConfig.ExecResource); err != nil {
		t.Fatal("ensure res failed: ", err)
	}

	lib.EnsurePostgresAvailable(namespace, postgresConfig)
	lib.EnsureFreeSpace(namespace, postgresConfig.ExecResource,
		postgresConfig.ExecContainer, filepath.Dir(postgresConfig.DumpFile), 90)
	lib.BackupPostgres(namespace, false, postgresConfig)

	vsLabels := lib.GenerateVSLabels(namespace, "data", labelVSConfig)
	vsAnnotations := lib.GenerateVSAnnotations(map[string]string{
		"BAK_NAMESPACE":                 namespace,
		"BAK_DB_POSTGRES":               "true",
		"BAK_DB_POSTGRES_EXEC_RESOURCE": "deployment/postgres",
	})

	vsObject := lib.GenerateVSObject(namespace, "csi-hostpath-snapclass", "data", vsName, vsLabels, vsAnnotations)

	test.Snapshoter.Redact("name", "backup-ns.sh/delete-after").SaveJSON(t, vsObject)

	if err := lib.CreateVolumeSnapshot(namespace, false, vsName, vsObject, false, "25s"); err != nil {
		t.Fatal("create vs failed: ", err)
	}

	cmd := exec.Command("kubectl", "get", "vs", vsName, "-n", namespace)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatal("get vs failed: ", err, string(output))
	}
}
