package mcp

import (
	"context"
	"encoding/json"

	mcplib "github.com/mark3labs/mcp-go/mcp"

	"github.com/softwaresalt/backlogit/internal/core"
)

// registerReconcileTools registers the lifecycle reconciliation and stash
// provenance correction MCP tools with the server.
func (s *Server) registerReconcileTools() {
	s.addTool(
		mcplib.NewTool("backlogit_reconcile_archived_lifecycle",
			mcplib.WithDescription("Reconcile archived items by correcting their lifecycle status. "+
				"Executes unarchive → status update → re-archive for each item, preserving the original archive history."),
			mcplib.WithString("item_ids",
				mcplib.Required(),
				mcplib.Description("Comma-separated item IDs to reconcile (e.g. '001-T,002-T')"),
			),
			mcplib.WithString("reason",
				mcplib.Required(),
				mcplib.Description("Human-readable reason for the reconciliation"),
			),
			mcplib.WithString("actor",
				mcplib.Required(),
				mcplib.Description("Agent or operator performing the reconciliation"),
			),
			mcplib.WithString("target_status",
				mcplib.Description("Target terminal status (default: done; allowed: done, accepted, rejected, abandoned)"),
				mcplib.DefaultString("done"),
			),
			mcplib.WithString("idempotency_key",
				mcplib.Description("Optional caller-supplied idempotency key"),
			),
		),
		s.handleReconcileArchivedLifecycle,
	)
	s.addTool(
		mcplib.NewTool("backlogit_correct_stash_provenance",
			mcplib.WithDescription("Record a stash provenance correction. Preserves the historical harvested_artifact_id "+
				"and records the canonical actual delivery artifact in an append-only correction log."),
			mcplib.WithString("stash_id",
				mcplib.Required(),
				mcplib.Description("Stash entry ID to correct"),
			),
			mcplib.WithString("canonical_delivery_artifact_id",
				mcplib.Required(),
				mcplib.Description("Artifact ID of the actual delivery (must have source_stash_id matching stash_id)"),
			),
			mcplib.WithString("reason",
				mcplib.Required(),
				mcplib.Description("Human-readable reason for the correction"),
			),
			mcplib.WithString("actor",
				mcplib.Required(),
				mcplib.Description("Agent or operator performing the correction"),
			),
		),
		s.handleCorrectStashProvenance,
	)
}

// handleReconcileArchivedLifecycle implements the backlogit_reconcile_archived_lifecycle tool.
// It parses the comma-separated item IDs, delegates to core.ReconcileArchivedLifecycle, and
// returns the serialised ReconciliationResult.
func (s *Server) handleReconcileArchivedLifecycle(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	ws, errResult := s.requireWorkspace(ctx)
	if errResult != nil {
		return errResult, nil
	}

	itemIDsStr, _ := req.Params.Arguments["item_ids"].(string)
	reason, _ := req.Params.Arguments["reason"].(string)
	actor, _ := req.Params.Arguments["actor"].(string)
	targetStatus, _ := req.Params.Arguments["target_status"].(string)
	idempotencyKey, _ := req.Params.Arguments["idempotency_key"].(string)

	itemIDs := splitCommaSeparated(itemIDsStr)

	reconReq := core.ReconciliationRequest{
		ItemIDs:        itemIDs,
		TargetStatus:   targetStatus,
		Reason:         reason,
		Actor:          actor,
		IdempotencyKey: idempotencyKey,
	}

	result, err := core.ReconcileArchivedLifecycle(ctx, ws.DB, ws, reconReq)
	if err != nil {
		return domainError("reconcile_archived_lifecycle", err), nil
	}

	b, marshalErr := json.Marshal(result)
	if marshalErr != nil {
		return InternalError("marshal reconciliation result: " + marshalErr.Error()), nil
	}
	return mcplib.NewToolResultText(string(b)), nil
}

// handleCorrectStashProvenance implements the backlogit_correct_stash_provenance tool.
// It delegates to core.CorrectStashProvenance and returns the serialised result.
func (s *Server) handleCorrectStashProvenance(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	ws, errResult := s.requireWorkspace(ctx)
	if errResult != nil {
		return errResult, nil
	}

	stashID, _ := req.Params.Arguments["stash_id"].(string)
	canonicalID, _ := req.Params.Arguments["canonical_delivery_artifact_id"].(string)
	reason, _ := req.Params.Arguments["reason"].(string)
	actor, _ := req.Params.Arguments["actor"].(string)

	corrReq := core.StashProvenanceCorrectionRequest{
		StashID:                     stashID,
		CanonicalDeliveryArtifactID: canonicalID,
		Reason:                      reason,
		Actor:                       actor,
	}

	result, err := core.CorrectStashProvenance(ctx, ws, corrReq)
	if err != nil {
		return domainError("correct_stash_provenance", err), nil
	}

	b, marshalErr := json.Marshal(result)
	if marshalErr != nil {
		return InternalError("marshal stash provenance result: " + marshalErr.Error()), nil
	}
	return mcplib.NewToolResultText(string(b)), nil
}
