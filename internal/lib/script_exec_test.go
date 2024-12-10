package lib_test

import (
	_ "embed"
	"testing"
	"text/template"

	"github.com/allaboutapps/backup-ns/internal/lib"
	"github.com/stretchr/testify/require"
)

//go:embed templates/test_trap.sh.tmpl
var testTrapScript string

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

	tmpl, err := template.New("traptest").Parse(testTrapScript)
	require.NoError(t, err)

	require.NoError(t, lib.KubectlExecTemplate("generic-test", "deployment/writer", "debian", tmpl, data))
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

	tmpl, err := template.New("traptest").Parse(testTrapScript)
	require.NoError(t, err)

	require.Error(t, lib.KubectlExecTemplate("generic-test", "deployment/writer", "debian", tmpl, data))
	// trap has deleted file
	require.Error(t, lib.KubectlExecCommand("generic-test", "deployment/writer", "debian", "test -f /tmp/trap_test.txt"))
}

// TODO test all bash traps in templates to ensure they work as expected!
