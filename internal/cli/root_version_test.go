package cli

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/version"
)

func TestNewRootCommand_VersionIsSet(t *testing.T) {
	cmd := NewRootCommand()
	assert.Equal(t, version.Resolve(), cmd.Version, "root command version should match the resolved version")
}

func TestNewRootCommand_VersionFlag(t *testing.T) {
	withCurrentVersion(t, "1.0.0")
	cmd := withVersionLatestLookup(t, func(context.Context) (string, error) {
		return "v9.9.9", nil
	})

	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--version"})
	err := cmd.Execute()
	require.NoError(t, err)
	assert.Contains(t, buf.String(), version.Resolve(), "--version output should contain the resolved version string")
	assert.Contains(t, buf.String(), "v9.9.9", "--version output should include the latest available version")
	assert.Contains(t, buf.String(), "update available", "--version output should show update availability")
}

func TestNewRootCommand_VersionFlagNoUpdateCheck(t *testing.T) {
	called := false
	cmd := withVersionLatestLookup(t, func(context.Context) (string, error) {
		called = true
		return "v9.9.9", nil
	})

	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--no-update-check", "--version"})

	err := cmd.Execute()
	require.NoError(t, err)
	assert.False(t, called, "--no-update-check should skip lookup for root --version")
	assert.Contains(t, buf.String(), "update check skipped")
}

func TestNewRootCommand_VersionSubcommandInheritsNoUpdateCheck(t *testing.T) {
	called := false
	cmd := withVersionLatestLookup(t, func(context.Context) (string, error) {
		called = true
		return "v9.9.9", nil
	})

	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--no-update-check", "version"})

	err := cmd.Execute()
	require.NoError(t, err)
	assert.False(t, called, "root --no-update-check should skip lookup for version subcommand")
	assert.Contains(t, buf.String(), "update check skipped")
}
