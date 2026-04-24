package contract_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	mcplib "github.com/mark3labs/mcp-go/mcp"

	"github.com/softwaresalt/backlogit/internal/events"
)

// writeV1Checkpoint creates a valid V1 checkpoint file in the workspace.
func writeV1Checkpoint(t *testing.T, rootPath, filename string, cp *events.CheckpointV1) {
	t.Helper()
	dir := filepath.Join(rootPath, ".backlogit", "checkpoints")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	data, err := json.Marshal(cp)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, filename), data, 0o644))
}

func newTestCheckpointV1() *events.CheckpointV1 {
	now := time.Now().UTC()
	return &events.CheckpointV1{
		SchemaVersion: 1,
		Agent:         "ship",
		SessionID:     "test-session-001",
		Phase:         "build-execution",
		Status:        "active",
		CreatedAt:     now,
		UpdatedAt:     now,
		Context: events.CheckpointContext{
			ShipmentID: "044-S",
			FeatureID:  "045-F",
			TaskIDs:    []string{"045.003-T"},
			Branch:     "feat/045",
		},
		ResumeHint: "Continue with build",
	}
}

func TestListCheckpoints_Empty(t *testing.T) {
	s := setupRealMCPServer(t)
	data := callToolAndParseJSON(t, s, "backlogit_list_checkpoints", map[string]any{})

	assert.Equal(t, float64(0), data["total"])
	assert.Equal(t, float64(0), data["quarantined"])
}

func TestListCheckpoints_WithFilters(t *testing.T) {
	s := setupRealMCPServer(t)

	// Create checkpoints directly in the workspace.
	cp := newTestCheckpointV1()
	writeV1Checkpoint(t, s.Workspace.RootPath, "checkpoint-20260423-100000.json", cp)

	cp2 := newTestCheckpointV1()
	cp2.Agent = "stage"
	writeV1Checkpoint(t, s.Workspace.RootPath, "checkpoint-20260423-110000.json", cp2)

	// List all.
	data := callToolAndParseJSON(t, s, "backlogit_list_checkpoints", map[string]any{})
	assert.Equal(t, float64(2), data["total"])

	// Filter by agent.
	data = callToolAndParseJSON(t, s, "backlogit_list_checkpoints", map[string]any{
		"consumer_id": "ship",
	})
	assert.Equal(t, float64(1), data["total"])
}

func TestGetCheckpoint_ValidFile(t *testing.T) {
	s := setupRealMCPServer(t)
	cp := newTestCheckpointV1()
	writeV1Checkpoint(t, s.Workspace.RootPath, "checkpoint-20260423-100000.json", cp)

	data := callToolAndParseJSON(t, s, "backlogit_get_checkpoint", map[string]any{
		"filename": "checkpoint-20260423-100000.json",
	})
	assert.Equal(t, true, data["valid"])
	assert.Equal(t, "checkpoint-20260423-100000.json", data["filename"])

	cpData, ok := data["checkpoint"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "ship", cpData["agent"])
	assert.Equal(t, "test-session-001", cpData["session_id"])
}

func TestGetCheckpoint_MissingFile(t *testing.T) {
	s := setupRealMCPServer(t)

	result, err := callToolForTest(t, s, "backlogit_get_checkpoint", map[string]any{
		"filename": "checkpoint-nonexistent.json",
	})
	require.NoError(t, err)
	require.True(t, result.IsError, "should return error for missing checkpoint")

	tc, ok := result.Content[0].(mcplib.TextContent)
	require.True(t, ok)
	assert.Contains(t, tc.Text, "not_found")
}

func TestResolveCheckpoint_Lifecycle(t *testing.T) {
	s := setupRealMCPServer(t)
	cp := newTestCheckpointV1()
	cp.Status = "active"
	writeV1Checkpoint(t, s.Workspace.RootPath, "checkpoint-20260423-100000.json", cp)

	data := callToolAndParseJSON(t, s, "backlogit_resolve_checkpoint", map[string]any{
		"filename": "checkpoint-20260423-100000.json",
	})
	assert.Equal(t, true, data["ok"])
	assert.Equal(t, "resolved", data["status"])

	// Verify via get.
	getData := callToolAndParseJSON(t, s, "backlogit_get_checkpoint", map[string]any{
		"filename": "checkpoint-20260423-100000.json",
	})
	cpData := getData["checkpoint"].(map[string]any)
	assert.Equal(t, "resolved", cpData["status"])
}

func TestResolveCheckpoint_Idempotent(t *testing.T) {
	s := setupRealMCPServer(t)
	cp := newTestCheckpointV1()
	cp.Status = "resolved"
	writeV1Checkpoint(t, s.Workspace.RootPath, "checkpoint-20260423-100000.json", cp)

	data := callToolAndParseJSON(t, s, "backlogit_resolve_checkpoint", map[string]any{
		"filename": "checkpoint-20260423-100000.json",
	})
	assert.Equal(t, true, data["ok"])
}

func TestCleanupCheckpoints_RetentionPolicy(t *testing.T) {
	s := setupRealMCPServer(t)

	cp := newTestCheckpointV1()
	cp.Status = "resolved"
	writeV1Checkpoint(t, s.Workspace.RootPath, "checkpoint-20260423-100000.json", cp)

	data := callToolAndParseJSON(t, s, "backlogit_cleanup_checkpoints", map[string]any{
		"retention_days": float64(7),
	})
	assert.Equal(t, float64(1), data["archived_count"])
}

func TestCleanupCheckpoints_DefaultsToConfig(t *testing.T) {
	s := setupRealMCPServer(t)

	cp := newTestCheckpointV1()
	cp.Status = "resolved"
	writeV1Checkpoint(t, s.Workspace.RootPath, "checkpoint-20260423-100000.json", cp)

	// No explicit retention_days — should use config default (7).
	data := callToolAndParseJSON(t, s, "backlogit_cleanup_checkpoints", map[string]any{})
	assert.Equal(t, float64(1), data["archived_count"])
}

func TestCreateCheckpoint_V1Schema(t *testing.T) {
	s := setupRealMCPServer(t)

	v1 := map[string]any{
		"schema_version": 1,
		"agent":          "ship",
		"session_id":     "test-session-v1",
		"phase":          "build-execution",
		"status":         "active",
		"context": map[string]any{
			"shipment_id": "044-S",
		},
	}
	v1JSON, err := json.Marshal(v1)
	require.NoError(t, err)

	data := callToolAndParseJSON(t, s, "backlogit_create_checkpoint", map[string]any{
		"state_dump": string(v1JSON),
	})
	assert.Contains(t, data["path"], "checkpoint-")

	// Verify the written file is valid V1.
	path := data["path"].(string)
	fileData, err := os.ReadFile(path)
	require.NoError(t, err)

	cp, err := events.ParseCheckpoint(fileData)
	require.NoError(t, err)
	assert.Equal(t, 1, cp.SchemaVersion)
	assert.Equal(t, "ship", cp.Agent)
	assert.False(t, cp.CreatedAt.IsZero(), "created_at should be auto-populated")
	assert.False(t, cp.UpdatedAt.IsZero(), "updated_at should be auto-populated")
}

func TestCreateCheckpoint_V1Schema_InvalidRejected(t *testing.T) {
	s := setupRealMCPServer(t)

	v1 := map[string]any{
		"schema_version": 1,
		"agent":          "invalid-agent",
		"session_id":     "test-session",
		"phase":          "build",
	}
	v1JSON, err := json.Marshal(v1)
	require.NoError(t, err)

	result, err := callToolForTest(t, s, "backlogit_create_checkpoint", map[string]any{
		"state_dump": string(v1JSON),
	})
	require.NoError(t, err)
	require.True(t, result.IsError, "should reject invalid V1 checkpoint")
}

func TestCreateCheckpoint_LegacyFormat(t *testing.T) {
	s := setupRealMCPServer(t)

	// Legacy format without schema_version.
	legacy := `{"agent":"ship","phase":"test","custom_data":"hello"}`
	data := callToolAndParseJSON(t, s, "backlogit_create_checkpoint", map[string]any{
		"state_dump": legacy,
	})
	assert.Contains(t, data["path"], "checkpoint-")

	// Verify written as-is.
	path := data["path"].(string)
	fileData, err := os.ReadFile(path)
	require.NoError(t, err)

	var parsed map[string]any
	require.NoError(t, json.Unmarshal(fileData, &parsed))
	assert.Equal(t, "hello", parsed["custom_data"])
}

func TestGetCheckpoint_ValidationError(t *testing.T) {
	s := setupRealMCPServer(t)

	result, err := callToolForTest(t, s, "backlogit_get_checkpoint", map[string]any{
		"filename": "",
	})
	require.NoError(t, err)
	require.True(t, result.IsError)
}
