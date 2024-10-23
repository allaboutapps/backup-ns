package lib_test

import (
	"fmt"
	"os/exec"
	"testing"

	"github.com/allaboutapps/backup-ns/internal/lib"
	"github.com/allaboutapps/backup-ns/internal/test"
	"github.com/stretchr/testify/require"
)

func TestSyncVSLabelsToVsc(t *testing.T) {
	vsName := fmt.Sprintf("test-backup-generic-%s", lib.GenerateRandomStringOrPanic(6))
	namespace := "generic-test"

	labelVSConfig := lib.LabelVSConfig{
		Type:       "adhoc",
		Pod:        "gotest",
		Retain:     "days",
		RetainDays: 0,
	}

	vsLabels := lib.GenerateVSLabels(namespace, "data", labelVSConfig)
	vsAnnotations := lib.GenerateVSAnnotations(lib.GetBAKEnvVars())

	vsObject := lib.GenerateVSObject(namespace, "csi-hostpath-snapclass", "data", vsName, vsLabels, vsAnnotations)

	require.NoError(t, lib.CreateVolumeSnapshot(namespace, false, vsName, vsObject, false, "1m"))

	// add an additional label that should not be synced
	lblCmd := exec.Command("kubectl", "-n", namespace, "label", "volumesnapshot", vsName, "extralabel=test")
	output, err := lblCmd.CombinedOutput()
	require.NoError(t, err, string(output))

	vscName, err := lib.GetVolumeSnapshotContentName(namespace, vsName)
	require.NoError(t, err)

	// first empty
	preSyncLabelMap, err := lib.GetBackupNsLabelMap("generic-test", "volumesnapshotcontent", vscName)
	require.NoError(t, err)
	require.Empty(t, preSyncLabelMap)

	// sync
	require.NoError(t, lib.SyncVSLabelsToVsc(namespace, vsName))

	// noop 2nd sync is ok
	require.NoError(t, lib.SyncVSLabelsToVsc(namespace, vsName))

	postSyncLabelMap, err := lib.GetBackupNsLabelMap(namespace, "volumesnapshotcontent", vscName)
	require.NoError(t, err)
	require.NotEmpty(t, postSyncLabelMap)

	test.Snapshoter.Redact("backup-ns.sh/delete-after").SaveJSON(t, postSyncLabelMap)
}

func TestSyncVSLabelsToVscFail(t *testing.T) {
	vsName := "test-backup-generic-not-available"
	namespace := "generic-test"

	// sync
	err := lib.SyncVSLabelsToVsc(namespace, vsName)
	require.Error(t, err)
}
