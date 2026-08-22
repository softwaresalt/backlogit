package cli_test

import (
	"context"
	"encoding/json"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/cli"
	"github.com/softwaresalt/backlogit/internal/core"
	mcpinternal "github.com/softwaresalt/backlogit/internal/mcp"
)

// U6 (078.006-T): `backlogit checkpoint create --state-dump {{state_dump}}` must
// mirror the MCP backlogit_create_checkpoint tool — it validates a V1 checkpoint
// state dump, writes it to the workspace checkpoints directory via the shared
// events.CreateCheckpoint pipeline, and returns the written path as JSON. The
// resulting file must be readable by the existing `checkpoint get` and
// `checkpoint list` surfaces so recovery tooling stays same-shape across CLI/MCP.

// TestCheckpointCreate_WritesReadableCheckpoint verifies the happy path: a valid
// V1 state dump produces a checkpoint file that list and get can consume.
func TestCheckpointCreate_WritesReadableCheckpoint(t *testing.T) {
	root := setupCLIWorkspace(t)

	stateDump := `{"schema_version":1,"agent":"ship","session_id":"sess-078","phase":"build"}`

	createOut := runCLIStdout(t, root, "checkpoint", "create", "--state-dump", stateDump)

	var created struct {
		Path string `json:"path"`
	}
	require.NoError(t, json.Unmarshal([]byte(createOut), &created))
	require.NotEmpty(t, created.Path, "create must return the written checkpoint path")

	filename := filepath.Base(created.Path)
	require.True(t, strings.HasPrefix(filename, "checkpoint-"), "unexpected checkpoint filename: %s", filename)
	require.True(t, strings.HasSuffix(filename, ".json"), "unexpected checkpoint filename: %s", filename)

	// The new checkpoint must appear in the list surface.
	listOut := runCLIStdout(t, root, "checkpoint", "list")
	var listed struct {
		Total       int `json:"total"`
		Checkpoints []struct {
			SessionID string `json:"session_id"`
			Agent     string `json:"agent"`
			Phase     string `json:"phase"`
			Status    string `json:"status"`
		} `json:"checkpoints"`
	}
	require.NoError(t, json.Unmarshal([]byte(listOut), &listed))
	require.Equal(t, 1, listed.Total, "created checkpoint must be listable")
	assert.Equal(t, "sess-078", listed.Checkpoints[0].SessionID)
	assert.Equal(t, "ship", listed.Checkpoints[0].Agent)
	assert.Equal(t, "active", listed.Checkpoints[0].Status, "status must default to active when omitted")

	// The new checkpoint must be retrievable and valid via get.
	getOut := runCLIStdout(t, root, "checkpoint", "get", filename)
	var got struct {
		Valid      bool `json:"valid"`
		Checkpoint struct {
			SessionID string `json:"session_id"`
		} `json:"checkpoint"`
	}
	require.NoError(t, json.Unmarshal([]byte(getOut), &got))
	assert.True(t, got.Valid, "created checkpoint must validate")
	assert.Equal(t, "sess-078", got.Checkpoint.SessionID)
}

// TestCheckpointCreate_MissingStateDump verifies the required-input contract.
func TestCheckpointCreate_MissingStateDump(t *testing.T) {
	root := setupCLIWorkspace(t)
	err := runCLIErr(t, root, "checkpoint", "create")
	require.Error(t, err, "create must fail when --state-dump is not provided")
}

// TestCheckpointCreate_InvalidSchema verifies that an invalid V1 checkpoint is
// rejected (mirroring the events.CreateCheckpoint validation gate).
func TestCheckpointCreate_InvalidSchema(t *testing.T) {
	root := setupCLIWorkspace(t)
	// schema_version=1 selects V1 validation; agent "bogus" violates oneof.
	stateDump := `{"schema_version":1,"agent":"bogus","session_id":"s","phase":"p"}`
	err := runCLIErr(t, root, "checkpoint", "create", "--state-dump", stateDump)
	require.Error(t, err, "create must reject an invalid V1 checkpoint")
}

func TestCheckpointCreate_MissingWorkspaceStorageRootFailsClosed(t *testing.T) {
	root := t.TempDir()

	stateDump := `{"schema_version":1,"agent":"ship","session_id":"sess-missing","phase":"build"}`
	err := runCLIErr(t, root, "checkpoint", "create", "--state-dump", stateDump)

	require.Error(t, err, "create must fail when no workspace storage root exists")
	assert.Contains(t, err.Error(), "resolve checkpoint dir")
}

// TestCheckpointCreate_NoHTMLEscape is a regression test for the checkpoint
// JSON readability fix (137-F): the CLI's JSON response encoder must not
// HTML-escape <, >, and & characters.
func TestCheckpointCreate_NoHTMLEscape(t *testing.T) {
	root := setupCLIWorkspace(t)
	stateDump := `{"schema_version":1,"agent":"ship","session_id":"sess-esc","phase":"build","resume_hint":"a > b && b < c"}`

	createOut := runCLIStdout(t, root, "checkpoint", "create", "--state-dump", stateDump)
	assert.NotContains(t, createOut, `\u003e`)
	assert.NotContains(t, createOut, `\u003c`)
	assert.NotContains(t, createOut, `\u0026`)

	var created struct {
		Path string `json:"path"`
	}
	require.NoError(t, json.Unmarshal([]byte(createOut), &created))
	filename := filepath.Base(created.Path)

	getOut := runCLIStdout(t, root, "checkpoint", "get", filename)
	assert.Contains(t, getOut, "a > b && b < c")
	assert.NotContains(t, getOut, `\u003e`)
	assert.NotContains(t, getOut, `\u003c`)
	assert.NotContains(t, getOut, `\u0026`)
}

// newMCPServerForRoot constructs an MCP server bound to the given workspace
// root, for cross-surface (CLI vs MCP) comparisons. Shared by the U5a/U5b
// context_keys scenarios below.
func newMCPServerForRoot(t *testing.T, root string) *mcpinternal.Server {
	t.Helper()
	ctx := context.Background()
	ws, err := core.NewWorkspace(ctx, root)
	require.NoError(t, err)
	t.Cleanup(func() { _ = ws.Close() })
	return mcpinternal.NewServer(ws)
}

// invokeCreateCheckpointMCP dispatches backlogit_create_checkpoint through the
// registered tool handle (InvokeTool), never a handler method directly, and
// returns the decoded JSON response body.
func invokeCreateCheckpointMCP(t *testing.T, srv *mcpinternal.Server, stateDump string) map[string]any {
	t.Helper()
	request := mcplib.CallToolRequest{}
	request.Params.Arguments = map[string]any{"state_dump": stateDump}

	result, err := srv.InvokeTool(context.Background(), "backlogit_create_checkpoint", request)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.False(t, result.IsError, "MCP create must succeed for a valid state dump")

	tc, ok := result.Content[0].(mcplib.TextContent)
	require.True(t, ok)
	var resp map[string]any
	require.NoError(t, json.Unmarshal([]byte(tc.Text), &resp))
	return resp
}

// contextKeysFrom extracts and sorts the string context_keys array from a
// decoded create response, for both the CLI and MCP surfaces.
func contextKeysFrom(t *testing.T, resp map[string]any) []string {
	t.Helper()
	raw, ok := resp["context_keys"].([]any)
	require.True(t, ok, "response must carry a context_keys array")
	out := make([]string, 0, len(raw))
	for _, v := range raw {
		keyName, ok := v.(string)
		require.True(t, ok)
		out = append(out, keyName)
	}
	return out
}

// TestCheckpointCreate_ContextKeysInJSONOutput is scenario 2 of 146.013-T
// (U5a): the CLI JSON output includes the same context_keys for the same
// dump, driven through the --state-dump flag rather than
// events.CreateCheckpoint directly.
func TestCheckpointCreate_ContextKeysInJSONOutput(t *testing.T) {
	root := setupCLIWorkspace(t)
	stateDump := `{"schema_version":1,"agent":"ship","session_id":"sess-ctxkeys","phase":"build","context":{"shipment_id":"129-S","pr_number":372}}`

	createOut := runCLIStdout(t, root, "checkpoint", "create", "--state-dump", stateDump)
	var resp map[string]any
	require.NoError(t, json.Unmarshal([]byte(createOut), &resp))

	keys := contextKeysFrom(t, resp)
	assert.Contains(t, keys, "shipment_id")
	assert.Contains(t, keys, "pr_number")
}

// TestCheckpointCreate_ContextKeysByteIdenticalAcrossSurfaces is scenario 3
// of 146.013-T (U5a): the MCP and CLI context_keys results for the same dump
// are byte-identical (as a sorted key list), mirroring the established
// docline.LintReport pattern where both surfaces encode the same type.
func TestCheckpointCreate_ContextKeysByteIdenticalAcrossSurfaces(t *testing.T) {
	root := setupCLIWorkspace(t)
	stateDump := `{"schema_version":1,"agent":"ship","session_id":"sess-parity","phase":"build","context":{"shipment_id":"129-S","feature_id":"146-F","review_gate":"PASS"}}`

	createOut := runCLIStdout(t, root, "checkpoint", "create", "--state-dump", stateDump)
	var cliResp map[string]any
	require.NoError(t, json.Unmarshal([]byte(createOut), &cliResp))
	cliKeys := contextKeysFrom(t, cliResp)

	srv := newMCPServerForRoot(t, root)
	mcpResp := invokeCreateCheckpointMCP(t, srv, stateDump)
	mcpKeys := contextKeysFrom(t, mcpResp)

	// A shared underlying function means both surfaces can trivially agree on
	// an EMPTY list (today's placeholder behavior); require non-empty content
	// first so this scenario cannot pass vacuously the way a bare equality
	// check on two shared-struct surfaces can (the same risk the docline
	// LintTree/CLI-MCP parity test documents for its own shared-struct check).
	require.NotEmpty(t, cliKeys, "CLI context_keys must be non-empty for a dump whose context carries keys")
	require.NotEmpty(t, mcpKeys, "MCP context_keys must be non-empty for a dump whose context carries keys")

	sort.Strings(cliKeys)
	sort.Strings(mcpKeys)
	assert.Equal(t, mcpKeys, cliKeys, "CLI and MCP context_keys must be byte-identical for the same dump")
}

// TestCheckpointCreateCommand_ContractText_OpenContextAndContextKeys is
// scenario 4 of 146.013-T (U5a): the CONSTRUCTED checkpoint create Cobra
// command's Short, Long, and Example strings must carry the open-context,
// closed-namespace, and context_keys sentences that 146.015-T (U6) writes.
// This exists so R11a holds for U6: every agent- and operator-facing
// contract-text edit in this plan has a red harness scheduled upstream of
// it, and none ships unasserted.
func TestCheckpointCreateCommand_ContractText_OpenContextAndContextKeys(t *testing.T) {
	root := cli.NewRootCommand()
	cmd, _, err := root.Find([]string{"checkpoint", "create"})
	require.NoError(t, err)
	require.NotNil(t, cmd)

	combined := strings.ToLower(cmd.Short + "\n" + cmd.Long + "\n" + cmd.Example)

	assert.Contains(t, combined, "context_keys", "the constructed command must mention context_keys")
	assert.True(t,
		strings.Contains(combined, "open") && strings.Contains(combined, "context"),
		"the constructed command must describe the open context namespace",
	)
	assert.True(t,
		strings.Contains(combined, "closed") || strings.Contains(combined, "unknown"),
		"the constructed command must describe the closed schema namespace / unknown-field rejection",
	)
}
