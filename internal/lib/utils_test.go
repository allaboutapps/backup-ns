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

func TestGetSelectorFromResource(t *testing.T) {
	tests := []struct {
		name      string
		namespace string
		resource  string
		want      string
		wantErr   bool
	}{
		{
			name:      "valid deployment single label",
			namespace: "postgres-test",
			resource:  "deployment/postgres",
			want:      "app=postgres",
			wantErr:   false,
		},
		{
			name:      "invalid resource format",
			namespace: "postgres-test",
			resource:  "invalid-format",
			want:      "",
			wantErr:   true,
		},
		{
			name:      "non-existent resource",
			namespace: "postgres-test",
			resource:  "deployment/non-existent",
			want:      "",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := lib.GetSelectorFromResource(tt.namespace, tt.resource)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestGetPodFromResource(t *testing.T) {
	tests := []struct {
		name      string
		namespace string
		resource  string
		wantErr   bool
	}{
		{
			name:      "valid deployment with pod",
			namespace: "postgres-test",
			resource:  "deployment/postgres",
			wantErr:   false,
		},
		{
			name:      "invalid resource format",
			namespace: "postgres-test",
			resource:  "invalid-format",
			wantErr:   true,
		},
		{
			name:      "non-existent resource",
			namespace: "postgres-test",
			resource:  "deployment/non-existent",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := lib.GetPodFromResource(tt.namespace, tt.resource)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Contains(t, got, "postgres")
		})
	}
}
