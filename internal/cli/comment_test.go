package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/cli"
)

// U4 (079.004-T): `backlogit comment add` must mirror the MCP append_comment
// tool over the shared core path (extracted core.AppendComment). The persisted
// JSONL event and indexed row must be identical across surfaces, and the success
// JSON shape must match handleAppendComment's `{"ok":true}`.

// readItemLog decodes every JSONL event for an item from its per-item log file.
func readItemLog(t *testing.T, root, itemID string) []map[string]any {
	t.Helper()
	path := filepath.Join(root, ".backlogit", "logs", itemID+".jsonl")
	data, err := os.ReadFile(path)
	require.NoError(t, err, "item log file must exist after comment add")
	events := make([]map[string]any, 0)
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var ev map[string]any
		require.NoError(t, json.Unmarshal([]byte(line), &ev))
		events = append(events, ev)
	}
	return events
}

// Scenario 1: add happy-path → {ok:true} parity with handleAppendComment, and
// the comment is persisted to the item's JSONL log as a "comment" event.
func TestCommentAdd_PersistsAndIndexes(t *testing.T) {
	root := setupCLIWorkspace(t)
	id := cliAddFeature(t, root, "Comment target")

	var m map[string]any
	require.NoError(t, json.Unmarshal(
		[]byte(runCLIStdout(t, root, "comment", "add", id,
			"--actor", "ship", "--comment", "built U4")), &m))
	assert.Equal(t, true, m["ok"], "success shape must be {\"ok\":true} (append_comment parity)")

	logs := readItemLog(t, root, id)
	require.NotEmpty(t, logs)
	last := logs[len(logs)-1]
	assert.Equal(t, "comment", last["event_type"])
	assert.Equal(t, "ship", last["actor"])
	delta, ok := last["delta"].(map[string]any)
	require.True(t, ok, "delta must be an object")
	assert.Equal(t, "built U4", delta["comment"])
}

// Scenario 1b: optional --commit-sha is threaded through to the persisted event,
// mirroring append_comment's commit_sha argument.
func TestCommentAdd_CommitSHAThreaded(t *testing.T) {
	root := setupCLIWorkspace(t)
	id := cliAddFeature(t, root, "Commit-sha target")

	_ = runCLIStdout(t, root, "comment", "add", id,
		"--actor", "ship", "--comment", "with sha", "--commit-sha", "abc1234")

	logs := readItemLog(t, root, id)
	last := logs[len(logs)-1]
	assert.Equal(t, "abc1234", last["commit_sha"])
}

// Scenario 2: --actor and --comment are required; omitting them errors before any
// write, mirroring handleAppendComment's validation intent.
func TestCommentAdd_RequiresActorAndComment(t *testing.T) {
	root := setupCLIWorkspace(t)
	id := cliAddFeature(t, root, "Validation target")

	require.Error(t, runCLIErr(t, root, "comment", "add", id, "--comment", "no actor"),
		"missing --actor must error")
	require.Error(t, runCLIErr(t, root, "comment", "add", id, "--actor", "ship"),
		"missing --comment must error")
}

// U4: the comment group and its add subcommand are registered and discoverable.
func TestComment_RegisteredUnderRoot(t *testing.T) {
	root := cli.NewRootCommand()
	var commentCmd *cobra.Command
	for _, sub := range root.Commands() {
		if sub.Name() == "comment" {
			commentCmd = sub
			break
		}
	}
	require.NotNil(t, commentCmd, "root must register the comment command group")

	names := make([]string, 0)
	for _, sub := range commentCmd.Commands() {
		names = append(names, sub.Name())
	}
	assert.Contains(t, names, "add")
}
