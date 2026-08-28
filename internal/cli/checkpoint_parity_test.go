package cli_test

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/events"
	"github.com/softwaresalt/backlogit/internal/mcp"
)

// U8b cross-surface parity harness (147-F / 147.016-T, RED, red-deliverable).
//
// This file exercises the CLI command handler, the MCP tool handler, and the
// events read layer against the SAME stored checkpoint file per fixture row,
// asserting the three surfaces agree on classification
// (docs/compound/2026-07-03-cli-mcp-honest-fallback-map-and-registry-drift-test.md
// Rule 3). It compiles against partition-1/3 declarations that already exist
// (BoundedFieldPathSet, RemediationIntent, CheckpointReadResult /
// GetCheckpointResult) and fails on assertions that the eighteen partition-4
// behavioural units (U3, U3b, U4, U5, U6, U6b, U6c, U7, U7b, U7c, U7d, U7e,
// U8, U8c, U9, U16, U17) have not yet landed. Per Ship's Step 0.5
// red-deliverable protocol this harness is not driven to green in this wave:
// it stays red until its declared green-makers land through wave 13.

func writeParityCheckpoint(t *testing.T, root, filename, body string) string {
	t.Helper()
	dir := filepath.Join(root, ".backlogit", "checkpoints")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	path := filepath.Join(dir, filename)
	require.NoError(t, os.WriteFile(path, []byte(body), 0o644))
	return path
}

func sha256Of(t *testing.T, path string) [32]byte {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	return sha256.Sum256(data)
}

func mcpGetCheckpointPayload(t *testing.T, root, filename string) (map[string]any, bool) {
	t.Helper()
	s := mcp.NewServerForRoot(root)
	request := mcplib.CallToolRequest{}
	request.Params.Arguments = map[string]any{"filename": filename}
	result, err := s.InvokeTool(context.Background(), "backlogit_get_checkpoint", request)
	require.NoError(t, err)
	require.NotNil(t, result)
	if result.IsError {
		return nil, true
	}
	tc, ok := result.Content[0].(mcplib.TextContent)
	require.True(t, ok)
	var payload map[string]any
	require.NoError(t, json.Unmarshal([]byte(tc.Text), &payload))
	return payload, false
}

func mcpResolveCheckpoint(t *testing.T, root, filename string) (map[string]any, bool) {
	t.Helper()
	s := mcp.NewServerForRoot(root)
	request := mcplib.CallToolRequest{}
	request.Params.Arguments = map[string]any{"filename": filename}
	result, err := s.InvokeTool(context.Background(), "backlogit_resolve_checkpoint", request)
	require.NoError(t, err)
	require.NotNil(t, result)
	tc, ok := result.Content[0].(mcplib.TextContent)
	require.True(t, ok)
	var payload map[string]any
	_ = json.Unmarshal([]byte(tc.Text), &payload)
	return payload, result.IsError
}

// TestU8b_LegacyShapedRow pins the legacy-shaped fixture (schema-invalid):
// resolve must be refused with the quarantine remedy across every surface,
// naming the same classification. Before U3/U7d land, ResolveCheckpoint
// mutates the file with a fabricated skeleton and succeeds, so this fails.
func TestU8b_LegacyShapedRow(t *testing.T) {
	root := setupCLIWorkspace(t)
	filename := "checkpoint-legacy-shaped.json"
	path := writeParityCheckpoint(t, root, filename, `{"status":"active"}`)
	before := sha256Of(t, path)

	// events layer: resolve must refuse.
	err := events.ResolveCheckpoint(context.Background(), filepath.Join(root, ".backlogit", "checkpoints"), filename)
	assert.Error(t, err, "resolve must refuse a legacy-shaped (schema-invalid) document")

	// CLI layer: resolve must refuse (non-zero / error).
	cliErr := runCLIErr(t, root, "checkpoint", "resolve", filename)
	assert.Error(t, cliErr, "CLI checkpoint resolve must refuse a legacy-shaped document")

	// MCP layer: resolve must refuse (IsError true).
	_, mcpIsErr := mcpResolveCheckpoint(t, root, filename)
	assert.True(t, mcpIsErr, "MCP resolve_checkpoint must refuse a legacy-shaped document")

	after := sha256Of(t, path)
	assert.Equal(t, before, after, "a refused resolve must not change the file's bytes")
}

// TestU8b_ValidButNonConformingRow pins the valid-but-non-conforming fixture
// (schema-valid, one unmodeled top-level key): every surface must classify
// it as non-conforming and refuse mutation. Before U6/U6b/U6c/U8c land, the
// read projections are all zero-valued; before U3b/U4 land, both mutation
// verbs still succeed.
func TestU8b_ValidButNonConformingRow(t *testing.T) {
	root := setupCLIWorkspace(t)
	filename := "checkpoint-valid-non-conforming.json"
	body := `{"schema_version":1,"agent":"ship","session_id":"s1","phase":"build","status":"active",` +
		`"created_at":"2026-08-24T00:00:00Z","updated_at":"2026-08-24T00:00:00Z","extra_key":"x"}`
	path := writeParityCheckpoint(t, root, filename, body)
	before := sha256Of(t, path)

	// events layer: GetCheckpointResult must project non-conforming.
	checkpointDir := filepath.Join(root, ".backlogit", "checkpoints")
	result, err := events.GetCheckpointResult(context.Background(), checkpointDir, filename)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.NeedsQuarantine, "GetCheckpointResult must set NeedsQuarantine for a non-conforming document")
	assert.NotNil(t, result.RemediationIntent, "GetCheckpointResult must populate RemediationIntent for a non-conforming document")
	assert.NotEmpty(t, result.NonConformingFields.Paths, "GetCheckpointResult must project the offending field paths")

	// MCP layer: get payload must carry conforming:false.
	mcpPayload, mcpIsErr := mcpGetCheckpointPayload(t, root, filename)
	require.False(t, mcpIsErr)
	assert.Equal(t, false, mcpPayload["conforming"], "MCP get_checkpoint must project conforming:false")

	// CLI layer: get stdout must carry conforming:false.
	cliOut := runCLIStdout(t, root, "checkpoint", "get", filename)
	var cliPayload map[string]any
	require.NoError(t, json.Unmarshal([]byte(cliOut), &cliPayload))
	assert.Equal(t, false, cliPayload["conforming"], "CLI checkpoint get must project conforming:false")

	// resolve must refuse.
	resolveErr := events.ResolveCheckpoint(context.Background(), checkpointDir, filename)
	assert.Error(t, resolveErr, "resolve must refuse a valid-but-non-conforming document")

	// abandon must refuse (via CLI fallback surface, which requires --reason/--operator).
	abandonErr := runCLIErr(t, root, "checkpoint", "abandon", filename, "--reason", "x", "--operator", "tester@example.com")
	assert.Error(t, abandonErr, "abandon must refuse a valid-but-non-conforming document")

	after := sha256Of(t, path)
	assert.Equal(t, before, after, "a refused mutation must not change the file's bytes")
}

// TestU8b_ConformingActiveRow pins the conforming-active fixture: every
// surface must classify it as conforming, and abandon must succeed. Before
// U6b/U6c/U8c land, the read projections default to Conforming:false.
func TestU8b_ConformingActiveRow(t *testing.T) {
	root := setupCLIWorkspace(t)
	checkpointDir := filepath.Join(root, ".backlogit", "checkpoints")

	// events layer: GetCheckpointResult must project conforming:true.
	filename1 := "checkpoint-conforming-active-read.json"
	writeParityCheckpoint(t, root, filename1,
		`{"schema_version":1,"agent":"ship","session_id":"s1","phase":"build","status":"active",`+
			`"created_at":"2026-08-24T00:00:00Z","updated_at":"2026-08-24T00:00:00Z"}`)
	result, err := events.GetCheckpointResult(context.Background(), checkpointDir, filename1)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.Conforming, "GetCheckpointResult must project Conforming:true for a conforming document")

	// MCP layer.
	mcpPayload, mcpIsErr := mcpGetCheckpointPayload(t, root, filename1)
	require.False(t, mcpIsErr)
	assert.Equal(t, true, mcpPayload["conforming"], "MCP get_checkpoint must project conforming:true")

	// CLI layer.
	cliOut := runCLIStdout(t, root, "checkpoint", "get", filename1)
	var cliPayload map[string]any
	require.NoError(t, json.Unmarshal([]byte(cliOut), &cliPayload))
	assert.Equal(t, true, cliPayload["conforming"], "CLI checkpoint get must project conforming:true")

	// abandon accepts on a fresh copy of the same shape (already-shipped
	// behaviour; asserts the intended outcome rather than byte identity).
	filename2 := "checkpoint-conforming-active-abandon.json"
	writeParityCheckpoint(t, root, filename2,
		`{"schema_version":1,"agent":"ship","session_id":"s1","phase":"build","status":"active",`+
			`"created_at":"2026-08-24T00:00:00Z","updated_at":"2026-08-24T00:00:00Z"}`)
	abandonOut := runCLIStdout(t, root, "checkpoint", "abandon", filename2, "--reason", "superseded", "--operator", "tester@example.com")
	var abandonPayload map[string]any
	require.NoError(t, json.Unmarshal([]byte(abandonOut), &abandonPayload))
	assert.Equal(t, "abandoned", abandonPayload["disposition"])
}
