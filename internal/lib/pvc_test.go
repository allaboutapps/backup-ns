package lib_test

import (
	"testing"

	"github.com/allaboutapps/backup-ns/internal/lib"
	"github.com/stretchr/testify/require"
)

func TestEnsureFreeSpace(t *testing.T) {
	// our current csi-driver-host-path does not provision actual block baked volumes...
	require.NoError(t, lib.EnsureFreeSpace("generic-test", "deployment/writer", "debian", "/app", 100))
	require.Error(t, lib.EnsureFreeSpace("generic-test", "deployment/writer", "debian", "/app", 0))
}
