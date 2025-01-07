package lib_test

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"testing"
	"time"

	"github.com/allaboutapps/backup-ns/internal/lib"
	"github.com/allaboutapps/backup-ns/internal/test"
	"github.com/stretchr/testify/require"
)

func TestVolumeCreateAndDelete(t *testing.T) {
	vsName := fmt.Sprintf("test-backup-generic-%s", lib.GenerateRandomStringOrPanic(6))
	namespace := "generic-test"

	labelVSConfig := lib.LabelVSConfig{
		Type:       "adhoc",
		Pod:        "gotest",
		Retain:     "days",
		RetainDays: 30,
	}

	vsLabels := lib.GenerateVSLabels(namespace, "data", labelVSConfig, time.Now())
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

	err = lib.PruneVolumeSnapshot(namespace, vsName, false)

	if err != nil {
		t.Fatal("delete vs failed: ", err)
	}
}

func TestGenerateVSLabelsRetainSchedule(t *testing.T) {

	labelVSConfig := lib.LabelVSConfig{
		Type:   "cronjob",
		Pod:    "gotest",
		Retain: "daily_weekly_monthly",
	}

	vsLabels := lib.GenerateVSLabels("generic-test", "data", labelVSConfig, time.Date(2022, 5, 21, 0, 17, 0, 0, time.Local))

	test.Snapshoter.SaveJSON(t, vsLabels)
}

func TestGenerateVSLabelsRetainDays(t *testing.T) {

	labelVSConfig := lib.LabelVSConfig{
		Type:       "cronjob",
		Pod:        "gotest",
		Retain:     "days",
		RetainDays: 30,
	}

	vsLabels := lib.GenerateVSLabels("generic-test", "data", labelVSConfig, time.Date(2022, 5, 21, 0, 17, 0, 0, time.Local))

	test.Snapshoter.SaveJSON(t, vsLabels)
}

func TestVolumeCreateFailsNamespace(t *testing.T) {
	vsName := fmt.Sprintf("test-backup-generic-%s", lib.GenerateRandomStringOrPanic(6))
	namespace := "non-existant-namespace" // !!!

	labelVSConfig := lib.LabelVSConfig{
		Type:   "adhoc",
		Pod:    "gotest",
		Retain: "daily_weekly_monthly",
	}

	vsLabels := lib.GenerateVSLabels(namespace, "data", labelVSConfig, time.Now())
	vsAnnotations := lib.GenerateVSAnnotations(lib.GetBAKEnvVars())

	vsObject := lib.GenerateVSObject(namespace, "csi-hostpath-snapclass", "data", vsName, vsLabels, vsAnnotations)

	require.Error(t, lib.CreateVolumeSnapshot(namespace, false, vsName, vsObject, true, "25s"))
}

func TestVolumeCreateSimulatedForWeek(t *testing.T) {
	namespace := "generic-test"
	pvcName := "data"

	testCases := []struct {
		date           time.Time
		expectedLabels map[string]string
	}{
		{
			date: time.Date(2024, 12, 30, 10, 0, 0, 0, time.UTC),
			expectedLabels: map[string]string{
				"backup-ns.sh/monthly": "2024-12",
				"backup-ns.sh/daily":   "2024-12-30",
				"backup-ns.sh/weekly":  "w01",
			},
		},
		{
			date: time.Date(2024, 12, 31, 10, 0, 0, 0, time.UTC),
			expectedLabels: map[string]string{
				"backup-ns.sh/daily": "2024-12-31",
			},
		},
		{
			date: time.Date(2025, 1, 1, 10, 0, 0, 0, time.UTC),
			expectedLabels: map[string]string{
				"backup-ns.sh/daily":   "2025-01-01",
				"backup-ns.sh/monthly": "2025-01",
			},
		},
		{
			date: time.Date(2025, 1, 2, 10, 0, 0, 0, time.UTC),
			expectedLabels: map[string]string{
				"backup-ns.sh/daily": "2025-01-02",
			},
		},
		{
			date: time.Date(2025, 1, 5, 10, 0, 0, 0, time.UTC),
			expectedLabels: map[string]string{
				"backup-ns.sh/daily": "2025-01-05",
			},
		},
		{
			date: time.Date(2025, 1, 6, 10, 0, 0, 0, time.UTC),
			expectedLabels: map[string]string{
				"backup-ns.sh/daily":  "2025-01-06",
				"backup-ns.sh/weekly": "w02",
			},
		},
	}

	labelVSConfig := lib.LabelVSConfig{
		Type:   "cronjob",
		Pod:    "gotest",
		Retain: "daily_weekly_monthly",
	}

	for _, tc := range testCases {
		t.Run(tc.date.Format("2006-01-02"), func(t *testing.T) {
			// Generate unique snapshot name
			vsName := fmt.Sprintf("test-backup-%s-%s", tc.date.Format("20060102"), lib.GenerateRandomStringOrPanic(6))

			// Generate labels and create snapshot
			labels := lib.GenerateVSLabels(namespace, pvcName, labelVSConfig, tc.date)
			vsAnnotations := lib.GenerateVSAnnotations(lib.GetBAKEnvVars())
			vsObject := lib.GenerateVSObject(namespace, "csi-hostpath-snapclass", pvcName, vsName, labels, vsAnnotations)

			// Create the snapshot
			err := lib.CreateVolumeSnapshot(namespace, false, vsName, vsObject, false, "0s")
			require.NoError(t, err)

			// Get snapshot and verify labels
			cmd := exec.Command("kubectl", "get", "volumesnapshot", vsName, "-n", namespace, "-o", "json")
			output, err := cmd.CombinedOutput()
			require.NoError(t, err)

			var vs map[string]interface{}
			err = json.Unmarshal(output, &vs)
			require.NoError(t, err)

			metadata := vs["metadata"].(map[string]interface{})
			actualLabels := metadata["labels"].(map[string]interface{})

			// Verify expected labels exist
			for key, expectedValue := range tc.expectedLabels {
				require.Equal(t, expectedValue, actualLabels[key], "Label %s mismatch", key)
			}

			// Verify other retention labels don't exist
			allPossibleLabels := []string{"backup-ns.sh/daily", "backup-ns.sh/weekly", "backup-ns.sh/monthly"}
			for _, label := range allPossibleLabels {
				if _, expected := tc.expectedLabels[label]; !expected {
					_, exists := actualLabels[label]
					require.False(t, exists, "Label %s should not exist", label)
				}
			}
		})
	}
}
