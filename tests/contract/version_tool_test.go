package contract_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGetVersion_ToolRegistered asserts that backlogit_get_version is registered
// on the MCP server and returns a structured response with the required fields.
func TestGetVersion_ToolRegistered(t *testing.T) {
	s := setupRealMCPServer(t)

	data := callToolAndParseJSON(t, s, "backlogit_get_version", map[string]any{})
	require.NotNil(t, data)
	assert.NotEmpty(t, data["version"], "version field must be present and non-empty")
	assert.Contains(t, data, "commit", "commit field must be present")
	assert.Contains(t, data, "build_date", "build_date field must be present")
	assert.NotEmpty(t, data["go_version"], "go_version field must be present and non-empty")
}
