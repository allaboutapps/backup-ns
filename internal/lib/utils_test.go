package lib_test

import (
	"testing"

	"github.com/allaboutapps/backup-ns/internal/lib"
	"github.com/stretchr/testify/require"
)

func TestEnsureResourceAvailable(t *testing.T) {
	require.NoError(t, lib.EnsureResourceAvailable("generic-test", "deployment/writer"))
	require.NoError(t, lib.EnsureResourceAvailable("generic-test", "PersistentVolumeClaim/data"))

	require.NoError(t, lib.EnsureResourceAvailable("mysql-test", "deployment/mysql"))
	require.NoError(t, lib.EnsureResourceAvailable("mysql-test", "PersistentVolumeClaim/data"))

	require.NoError(t, lib.EnsureResourceAvailable("postgres-test", "deployment/postgres"))
	require.NoError(t, lib.EnsureResourceAvailable("postgres-test", "PersistentVolumeClaim/data"))

	require.Error(t, lib.EnsureResourceAvailable("generic-test", "deployment/not-available"))
	require.Error(t, lib.EnsureResourceAvailable("generic-test", "nonKind/not-available"))
	require.Error(t, lib.EnsureResourceAvailable("non-ns", "deployment/writer"))
}
