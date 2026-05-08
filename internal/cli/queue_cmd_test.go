package cli_test

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/cli"
)

type queueViewResult struct {
	Items []struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	} `json:"items"`
}

func runRootCommand(t *testing.T, args ...string) (string, error) {
	t.Helper()

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs(args)

	err := cmd.Execute()
	return buf.String(), err
}

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

func TestQueueView_RejectsInvalidFormat(t *testing.T) {
	root := setupCLIWorkspace(t)
	t.Chdir(root)

	output, err := runRootCommand(t, "queue", "view", "--format", "banana")

	require.Error(t, err)
	assert.Contains(t, err.Error(), `"banana"`)
	assert.NotEmpty(t, output)
}

func TestQueueView_UsesCWDWorkspace(t *testing.T) {
	targetRoot := setupCLIWorkspace(t)
	otherRoot := setupCLIWorkspace(t)

	addOutput, err := runRootCommand(t, "--cwd", targetRoot, "add", "--type", "feature", "--title", "Target feature")
	require.NoError(t, err)
	targetID := extractID(t, addOutput)

	t.Chdir(otherRoot)
	viewOutput, err := runRootCommand(t, "--cwd", targetRoot, "queue", "view", "--format", "json")
	require.NoError(t, err)

	var view queueViewResult
	require.NoError(t, json.Unmarshal([]byte(viewOutput), &view))
	require.Len(t, view.Items, 1)
	assert.Equal(t, targetID, view.Items[0].ID)
}

func TestQueueView_DefaultExcludesDoneItems(t *testing.T) {
	root := setupCLIWorkspace(t)

	queuedOutput, err := runRootCommand(t, "--cwd", root, "add", "--type", "feature", "--title", "Queued feature")
	require.NoError(t, err)
	queuedID := extractID(t, queuedOutput)

	doneOutput, err := runRootCommand(t, "--cwd", root, "add", "--type", "feature", "--title", "Done feature", "--status", "done")
	require.NoError(t, err)
	_ = extractID(t, doneOutput)

	viewOutput, err := runRootCommand(t, "--cwd", root, "queue", "view", "--format", "json")
	require.NoError(t, err)

	var view queueViewResult
	require.NoError(t, json.Unmarshal([]byte(viewOutput), &view))
	require.Len(t, view.Items, 1)
	assert.Equal(t, queuedID, view.Items[0].ID)
}

func TestQueueMove_ReordersVisibleQueue(t *testing.T) {
	targetRoot := setupCLIWorkspace(t)
	otherRoot := setupCLIWorkspace(t)

	firstOutput, err := runRootCommand(t, "--cwd", targetRoot, "add", "--type", "feature", "--title", "First feature")
	require.NoError(t, err)
	firstID := extractID(t, firstOutput)

	secondOutput, err := runRootCommand(t, "--cwd", targetRoot, "add", "--type", "feature", "--title", "Second feature")
	require.NoError(t, err)
	secondID := extractID(t, secondOutput)

	t.Chdir(otherRoot)
	_, err = runRootCommand(t, "--cwd", targetRoot, "queue", "move", secondID, "--position", "1")
	require.NoError(t, err)

	viewOutput, err := runRootCommand(t, "--cwd", targetRoot, "queue", "view", "--format", "json")
	require.NoError(t, err)

	var view queueViewResult
	require.NoError(t, json.Unmarshal([]byte(viewOutput), &view))
	require.Len(t, view.Items, 2)
	assert.Equal(t, secondID, view.Items[0].ID)
	assert.Equal(t, firstID, view.Items[1].ID)
}
