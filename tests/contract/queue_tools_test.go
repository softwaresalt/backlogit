package contract_test

// Contract tests for the 8 new queue-feature MCP tools introduced in the
// 009-queue-features-v2 branch. Each tool is tested for:
//   - missing required parameter validation
//   - success path with real data

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	mcpinternal "github.com/backlogit/backlogit/internal/mcp"
)

// setupServerWithArtifact creates a real workspace and a single feature artifact,
// returning the server and the artifact ID.
func setupServerWithArtifact(t *testing.T) (*mcpinternal.Server, string) {
	t.Helper()
	s := setupRealMCPServer(t)
	data := callToolAndParseJSON(t, s, "backlogit_create_item", map[string]any{
		"title":         "Queue contract test artifact",
		"artifact_type": "feature",
		"status":        "queued",
	})
	id, ok := data["id"].(string)
	require.True(t, ok, "created artifact should have a string id")
	return s, id
}

// callToolAndParseJSONSlice invokes a tool and unmarshals the result into a
// JSON array. Use for tools that return slices (e.g., get_dependencies).
func callToolAndParseJSONSlice(t *testing.T, s *mcpinternal.Server, toolName string, args map[string]any) []map[string]any {
	t.Helper()
	result, err := callToolForTest(t, s, toolName, args)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.False(t, result.IsError, "tool call should not return error, got: %v", result.Content)
	require.NotEmpty(t, result.Content)

	tc, ok := result.Content[0].(mcplib.TextContent)
	require.True(t, ok, "expected TextContent in result")

	var data []map[string]any
	require.NoError(t, json.Unmarshal([]byte(tc.Text), &data))
	return data
}

// ---------------------------------------------------------------------------
// backlogit_get_wit_metadata
// ---------------------------------------------------------------------------

func TestGetWITMetadata_MissingTypeParam(t *testing.T) {
	s := setupRealMCPServer(t)
	result, err := callToolForTest(t, s, "backlogit_get_wit_metadata", map[string]any{})
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError, "missing type parameter should return error")
}

func TestGetWITMetadata_ReturnsTypeInfo(t *testing.T) {
	s := setupRealMCPServer(t)
	data := callToolAndParseJSON(t, s, "backlogit_get_wit_metadata", map[string]any{
		"type": "task",
	})
	assert.NotEmpty(t, data, "should return type metadata")
}

// ---------------------------------------------------------------------------
// backlogit_list_types
// ---------------------------------------------------------------------------

func TestListTypes_ReturnsTypeList(t *testing.T) {
	s := setupRealMCPServer(t)
	result, err := callToolForTest(t, s, "backlogit_list_types", map[string]any{})
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError, "list types should succeed")
	assert.NotEmpty(t, result.Content)
}

func TestGetMetadataCatalog_ReturnsUnifiedCatalog(t *testing.T) {
	s := setupRealMCPServer(t)
	data := callToolAndParseJSON(t, s, "backlogit_get_metadata_catalog", map[string]any{})
	assert.Contains(t, data, "artifact_types")
	assert.Contains(t, data, "mcp_tools")
	assert.Contains(t, data, "cli")
}

func TestExportCommandMap_WritesWorkspaceFile(t *testing.T) {
	s := setupRealMCPServer(t)
	data := callToolAndParseJSON(t, s, "backlogit_export_command_map", map[string]any{
		"path": filepath.Join(".github", "instructions", "backlogit-command-map.md"),
	})
	assert.Equal(t, "written", data["status"])
	assert.Equal(t, "markdown", data["format"])
}

// ---------------------------------------------------------------------------
// backlogit_add_dependency
// ---------------------------------------------------------------------------

func TestAddDependency_MissingItemID(t *testing.T) {
	s := setupRealMCPServer(t)
	result, err := callToolForTest(t, s, "backlogit_add_dependency", map[string]any{
		"depends_on": "001.002-T",
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError, "missing item_id should return error")
}

func TestAddDependency_MissingDependsOn(t *testing.T) {
	s := setupRealMCPServer(t)
	result, err := callToolForTest(t, s, "backlogit_add_dependency", map[string]any{
		"item_id": "001-T",
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError, "missing depends_on should return error")
}

func TestAddDependency_Success(t *testing.T) {
	s, id := setupServerWithArtifact(t)
	data2 := callToolAndParseJSON(t, s, "backlogit_create_item", map[string]any{
		"title":         "Dependency target",
		"artifact_type": "feature",
	})
	id2 := data2["id"].(string)

	data := callToolAndParseJSON(t, s, "backlogit_add_dependency", map[string]any{
		"item_id":    id,
		"depends_on": id2,
	})
	assert.Equal(t, id, data["item_id"])
	assert.Equal(t, id2, data["depends_on"])
	assert.Equal(t, "added", data["status"])
}

// ---------------------------------------------------------------------------
// backlogit_remove_dependency
// ---------------------------------------------------------------------------

func TestRemoveDependency_MissingParams(t *testing.T) {
	s := setupRealMCPServer(t)
	result, err := callToolForTest(t, s, "backlogit_remove_dependency", map[string]any{
		"item_id": "001-T",
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError, "missing depends_on should return error")
}

// ---------------------------------------------------------------------------
// backlogit_get_dependencies
// ---------------------------------------------------------------------------

func TestGetDependencies_MissingID(t *testing.T) {
	s := setupRealMCPServer(t)
	result, err := callToolForTest(t, s, "backlogit_get_dependencies", map[string]any{})
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError, "missing id should return error")
}

func TestGetDependencies_ReturnsEdgeSlice(t *testing.T) {
	s, id := setupServerWithArtifact(t)

	// Add a dependency so the edge list is non-empty.
	data2 := callToolAndParseJSON(t, s, "backlogit_create_item", map[string]any{
		"title":         "Dep target for edge test",
		"artifact_type": "feature",
	})
	id2 := data2["id"].(string)
	callToolAndParseJSON(t, s, "backlogit_add_dependency", map[string]any{
		"item_id":    id,
		"depends_on": id2,
	})

	edges := callToolAndParseJSONSlice(t, s, "backlogit_get_dependencies", map[string]any{
		"id": id,
	})
	require.Len(t, edges, 1, "should return one dependency edge")
	assert.Equal(t, id, edges[0]["item_id"])
	assert.Equal(t, id2, edges[0]["depends_on"])
}

// ---------------------------------------------------------------------------
// backlogit_archive_item
// ---------------------------------------------------------------------------

func TestArchiveItem_MissingID(t *testing.T) {
	s := setupRealMCPServer(t)
	result, err := callToolForTest(t, s, "backlogit_archive_item", map[string]any{})
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError, "missing id should return error")
}

func TestArchiveItem_Success(t *testing.T) {
	s, id := setupServerWithArtifact(t)
	data := callToolAndParseJSON(t, s, "backlogit_archive_item", map[string]any{
		"id": id,
	})
	assert.Equal(t, id, data["id"])
	assert.NotEmpty(t, data["archive_path"], "archive_path should be set")
}

// ---------------------------------------------------------------------------
// backlogit_get_queue
// ---------------------------------------------------------------------------

func TestGetQueue_ReturnsQueueView(t *testing.T) {
	s, _ := setupServerWithArtifact(t)
	data := callToolAndParseJSON(t, s, "backlogit_get_queue", map[string]any{})
	_, hasTotalCount := data["total_count"]
	_, hasItems := data["items"]
	assert.True(t, hasTotalCount || hasItems, "queue view should have total_count or items")
}

// ---------------------------------------------------------------------------
// backlogit_track_commit
// ---------------------------------------------------------------------------

func TestTrackCommit_MissingItemID(t *testing.T) {
	s := setupRealMCPServer(t)
	result, err := callToolForTest(t, s, "backlogit_track_commit", map[string]any{
		"sha": "abc123",
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError, "missing item_id should return error")
}

func TestTrackCommit_MissingSHA(t *testing.T) {
	s := setupRealMCPServer(t)
	result, err := callToolForTest(t, s, "backlogit_track_commit", map[string]any{
		"item_id": "001-T",
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError, "missing sha should return error")
}

func TestTrackCommit_Success(t *testing.T) {
	s, id := setupServerWithArtifact(t)
	data := callToolAndParseJSON(t, s, "backlogit_track_commit", map[string]any{
		"item_id": id,
		"sha":     "deadbeef1234567890",
		"message": "feat: contract test commit",
		"author":  "tester@example.com",
	})
	assert.Equal(t, id, data["item_id"])
	assert.Equal(t, "deadbeef1234567890", data["sha"])
	assert.Equal(t, "linked", data["status"])
}

// ---------------------------------------------------------------------------
// commit_sha parameter on mutation tools (006-S event traceability)
// ---------------------------------------------------------------------------

func TestArchiveItem_WithCommitSHA_PopulatesEvent(t *testing.T) {
	s, id := setupServerWithArtifact(t)
	data := callToolAndParseJSON(t, s, "backlogit_archive_item", map[string]any{
		"id":         id,
		"commit_sha": "cafe0123456789ab",
	})
	assert.Equal(t, id, data["id"])
	assert.NotEmpty(t, data["archive_path"])

	// Verify the archived event in the JSONL log includes commit_sha
	logsDir := filepath.Join(s.RootPath, ".backlogit", "logs")
	logPath := filepath.Join(logsDir, id+".jsonl")
	assert.FileExists(t, logPath)

	raw, err := os.ReadFile(logPath)
	require.NoError(t, err)
	assert.Contains(t, string(raw), "cafe0123456789ab", "commit_sha should appear in JSONL log")
}

func TestArchiveItem_WithoutCommitSHA_OmitsFromEvent(t *testing.T) {
	s, id := setupServerWithArtifact(t)
	_ = callToolAndParseJSON(t, s, "backlogit_archive_item", map[string]any{
		"id": id,
	})

	// Verify the archived event in the JSONL log omits commit_sha
	logsDir := filepath.Join(s.RootPath, ".backlogit", "logs")
	logPath := filepath.Join(logsDir, id+".jsonl")
	raw, err := os.ReadFile(logPath)
	require.NoError(t, err)
	assert.NotContains(t, string(raw), "commit_sha", "commit_sha should be omitted when not provided")
}

func TestAppendComment_WithCommitSHA_PopulatesEvent(t *testing.T) {
	s, id := setupServerWithArtifact(t)
	result, err := callToolForTest(t, s, "backlogit_append_comment", map[string]any{
		"item_id":    id,
		"actor":      "test-agent",
		"comment":    "commit-linked comment",
		"commit_sha": "deadbeef00001111",
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	require.False(t, result.IsError)

	// Verify the comment event in the JSONL log includes commit_sha
	logsDir := filepath.Join(s.RootPath, ".backlogit", "logs")
	logPath := filepath.Join(logsDir, id+".jsonl")
	raw, err := os.ReadFile(logPath)
	require.NoError(t, err)
	assert.Contains(t, string(raw), "deadbeef00001111", "commit_sha should appear in comment event")
}

func TestAppendComment_WithoutCommitSHA_WorksIdentically(t *testing.T) {
	s, id := setupServerWithArtifact(t)
	result, err := callToolForTest(t, s, "backlogit_append_comment", map[string]any{
		"item_id": id,
		"actor":   "test-agent",
		"comment": "plain comment",
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	require.False(t, result.IsError)

	logsDir := filepath.Join(s.RootPath, ".backlogit", "logs")
	logPath := filepath.Join(logsDir, id+".jsonl")
	raw, err := os.ReadFile(logPath)
	require.NoError(t, err)
	assert.NotContains(t, string(raw), "commit_sha", "commit_sha should be omitted when not provided")
}

func TestMoveItem_WithCommitSHA_EmitsTrackedEvent(t *testing.T) {
	s, id := setupServerWithArtifact(t)
	data := callToolAndParseJSON(t, s, "backlogit_move_item", map[string]any{
		"id":         id,
		"status":     "active",
		"commit_sha": "aabbccdd11223344",
	})
	assert.Equal(t, id, data["id"])
	assert.Equal(t, "active", data["status"])

	// Verify a status_changed event with commit_sha appears in the JSONL log
	logsDir := filepath.Join(s.RootPath, ".backlogit", "logs")
	logPath := filepath.Join(logsDir, id+".jsonl")
	raw, err := os.ReadFile(logPath)
	require.NoError(t, err)
	logContent := string(raw)
	assert.Contains(t, logContent, "aabbccdd11223344", "commit_sha should appear in move event")
	// Verify delta schema matches core pattern: {from, to, reason}
	assert.Contains(t, logContent, `"to"`, "delta should contain 'to' field matching core status_changed schema")
	assert.Contains(t, logContent, `"from"`, "delta should contain 'from' field matching core status_changed schema")
	assert.Contains(t, logContent, `"reason"`, "delta should contain 'reason' field matching core status_changed schema")
}

func TestMoveItem_WithoutCommitSHA_NoExtraEvent(t *testing.T) {
	s, id := setupServerWithArtifact(t)
	data := callToolAndParseJSON(t, s, "backlogit_move_item", map[string]any{
		"id":     id,
		"status": "active",
	})
	assert.Equal(t, id, data["id"])
	assert.Equal(t, "active", data["status"])

	// Verify no commit_sha-bearing event was emitted
	logsDir := filepath.Join(s.RootPath, ".backlogit", "logs")
	logPath := filepath.Join(logsDir, id+".jsonl")

	// Log file may not exist at all if no events were emitted (which is fine)
	if raw, err := os.ReadFile(logPath); err == nil {
		assert.NotContains(t, string(raw), "commit_sha", "commit_sha should not appear without parameter")
	}
}
