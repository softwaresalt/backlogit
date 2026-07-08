package cli

import (
	"bytes"
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
	cmd := NewRootCommand()
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--version"})
	err := cmd.Execute()
	require.NoError(t, err)
	assert.Contains(t, buf.String(), version.Resolve(), "--version output should contain the resolved version string")
}
