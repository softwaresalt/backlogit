package cli_test

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/backlogit/backlogit/internal/cli"
)

func TestStashCommand_AddFetchAndHarvest(t *testing.T) {
	root := setupCLIWorkspace(t)
	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	cmd.SetArgs([]string{"--cwd", root, "stash", "add", "Deferred planner item", "--kind", "task", "--priority", "critical"})
	require.NoError(t, cmd.Execute())
	assert.Contains(t, buf.String(), "Deferred planner item")
	var added map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &added))
	stashID, _ := added["id"].(string)
	require.NotEmpty(t, stashID)
	assert.Equal(t, "critical", added["priority"])

	buf.Reset()
	cmd.SetArgs([]string{"--cwd", root, "stash", "fetch-stash", "--priority", "critical"})
	require.NoError(t, cmd.Execute())
	assert.Contains(t, buf.String(), "Deferred planner item")

	buf.Reset()
	cmd.SetArgs([]string{"--cwd", root, "stash", "harvest", stashID, "--type", "task"})
	require.NoError(t, cmd.Execute())
	assert.Contains(t, buf.String(), stashID)
}
