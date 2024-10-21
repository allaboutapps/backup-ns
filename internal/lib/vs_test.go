package lib_test

import (
	"fmt"
	"os/exec"
	"testing"

	"github.com/allaboutapps/backup-ns/internal/lib"
)

func TestVolumeCreateDelete(t *testing.T) {
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

	lib.CreateVolumeSnapshot(namespace, false, vsName, vsObject, true, "25s")

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
