package cli_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/cli"
	blerrors "github.com/softwaresalt/backlogit/internal/errors"
)

const (
	cliInWindowLegacyDump = `{"agent":"ship","session_id":"s1","phase":"build"}`
	cliOutOfWindowDump    = `{"consumer":"ship","phase":"build"}`
)

// 171.003-T: the migration-only --allow-legacy-import flag on checkpoint create.

func TestCheckpointCreate_AllowLegacyImportFlagDefaultsFalse(t *testing.T) {
	root := cli.NewRootCommand()
	create, _, err := root.Find([]string{"checkpoint", "create"})
	require.NoError(t, err)
	flag := create.Flags().Lookup("allow-legacy-import")
	require.NotNil(t, flag, "checkpoint create must declare --allow-legacy-import")
	assert.Equal(t, "false", flag.DefValue)
	assert.Equal(t, "bool", flag.Value.Type())
}

func TestCheckpointCreate_LegacyDumpRejectedByDefaultWithFlagRemediation(t *testing.T) {
	root := setupCLIWorkspace(t)
	err := runCLIErr(t, root, "checkpoint", "create", "--state-dump", cliInWindowLegacyDump)
	require.Error(t, err)
	assert.True(t, errors.Is(err, blerrors.ErrCheckpointSchemaRejected), "sentinel must survive via %%w: %v", err)
	assert.Contains(t, err.Error(), "--allow-legacy-import")
	assertNoCheckpointFiles(t, root)
}

func TestCheckpointCreate_AllowLegacyImportUpgradesInWindowDump(t *testing.T) {
	root := setupCLIWorkspace(t)
	out := runCLIStdout(t, root, "checkpoint", "create", "--allow-legacy-import", "--state-dump", cliInWindowLegacyDump)
	var created struct {
		Path string `json:"path"`
	}
	require.NoError(t, json.Unmarshal([]byte(out), &created))
	require.NotEmpty(t, created.Path)

	getOut := runCLIStdout(t, root, "checkpoint", "get", filepath.Base(created.Path))
	var got struct {
		Valid      bool `json:"valid"`
		Checkpoint struct {
			SchemaVersion int `json:"schema_version"`
		} `json:"checkpoint"`
	}
	require.NoError(t, json.Unmarshal([]byte(getOut), &got))
	assert.True(t, got.Valid)
	assert.Equal(t, 1, got.Checkpoint.SchemaVersion, "import upgrades to V1, never verbatim")
}

func TestCheckpointCreate_OutOfWindowDumpStillFailsWithoutFlagRemediation(t *testing.T) {
	for _, args := range [][]string{
		{"checkpoint", "create", "--state-dump", cliOutOfWindowDump},
		{"checkpoint", "create", "--allow-legacy-import", "--state-dump", cliOutOfWindowDump},
	} {
		root := setupCLIWorkspace(t)
		err := runCLIErr(t, root, args...)
		require.Error(t, err)
		assert.True(t, errors.Is(err, blerrors.ErrCheckpointSchemaUnsupported), "got %v", err)
		assert.False(t, errors.Is(err, blerrors.ErrCheckpointSchemaRejected))
		assert.NotContains(t, err.Error(), "--allow-legacy-import", "enabling the flag cannot help")
		assertNoCheckpointFiles(t, root)
	}
}

// With the opt-in set, unsupported dumps (out-of-window, future schema) must never advise the flag.
func TestCheckpointCreate_OptInUnsupportedDumpNeverAdvisesFlag(t *testing.T) {
	dumps := map[string]string{
		"out_of_window":  cliOutOfWindowDump,
		"future_version": `{"schema_version":2,"agent":"ship","session_id":"s1","phase":"build"}`,
	}
	for name, dump := range dumps {
		t.Run(name, func(t *testing.T) {
			root := setupCLIWorkspace(t)
			err := runCLIErr(t, root, "checkpoint", "create", "--allow-legacy-import", "--state-dump", dump)
			require.Error(t, err)
			assert.NotContains(t, err.Error(), "--allow-legacy-import")
			assert.NotContains(t, err.Error(), "re-run with")
			assertNoCheckpointFiles(t, root)
		})
	}
}

func TestCheckpointCreate_HelpDescribesDefaultRejectAndOptInScope(t *testing.T) {
	cmd := cli.NewRootCommand()
	out := new(bytes.Buffer)
	cmd.SetOut(out)
	cmd.SetErr(out)
	cmd.SetArgs([]string{"checkpoint", "create", "--help"})
	require.NoError(t, cmd.Execute())
	help := out.String()
	assert.Contains(t, help, "--allow-legacy-import")
	assert.NotContains(t, help, "written verbatim")
	assert.Contains(t, help, "rejected")
	assert.Contains(t, help, "upgrade")
}

// 171.004-T: the CLI error must not echo unbounded caller-controlled input.
func TestCheckpointCreate_SchemaErrorDiagnosticIsBounded(t *testing.T) {
	dumps := map[string]string{
		"invalid_created_at": `{"agent":"ship","session_id":"s1","phase":"build","created_at":"` + strings.Repeat("x", 60000) + `"}`,
		"duplicate_key": `{"agent":"ship","session_id":"s1","phase":"build","context":{"` +
			strings.Repeat("k", 30000) + `":1,"` + strings.Repeat("k", 30000) + `":2}}`,
	}
	for name, dump := range dumps {
		t.Run(name, func(t *testing.T) {
			root := setupCLIWorkspace(t)
			err := runCLIErr(t, root, "checkpoint", "create", "--state-dump", dump)
			require.Error(t, err)
			assert.Less(t, len(err.Error()), 2048)
			assert.Contains(t, err.Error(), "[truncated]")
			assert.True(t, utf8.ValidString(err.Error()))
			assert.True(t, errors.Is(err, blerrors.ErrCheckpointSchemaUnsupported), "sentinel must survive: %v", err)
		})
	}
}
