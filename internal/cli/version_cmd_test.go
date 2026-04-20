package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestVersionCommand_HumanOutput asserts that `backlogit version` prints
// version, commit, build date, and Go runtime version in human-readable form.
func TestVersionCommand_HumanOutput(t *testing.T) {
	cmd := newVersionCommand()
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{})

	err := cmd.Execute()
	require.NoError(t, err, "version command should not return an error")

	out := buf.String()
	assert.True(t, strings.Contains(out, "version"), "output should contain version label")
	assert.True(t, strings.Contains(out, "commit"), "output should contain commit label")
	assert.True(t, strings.Contains(out, "build"), "output should contain build date label")
	assert.True(t, strings.Contains(out, "go"), "output should contain go version label")
}

// TestVersionCommand_JSONOutput asserts that `backlogit version --format json`
// emits a JSON object with version, commit, build_date, and go_version fields.
func TestVersionCommand_JSONOutput(t *testing.T) {
	cmd := newVersionCommand()
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--format", "json"})

	err := cmd.Execute()
	require.NoError(t, err, "version command --format json should not error")

	var out map[string]string
	require.NoError(t, json.Unmarshal(buf.Bytes(), &out), "output must be valid JSON")
	assert.NotEmpty(t, out["version"], "JSON output must have version field")
	assert.NotEmpty(t, out["commit"], "JSON output must have commit field")
	assert.Contains(t, out, "build_date", "JSON output must have build_date field")
	assert.NotEmpty(t, out["go_version"], "JSON output must have go_version field")
}

// TestVersionCommand_RegisteredOnRoot asserts that the root command includes
// the "version" subcommand after wiring.
func TestVersionCommand_RegisteredOnRoot(t *testing.T) {
	root := NewRootCommand()
	var found bool
	for _, c := range root.Commands() {
		if c.Name() == "version" {
			found = true
			break
		}
	}
	assert.True(t, found, "root command should have a 'version' subcommand registered")
}
