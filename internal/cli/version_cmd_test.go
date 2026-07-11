package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/version"
)

// TestVersionCommand_HumanOutput asserts that `backlogit version` prints
// version, commit, build date, and Go runtime version in human-readable form.
func TestVersionCommand_HumanOutput(t *testing.T) {
	cmd := newVersionCommandWithLookup(func(context.Context) (string, error) {
		return "", errors.New("unexpected lookup")
	})
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--no-update-check"})

	err := cmd.Execute()
	require.NoError(t, err, "version command should not return an error")

	out := buf.String()
	assert.True(t, strings.Contains(out, "version"), "output should contain version label")
	assert.True(t, strings.Contains(out, "commit"), "output should contain commit label")
	assert.True(t, strings.Contains(out, "build"), "output should contain build date label")
	assert.True(t, strings.Contains(out, "go"), "output should contain go version label")
}

func TestVersionCommand_HumanOutputShowsUpdateAvailable(t *testing.T) {
	withCurrentVersion(t, "1.0.0")
	cmd := newVersionCommandWithLookup(func(context.Context) (string, error) {
		return "v9.9.9", nil
	})
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{})

	err := cmd.Execute()
	require.NoError(t, err)

	out := buf.String()
	assert.Contains(t, out, "latest", "output should show the latest available version")
	assert.Contains(t, out, "v9.9.9", "output should include the latest tag")
	assert.Contains(t, out, "update available", "output should identify newer releases")
	assert.Contains(t, out, "backlogit update", "output should hint at the update command")
}

func TestVersionCommand_GracefulUnavailable(t *testing.T) {
	cmd := newVersionCommandWithLookup(func(context.Context) (string, error) {
		return "", errors.New("offline")
	})
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{})

	err := cmd.Execute()
	require.NoError(t, err, "update check failures must not break version output")

	out := buf.String()
	assert.Contains(t, out, "version", "installed version should still print")
	assert.Contains(t, out, "update check unavailable", "failure should degrade with a brief note")
}

func TestVersionCommand_ShowsLatestWhenCurrentIsUncomparable(t *testing.T) {
	withCurrentVersion(t, version.DevVersion)
	cmd := newVersionCommandWithLookup(func(context.Context) (string, error) {
		return "v9.9.9", nil
	})
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{})

	err := cmd.Execute()
	require.NoError(t, err)

	out := buf.String()
	assert.Contains(t, out, "v9.9.9", "successful lookup should still show latest tag")
	assert.Contains(t, out, "update status unavailable", "uncomparable current version should be distinct from lookup failure")
}

func TestVersionCommand_UpdateCheckTimeoutBudget(t *testing.T) {
	assert.LessOrEqual(t, updateCheckTimeout, 1*time.Second, "default version checks should stay fast")
}

func TestVersionCommand_NoUpdateCheckFlagSkipsLookup(t *testing.T) {
	called := false
	cmd := newVersionCommandWithLookup(func(context.Context) (string, error) {
		called = true
		return "v9.9.9", nil
	})
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--no-update-check"})

	err := cmd.Execute()
	require.NoError(t, err)

	assert.False(t, called, "--no-update-check should skip the remote lookup")
	assert.Contains(t, buf.String(), "update check skipped")
}

func TestVersionCommand_EnvSkipsLookup(t *testing.T) {
	t.Setenv("BACKLOGIT_NO_UPDATE_CHECK", "1")
	called := false
	cmd := newVersionCommandWithLookup(func(context.Context) (string, error) {
		called = true
		return "v9.9.9", nil
	})
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{})

	err := cmd.Execute()
	require.NoError(t, err)

	assert.False(t, called, "BACKLOGIT_NO_UPDATE_CHECK should skip the remote lookup")
	assert.Contains(t, buf.String(), "update check skipped")
}

// TestVersionCommand_JSONOutput asserts that `backlogit version --format json`
// emits a JSON object with version, commit, build_date, go_version, and update
// check fields.
func TestVersionCommand_JSONOutput(t *testing.T) {
	withCurrentVersion(t, "1.0.0")
	cmd := newVersionCommandWithLookup(func(context.Context) (string, error) {
		return "v9.9.9", nil
	})
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--format", "json"})

	err := cmd.Execute()
	require.NoError(t, err, "version command --format json should not error")

	var out map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &out), "output must be valid JSON")
	assert.NotEmpty(t, out["version"], "JSON output must have version field")
	assert.NotEmpty(t, out["commit"], "JSON output must have commit field")
	assert.Contains(t, out, "build_date", "JSON output must have build_date field")
	assert.NotEmpty(t, out["go_version"], "JSON output must have go_version field")
	assert.NotEmpty(t, out["current"], "JSON output must have current field")
	assert.Equal(t, "v9.9.9", out["latest"], "JSON output must have latest field")
	assert.Equal(t, true, out["update_available"], "JSON output must show update availability")
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

func withVersionLatestLookup(t *testing.T, lookup latestVersionLookupFunc) *cobra.Command {
	t.Helper()

	return newRootCommandImpl(&jsonrpcInterceptor{}, lookup)
}

func withCurrentVersion(t *testing.T, current string) {
	t.Helper()

	orig := version.Version
	version.Version = current
	t.Cleanup(func() {
		version.Version = orig
	})
}
