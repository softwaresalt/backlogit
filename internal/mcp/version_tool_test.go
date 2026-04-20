package mcp

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestHandleGetVersion_ReturnsVersionFields asserts that handleGetVersion returns
// a non-error result containing version, commit, build_date, and go_version.
func TestHandleGetVersion_ReturnsVersionFields(t *testing.T) {
	root := t.TempDir()
	s := NewServerForRoot(root)

	result, err := s.handleGetVersion(context.Background(), mcplib.CallToolRequest{})
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError, "backlogit_get_version must not return an error result")
	require.NotEmpty(t, result.Content, "result must have at least one content item")

	tc, ok := result.Content[0].(mcplib.TextContent)
	require.True(t, ok, "result content must be TextContent")

	var data map[string]string
	require.NoError(t, json.Unmarshal([]byte(tc.Text), &data), "result text must be valid JSON")
	assert.NotEmpty(t, data["version"], "version field must be present and non-empty")
	assert.Contains(t, data, "commit", "commit field must be present")
	assert.Contains(t, data, "build_date", "build_date field must be present")
	assert.NotEmpty(t, data["go_version"], "go_version field must be present and non-empty")
}

// TestHandleGetVersion_WorksWithoutWorkspace asserts that handleGetVersion succeeds
// even when the .backlogit directory has not been initialized.
func TestHandleGetVersion_WorksWithoutWorkspace(t *testing.T) {
	emptyDir := t.TempDir()
	_, err := os.Stat(filepath.Join(emptyDir, ".backlogit"))
	require.True(t, os.IsNotExist(err), "pre-condition: .backlogit should not exist")

	s := NewServerForRoot(emptyDir)
	result, err := s.handleGetVersion(context.Background(), mcplib.CallToolRequest{})
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError, "tool must succeed without a workspace")
}
