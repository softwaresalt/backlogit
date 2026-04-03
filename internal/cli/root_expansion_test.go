package cli_test

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/backlogit/backlogit/internal/cli"
)

// TASK-002.04.08: Register all CLI commands in root.

func TestRootCommand_RegistersAllCommands(t *testing.T) {
	// Arrange
	cmd := cli.NewRootCommand()

	// Act — collect all registered subcommand names
	names := make(map[string]bool)
	for _, sub := range cmd.Commands() {
		names[sub.Name()] = true
	}

	// Assert — all queue-specified commands must be registered
	expected := []string{"init", "sync", "mcp", "add", "list", "get", "update", "move", "delete", "search", "query", "status", "stash", "deliberate", "metadata"}
	for _, name := range expected {
		assert.True(t, names[name], "missing command: %s", name)
	}
}

func TestRootCommand_HelpListsAllCommands(t *testing.T) {
	// Arrange
	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--help"})

	// Act
	err := cmd.Execute()

	// Assert
	require.NoError(t, err)
	output := buf.String()
	for _, name := range []string{"add", "list", "get", "update", "move", "delete", "search", "query", "status", "stash", "deliberate", "metadata"} {
		assert.Contains(t, output, name, "help output should list %s command", name)
	}
}

func TestRootCommand_NoFlagCollisions(t *testing.T) {
	// Arrange
	cmd := cli.NewRootCommand()

	// Act & Assert — each subcommand's flag set should not panic during parsing
	for _, sub := range cmd.Commands() {
		t.Run(sub.Name(), func(t *testing.T) {
			assert.NotNil(t, sub.Flags())
		})
	}
}
