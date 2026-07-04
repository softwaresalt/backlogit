package mcp

import (
	"context"
	"fmt"

	mcplib "github.com/mark3labs/mcp-go/mcp"

	"github.com/softwaresalt/backlogit/internal/core"
	"github.com/softwaresalt/backlogit/internal/db"
)

// handleAddLink is the MCP handler for backlogit_add_link.
// Creates a directed semantic link from source_id to target_id with the given link_type.
func (s *Server) handleAddLink(ctx context.Context, request mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	ws, errResult := s.requireWorkspace(ctx)
	if errResult != nil {
		return errResult, nil
	}

	sourceID, _ := request.Params.Arguments["source_id"].(string)
	targetID, _ := request.Params.Arguments["target_id"].(string)
	linkType, _ := request.Params.Arguments["link_type"].(string)

	if sourceID == "" {
		return ValidationFailed("source_id is required"), nil
	}
	if targetID == "" {
		return ValidationFailed("target_id is required"), nil
	}
	if linkType == "" {
		return ValidationFailed("link_type is required"), nil
	}

	if _, err := db.GetItem(ctx, ws.DB, sourceID); err != nil {
		return domainError("add link source_id", err), nil
	}
	if _, err := db.GetItem(ctx, ws.DB, targetID); err != nil {
		return domainError("add link target_id", err), nil
	}

	if err := core.AddArtifactLink(ctx, ws, sourceID, targetID, linkType); err != nil {
		return domainError("add link", err), nil
	}

	resp := map[string]string{
		"source_id": sourceID,
		"target_id": targetID,
		"link_type": linkType,
	}
	return toolResultJSON(resp)
}

// handleGetLinks is the MCP handler for backlogit_get_links.
// Returns all outgoing links from the given item ID, optionally filtered by link_type.
func (s *Server) handleGetLinks(ctx context.Context, request mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	ws, errResult := s.requireWorkspace(ctx)
	if errResult != nil {
		return errResult, nil
	}

	id, _ := request.Params.Arguments["id"].(string)
	if id == "" {
		return ValidationFailed("id is required"), nil
	}

	linkType, _ := request.Params.Arguments["link_type"].(string)

	edges, err := core.GetLinks(ctx, ws, id, linkType)
	if err != nil {
		return InternalError(fmt.Sprintf("get links: %v", err)), nil
	}

	resp := map[string]any{
		"id":    id,
		"links": edges,
	}
	return toolResultJSON(resp)
}

// handleRemoveLink is the MCP handler for backlogit_remove_link.
// Deletes the directed link matching (source_id, target_id, link_type).
func (s *Server) handleRemoveLink(ctx context.Context, request mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	ws, errResult := s.requireWorkspace(ctx)
	if errResult != nil {
		return errResult, nil
	}

	sourceID, _ := request.Params.Arguments["source_id"].(string)
	targetID, _ := request.Params.Arguments["target_id"].(string)
	linkType, _ := request.Params.Arguments["link_type"].(string)

	if sourceID == "" {
		return ValidationFailed("source_id is required"), nil
	}
	if targetID == "" {
		return ValidationFailed("target_id is required"), nil
	}
	if linkType == "" {
		return ValidationFailed("link_type is required"), nil
	}

	if err := core.RemoveArtifactLink(ctx, ws, sourceID, targetID, linkType); err != nil {
		return domainError("remove link", err), nil
	}

	resp := map[string]string{
		"source_id": sourceID,
		"target_id": targetID,
		"link_type": linkType,
		"status":    "removed",
	}
	return toolResultJSON(resp)
}
