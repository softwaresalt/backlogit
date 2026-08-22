package mcp

import (
	"context"
	"encoding/json"
	"testing"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestHandleCreateCheckpoint_ResultIncludesContextKeys is scenario 1 of
// 146.013-T (U5a): the MCP create result carries the persisted context key
// names. Dispatched through the registered handleCreateCheckpoint with a
// state_dump string argument, never through events.CreateCheckpoint
// directly, because argument extraction and marshalling are part of the
// loss path.
func TestHandleCreateCheckpoint_ResultIncludesContextKeys(t *testing.T) {
	s, _ := setupBugFixServer(t)
	ctx := context.Background()

	request := mcplib.CallToolRequest{}
	request.Params.Name = "backlogit_create_checkpoint"
	request.Params.Arguments = map[string]any{
		"state_dump": `{"schema_version":1,"agent":"ship","session_id":"s1","phase":"build","context":{"shipment_id":"129-S","pr_number":372}}`,
	}

	result, err := s.handleCreateCheckpoint(ctx, request)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.False(t, result.IsError, "create with a modeled + unmodeled context key must succeed")

	tc, ok := result.Content[0].(mcplib.TextContent)
	require.True(t, ok)
	var resp map[string]any
	require.NoError(t, json.Unmarshal([]byte(tc.Text), &resp))

	raw, ok := resp["context_keys"].([]any)
	require.True(t, ok, "result must carry a context_keys array")
	got := make([]string, 0, len(raw))
	for _, v := range raw {
		keyName, ok := v.(string)
		require.True(t, ok)
		got = append(got, keyName)
	}
	assert.Contains(t, got, "shipment_id", "context_keys must list the modeled key that was persisted")
	assert.Contains(t, got, "pr_number", "context_keys must list the unmodeled key that was persisted")
}
