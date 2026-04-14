package contract_test

// Contract tests for the backlogit_telemetry_harvest MCP tool (021-F / 021.008-T).
// These tests validate tool registration, schema, and error-path behavior.

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mark3labs/mcp-go/client"
	mcplib "github.com/mark3labs/mcp-go/mcp"

	"github.com/softwaresalt/backlogit/internal/config"
	"github.com/softwaresalt/backlogit/internal/core"
	mcpinternal "github.com/softwaresalt/backlogit/internal/mcp"
)

// setupTelemetryMCPServer creates a workspace with a .copilot/logs directory
// seeded with sample process log content.
func setupTelemetryMCPServer(t *testing.T) (s *mcpinternal.Server, copilotDir string) {
	t.Helper()
	root := t.TempDir()
	backlogitDir := filepath.Join(root, ".backlogit")
	copilotDir = filepath.Join(root, ".copilot")
	logsDir := filepath.Join(copilotDir, "logs")
	require.NoError(t, os.MkdirAll(backlogitDir, 0o755))
	require.NoError(t, os.MkdirAll(logsDir, 0o755))
	require.NoError(t, config.WriteDefaults(backlogitDir))

	logContent := `2026-04-09T00:00:01.000Z [info] startup
2026-04-09T00:00:02.000Z [telemetry] {"event":"cli.model_call","session_id":"sess-contract-1","request_id":"req-c1","model":"claude-sonnet-4","prompt_tokens_count":1000,"completion_tokens_count":500,"total_tokens_count":1500,"cached_tokens_count":200,"duration_ms":1200}
2026-04-09T00:00:03.000Z [telemetry] {"event":"cli.tool_call","session_id":"sess-contract-1","model_call_id":"req-c1","tool_name":"backlogit_create_item","result_type":"text","duration_ms":45}
`
	require.NoError(t, os.WriteFile(filepath.Join(logsDir, "process-001.log"), []byte(logContent), 0o644))

	ctx := context.Background()
	ws, err := core.NewWorkspace(ctx, root)
	require.NoError(t, err)
	t.Cleanup(func() { ws.Close() })

	return mcpinternal.NewServer(ws), copilotDir
}

// TestTelemetryHarvestTool_IsRegistered asserts the tool is visible in the
// server's tool list so agents can discover and invoke it.
func TestTelemetryHarvestTool_IsRegistered(t *testing.T) {
	s := setupRealMCPServer(t)
	ctx := context.Background()

	c, err := client.NewInProcessClient(s.MCPServer())
	require.NoError(t, err)
	require.NoError(t, c.Start(ctx))
	defer c.Close() //nolint:errcheck

	initReq := mcplib.InitializeRequest{}
	initReq.Params.ClientInfo = mcplib.Implementation{Name: "test", Version: "0.0.1"}
	initReq.Params.ProtocolVersion = mcplib.LATEST_PROTOCOL_VERSION
	_, err = c.Initialize(ctx, initReq)
	require.NoError(t, err)

	listReq := mcplib.ListToolsRequest{}
	resp, err := c.ListTools(ctx, listReq)
	require.NoError(t, err)

	var found bool
	for _, tool := range resp.Tools {
		if tool.Name == "backlogit_telemetry_harvest" {
			found = true
			assert.NotEmpty(t, tool.Description, "tool must have a description")
			break
		}
	}
	assert.True(t, found, "backlogit_telemetry_harvest must be registered in the MCP server")
}

// TestTelemetryHarvestTool_MissingCopilotPath_ReturnsError confirms the tool
// returns an error result (not a crash) when the .copilot directory is absent.
func TestTelemetryHarvestTool_MissingCopilotPath_ReturnsError(t *testing.T) {
	s := setupRealMCPServer(t)
	result, err := callToolForTest(t, s, "backlogit_telemetry_harvest", map[string]any{
		"copilot_path": "/nonexistent/.copilot",
	})
	require.NoError(t, err, "MCP transport must not fail")
	require.NotNil(t, result)
	assert.True(t, result.IsError, "tool must set IsError=true when .copilot path is missing")
}

// TestTelemetryHarvestTool_WithSampleLogs_ReturnsSessionCount validates the
// happy path: logs exist, harvest completes, response contains sessions_harvested.
func TestTelemetryHarvestTool_WithSampleLogs_ReturnsSessionCount(t *testing.T) {
	s, copilotDir := setupTelemetryMCPServer(t)
	data := callToolAndParseJSON(t, s, "backlogit_telemetry_harvest", map[string]any{
		"copilot_path": copilotDir,
	})
	_, hasCount := data["sessions_harvested"]
	assert.True(t, hasCount, "response should include 'sessions_harvested' field")
}

// TestTelemetryHarvestTool_EmptyLogsDir_SucceedsWithZeroSessions confirms that
// when the logs directory exists but contains no log files, the tool returns
// success with sessions_harvested = 0.
func TestTelemetryHarvestTool_EmptyLogsDir_SucceedsWithZeroSessions(t *testing.T) {
	root := t.TempDir()
	backlogitDir := filepath.Join(root, ".backlogit")
	copilotDir := filepath.Join(root, ".copilot")
	logsDir := filepath.Join(copilotDir, "logs")
	require.NoError(t, os.MkdirAll(backlogitDir, 0o755))
	require.NoError(t, os.MkdirAll(logsDir, 0o755))
	require.NoError(t, config.WriteDefaults(backlogitDir))

	ctx := context.Background()
	ws, err := core.NewWorkspace(ctx, root)
	require.NoError(t, err)
	t.Cleanup(func() { ws.Close() })
	s := mcpinternal.NewServer(ws)

	data := callToolAndParseJSON(t, s, "backlogit_telemetry_harvest", map[string]any{
		"copilot_path": copilotDir,
	})
	count, _ := data["sessions_harvested"].(float64)
	assert.Equal(t, float64(0), count, "empty logs dir should yield sessions_harvested = 0")
}
