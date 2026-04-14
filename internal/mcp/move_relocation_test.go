package mcp

// 026.009-T: handleMoveItem file relocation harness.
//
// These tests verify that handleMoveItem relocates the artifact's Markdown file
// to the registry-mapped directory when the status changes:
//
//   - moving a feature to "done" must relocate the file to the archive directory
//   - moving a task to a status without a matching registry rule keeps the file
//     in its current directory

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	mcplib "github.com/mark3labs/mcp-go/mcp"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/backlogit/backlogit/internal/core"
	"github.com/backlogit/backlogit/internal/models"
)

// TestMoveItem_RelocatesFile verifies that moving a feature to "done" physically
// moves its Markdown file into the archive directory defined by registry.yaml.
func TestMoveItem_RelocatesFile(t *testing.T) {
	s, ws := setupBugFixServer(t)
	ctx := context.Background()

	feature, err := core.CreateArtifact(ctx, ws, "Feature to relocate", "feature")
	require.NoError(t, err)

	initialPath, err := core.FindArtifactPath(ctx, ws, feature.ID)
	require.NoError(t, err)
	assert.NotContains(t, filepath.ToSlash(initialPath), "/archive/",
		"newly created feature must not be in archive yet")

	// Transition to active first (queued→done is not a valid transition).
	_, err = core.UpdateArtifact(ctx, ws, feature.ID, map[string]any{"status": "active"})
	require.NoError(t, err)

	result, err := s.handleMoveItem(ctx, shipmentRequest(map[string]any{
		"id":     feature.ID,
		"status": "done",
	}))

	require.NoError(t, err)
	require.False(t, result.IsError, "move to done must succeed: %s", resultText(t, result))

	// Verify the artifact reflects the new status.
	var artifact models.Artifact
	textContent, ok := result.Content[0].(mcplib.TextContent)
	require.True(t, ok, "response must be text content")
	require.NoError(t, json.Unmarshal([]byte(textContent.Text), &artifact))
	assert.Equal(t, models.StatusDone, artifact.Status)

	// Verify the Markdown file is now in the archive directory.
	finalPath, pathErr := core.FindArtifactPath(ctx, ws, feature.ID)
	require.NoError(t, pathErr)
	assert.Contains(t, filepath.ToSlash(finalPath), "/archive/",
		"Markdown file must be relocated to archive after moving to done")
}

// TestMoveItem_NoRegistryRule_FileStaysInPlace verifies that moving to a status
// that has no matching directory rule in registry.yaml leaves the file in its
// current location (no erroneous relocation).
func TestMoveItem_NoRegistryRule_FileStaysInPlace(t *testing.T) {
	s, ws := setupBugFixServer(t)
	ctx := context.Background()

	feature, err := core.CreateArtifact(ctx, ws, "Feature to stay put", "feature")
	require.NoError(t, err)

	initialPath, err := core.FindArtifactPath(ctx, ws, feature.ID)
	require.NoError(t, err)
	initialDir := filepath.Dir(initialPath)

	// "active" has no registry routing rule — file should stay in queue.
	result, err := s.handleMoveItem(ctx, shipmentRequest(map[string]any{
		"id":     feature.ID,
		"status": "active",
	}))

	require.NoError(t, err)
	require.False(t, result.IsError, "move to active must succeed: %s", resultText(t, result))

	finalPath, pathErr := core.FindArtifactPath(ctx, ws, feature.ID)
	require.NoError(t, pathErr)
	assert.Equal(t, initialDir, filepath.Dir(finalPath),
		"file must remain in the same directory when status has no routing rule")
}

// resultText is a test helper that returns the raw text content of an MCP result.
func resultText(t *testing.T, result *mcplib.CallToolResult) string {
	t.Helper()
	if len(result.Content) == 0 {
		return "<empty>"
	}
	text, ok := result.Content[0].(mcplib.TextContent)
	if !ok {
		return "<non-text>"
	}
	return text.Text
}
