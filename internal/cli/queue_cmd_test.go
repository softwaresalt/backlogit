package cli_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/cli"
)

func TestNewQueueCmd_HasSubcommands(t *testing.T) {
	// Act
	cmd := cli.NewQueueCmd()

	// Assert
	require.NotNil(t, cmd)
	assert.Equal(t, "queue", cmd.Use)
	subNames := make([]string, 0, len(cmd.Commands()))
	for _, sub := range cmd.Commands() {
		subNames = append(subNames, sub.Name())
	}
	assert.Contains(t, subNames, "view")
	assert.Contains(t, subNames, "move")
	assert.Contains(t, subNames, "bulk-status")
}

func TestNewQueueViewCmd_HasFilterFlags(t *testing.T) {
	// Act
	cmd := cli.NewQueueViewCmd()

	// Assert
	assert.NotNil(t, cmd.Flags().Lookup("type"))
	assert.NotNil(t, cmd.Flags().Lookup("status"))
	assert.NotNil(t, cmd.Flags().Lookup("group-by"))
	assert.NotNil(t, cmd.Flags().Lookup("sort"))
}

func TestNewQueueMoveCmd_RequiresPosition(t *testing.T) {
	// Act
	cmd := cli.NewQueueMoveCmd()

	// Assert
	flag := cmd.Flags().Lookup("position")
	assert.NotNil(t, flag, "should have --position flag")
}

func TestNewQueueBulkStatusCmd_HasRequiredFlags(t *testing.T) {
	// Act
	cmd := cli.NewQueueBulkStatusCmd()

	// Assert
	assert.NotNil(t, cmd.Flags().Lookup("ids"))
	assert.NotNil(t, cmd.Flags().Lookup("status"))
}
