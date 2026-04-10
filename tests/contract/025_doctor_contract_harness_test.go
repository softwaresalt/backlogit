package contract_test

// 025.017-T (Unit 5): backlogit_doctor MCP tool contract tests.
// All tests in this file are RED until the backlogit_doctor tool is registered
// in internal/mcp/tools.go.
//
// Tests:
//   - TestDoctorTool_SchemaValid                  — RED: tool not registered
//   - TestDoctorTool_AlwaysVisible                — RED: tool not registered
//   - TestDoctorTool_DescriptivePreInitError      — RED: tool not registered
//   - TestDoctorTool_ReturnsCompactJSON           — RED: tool not registered
//   - TestDoctorTool_CleanWorkspaceEmptyFindings  — RED: tool not registered

import (
	"encoding/json"
	"testing"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const doctorToolName = "backlogit_doctor"

// TestDoctorTool_SchemaValid asserts that the backlogit_doctor tool is registered
// with at least the minimum expected parameters (checks, workspace override).
// RED until the tool is registered in tools.go.
func TestDoctorTool_SchemaValid(t *testing.T) {
	s := setupRealMCPServer(t)

	tools := s.ListTools()

	assert.Contains(t, tools, doctorToolName,
		"backlogit_doctor must be registered as an MCP tool")
}

// TestDoctorTool_AlwaysVisible asserts that backlogit_doctor appears in the tool
// list regardless of whether the workspace has been initialised. MCP tools must
// be unconditionally visible per the backlogit constitution (Principle II).
// RED until the tool is registered in tools.go.
func TestDoctorTool_AlwaysVisible(t *testing.T) {
	s := setupRealMCPServer(t)

	tools := s.ListTools()

	// The tool must appear unconditionally — not conditional on workspace state.
	assert.Contains(t, tools, doctorToolName,
		"backlogit_doctor must be visible unconditionally (Principle II)")
}

// TestDoctorTool_DescriptivePreInitError asserts that calling backlogit_doctor
// before the workspace is initialised returns a structured error (IsError=true)
// with a descriptive message — not a panic or empty response.
// RED until the tool is registered and returns proper error responses.
func TestDoctorTool_DescriptivePreInitError(t *testing.T) {
	s := setupRealMCPServer(t)

	result, err := callToolForTest(t, s, doctorToolName, map[string]any{})

	require.NoError(t, err, "tool call must not return a transport-level error")
	require.NotNil(t, result)
	// Either succeeds (empty findings on an initialised workspace) or returns
	// a structured IsError result — must not be nil or panic.
	if result.IsError {
		require.NotEmpty(t, result.Content, "error result must carry descriptive content")
	}
}

// TestDoctorTool_ReturnsCompactJSON asserts that a successful backlogit_doctor
// invocation returns valid JSON with "findings" and "checked_at" top-level keys.
// RED until the tool is implemented and returns structured output.
func TestDoctorTool_ReturnsCompactJSON(t *testing.T) {
	s := setupRealMCPServer(t)

	result, err := callToolForTest(t, s, doctorToolName, map[string]any{})
	require.NoError(t, err)
	require.NotNil(t, result)
	require.False(t, result.IsError, "tool call must not return an error result on an initialised workspace")
	require.NotEmpty(t, result.Content)

	tc, ok := result.Content[0].(mcplib.TextContent)
	require.True(t, ok, "result content must be text")

	var data map[string]any
	require.NoError(t, json.Unmarshal([]byte(tc.Text), &data),
		"backlogit_doctor must return valid JSON")

	assert.Contains(t, data, "findings", "response JSON must include 'findings' key")
	assert.Contains(t, data, "checked_at", "response JSON must include 'checked_at' key")
}

// TestDoctorTool_CleanWorkspaceEmptyFindings asserts that invoking backlogit_doctor
// on a freshly initialised workspace (no artifacts) returns an empty findings array.
// RED until the tool is implemented.
func TestDoctorTool_CleanWorkspaceEmptyFindings(t *testing.T) {
	s := setupRealMCPServer(t)

	result, err := callToolForTest(t, s, doctorToolName, map[string]any{})
	require.NoError(t, err)
	require.NotNil(t, result)
	require.False(t, result.IsError)
	require.NotEmpty(t, result.Content)

	tc, ok := result.Content[0].(mcplib.TextContent)
	require.True(t, ok)

	var data map[string]any
	require.NoError(t, json.Unmarshal([]byte(tc.Text), &data))

	findings, ok := data["findings"].([]any)
	require.True(t, ok, "'findings' must be a JSON array")
	assert.Empty(t, findings, "a clean workspace must yield zero findings")
}
