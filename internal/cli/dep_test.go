package cli_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/cli"
)

func TestNewDepCmd_HasSubcommands(t *testing.T) {
	// Act
	cmd := cli.NewDepCmd()

	// Assert
	require.NotNil(t, cmd)
	assert.Equal(t, "dep", cmd.Use)
	subNames := make([]string, 0, len(cmd.Commands()))
	for _, sub := range cmd.Commands() {
		subNames = append(subNames, sub.Name())
	}
	assert.Contains(t, subNames, "add")
	assert.Contains(t, subNames, "remove")
	assert.Contains(t, subNames, "list")
}

func TestNewDepAddCmd_RejectsMissingArgs(t *testing.T) {
	// Act
	cmd := cli.NewDepAddCmd()
	cmd.SetArgs([]string{})
	err := cmd.Execute()

	// Assert
	require.Error(t, err)
}

func TestNewDepListCmd_HasReverseFlag(t *testing.T) {
	// Act
	cmd := cli.NewDepListCmd()

	// Assert
	flag := cmd.Flags().Lookup("reverse")
	assert.NotNil(t, flag, "should have --reverse flag")
}
