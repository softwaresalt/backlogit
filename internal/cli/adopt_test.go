package cli

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewAdoptCommand_HasParentFlag(t *testing.T) {
	cwd := "."
	cmd := newAdoptCommand(&cwd)

	require.NotNil(t, cmd)
	assert.Equal(t, "adopt <item-id>", cmd.Use)
	assert.NotNil(t, cmd.Flags().Lookup("parent"), "adopt must have --parent flag")
}

func TestNewAdoptCommand_RequiresArg(t *testing.T) {
	cwd := "."
	cmd := newAdoptCommand(&cwd)
	cmd.SetArgs([]string{})

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	err := cmd.Execute()
	require.Error(t, err, "should fail without an item-id argument")
}

func TestNewAdoptCommand_RequiresParent(t *testing.T) {
	cwd := "."
	cmd := newAdoptCommand(&cwd)
	cmd.SetArgs([]string{"T001"})

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	err := cmd.Execute()
	require.Error(t, err, "should fail without --parent flag")
}
