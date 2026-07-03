package cli_test

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/cli"
	"github.com/softwaresalt/backlogit/internal/core"
	bldb "github.com/softwaresalt/backlogit/internal/db"
)

// runCLIStdout executes a fresh root command against the workspace and returns
// stdout. slog logging is emitted to the process stderr (root.go), so the
// stdout buffer holds only the command's rendered output.
func runCLIStdout(t *testing.T, root string, args ...string) string {
	t.Helper()
	cmd := cli.NewRootCommand()
	out := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	cmd.SetOut(out)
	cmd.SetErr(errBuf)
	cmd.SetArgs(append([]string{"--cwd", root}, args...))
	require.NoError(t, cmd.Execute(), "cli %v failed: %s", args, errBuf.String())
	return out.String()
}

func cliAddFeature(t *testing.T, root, title string) string {
	t.Helper()
	return extractID(t, runCLIStdout(t, root, "add", "--type", "feature", "--title", title))
}

func cliAddTask(t *testing.T, root, title, parent string) string {
	t.Helper()
	return extractID(t, runCLIStdout(t, root, "add", "--type", "task", "--title", title, "--parent", parent))
}

func cliCreateShipment(t *testing.T, root, title, items string) string {
	t.Helper()
	var m map[string]any
	require.NoError(t, json.Unmarshal([]byte(runCLIStdout(t, root, "shipment", "create", "--title", title, "--items", items)), &m))
	id, _ := m["id"].(string)
	require.NotEmpty(t, id, "shipment create must return an id")
	return id
}

// readArtifactFile (from add_test.go, same package) locates the stored Markdown
// file for an artifact ID under the workspace .backlogit tree.

// dbRowJSON opens the workspace read-only and returns the marshaled index row.
func dbRowJSON(t *testing.T, root, id string) string {
	t.Helper()
	ctx := context.Background()
	ws, err := core.NewWorkspace(ctx, root)
	require.NoError(t, err)
	defer ws.Close()
	art, err := bldb.GetItem(ctx, ws.DB, id)
	require.NoError(t, err)
	b, err := json.Marshal(art)
	require.NoError(t, err)
	return string(b)
}

// Unit 2 scenario 1: shipment list table view gains a COVERING FEATURE column
// while the shared list/queue views are unchanged.
func TestShipmentList_TableIncludesCoveringFeatureColumn(t *testing.T) {
	root := setupCLIWorkspace(t)
	featID := cliAddFeature(t, root, "Covered feature")
	_ = cliCreateShipment(t, root, "Table shipment", featID)

	out := runCLIStdout(t, root, "shipment", "list", "--format", "table")
	assert.Contains(t, out, "COVERING FEATURE", "shipment table must carry the covering feature column")
	assert.Contains(t, out, featID)
	assert.Contains(t, out, "Covered feature")

	// The shared `list` view must NOT gain the shipment-only column.
	listOut := runCLIStdout(t, root, "list", "--format", "table")
	assert.NotContains(t, listOut, "COVERING FEATURE", "shared list view must not inherit the shipment column")
}

// Unit 2 scenario 2: shipment get / list --json emit a top-level covering_feature
// object and never leak the derived data into custom_fields.
func TestShipmentGet_JSONIncludesTopLevelCoveringFeature(t *testing.T) {
	root := setupCLIWorkspace(t)
	featID := cliAddFeature(t, root, "Covered feature")
	shipID := cliCreateShipment(t, root, "JSON shipment", featID)

	var m map[string]any
	require.NoError(t, json.Unmarshal([]byte(runCLIStdout(t, root, "shipment", "get", shipID)), &m))

	cf, ok := m["covering_feature"].(map[string]any)
	require.True(t, ok, "covering_feature object must be present at top level")
	assert.Equal(t, featID, cf["id"])
	assert.Equal(t, "Covered feature", cf["title"])

	custom, _ := m["custom_fields"].(map[string]any)
	_, leaked := custom["covering_feature"]
	assert.False(t, leaked, "covering_feature must never appear inside custom_fields")
}

func TestShipmentList_JSONIncludesTopLevelCoveringFeature(t *testing.T) {
	root := setupCLIWorkspace(t)
	featID := cliAddFeature(t, root, "Covered feature")
	shipID := cliCreateShipment(t, root, "JSON list shipment", featID)

	var arr []map[string]any
	require.NoError(t, json.Unmarshal([]byte(runCLIStdout(t, root, "shipment", "list", "--format", "json")), &arr))
	var found map[string]any
	for _, m := range arr {
		if m["id"] == shipID {
			found = m
			break
		}
	}
	require.NotNil(t, found, "created shipment must appear in list")
	cf, ok := found["covering_feature"].(map[string]any)
	require.True(t, ok, "covering_feature must be present in list json")
	assert.Equal(t, featID, cf["id"])
}

// Unit 2 scenario 3: a zero-feature shipment omits covering_feature from JSON and
// renders an empty column without panicking.
func TestShipmentGet_ZeroFeatureOmitsCoveringFeature(t *testing.T) {
	root := setupCLIWorkspace(t)
	featID := cliAddFeature(t, root, "Parent for task")
	taskID := cliAddTask(t, root, "Only a task", featID)
	shipID := cliCreateShipment(t, root, "Task-only shipment", taskID)

	var m map[string]any
	require.NoError(t, json.Unmarshal([]byte(runCLIStdout(t, root, "shipment", "get", shipID)), &m))
	_, present := m["covering_feature"]
	assert.False(t, present, "covering_feature must be omitted for a zero-feature shipment")

	tbl := runCLIStdout(t, root, "shipment", "list", "--format", "table")
	assert.Contains(t, tbl, "COVERING FEATURE", "column header present even when a shipment has no covering feature")
}

// Unit 2 scenario 4: rendering shipment views mutates neither the stored file nor
// the DB row, and never persists covering_feature.
func TestShipmentViews_ReadOnly_NoStoreMutation(t *testing.T) {
	root := setupCLIWorkspace(t)
	featID := cliAddFeature(t, root, "Covered feature")
	shipID := cliCreateShipment(t, root, "Read-only shipment", featID)

	fileBefore := readArtifactFile(t, root, shipID)
	rowBefore := dbRowJSON(t, root, shipID)

	_ = runCLIStdout(t, root, "shipment", "get", shipID)
	_ = runCLIStdout(t, root, "shipment", "list", "--format", "json")
	_ = runCLIStdout(t, root, "shipment", "list", "--format", "table")

	fileAfter := readArtifactFile(t, root, shipID)
	rowAfter := dbRowJSON(t, root, shipID)

	assert.Equal(t, fileBefore, fileAfter, "stored shipment file must be byte-for-byte unchanged")
	assert.JSONEq(t, rowBefore, rowAfter, "DB row must be unchanged after rendering")
	assert.NotContains(t, fileAfter, "covering_feature", "covering_feature must never be persisted to the manifest")
}
