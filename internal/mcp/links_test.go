package mcp

// 018.002-T: MCP link tool harness.
//
// Implementation complete. These tests validate the handler contract for
// backlogit_add_link, backlogit_get_links, and backlogit_remove_link:
//
//   - backlogit_add_link accepts source_id, target_id, link_type and creates a row
//   - backlogit_add_link rejects an invalid link_type with error="validation_failed"
//   - backlogit_add_link rejects a non-existent source_id with error="not_found"
//   - backlogit_add_link rejects a non-existent target_id with error="not_found"
//   - backlogit_get_links returns all links for a given item ID
//   - backlogit_get_links with link_type filter returns only matching links
//   - backlogit_remove_link removes the specified edge

import (
	"context"
	"encoding/json"
	"testing"

	mcplib "github.com/mark3labs/mcp-go/mcp"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/backlogit/backlogit/internal/core"
)

func TestHandleAddLink_Success(t *testing.T) {
	s, ws := setupBugFixServer(t)
	ctx := context.Background()

	src, err := core.CreateArtifact(ctx, ws, "Source artifact", "task")
	require.NoError(t, err)
	tgt, err := core.CreateArtifact(ctx, ws, "Target artifact", "task")
	require.NoError(t, err)

	result, err := s.handleAddLink(ctx, linkRequest(map[string]any{
		"source_id": src.ID,
		"target_id": tgt.ID,
		"link_type": "related_to",
	}))

	require.NoError(t, err)
	assert.False(t, result.IsError, "add_link must succeed for valid inputs")
}

func TestHandleAddLink_InvalidLinkType_ValidationFailed(t *testing.T) {
	s, ws := setupBugFixServer(t)
	ctx := context.Background()

	src, err := core.CreateArtifact(ctx, ws, "Src", "task")
	require.NoError(t, err)
	tgt, err := core.CreateArtifact(ctx, ws, "Tgt", "task")
	require.NoError(t, err)

	result, err := s.handleAddLink(ctx, linkRequest(map[string]any{
		"source_id": src.ID,
		"target_id": tgt.ID,
		"link_type": "made_up_type",
	}))

	require.NoError(t, err)
	assert.Equal(t, "validation_failed", linkErrorField(t, result),
		"invalid link_type must surface as validation_failed")
}

func TestHandleAddLink_MissingSourceID_ValidationFailed(t *testing.T) {
	s, _ := setupBugFixServer(t)
	ctx := context.Background()

	result, err := s.handleAddLink(ctx, linkRequest(map[string]any{
		"target_id": "X001",
		"link_type": "informs",
	}))

	require.NoError(t, err)
	assert.Equal(t, "validation_failed", linkErrorField(t, result))
}

func TestHandleAddLink_NonExistentSourceID_NotFound(t *testing.T) {
	s, ws := setupBugFixServer(t)
	ctx := context.Background()

	tgt, err := core.CreateArtifact(ctx, ws, "Target", "task")
	require.NoError(t, err)

	result, err := s.handleAddLink(ctx, linkRequest(map[string]any{
		"source_id": "DOES-NOT-EXIST",
		"target_id": tgt.ID,
		"link_type": "informs",
	}))

	require.NoError(t, err)
	assert.Equal(t, "not_found", linkErrorField(t, result),
		"non-existent source_id must surface as not_found")
}

func TestHandleAddLink_NonExistentTargetID_NotFound(t *testing.T) {
	s, ws := setupBugFixServer(t)
	ctx := context.Background()

	src, err := core.CreateArtifact(ctx, ws, "Source", "task")
	require.NoError(t, err)

	result, err := s.handleAddLink(ctx, linkRequest(map[string]any{
		"source_id": src.ID,
		"target_id": "DOES-NOT-EXIST",
		"link_type": "informs",
	}))

	require.NoError(t, err)
	assert.Equal(t, "not_found", linkErrorField(t, result),
		"non-existent target_id must surface as not_found")
}

func TestHandleGetLinks_ReturnsLinks(t *testing.T) {
	s, ws := setupBugFixServer(t)
	ctx := context.Background()

	src, err := core.CreateArtifact(ctx, ws, "Source", "task")
	require.NoError(t, err)
	tgt, err := core.CreateArtifact(ctx, ws, "Target", "task")
	require.NoError(t, err)

	_, err = s.handleAddLink(ctx, linkRequest(map[string]any{
		"source_id": src.ID,
		"target_id": tgt.ID,
		"link_type": "informs",
	}))
	require.NoError(t, err)

	result, err := s.handleGetLinks(ctx, linkRequest(map[string]any{
		"id": src.ID,
	}))

	require.NoError(t, err)
	require.False(t, result.IsError)

	text, ok := result.Content[0].(mcplib.TextContent)
	require.True(t, ok)
	var payload struct {
		Links []struct {
			TargetID string `json:"target_id"`
			LinkType string `json:"link_type"`
		} `json:"links"`
	}
	require.NoError(t, json.Unmarshal([]byte(text.Text), &payload))
	require.Len(t, payload.Links, 1)
	assert.Equal(t, tgt.ID, payload.Links[0].TargetID)
	assert.Equal(t, "informs", payload.Links[0].LinkType)
}

func TestHandleGetLinks_WithTypeFilter(t *testing.T) {
	s, ws := setupBugFixServer(t)
	ctx := context.Background()

	src, err := core.CreateArtifact(ctx, ws, "Source", "task")
	require.NoError(t, err)
	tgt1, err := core.CreateArtifact(ctx, ws, "Target1", "task")
	require.NoError(t, err)
	tgt2, err := core.CreateArtifact(ctx, ws, "Target2", "task")
	require.NoError(t, err)

	_, err = s.handleAddLink(ctx, linkRequest(map[string]any{
		"source_id": src.ID, "target_id": tgt1.ID, "link_type": "informs",
	}))
	require.NoError(t, err)
	_, err = s.handleAddLink(ctx, linkRequest(map[string]any{
		"source_id": src.ID, "target_id": tgt2.ID, "link_type": "related_to",
	}))
	require.NoError(t, err)

	result, err := s.handleGetLinks(ctx, linkRequest(map[string]any{
		"id":        src.ID,
		"link_type": "informs",
	}))

	require.NoError(t, err)
	require.False(t, result.IsError)

	text, ok := result.Content[0].(mcplib.TextContent)
	require.True(t, ok)
	var payload struct {
		Links []map[string]any `json:"links"`
	}
	require.NoError(t, json.Unmarshal([]byte(text.Text), &payload))
	assert.Len(t, payload.Links, 1, "link_type filter must return only matching links")
}

func TestHandleRemoveLink_Success(t *testing.T) {
	s, ws := setupBugFixServer(t)
	ctx := context.Background()

	src, err := core.CreateArtifact(ctx, ws, "Src", "task")
	require.NoError(t, err)
	tgt, err := core.CreateArtifact(ctx, ws, "Tgt", "task")
	require.NoError(t, err)

	_, err = s.handleAddLink(ctx, linkRequest(map[string]any{
		"source_id": src.ID, "target_id": tgt.ID, "link_type": "duplicate_of",
	}))
	require.NoError(t, err)

	result, err := s.handleRemoveLink(ctx, linkRequest(map[string]any{
		"source_id": src.ID,
		"target_id": tgt.ID,
		"link_type": "duplicate_of",
	}))

	require.NoError(t, err)
	assert.False(t, result.IsError)

	listResult, err := s.handleGetLinks(ctx, linkRequest(map[string]any{"id": src.ID}))
	require.NoError(t, err)
	text, ok := listResult.Content[0].(mcplib.TextContent)
	require.True(t, ok)
	var payload struct {
		Links []any `json:"links"`
	}
	require.NoError(t, json.Unmarshal([]byte(text.Text), &payload))
	assert.Empty(t, payload.Links, "link must be removed after handleRemoveLink")
}

// linkRequest builds a CallToolRequest for link tool handlers.
func linkRequest(args map[string]any) mcplib.CallToolRequest {
	req := mcplib.CallToolRequest{}
	req.Params.Arguments = args
	return req
}

// linkErrorField extracts the "error" key from an error result.
func linkErrorField(t *testing.T, result *mcplib.CallToolResult) string {
	t.Helper()
	require.True(t, result.IsError, "expected error result")
	require.NotEmpty(t, result.Content)
	text, ok := result.Content[0].(mcplib.TextContent)
	require.True(t, ok, "expected text content")
	var resp struct {
		Error string `json:"error"`
	}
	require.NoError(t, json.Unmarshal([]byte(text.Text), &resp))
	return resp.Error
}
