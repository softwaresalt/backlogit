package cli_test

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/cli"
)

func TestStashCommand_AddFetchAndHarvest(t *testing.T) {
	root := setupCLIWorkspace(t)

	// Create a parent feature for task harvesting
	featCmd := cli.NewRootCommand()
	featBuf := new(bytes.Buffer)
	featCmd.SetOut(featBuf)
	featCmd.SetErr(featBuf)
	featCmd.SetArgs([]string{"--cwd", root, "add", "--type", "feature", "--title", "Stash feature"})
	require.NoError(t, featCmd.Execute())
	featID := extractID(t, featBuf.String())

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
	cmd.SetArgs([]string{"--cwd", root, "stash", "harvest", stashID, "--type", "task", "--parent-id", featID})
	require.NoError(t, cmd.Execute())
	assert.Contains(t, buf.String(), stashID)
}

func TestStashCommand_ListAlias(t *testing.T) {
	root := setupCLIWorkspace(t)
	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	cmd.SetArgs([]string{"--cwd", root, "stash", "add", "List alias item", "--kind", "feature", "--priority", "high"})
	require.NoError(t, cmd.Execute())

	buf.Reset()
	cmd.SetArgs([]string{"--cwd", root, "stash", "list", "--priority", "high"})
	require.NoError(t, cmd.Execute())
	assert.Contains(t, buf.String(), "List alias item")
}

func TestStashCommand_ListKindFilter(t *testing.T) {
	root := setupCLIWorkspace(t)
	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	cmd.SetArgs([]string{"--cwd", root, "stash", "add", "Feature item", "--kind", "feature"})
	require.NoError(t, cmd.Execute())
	buf.Reset()
	cmd.SetArgs([]string{"--cwd", root, "stash", "add", "Task item", "--kind", "task"})
	require.NoError(t, cmd.Execute())

	buf.Reset()
	cmd.SetArgs([]string{"--cwd", root, "stash", "list", "--kind", "feature"})
	require.NoError(t, cmd.Execute())
	assert.Contains(t, buf.String(), "Feature item")
	assert.NotContains(t, buf.String(), "Task item")
}

func TestStashCommand_Get(t *testing.T) {
	root := setupCLIWorkspace(t)
	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	cmd.SetArgs([]string{"--cwd", root, "stash", "add", "Get test item", "--kind", "task", "--priority", "medium"})
	require.NoError(t, cmd.Execute())
	var added map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &added))
	stashID, _ := added["id"].(string)
	require.NotEmpty(t, stashID)

	buf.Reset()
	cmd.SetArgs([]string{"--cwd", root, "stash", "get", stashID})
	require.NoError(t, cmd.Execute())
	assert.Contains(t, buf.String(), "Get test item")
	assert.Contains(t, buf.String(), stashID)
}

func TestStashCommand_Edit(t *testing.T) {
	root := setupCLIWorkspace(t)
	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	cmd.SetArgs([]string{"--cwd", root, "stash", "add", "Original text", "--kind", "task", "--priority", "low"})
	require.NoError(t, cmd.Execute())
	var added map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &added))
	stashID, _ := added["id"].(string)
	require.NotEmpty(t, stashID)

	buf.Reset()
	cmd.SetArgs([]string{"--cwd", root, "stash", "edit", stashID, "--text", "Updated text", "--priority", "high"})
	require.NoError(t, cmd.Execute())
	assert.Contains(t, buf.String(), "Updated text")
	assert.Contains(t, buf.String(), "high")
}

func TestStashCommand_Edit_NoFlags(t *testing.T) {
	root := setupCLIWorkspace(t)
	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	cmd.SetArgs([]string{"--cwd", root, "stash", "add", "Some text", "--kind", "task"})
	require.NoError(t, cmd.Execute())
	var added map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &added))
	stashID, _ := added["id"].(string)
	require.NotEmpty(t, stashID)

	buf.Reset()
	cmd2 := cli.NewRootCommand()
	cmd2.SetOut(buf)
	cmd2.SetErr(buf)
	cmd2.SetArgs([]string{"--cwd", root, "stash", "edit", stashID})
	err := cmd2.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "at least one of --text, --kind, or --priority is required")
}

func TestStashCommand_Remove(t *testing.T) {
	root := setupCLIWorkspace(t)
	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	cmd.SetArgs([]string{"--cwd", root, "stash", "add", "Item to remove", "--kind", "task"})
	require.NoError(t, cmd.Execute())
	var added map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &added))
	stashID, _ := added["id"].(string)
	require.NotEmpty(t, stashID)

	buf.Reset()
	cmd.SetArgs([]string{"--cwd", root, "stash", "remove", stashID})
	require.NoError(t, cmd.Execute())
	assert.Contains(t, buf.String(), stashID)
	assert.Contains(t, buf.String(), "removed")

	buf.Reset()
	cmd.SetArgs([]string{"--cwd", root, "stash", "list"})
	require.NoError(t, cmd.Execute())
	assert.NotContains(t, buf.String(), "Item to remove")
}

func TestStashCommand_FetchReturnsLinkedDeliberation(t *testing.T) {
	root := setupCLIWorkspace(t)
	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	cmd.SetArgs([]string{"--cwd", root, "stash", "add", "Capture queue redesign follow-up", "--kind", "feature", "--priority", "critical"})
	require.NoError(t, cmd.Execute())

	var added map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &added))
	stashID, _ := added["id"].(string)
	require.NotEmpty(t, stashID)

	buf.Reset()
	cmd.SetArgs([]string{"--cwd", root, "deliberate", stashID, "--notes", "Capture the reasons before implementation."})
	require.NoError(t, cmd.Execute())

	buf.Reset()
	cmd.SetArgs([]string{"--cwd", root, "stash", "fetch-stash", "--priority", "critical"})
	require.NoError(t, cmd.Execute())
	assert.Contains(t, buf.String(), `"deliberation_id"`)
	assert.Contains(t, buf.String(), `"artifact_type": "deliberation"`)
}
