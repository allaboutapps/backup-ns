package lib_test

import (
	"testing"

	"github.com/allaboutapps/backup-ns/internal/lib"
	"github.com/stretchr/testify/require"
)

func TestTrap(t *testing.T) {
	// Create template data with computed fields
	type templateData struct {
		TestFile string
		Cmd      string
	}
	data := templateData{
		TestFile: "/tmp/trap_test.txt",
		Cmd:      "sleep 0.1",
	}

	require.NoError(t, lib.KubectlExecTemplate("generic-test", "deployment/writer", "debian", lib.GetTemplateAtlas().TestTrap, data))
	// succeess testfile exists!
	require.NoError(t, lib.KubectlExecCommand("generic-test", "deployment/writer", "debian", "test -f /tmp/trap_test.txt"))
}

func TestTrapCommandFail(t *testing.T) {
	// Create template data with computed fields
	type templateData struct {
		TestFile string
		Cmd      string
	}
	data := templateData{
		TestFile: "/tmp/trap_test.txt",
		Cmd:      "thiscommanddoesnotexist",
	}

	require.Error(t, lib.KubectlExecTemplate("generic-test", "deployment/writer", "debian", lib.GetTemplateAtlas().TestTrap, data))
	// trap has deleted file
	require.Error(t, lib.KubectlExecCommand("generic-test", "deployment/writer", "debian", "test -f /tmp/trap_test.txt"))
}

// TODO test all bash traps in templates to ensure they work as expected!
