package mcp

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/core"
	"github.com/softwaresalt/backlogit/internal/events"
)

func TestU20C4_WarmServerNormalizeUsesTargetGuardedRecoveryPath(t *testing.T) {
	ctx := context.Background()
	root, shipmentID, snapshotRef := newPoisonedNormalizationWorkspace(t)
	poisonPath := filepath.Join(root, ".backlogit", "ops", "unrelated-poison.json")
	poisonBytes, err := os.ReadFile(poisonPath)
	require.NoError(t, err)
	poisonAsidePath := filepath.Join(root, "unrelated-poison.json.aside")
	require.NoError(t, os.Rename(poisonPath, poisonAsidePath))
	poisonMovedAside := true
	t.Cleanup(func() {
		if poisonMovedAside {
			require.NoError(t, os.Rename(poisonAsidePath, poisonPath))
		}
	})

	server := NewServerForRoot(root)
	warmRequest := mcplib.CallToolRequest{}
	warmRequest.Params.Arguments = map[string]any{"id": shipmentID}
	warmResult, err := server.InvokeTool(ctx, "backlogit_get_shipment", warmRequest)
	require.NoError(t, err)
	require.False(t, warmResult.IsError, "the workspace-requiring shipment read must warm the server")
	require.NotNil(t, server.Workspace)
	t.Cleanup(func() { require.NoError(t, server.Workspace.Close()) })

	require.NoError(t, os.Rename(poisonAsidePath, poisonPath))
	poisonMovedAside = false

	normalizeRequest := mcplib.CallToolRequest{}
	normalizeRequest.Params.Arguments = map[string]any{
		"id":           shipmentID,
		"snapshot_ref": snapshotRef,
		"by":           "diagnostic-test",
	}
	result, err := server.InvokeTool(ctx, "backlogit_normalize_blocked_shipment", normalizeRequest)
	require.NoError(t, err)
	assert.False(t, result.IsError,
		"normalization on a warm server must use the target-guarded recovery path despite unrelated poison")

	poisonAfter, err := os.ReadFile(poisonPath)
	require.NoError(t, err)
	assert.Equal(t, poisonBytes, poisonAfter, "normalization must preserve the unrelated poison byte-for-byte")

	ordinary, ordinaryErr := core.NewWorkspace(ctx, root)
	require.Error(t, ordinaryErr, "ordinary workspace initialization must remain fail-closed")
	if ordinary != nil {
		require.NoError(t, ordinary.Close())
	}
	assert.True(t,
		strings.Contains(ordinaryErr.Error(), "unrelated-poison.json") ||
			strings.Contains(strings.ToLower(ordinaryErr.Error()), "shipment operation journal") ||
			strings.Contains(strings.ToLower(ordinaryErr.Error()), "shipment-operation journal"),
		"ordinary initialization must fail specifically on the poison journal, not lock contention: %v",
		ordinaryErr)
}

func TestU20C4_NormalizeToolRequiresActor(t *testing.T) {
	ctx := context.Background()
	root, shipmentID, snapshotRef := newPoisonedNormalizationWorkspace(t)
	poisonPath := filepath.Join(root, ".backlogit", "ops", "unrelated-poison.json")
	poisonAsidePath := filepath.Join(root, "unrelated-poison.json.aside")
	require.NoError(t, os.Rename(poisonPath, poisonAsidePath))
	poisonMovedAside := true
	t.Cleanup(func() {
		if poisonMovedAside {
			require.NoError(t, os.Rename(poisonAsidePath, poisonPath))
		}
	})

	server := NewServerForRoot(root)
	warmRequest := mcplib.CallToolRequest{}
	warmRequest.Params.Arguments = map[string]any{"id": shipmentID}
	warmResult, err := server.InvokeTool(ctx, "backlogit_get_shipment", warmRequest)
	require.NoError(t, err)
	require.False(t, warmResult.IsError, "the workspace-requiring shipment read must initialize the workspace")
	require.NotNil(t, server.Workspace)
	t.Cleanup(func() { require.NoError(t, server.Workspace.Close()) })

	var required []string
	for _, tool := range server.ToolDefs() {
		if tool.Name == "backlogit_normalize_blocked_shipment" {
			required = tool.InputSchema.Required
			break
		}
	}
	for _, parameter := range []string{"id", "snapshot_ref", "by"} {
		assert.Contains(t, required, parameter, "normalize tool schema must require %s", parameter)
	}

	shipmentPath, err := core.FindArtifactPath(ctx, server.Workspace, shipmentID)
	require.NoError(t, err)
	shipmentBytesBefore, err := os.ReadFile(shipmentPath)
	require.NoError(t, err)
	eventsBefore, err := events.ReadAllEvents(ctx, core.WorkspaceLogsRoot(root), shipmentID)
	require.NoError(t, err)

	actorCases := []struct {
		name      string
		includeBy bool
		actor     string
	}{
		{name: "missing_by"},
		{name: "whitespace_only_by", includeBy: true, actor: "   "},
	}
	for _, actorCase := range actorCases {
		t.Run(actorCase.name, func(t *testing.T) {
			arguments := map[string]any{
				"id":           shipmentID,
				"snapshot_ref": snapshotRef,
			}
			if actorCase.includeBy {
				arguments["by"] = actorCase.actor
			}
			request := mcplib.CallToolRequest{}
			request.Params.Arguments = arguments
			result, invokeErr := server.InvokeTool(ctx, "backlogit_normalize_blocked_shipment", request)
			require.NoError(t, invokeErr)
			assert.Equal(t, "validation_failed", contractErrorType(t, result))
			require.NotEmpty(t, result.Content)
			text, ok := result.Content[0].(mcplib.TextContent)
			require.True(t, ok, "expected text error content, got %T", result.Content[0])
			assert.Contains(t, text.Text, "by is required",
				"the handler must reject a missing or whitespace-only actor before core normalization")
		})
	}

	shipmentBytesAfter, err := os.ReadFile(shipmentPath)
	require.NoError(t, err)
	assert.Equal(t, shipmentBytesBefore, shipmentBytesAfter,
		"invalid actor calls must leave the shipment file unchanged")
	eventsAfter, err := events.ReadAllEvents(ctx, core.WorkspaceLogsRoot(root), shipmentID)
	require.NoError(t, err)
	assert.Len(t, eventsAfter, len(eventsBefore), "invalid actor calls must not append shipment events")
}
