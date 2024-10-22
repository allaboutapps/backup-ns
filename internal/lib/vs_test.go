package lib_test

import (
	"fmt"
	"os/exec"
	"testing"

	"github.com/allaboutapps/backup-ns/internal/lib"
	"github.com/stretchr/testify/require"
)

func TestVolumeCreateAndDelete(t *testing.T) {
	vsName := fmt.Sprintf("test-backup-generic-%s", lib.GenerateRandomString(6))
	namespace := "generic-test"

	labelVSConfig := lib.LabelVSConfig{
		Type:   "adhoc",
		Pod:    "gotest",
		Retain: "daily_weekly_monthly",
	}

	vsLabels := lib.GenerateVSLabels(namespace, "data", labelVSConfig)
	vsAnnotations := lib.GenerateVSAnnotations(lib.GetBAKEnvVars())

	vsObject := lib.GenerateVSObject(namespace, "csi-hostpath-snapclass", "data", vsName, vsLabels, vsAnnotations)

	if err := lib.CreateVolumeSnapshot(namespace, false, vsName, vsObject, true, "25s"); err != nil {
		t.Fatal("create vs failed: ", err)
	}

	cmd := exec.Command("kubectl", "get", "vs", vsName, "-n", namespace)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatal("get vs failed: ", err, string(output))
	}

	err = lib.PruneVolumeSnapshot(namespace, vsName)

	if err != nil {
		t.Fatal("delete vs failed: ", err)
	}
}

func TestVolumeCreateFailsNameSpace(t *testing.T) {
	vsName := fmt.Sprintf("test-backup-generic-%s", lib.GenerateRandomString(6))
	namespace := "non-existant-namespace" // !!!

	labelVSConfig := lib.LabelVSConfig{
		Type:   "adhoc",
		Pod:    "gotest",
		Retain: "daily_weekly_monthly",
	}

	vsLabels := lib.GenerateVSLabels(namespace, "data", labelVSConfig)
	vsAnnotations := lib.GenerateVSAnnotations(lib.GetBAKEnvVars())

	vsObject := lib.GenerateVSObject(namespace, "csi-hostpath-snapclass", "data", vsName, vsLabels, vsAnnotations)

	require.Error(t, lib.CreateVolumeSnapshot(namespace, false, vsName, vsObject, true, "25s"))
}
