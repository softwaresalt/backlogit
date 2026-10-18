package cli_test

import (
	"context"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCheckpointCreate_ContextKeysNamesActualKeys is scenario 1 of 146.014-T
// (U5b), updated for 171.002-T (create now requires schema_version 1): a V1
// dump whose context object carries keys yields a context_keys list naming
// exactly those keys on both surfaces.
func TestCheckpointCreate_ContextKeysNamesActualKeys(t *testing.T) {
	stateDump := `{"schema_version":1,"agent":"ship","session_id":"s1","phase":"build","context":{"shipment_id":"129-S","pr_number":372}}`

	t.Run("cli", func(t *testing.T) {
		root := setupCLIWorkspace(t)
		createOut := runCLIStdout(t, root, "checkpoint", "create", "--state-dump", stateDump)
		var resp map[string]any
		require.NoError(t, json.Unmarshal([]byte(createOut), &resp))
		keys := contextKeysFrom(t, resp)
		assert.ElementsMatch(t, []string{"shipment_id", "pr_number"}, keys,
			"a legacy dump's context_keys must name exactly the keys actually written, not default to []")
	})

	t.Run("mcp", func(t *testing.T) {
		root := setupCLIWorkspace(t)
		srv := newMCPServerForRoot(t, root)
		resp := invokeCreateCheckpointMCP(t, srv, stateDump)
		keys := contextKeysFrom(t, resp)
		assert.ElementsMatch(t, []string{"shipment_id", "pr_number"}, keys,
			"a legacy dump's context_keys must name exactly the keys actually written, not default to []")
	})
}

// TestCheckpointCreate_DegenerateShapes_ContextKeysPresentEmpty is scenario 2
// of 146.014-T (U5b), updated for 171.002-T: V1 dumps with context absent or
// null still yield a present, empty context_keys array on both surfaces.
func TestCheckpointCreate_DegenerateShapes_ContextKeysPresentEmpty(t *testing.T) {
	tests := []struct {
		name string
		dump string
	}{
		{"context_absent", `{"schema_version":1,"agent":"ship","session_id":"s1","phase":"build"}`},
		{"context_null", `{"schema_version":1,"agent":"ship","session_id":"s1","phase":"build","context":null}`},
	}

	for _, tc := range tests {
		t.Run(tc.name+"_cli", func(t *testing.T) {
			root := setupCLIWorkspace(t)
			createOut := runCLIStdout(t, root, "checkpoint", "create", "--state-dump", tc.dump)
			var resp map[string]any
			require.NoError(t, json.Unmarshal([]byte(createOut), &resp))
			raw, ok := resp["context_keys"].([]any)
			require.True(t, ok, "context_keys must be present and decode as a JSON array, not absent or null")
			assert.Empty(t, raw)
		})
		t.Run(tc.name+"_mcp", func(t *testing.T) {
			root := setupCLIWorkspace(t)
			srv := newMCPServerForRoot(t, root)
			resp := invokeCreateCheckpointMCP(t, srv, tc.dump)
			raw, ok := resp["context_keys"].([]any)
			require.True(t, ok, "context_keys must be present and decode as a JSON array, not absent or null")
			assert.Empty(t, raw)
		})
	}
}

// TestCheckpointCreate_NonObjectAndSchemaLessRejected pins (171.002-T) that
// arrays, scalars, null, and schema-less/legacy-shaped objects are rejected on
// both surfaces by default, writing no checkpoint file.
func TestCheckpointCreate_NonObjectAndSchemaLessRejected(t *testing.T) {
	tests := []struct {
		name string
		dump string
	}{
		{"top_level_array", `[1,2,3]`},
		{"top_level_number", `42`},
		{"top_level_string", `"x"`},
		{"top_level_null", `null`},
		{"legacy_no_schema", `{"phase":"build"}`},
		{"legacy_context_number", `{"schema_version":1,"agent":"ship","session_id":"s1","phase":"build","context":42}`},
	}

	for _, tc := range tests {
		t.Run(tc.name+"_cli", func(t *testing.T) {
			root := setupCLIWorkspace(t)
			err := runCLIErr(t, root, "checkpoint", "create", "--state-dump", tc.dump)
			require.Error(t, err)
			assertNoCheckpointFiles(t, root)
		})
		t.Run(tc.name+"_mcp", func(t *testing.T) {
			root := setupCLIWorkspace(t)
			srv := newMCPServerForRoot(t, root)
			request := mcplib.CallToolRequest{}
			request.Params.Arguments = map[string]any{"state_dump": tc.dump}
			result, err := srv.InvokeTool(context.Background(), "backlogit_create_checkpoint", request)
			require.NoError(t, err)
			require.NotNil(t, result)
			assert.True(t, result.IsError)
			assertNoCheckpointFiles(t, root)
		})
	}
}

// assertNoCheckpointFiles fails if any checkpoint-*.json exists under root.
func assertNoCheckpointFiles(t *testing.T, root string) {
	t.Helper()
	require.NoError(t, filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasPrefix(d.Name(), "checkpoint-") && strings.HasSuffix(d.Name(), ".json") {
			t.Errorf("rejected create wrote a file: %s", p)
		}
		return nil
	}))
}

// TestCheckpointCreate_EmptyTaskIDsContextKeysMatchWrittenBytes is scenario 3
// of 146.014-T (U5b): a V1 dump whose context supplies "task_ids": [] must
// produce on-disk bytes and a context_keys list that agree exactly, so
// omitempty cannot make the result name a key the file does not have (an
// empty slice with ,omitempty is elided from the written JSON).
func TestCheckpointCreate_EmptyTaskIDsContextKeysMatchWrittenBytes(t *testing.T) {
	stateDump := `{"schema_version":1,"agent":"ship","session_id":"s1","phase":"build","context":{"shipment_id":"129-S","task_ids":[]}}`

	t.Run("cli", func(t *testing.T) {
		root := setupCLIWorkspace(t)
		createOut := runCLIStdout(t, root, "checkpoint", "create", "--state-dump", stateDump)
		var resp map[string]any
		require.NoError(t, json.Unmarshal([]byte(createOut), &resp))
		var created struct {
			Path string `json:"path"`
		}
		require.NoError(t, json.Unmarshal([]byte(createOut), &created))

		raw, err := os.ReadFile(created.Path)
		require.NoError(t, err)
		assertContextKeysMatchWrittenBytes(t, raw, contextKeysFrom(t, resp))
	})

	t.Run("mcp", func(t *testing.T) {
		root := setupCLIWorkspace(t)
		srv := newMCPServerForRoot(t, root)
		resp := invokeCreateCheckpointMCP(t, srv, stateDump)
		pathAny, ok := resp["path"].(string)
		require.True(t, ok)

		raw, err := os.ReadFile(pathAny)
		require.NoError(t, err)
		assertContextKeysMatchWrittenBytes(t, raw, contextKeysFrom(t, resp))
	})
}

// assertContextKeysMatchWrittenBytes decodes the written checkpoint's context
// object and asserts its key set is exactly the reported context_keys list —
// no more (a key omitempty-elided from disk must not still be named) and no
// fewer.
func assertContextKeysMatchWrittenBytes(t *testing.T, raw []byte, reportedKeys []string) {
	t.Helper()
	var doc map[string]any
	require.NoError(t, json.Unmarshal(raw, &doc))
	ctx, _ := doc["context"].(map[string]any)
	writtenKeys := make([]string, 0, len(ctx))
	for k := range ctx {
		writtenKeys = append(writtenKeys, k)
	}
	assert.ElementsMatch(t, writtenKeys, reportedKeys,
		"context_keys must name exactly the keys present on disk; an omitempty-elided empty task_ids must not still be reported")
}
