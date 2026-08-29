package mcp

import (
	"context"

	mcplib "github.com/mark3labs/mcp-go/mcp"
)

// registerReconcileTools registers the lifecycle reconciliation and stash
// provenance correction MCP tools with the server.  The handlers are stubs
// that return an internal error until the green-phase implementation lands.
func (s *Server) registerReconcileTools() {
	s.addTool(
		mcplib.NewTool("backlogit_reconcile_archived_lifecycle",
			mcplib.WithDescription("Reconcile archived items by correcting their lifecycle status. "+
				"Each item is unarchived, updated to the target terminal status, and re-archived "+
				"with a durable lifecycle_reconciliation event. Idempotent: items already at the "+
				"target status are returned as no_op without modification."),
			mcplib.WithString("item_ids",
				mcplib.Required(),
				mcplib.Description("Comma-separated list of archived item IDs to reconcile"),
			),
			mcplib.WithString("reason",
				mcplib.Required(),
				mcplib.Description("Human-readable reason for the reconciliation"),
			),
			mcplib.WithString("actor",
				mcplib.Required(),
				mcplib.Description("Operator or agent performing the reconciliation"),
			),
			mcplib.WithString("target_status",
				mcplib.Description("Target terminal status (done, accepted, rejected, abandoned, shipped); defaults to done"),
			),
			mcplib.WithString("idempotency_key",
				mcplib.Description("Optional caller-supplied key for deduplication"),
			),
		),
		s.handleReconcileArchivedLifecycle,
	)
	s.addTool(
		mcplib.NewTool("backlogit_correct_stash_provenance",
			mcplib.WithDescription("Record a stash provenance correction for a mismatched delivery artifact. "+
				"Links the stash entry to the correct canonical delivery artifact ID and appends a "+
				"durable correction event to the item log."),
			mcplib.WithString("stash_id",
				mcplib.Required(),
				mcplib.Description("Stash entry ID to correct"),
			),
			mcplib.WithString("canonical_delivery_artifact_id",
				mcplib.Required(),
				mcplib.Description("Canonical delivery artifact ID that should be associated with the stash entry"),
			),
			mcplib.WithString("reason",
				mcplib.Required(),
				mcplib.Description("Human-readable reason for the correction"),
			),
			mcplib.WithString("actor",
				mcplib.Required(),
				mcplib.Description("Operator or agent performing the correction"),
			),
		),
		s.handleCorrectStashProvenance,
	)
}

// handleReconcileArchivedLifecycle is the stub handler for
// backlogit_reconcile_archived_lifecycle.  Returns an internal error until the
// green-phase implementation in 152.005-T wires the real reconciliation logic.
func (s *Server) handleReconcileArchivedLifecycle(_ context.Context, _ mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	return InternalError("backlogit_reconcile_archived_lifecycle not implemented"), nil
}

// handleCorrectStashProvenance is the stub handler for
// backlogit_correct_stash_provenance.  Returns an internal error until the
// green-phase implementation in 152.008-T wires the real provenance correction
// logic.
func (s *Server) handleCorrectStashProvenance(_ context.Context, _ mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	return InternalError("backlogit_correct_stash_provenance not implemented"), nil
}
