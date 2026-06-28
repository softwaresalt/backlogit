package mcp

// 062.003-T: Re-upsert rewritten section bodies (stash 6235FF06).
//
// After an MCP section-body rewrite, the artifact must be re-upserted so that
// SQLite (items.description) and the FTS index match the file content
// immediately. Before the fix, writeSectionsToFile rewrote the Markdown body
// but never re-upserted, so handleGetItem (without a section view) and FTS
// search returned the stale, pre-rewrite body until a manual sync.

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	mcplib "github.com/mark3labs/mcp-go/mcp"

	"github.com/softwaresalt/backlogit/internal/db"
)

// After handleUpdateItem rewrites a section, the DB description must reflect the
// new body immediately (no manual sync).
func TestHandleUpdateItem_SectionRewrite_ReupsertsDB(t *testing.T) {
	s, ws := setupBugFixServer(t)
	ctx := context.Background()

	artifact := seedArtifactWithSections(t, ws)
	require.NoError(t, db.UpsertItem(ctx, ws.DB, artifact))

	const newBody = "Re-upserted description body after rewrite"
	req := mcplib.CallToolRequest{}
	req.Params.Name = "backlogit_update_item"
	req.Params.Arguments = map[string]any{
		"id": artifact.ID,
		"sections": map[string]any{
			"description": newBody,
		},
	}

	result, err := s.handleUpdateItem(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.False(t, result.IsError, "update with sections should succeed")

	// Read straight from the DB — no manual sync — and confirm the new body is
	// reflected in items.description.
	stored, err := db.GetItem(ctx, ws.DB, artifact.ID)
	require.NoError(t, err)
	assert.Contains(t, stored.Description, newBody,
		"items.description must reflect the rewritten section body immediately")
	assert.NotContains(t, stored.Description, "This is the description section",
		"the stale pre-rewrite body must not remain in the DB")
}

// handleGetItem without a section view must return the updated body after a
// section rewrite, sourced from the DB.
func TestHandleGetItem_AfterSectionRewrite_ReturnsUpdatedBody(t *testing.T) {
	s, ws := setupBugFixServer(t)
	ctx := context.Background()

	artifact := seedArtifactWithSections(t, ws)
	require.NoError(t, db.UpsertItem(ctx, ws.DB, artifact))

	const newBody = "Body visible without a manual sync"
	updateReq := mcplib.CallToolRequest{}
	updateReq.Params.Name = "backlogit_update_item"
	updateReq.Params.Arguments = map[string]any{
		"id": artifact.ID,
		"sections": map[string]any{
			"description": newBody,
		},
	}
	updateResult, err := s.handleUpdateItem(ctx, updateReq)
	require.NoError(t, err)
	require.False(t, updateResult.IsError)

	getReq := mcplib.CallToolRequest{}
	getReq.Params.Name = "backlogit_get_item"
	getReq.Params.Arguments = map[string]any{"id": artifact.ID}
	getResult, err := s.handleGetItem(ctx, getReq)
	require.NoError(t, err)
	require.False(t, getResult.IsError)

	data := extractResultJSON(t, getResult)
	desc, _ := data["description"].(string)
	assert.Contains(t, desc, newBody,
		"get-item (no section view) must show the rewritten body without a manual sync")
}
