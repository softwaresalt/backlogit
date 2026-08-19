package mcp

// 144.007-T (U7): Governed-parity tests for guard 1 and guard 2.
//
// These tests confirm that move_item / create_item and archive_item return the
// stable per-sentinel error contract on the MCP surface (error_type field),
// with gate OFF (no BACKLOGIT_FORMAL_GATE_REQUIRED env var), and that no MCP
// force lever bypasses the guards.

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/core"
	"github.com/softwaresalt/backlogit/internal/events"
)

// TestMCP_Guard1_MoveItemShipmentToShipped_RefusedWithStableSentinel verifies
// that backlogit_move_item for a shipment → shipped returns the
// "shipment_shipped_requires_envelope" error_type, gate OFF, no force lever.
func TestMCP_Guard1_MoveItemShipmentToShipped_RefusedWithStableSentinel(t *testing.T) {
	s, ws := setupBugFixServer(t)
	ctx := context.Background()

	shipment, err := core.CreateShipment(ctx, ws, "Guard-1 MCP parity shipment", nil)
	require.NoError(t, err)
	_, err = core.ClaimShipment(ctx, ws, shipment.ID)
	require.NoError(t, err)

	result, err := s.handleMoveItem(ctx, contractRequest(map[string]any{
		"id":     shipment.ID,
		"status": "shipped",
	}))
	require.NoError(t, err)
	assert.Equal(t, "shipment_shipped_requires_envelope", contractErrorType(t, result),
		"handleMoveItem shipment→shipped must return shipment_shipped_requires_envelope (gate OFF)")
}

// TestMCP_Guard1_UpdateItemShipmentToShipped_RefusedWithStableSentinel verifies
// that backlogit_update_item for a shipment → shipped returns the same sentinel.
func TestMCP_Guard1_UpdateItemShipmentToShipped_RefusedWithStableSentinel(t *testing.T) {
	s, ws := setupBugFixServer(t)
	ctx := context.Background()

	shipment, err := core.CreateShipment(ctx, ws, "Guard-1 update MCP parity shipment", nil)
	require.NoError(t, err)
	_, err = core.ClaimShipment(ctx, ws, shipment.ID)
	require.NoError(t, err)

	result, err := s.handleUpdateItem(ctx, contractRequest(map[string]any{
		"id":     shipment.ID,
		"status": "shipped",
	}))
	require.NoError(t, err)
	assert.Equal(t, "shipment_shipped_requires_envelope", contractErrorType(t, result),
		"handleUpdateItem shipment→shipped must return shipment_shipped_requires_envelope")
}

// TestMCP_Guard1_CreateItemShipmentShipped_RefusedWithStableSentinel verifies
// that backlogit_create_item with artifact_type shipment and status shipped
// returns "shipment_shipped_requires_envelope".
func TestMCP_Guard1_CreateItemShipmentShipped_RefusedWithStableSentinel(t *testing.T) {
	s, _ := setupBugFixServer(t)
	ctx := context.Background()

	result, err := s.handleCreateItem(ctx, contractRequest(map[string]any{
		"title":         "Born-shipped shipment",
		"artifact_type": "shipment",
		"status":        "shipped",
	}))
	require.NoError(t, err)
	assert.Equal(t, "shipment_shipped_requires_envelope", contractErrorType(t, result),
		"handleCreateItem shipment with initial shipped must return shipment_shipped_requires_envelope")
}

// TestMCP_Guard2_ArchiveShippedWithoutEvent_RefusedWithStableSentinel verifies
// that backlogit_archive_item on a shipped-without-event shipment returns
// "archive_shipped_requires_event".
func TestMCP_Guard2_ArchiveShippedWithoutEvent_RefusedWithStableSentinel(t *testing.T) {
	s, ws := setupBugFixServer(t)
	ctx := context.Background()

	// Create a shipped-without-event fixture by writing frontmatter directly.
	art, err := core.CreateShipment(ctx, ws, "Guard-2 MCP parity shipment", nil)
	require.NoError(t, err)
	path, findErr := core.FindArtifactPath(ctx, ws, art.ID)
	require.NoError(t, findErr)
	raw, readErr := os.ReadFile(path)
	require.NoError(t, readErr)
	updated := strings.ReplaceAll(string(raw), "status: queued", "status: shipped")
	require.NoError(t, os.WriteFile(path, []byte(updated), 0o644))
	// No shipped event written — this is the residue guard 2 detects.

	result, err := s.handleArchiveItem(ctx, contractRequest(map[string]any{
		"id": art.ID,
	}))
	require.NoError(t, err)
	assert.Equal(t, "archive_shipped_requires_event", contractErrorType(t, result),
		"handleArchiveItem on shipped-without-event shipment must return archive_shipped_requires_event")
}

// TestMCP_Guard2_ArchiveShippedWithEvent_Allowed verifies that
// backlogit_archive_item succeeds when the durable shipped event is present.
func TestMCP_Guard2_ArchiveShippedWithEvent_Allowed(t *testing.T) {
	s, ws := setupBugFixServer(t)
	ctx := context.Background()

	art, err := core.CreateShipment(ctx, ws, "Guard-2 event-present parity shipment", nil)
	require.NoError(t, err)
	path, findErr := core.FindArtifactPath(ctx, ws, art.ID)
	require.NoError(t, findErr)
	raw, readErr := os.ReadFile(path)
	require.NoError(t, readErr)
	updated := strings.ReplaceAll(string(raw), "status: queued", "status: shipped")
	require.NoError(t, os.WriteFile(path, []byte(updated), 0o644))

	// Write the required durable shipped event.
	logsDir := core.WorkspaceLogsRoot(ws.RootPath)
	require.NoError(t, os.MkdirAll(logsDir, 0o755))
	eventLine := `{"timestamp":"` + time.Now().UTC().Format(time.RFC3339Nano) + `","item_id":"` + art.ID + `","event_type":"shipment_status_changed","delta":{"status":"shipped"},"actor":"backlogit"}` + "\n"
	require.NoError(t, os.WriteFile(filepath.Join(logsDir, art.ID+".jsonl"), []byte(eventLine), 0o644))

	_ = events.LogPathForItem(logsDir, art.ID)

	result, err := s.handleArchiveItem(ctx, contractRequest(map[string]any{
		"id": art.ID,
	}))
	require.NoError(t, err)
	assert.False(t, result.IsError,
		"handleArchiveItem on shipped-with-event shipment must succeed: %s",
		resultTextForHarness(t, result))
}
