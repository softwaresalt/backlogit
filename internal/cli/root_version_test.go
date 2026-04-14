package cli

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/backlogit/backlogit/internal/version"
)

func TestNewRootCommand_VersionIsSet(t *testing.T) {
	cmd := NewRootCommand()
	assert.Equal(t, version.Version, cmd.Version, "root command version should match version package")
}

func TestNewRootCommand_VersionFlag(t *testing.T) {
	cmd := NewRootCommand()
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--version"})
	err := cmd.Execute()
	require.NoError(t, err)
	assert.Contains(t, buf.String(), version.Version, "--version output should contain the version string")
}
