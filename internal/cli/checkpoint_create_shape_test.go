package cli_test

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCheckpointCreate_LegacyDumpContextKeysNamesActualKeys is scenario 1 of
// 146.014-T (U5b): a legacy dump (no schema_version) whose context object
// carries keys yields a context_keys list naming exactly those keys on both
// surfaces, proving the legacy path reports what it actually wrote rather
// than defaulting to [].
func TestCheckpointCreate_LegacyDumpContextKeysNamesActualKeys(t *testing.T) {
	stateDump := `{"phase":"build","context":{"shipment_id":"129-S","pr_number":372}}`

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

// TestCheckpointCreate_DegenerateLegacyShapes_ContextKeysPresentEmpty is
// scenario 2 of 146.014-T (U5b): a table over four degenerate legacy dumps —
// context absent, "context": null, "context": 42, and a top level that is a
// JSON array rather than an object — each asserts the create succeeds and
// context_keys decodes as a present, empty .([]any) on both surfaces, an
// assertion that fails for both an absent key and a JSON null.
//
// Note: 146.001-T (U0a) already declares CreateCheckpointResult.ContextKeys
// with no omitempty and always populates it with make([]string, 0), so this
// scenario's "present, empty" half is already satisfied ahead of 146.015-T
// (U6). It is retained as the specified forward-compatible shape guard for
// U6 (which must not regress it to nil/omitted for any degenerate legacy
// shape) rather than weakened, per the acceptance criteria in the plan.
func TestCheckpointCreate_DegenerateLegacyShapes_ContextKeysPresentEmpty(t *testing.T) {
	tests := []struct {
		name string
		dump string
	}{
		{"context_absent", `{"phase":"build"}`},
		{"context_null", `{"phase":"build","context":null}`},
		{"context_number", `{"phase":"build","context":42}`},
		{"top_level_array", `[1,2,3]`},
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
