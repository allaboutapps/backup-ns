package lib_test

import (
	"fmt"
	"os/exec"
	"testing"

	"github.com/allaboutapps/backup-ns/internal/lib"
	"github.com/allaboutapps/backup-ns/internal/test"
	"github.com/stretchr/testify/require"
)

func createTestVS(t *testing.T) (string /* namespace*/, string /* vsName */) {
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

	require.NoError(t, lib.CreateVolumeSnapshot(namespace, false, vsName, vsObject, true, "25s"))

	return namespace, vsName
}

func TestSyncVSLabelsToVsc(t *testing.T) {
	namespace, vsName := createTestVS(t)

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

func TestPreProvisionedVSC(t *testing.T) {

	namespace, vsName := createTestVS(t)

	// sync labels
	require.NoError(t, lib.SyncVSLabelsToVsc(namespace, vsName))

	// grab the created VSC name
	vscName, err := lib.GetVolumeSnapshotContentName(namespace, vsName)
	require.NoError(t, err)

	// now delete the vs without affecting the VSC (like what would happen if the ns was deleted)
	cmd := exec.Command("kubectl", "-n", namespace, "delete", "volumesnapshot", vsName)
	require.NoError(t, cmd.Run())

	// Get the VolumeSnapshotContent object
	vscObject, err := lib.GetVolumeSnapshotContentObject(vscName)
	require.NoError(t, err)

	// Create a restored VSC from the existing VSC
	restoredVSC, err := lib.CreatePreProvisionedVSC(vscObject, lib.GenerateRandomStringOrPanic(6))
	require.NoError(t, err)

	newVSCName, ok := restoredVSC["metadata"].(map[string]interface{})["name"].(string)
	require.True(t, ok, "invalid name in generated VolumeSnapshotContent object")

	// Generate the VolumeSnapshot object from the restored VolumeSnapshotContent
	vsObject, err := lib.GenerateVSObjectFromVSC(newVSCName, restoredVSC)
	require.NoError(t, err)

	// Extract necessary information from the generated VS object
	metadata, ok := vsObject["metadata"].(map[string]interface{})
	require.True(t, ok, "invalid metadata in generated VolumeSnapshot object")

	newNamespace, ok := metadata["namespace"].(string)
	require.True(t, ok, "invalid namespace in generated VolumeSnapshot object")

	newVsName, ok := metadata["name"].(string)
	require.True(t, ok, "invalid name in generated VolumeSnapshot object")

	// Create the VolumeSnapshot
	err = lib.CreateVolumeSnapshot(newNamespace, false, newVsName, vsObject, true, "25s")
	require.NoError(t, err)

	postSyncLabelMap, err := lib.GetBackupNsLabelMap(namespace, "volumesnapshotcontent", newVSCName)
	require.NoError(t, err)
	require.NotEmpty(t, postSyncLabelMap)

	// ensure labels were successfully synced
	test.Snapshoter.Redact("backup-ns.sh/delete-after").SaveJSON(t, postSyncLabelMap)
}

func TestRebindVSC(t *testing.T) {

	rndStr := lib.GenerateRandomStringOrPanic(6)

	namespace, vsName := createTestVS(t)

	// sync labels
	require.NoError(t, lib.SyncVSLabelsToVsc(namespace, vsName))

	// grab the created VSC name
	vscName, err := lib.GetVolumeSnapshotContentName(namespace, vsName)
	require.NoError(t, err)

	// now delete the vs without affecting the VSC (like what would happen if the ns was deleted)
	cmd := exec.Command("kubectl", "-n", namespace, "delete", "volumesnapshot", vsName)
	require.NoError(t, cmd.Run())

	// Rebind the VSC to a new VS
	require.NoError(t, lib.RebindVsc(vscName, rndStr, true, "25s"))

	// old vsc should now be gone -> error!
	_, err = lib.GetVolumeSnapshotContentObject(vscName)
	require.Error(t, err)

	// new vs should be created with the name "test-backup-generic-random"
	newVsName := fmt.Sprintf("test-backup-generic-%s", rndStr)

	// grab the created VSC name
	newVscName, err := lib.GetVolumeSnapshotContentName(namespace, newVsName)
	require.NoError(t, err)

	// new vsc name should start with "restoredvsc-"
	require.Contains(t, newVscName, "restoredvsc-")
}

func TestSyncVSLabelsToVscFail(t *testing.T) {
	vsName := "test-backup-generic-not-available"
	namespace := "generic-test"

	// sync
	err := lib.SyncVSLabelsToVsc(namespace, vsName)
	require.Error(t, err)
}
