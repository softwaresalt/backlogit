package cli

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestManifestCommand_Registered verifies the command is wired to root.
func TestManifestCommand_Registered(t *testing.T) {
	root := NewRootCommand()
	cmd, _, err := root.Find([]string{"manifest"})
	require.NoError(t, err)
	require.NotNil(t, cmd, "manifest command must be registered")
	assert.Equal(t, "manifest", cmd.Name())
}

// TestManifestCommand_ProducesValidJSON verifies the manifest emits valid JSON.
func TestManifestCommand_ProducesValidJSON(t *testing.T) {
	root := NewRootCommand()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)

	root.SetArgs([]string{"manifest"})
	err := root.Execute()
	require.NoError(t, err)

	var out map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &out),
		"manifest must emit valid JSON; got: %s", buf.String())
}

// TestManifestCommand_HasToolsKey verifies the manifest includes a "tools" array.
func TestManifestCommand_HasToolsKey(t *testing.T) {
	root := NewRootCommand()
	var buf bytes.Buffer
	root.SetOut(&buf)

	root.SetArgs([]string{"manifest"})
	require.NoError(t, root.Execute())

	var out map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &out))

	tools, ok := out["tools"].([]any)
	require.True(t, ok, "manifest must have a 'tools' array field")
	assert.NotEmpty(t, tools, "tools array must not be empty")
}

// TestManifestCommand_EachToolHasRequiredFields verifies MCP parity.
func TestManifestCommand_EachToolHasRequiredFields(t *testing.T) {
	root := NewRootCommand()
	var buf bytes.Buffer
	root.SetOut(&buf)

	root.SetArgs([]string{"manifest"})
	require.NoError(t, root.Execute())

	var out map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &out))

	tools := out["tools"].([]any)
	for _, raw := range tools {
		tool := raw.(map[string]any)
		name, _ := tool["name"].(string)
		assert.NotEmpty(t, name, "each tool must have a non-empty name")

		_, hasDesc := tool["description"]
		assert.True(t, hasDesc, "tool %q must have a description field", name)

		inputSchema, hasSchema := tool["inputSchema"]
		assert.True(t, hasSchema, "tool %q must have an inputSchema field", name)
		if hasSchema {
			schema, ok := inputSchema.(map[string]any)
			require.True(t, ok, "tool %q inputSchema must be an object", name)
			assert.Equal(t, "object", schema["type"], "tool %q inputSchema.type must be 'object'", name)
		}
	}
}

// TestManifestCommand_KnownTool verifies a specific well-known tool is present.
func TestManifestCommand_KnownTool(t *testing.T) {
	root := NewRootCommand()
	var buf bytes.Buffer
	root.SetOut(&buf)

	root.SetArgs([]string{"manifest"})
	require.NoError(t, root.Execute())

	var out map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &out))

	tools := out["tools"].([]any)
	names := make([]string, 0, len(tools))
	for _, raw := range tools {
		tool := raw.(map[string]any)
		if name, ok := tool["name"].(string); ok {
			names = append(names, name)
		}
	}
	assert.Contains(t, names, "backlogit_get_item", "manifest must include backlogit_get_item")
	assert.Contains(t, names, "backlogit_create_item", "manifest must include backlogit_create_item")
	assert.Contains(t, names, "backlogit_list_items", "manifest must include backlogit_list_items")
}

// TestManifestCommand_JSONRPCIntegration verifies --jsonrpc wraps the manifest.
func TestManifestCommand_JSONRPCIntegration(t *testing.T) {
	root := NewRootCommand()
	var buf bytes.Buffer
	root.SetOut(&buf)

	root.SetArgs([]string{"--jsonrpc", "manifest"})
	require.NoError(t, root.Execute())

	var resp map[string]any
	require.NoError(t, json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &resp))

	assert.Equal(t, "2.0", resp["jsonrpc"])
	assert.Equal(t, "backlogit manifest", resp["id"])
	result, ok := resp["result"].(map[string]any)
	require.True(t, ok, "result should be a JSON object")
	assert.Contains(t, result, "tools")
}
