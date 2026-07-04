package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/cli"
)

// U3 (079.003-T): `backlogit memory save --key --summary` must mirror the MCP
// save_memory tool over events.SaveMemory, writing .backlogit/memories.json at
// the resolved workspace root and returning {ok:true}.

// Scenario 1: save writes a readable entry and returns {ok:true} shape parity.
func TestMemorySave_WritesReadableEntry(t *testing.T) {
	root := setupCLIWorkspace(t)
	var resp struct {
		OK bool `json:"ok"`
	}
	require.NoError(t, json.Unmarshal(
		[]byte(runCLIStdout(t, root, "memory", "save", "--key", "k1", "--summary", "hello world")), &resp))
	assert.True(t, resp.OK, "memory save must return {ok:true}")

	data, err := os.ReadFile(filepath.Join(root, ".backlogit", "memories.json"))
	require.NoError(t, err, "memories.json must be written at the resolved workspace root")
	var memories map[string]string
	require.NoError(t, json.Unmarshal(data, &memories))
	assert.Equal(t, "hello world", memories["k1"])
}

// Scenario 2: missing --key or --summary → required-flag error and no partial write.
func TestMemorySave_MissingRequiredFlags(t *testing.T) {
	root := setupCLIWorkspace(t)
	require.Error(t, runCLIErr(t, root, "memory", "save", "--summary", "no key"),
		"missing --key must error")
	require.Error(t, runCLIErr(t, root, "memory", "save", "--key", "no summary"),
		"missing --summary must error")

	_, statErr := os.Stat(filepath.Join(root, ".backlogit", "memories.json"))
	assert.True(t, os.IsNotExist(statErr), "no memories file should be written on a validation error")
}

// U3: the memory group and its save subcommand are registered.
func TestMemory_RegisteredUnderRoot(t *testing.T) {
	root := cli.NewRootCommand()
	var memCmd *cobra.Command
	for _, sub := range root.Commands() {
		if sub.Name() == "memory" {
			memCmd = sub
			break
		}
	}
	require.NotNil(t, memCmd, "root must register the memory command group")

	names := make([]string, 0)
	for _, sub := range memCmd.Commands() {
		names = append(names, sub.Name())
	}
	assert.Contains(t, names, "save")
}
