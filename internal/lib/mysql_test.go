package lib_test

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/allaboutapps/backup-ns/internal/lib"
)

func TestDumpAndRestoreMySQL(t *testing.T) {
	vsName := fmt.Sprintf("test-backup-mysql-%s", lib.GenerateRandomStringOrPanic(6))
	namespace := "mysql-test"

	mysqlConfig := lib.MySQLConfig{
		Enabled:             true,
		ExecResource:        "deployment/mysql",
		ExecContainer:       "mysql",
		DumpFile:            "/var/lib/mysql/dump.sql.gz",
		Host:                "127.0.0.1",
		Port:                "3306",
		User:                "root",
		Password:            "${MYSQL_ROOT_PASSWORD}",
		DB:                  "${MYSQL_DATABASE}",
		DefaultCharacterSet: "utf8",
	}

	labelVSConfig := lib.LabelVSConfig{
		Type:       "adhoc",
		Pod:        "gotest",
		Retain:     "days",
		RetainDays: 1,
	}

	if err := lib.EnsurePVCAvailable("mysql-test", "data"); err != nil {
		t.Fatal("ensure pvc failed: ", err)
	}

	if err := lib.EnsureResourceAvailable(namespace, mysqlConfig.ExecResource); err != nil {
		t.Fatal("ensure res failed: ", err)
	}

	if err := lib.EnsureMySQLAvailable(namespace, mysqlConfig); err != nil {
		t.Fatal("ensure MySQL available failed: ", err)
	}

	if err := lib.EnsureFreeSpace(namespace, mysqlConfig.ExecResource, mysqlConfig.ExecContainer, filepath.Dir(mysqlConfig.DumpFile), 90); err != nil {
		t.Fatal("ensure free space failed: ", err)
	}

	if err := lib.DumpMySQL(namespace, false, mysqlConfig); err != nil {
		t.Fatal("backup MySQL failed: ", err)
	}

	timestamp, err := lib.GetRemoteFileTimestamp(namespace, mysqlConfig.ExecResource, mysqlConfig.ExecContainer, mysqlConfig.DumpFile)
	if err != nil {
		t.Fatal("get remote file timestamp failed: ", err)
	}

	// Verify timestamp is recent (within last minute)
	now := time.Now()
	if !timestamp.After(now.Add(-1*time.Minute)) || !timestamp.Before(now) {
		t.Fatal("dump file timestamp not within expected range")
	}

	vsLabels := lib.GenerateVSLabels(namespace, "data", labelVSConfig, time.Now())
	vsAnnotations := lib.GenerateVSAnnotations(lib.GetBAKEnvVars())

	vsObject := lib.GenerateVSObject(namespace, "csi-hostpath-snapclass", "data", vsName, vsLabels, vsAnnotations)

	if err := lib.CreateVolumeSnapshot(namespace, false, vsName, vsObject, false, "25s"); err != nil {
		t.Fatal("create vs failed: ", err)
	}

	cmd := exec.Command("kubectl", "get", "vs", vsName, "-n", namespace)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatal("get vs failed: ", err, string(output))
	}

	if err := lib.RestoreMySQL(namespace, false, mysqlConfig); err != nil {
		t.Fatal("restore Postgres failed: ", err)
	}
}
