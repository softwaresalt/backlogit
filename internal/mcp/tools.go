package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	mcplib "github.com/mark3labs/mcp-go/mcp"

	"github.com/softwaresalt/backlogit/internal/config"
	"github.com/softwaresalt/backlogit/internal/core"
	"github.com/softwaresalt/backlogit/internal/core/templates"
	"github.com/softwaresalt/backlogit/internal/db"
	backlogiterrors "github.com/softwaresalt/backlogit/internal/errors"
	"github.com/softwaresalt/backlogit/internal/events"
	"github.com/softwaresalt/backlogit/internal/models"
	"github.com/softwaresalt/backlogit/internal/parser"
	"github.com/softwaresalt/backlogit/internal/telemetry"
)

// RegisterTools adds all backlogit tools to the MCP server.
func (s *Server) RegisterTools() {
	s.addTool(
		mcplib.NewTool("backlogit_get_item",
			mcplib.WithDescription("Get a backlogit item by ID"),
			mcplib.WithString("id", mcplib.Required(), mcplib.Description("Item ID")),
			mcplib.WithString("section", mcplib.Description("Extract a named section from the body")),
		),
		s.handleGetItem,
	)
	s.addTool(
		mcplib.NewTool("backlogit_create_item",
			mcplib.WithDescription("Create a new backlogit artifact"),
			mcplib.WithString("title", mcplib.Required(), mcplib.Description("Artifact title")),
			mcplib.WithString("artifact_type", mcplib.Required(), mcplib.Description("Artifact type (feature, task, subtask)")),
			mcplib.WithString("status", mcplib.Description("Initial status"), mcplib.DefaultString("queued")),
			mcplib.WithString("description", mcplib.Description("Artifact description")),
			mcplib.WithString("parent_id", mcplib.Description("Parent artifact ID")),
			mcplib.WithString("sprint", mcplib.Description("Sprint ID")),
			mcplib.WithString("assigned_to", mcplib.Description("Assignee")),
			mcplib.WithString("owner", mcplib.Description("Owner")),
			mcplib.WithString("labels", mcplib.Description("Comma-separated labels")),
			mcplib.WithString("dependencies", mcplib.Description("Comma-separated dependency IDs")),
			mcplib.WithString("references", mcplib.Description("Comma-separated reference paths")),
			mcplib.WithString("commit", mcplib.Description("Commit SHA")),
			mcplib.WithString("sections", mcplib.Description("Section content as JSON object {name: content}")),
		),
		s.handleCreateItem,
	)
	s.addTool(
		mcplib.NewTool("backlogit_update_item",
			mcplib.WithDescription("Update an existing backlogit artifact"),
			mcplib.WithString("id", mcplib.Required(), mcplib.Description("Item ID")),
			mcplib.WithString("title", mcplib.Description("New title")),
			mcplib.WithString("status", mcplib.Description("New status")),
			mcplib.WithString("description", mcplib.Description("New description")),
			mcplib.WithString("sprint", mcplib.Description("Sprint ID")),
			mcplib.WithString("priority", mcplib.Description("Priority")),
			mcplib.WithString("assigned_to", mcplib.Description("Assignee")),
			mcplib.WithString("owner", mcplib.Description("Owner")),
			mcplib.WithString("labels", mcplib.Description("Comma-separated labels")),
			mcplib.WithString("commit", mcplib.Description("Commit SHA")),
			mcplib.WithString("sections", mcplib.Description("Section updates as JSON object {name: content}")),
		),
		s.handleUpdateItem,
	)
	s.addTool(
		mcplib.NewTool("backlogit_list_items",
			mcplib.WithDescription("List artifacts with optional filters"),
			mcplib.WithString("type", mcplib.Description("Filter by artifact type")),
			mcplib.WithString("status", mcplib.Description("Filter by status")),
			mcplib.WithString("assigned_to", mcplib.Description("Filter by assignee")),
			mcplib.WithString("sprint", mcplib.Description("Filter by sprint ID")),
		),
		s.handleListItems,
	)
	s.addTool(
		mcplib.NewTool("backlogit_search_items",
			mcplib.WithDescription("Full-text search across artifact titles and descriptions"),
			mcplib.WithString("query", mcplib.Required(), mcplib.Description("Search query")),
			mcplib.WithNumber("limit", mcplib.Description("Maximum results (default 20)")),
		),
		s.handleSearchItems,
	)
	s.addTool(
		mcplib.NewTool("backlogit_move_item",
			mcplib.WithDescription("Change an artifact's status"),
			mcplib.WithString("id", mcplib.Required(), mcplib.Description("Artifact ID")),
			mcplib.WithString("status", mcplib.Required(), mcplib.Description("New status")),
			mcplib.WithString("commit_sha", mcplib.Description("Optional git commit SHA for event traceability")),
		),
		s.handleMoveItem,
	)
	s.addTool(
		mcplib.NewTool("backlogit_delete_item",
			mcplib.WithDescription("Delete an artifact by ID"),
			mcplib.WithString("id", mcplib.Required(), mcplib.Description("Artifact ID")),
		),
		s.handleDeleteItem,
	)
	s.addTool(
		mcplib.NewTool("backlogit_query_sql",
			mcplib.WithDescription("Execute a read-only SQL query against the backlogit index"),
			mcplib.WithString("sql", mcplib.Required(), mcplib.Description("SELECT statement to execute")),
		),
		s.handleQuerySQL,
	)
	s.addTool(
		mcplib.NewTool("backlogit_sync_index",
			mcplib.WithDescription("Rehydrate the SQLite index from Markdown source files"),
		),
		s.handleSyncIndex,
	)
	s.addTool(
		mcplib.NewTool("backlogit_append_comment",
			mcplib.WithDescription("Append a comment event to the item's JSONL log"),
			mcplib.WithString("item_id", mcplib.Required(), mcplib.Description("Item ID")),
			mcplib.WithString("actor", mcplib.Required(), mcplib.Description("Actor name")),
			mcplib.WithString("comment", mcplib.Required(), mcplib.Description("Comment text")),
			mcplib.WithString("commit_sha", mcplib.Description("Optional git commit SHA for event traceability")),
		),
		s.handleAppendComment,
	)
	s.addTool(
		mcplib.NewTool("backlogit_log_telemetry",
			mcplib.WithDescription("Write agent telemetry to telemetry.jsonl"),
			mcplib.WithString("event_type", mcplib.Required(), mcplib.Description("Telemetry event type")),
		),
		s.handleLogTelemetry,
	)
	s.addTool(
		mcplib.NewTool("backlogit_save_memory",
			mcplib.WithDescription("Save a key-value pair to agent memories"),
			mcplib.WithString("key", mcplib.Required(), mcplib.Description("Memory key")),
			mcplib.WithString("summary", mcplib.Required(), mcplib.Description("Memory summary")),
		),
		s.handleSaveMemory,
	)
	s.addTool(
		mcplib.NewTool("backlogit_create_checkpoint",
			mcplib.WithDescription("Save a session state checkpoint"),
			mcplib.WithString("state_dump", mcplib.Required(), mcplib.Description("JSON state dump to persist")),
		),
		s.handleCreateCheckpoint,
	)
	s.addTool(
		mcplib.NewTool("backlogit_list_checkpoints",
			mcplib.WithDescription("List session state checkpoints with optional filters"),
			mcplib.WithString("consumer_id", mcplib.Description("Filter by consumer/agent ID")),
			mcplib.WithString("status", mcplib.Description("Filter by status (active, resolved)")),
			mcplib.WithString("shipment_id", mcplib.Description("Filter by shipment ID")),
			mcplib.WithString("feature_id", mcplib.Description("Filter by feature ID")),
			mcplib.WithNumber("max_age_hours", mcplib.Description("Maximum age in hours")),
		),
		s.handleListCheckpoints,
	)
	s.addTool(
		mcplib.NewTool("backlogit_get_checkpoint",
			mcplib.WithDescription("Get and validate a specific checkpoint by filename"),
			mcplib.WithString("filename", mcplib.Required(), mcplib.Description("Checkpoint filename (basename only)")),
		),
		s.handleGetCheckpoint,
	)
	s.addTool(
		mcplib.NewTool("backlogit_resolve_checkpoint",
			mcplib.WithDescription("Mark a checkpoint as resolved"),
			mcplib.WithString("filename", mcplib.Required(), mcplib.Description("Checkpoint filename (basename only)")),
		),
		s.handleResolveCheckpoint,
	)
	s.addTool(
		mcplib.NewTool("backlogit_cleanup_checkpoints",
			mcplib.WithDescription("Archive resolved and stale checkpoints based on retention policy"),
			mcplib.WithNumber("retention_days", mcplib.Description("Override retention days (defaults to config)")),
		),
		s.handleCleanupCheckpoints,
	)
	s.addTool(
		mcplib.NewTool("backlogit_get_wit_metadata",
			mcplib.WithDescription("Get complete WIT metadata for an artifact type including fields, sections, and relationships"),
			mcplib.WithString("type", mcplib.Required(), mcplib.Description("Artifact type (feature, task, subtask)")),
		),
		s.handleGetWITMetadata,
	)
	s.addTool(
		mcplib.NewTool("backlogit_list_types",
			mcplib.WithDescription("List all configured WIT types with hierarchy levels and descriptions"),
		),
		s.handleListTypes,
	)
	s.addTool(
		mcplib.NewTool("backlogit_get_metadata_catalog",
			mcplib.WithDescription("Get a unified workspace metadata catalog for agent discovery"),
		),
		s.handleGetMetadataCatalog,
	)
	s.addTool(
		mcplib.NewTool("backlogit_export_command_map",
			mcplib.WithDescription("Write an agent-readable command map file into the .backlogit/ workspace directory"),
			mcplib.WithString("path", mcplib.Required(), mcplib.Description("Workspace-relative output path")),
			mcplib.WithString("format", mcplib.Description("Output format: markdown or json"), mcplib.DefaultString("markdown")),
		),
		s.handleExportCommandMap,
	)
	s.addTool(
		mcplib.NewTool("backlogit_add_dependency",
			mcplib.WithDescription("Add a dependency between two artifacts with cycle detection"),
			mcplib.WithString("item_id", mcplib.Required(), mcplib.Description("Source artifact ID")),
			mcplib.WithString("depends_on", mcplib.Required(), mcplib.Description("Target artifact ID")),
			mcplib.WithString("dep_type", mcplib.Description("Dependency type: blocks, relates_to, parent_of"), mcplib.DefaultString("blocks")),
		),
		s.handleAddDependency,
	)
	s.addTool(
		mcplib.NewTool("backlogit_remove_dependency",
			mcplib.WithDescription("Remove a dependency between two artifacts"),
			mcplib.WithString("item_id", mcplib.Required(), mcplib.Description("Source artifact ID")),
			mcplib.WithString("depends_on", mcplib.Required(), mcplib.Description("Target artifact ID")),
		),
		s.handleRemoveDependency,
	)
	s.addTool(
		mcplib.NewTool("backlogit_get_dependencies",
			mcplib.WithDescription("Get dependency graph for an artifact including upstream and downstream edges"),
			mcplib.WithString("id", mcplib.Required(), mcplib.Description("Artifact ID")),
			mcplib.WithBoolean("reverse", mcplib.Description("Show items that depend on this item")),
		),
		s.handleGetDependencies,
	)
	s.addTool(
		mcplib.NewTool("backlogit_archive_item",
			mcplib.WithDescription("Archive a completed artifact to the archive directory"),
			mcplib.WithString("id", mcplib.Required(), mcplib.Description("Artifact ID to archive")),
			mcplib.WithString("commit_sha", mcplib.Description("Optional git commit SHA for event traceability")),
		),
		s.handleArchiveItem,
	)
	s.addTool(
		mcplib.NewTool("backlogit_get_queue",
			mcplib.WithDescription("Get prioritized work queue items respecting dependency constraints"),
			mcplib.WithString("type", mcplib.Description("Filter by artifact type")),
			mcplib.WithString("status", mcplib.Description("Filter by status")),
			mcplib.WithString("assigned_to", mcplib.Description("Filter by assignee")),
			mcplib.WithNumber("limit", mcplib.Description("Maximum results")),
			mcplib.WithNumber("offset", mcplib.Description("Result offset for pagination")),
			mcplib.WithString("group_by", mcplib.Description("Group output by field: type, status, priority")),
		),
		s.handleGetQueue,
	)
	s.addTool(
		mcplib.NewTool("backlogit_track_commit",
			mcplib.WithDescription("Associate a git commit SHA with an artifact for traceability"),
			mcplib.WithString("item_id", mcplib.Required(), mcplib.Description("Artifact ID")),
			mcplib.WithString("sha", mcplib.Required(), mcplib.Description("Git commit SHA")),
			mcplib.WithString("message", mcplib.Description("Commit message")),
			mcplib.WithString("author", mcplib.Description("Commit author")),
		),
		s.handleTrackCommit,
	)
	s.addTool(
		mcplib.NewTool("backlogit_fetch_stash",
			mcplib.WithDescription("Fetch the current active stash entries from .backlogit/stash.jsonl"),
			mcplib.WithString("priority", mcplib.Description("Optional stash priority filter (low, medium, high, critical)")),
			mcplib.WithString("kind", mcplib.Description("Optional stash kind filter (feature, task, bug, epic, unknown)")),
			mcplib.WithBoolean("group_by_priority", mcplib.Description("Group stash entries by priority")),
		),
		s.handleFetchStash,
	)
	s.addTool(
		mcplib.NewTool("backlogit_stash",
			mcplib.WithDescription("Add a deferred work item to the stash"),
			mcplib.WithString("kind", mcplib.DefaultString("task"), mcplib.Description("Stash kind (feature, task, bug, epic, unknown)")),
			mcplib.WithString("priority", mcplib.Description("Stash priority (low, medium, high, critical)"), mcplib.DefaultString("medium")),
			mcplib.WithString("text", mcplib.Required(), mcplib.Description("Stash item text")),
		),
		s.handleStash,
	)
	s.addTool(
		mcplib.NewTool("backlogit_harvest_stash",
			mcplib.WithDescription("Harvest a stash entry or all stash entries at a priority into backlogit work items"),
			mcplib.WithString("stash_id", mcplib.Description("Stash entry ID")),
			mcplib.WithString("priority", mcplib.Description("Harvest all stash entries at this priority (low, medium, high, critical)")),
			mcplib.WithString("artifact_type", mcplib.DefaultString("task"), mcplib.Description("Target artifact type (feature, task, subtask)")),
			mcplib.WithString("title", mcplib.Description("Override title for the harvested work item")),
			mcplib.WithString("description", mcplib.Description("Description for the harvested work item")),
			mcplib.WithString("status", mcplib.Description("Initial status"), mcplib.DefaultString("queued")),
			mcplib.WithString("parent_id", mcplib.Description("Optional parent artifact ID")),
		),
		s.handleHarvestStash,
	)
	s.addTool(
		mcplib.NewTool("backlogit_stash_get",
			mcplib.WithDescription("Get a single stash entry by ID"),
			mcplib.WithString("stash_id", mcplib.Required(), mcplib.Description("Stash entry ID")),
		),
		s.handleStashGet,
	)
	s.addTool(
		mcplib.NewTool("backlogit_stash_edit",
			mcplib.WithDescription("Edit a stash entry's text, kind, or priority"),
			mcplib.WithString("stash_id", mcplib.Required(), mcplib.Description("Stash entry ID")),
			mcplib.WithString("text", mcplib.Description("New stash item text")),
			mcplib.WithString("kind", mcplib.Description("New stash item kind (feature, task, bug, epic, unknown)")),
			mcplib.WithString("priority", mcplib.Description("New stash priority (low, medium, high, critical)")),
		),
		s.handleStashEdit,
	)
	s.addTool(
		mcplib.NewTool("backlogit_stash_remove",
			mcplib.WithDescription("Remove an active stash entry"),
			mcplib.WithString("stash_id", mcplib.Required(), mcplib.Description("Stash entry ID")),
		),
		s.handleStashRemove,
	)
	s.addTool(
		mcplib.NewTool("backlogit_deliberate",
			mcplib.WithDescription("Create a deliberation artifact linked to an active stash entry"),
			mcplib.WithString("stash_id", mcplib.Required(), mcplib.Description("Stash entry ID to deliberate")),
			mcplib.WithString("title", mcplib.Description("Deliberation title (defaults to stash text)")),
			mcplib.WithString("problem_frame", mcplib.Description("Problem frame content")),
			mcplib.WithString("options", mcplib.Description("Options or alternatives considered")),
			mcplib.WithString("chosen_direction", mcplib.Description("Chosen direction and rationale")),
			mcplib.WithString("open_questions", mcplib.Description("Outstanding questions or risks")),
			mcplib.WithString("notes", mcplib.Description("Supporting notes or research")),
		),
		s.handleDeliberate,
	)
	s.addTool(
		mcplib.NewTool("backlogit_create_shipment",
			mcplib.WithDescription("Create a new shipment artifact"),
			mcplib.WithString("title", mcplib.Required(), mcplib.Description("Shipment title")),
			mcplib.WithString("items", mcplib.Description("Optional comma-separated item IDs")),
		),
		s.handleCreateShipment,
	)
	s.addTool(
		mcplib.NewTool("backlogit_get_shipment",
			mcplib.WithDescription("Get a shipment by ID"),
			mcplib.WithString("id", mcplib.Required(), mcplib.Description("Shipment ID")),
		),
		s.handleGetShipment,
	)
	s.addTool(
		mcplib.NewTool("backlogit_list_shipments",
			mcplib.WithDescription("List shipments with an optional status filter"),
			mcplib.WithString("status", mcplib.Description("Optional shipment status filter")),
		),
		s.handleListShipments,
	)
	s.addTool(
		mcplib.NewTool("backlogit_claim_shipment",
			mcplib.WithDescription("Move a queued shipment to active"),
			mcplib.WithString("id", mcplib.Required(), mcplib.Description("Shipment ID")),
		),
		s.handleClaimShipment,
	)
	s.addTool(
		mcplib.NewTool("backlogit_ship_shipment",
			mcplib.WithDescription("Close a released shipment, archive the released scope, and record merge commit traceability"),
			mcplib.WithString("id", mcplib.Required(), mcplib.Description("Shipment ID")),
			mcplib.WithString("sha", mcplib.Description("Optional merge commit SHA to record on released artifacts")),
			mcplib.WithString("message", mcplib.Description("Optional merge commit message")),
			mcplib.WithString("author", mcplib.Description("Optional merge commit author")),
		),
		s.handleShipShipment,
	)
	s.addTool(
		mcplib.NewTool("backlogit_return_blocked",
			mcplib.WithDescription("Return a blocked item from a shipment"),
			mcplib.WithString("shipment_id", mcplib.Required(), mcplib.Description("Shipment ID")),
			mcplib.WithString("item_id", mcplib.Required(), mcplib.Description("Item ID")),
			mcplib.WithString("reason", mcplib.Required(), mcplib.Description("Reason the item is blocked")),
		),
		s.handleReturnBlocked,
	)
	s.addTool(
		mcplib.NewTool("backlogit_add_to_shipment",
			mcplib.WithDescription("Add an item to a shipment"),
			mcplib.WithString("shipment_id", mcplib.Required(), mcplib.Description("Shipment ID")),
			mcplib.WithString("item_id", mcplib.Required(), mcplib.Description("Item ID")),
		),
		s.handleAddToShipment,
	)
	s.addTool(
		mcplib.NewTool("backlogit_adopt_item",
			mcplib.WithDescription("Adopt an orphaned item under a new parent feature"),
			mcplib.WithString("item_id", mcplib.Required(), mcplib.Description("Item ID to adopt")),
			mcplib.WithString("new_parent_id", mcplib.Required(), mcplib.Description("New parent feature ID")),
		),
		s.handleAdoptItem,
	)
	s.addTool(
		mcplib.NewTool("backlogit_add_link",
			mcplib.WithDescription("Add a directed semantic link between two artifacts"),
			mcplib.WithString("source_id", mcplib.Required(), mcplib.Description("Source artifact ID")),
			mcplib.WithString("target_id", mcplib.Required(), mcplib.Description("Target artifact ID")),
			mcplib.WithString("link_type", mcplib.Required(), mcplib.Description("Link type: related_to, duplicate_of, informs, supersedes, spike_ref")),
		),
		s.handleAddLink,
	)
	s.addTool(
		mcplib.NewTool("backlogit_get_links",
			mcplib.WithDescription("Get all outgoing semantic links from an artifact"),
			mcplib.WithString("id", mcplib.Required(), mcplib.Description("Source artifact ID")),
			mcplib.WithString("link_type", mcplib.Description("Optional filter by link type")),
		),
		s.handleGetLinks,
	)
	s.addTool(
		mcplib.NewTool("backlogit_remove_link",
			mcplib.WithDescription("Remove a directed semantic link between two artifacts"),
			mcplib.WithString("source_id", mcplib.Required(), mcplib.Description("Source artifact ID")),
			mcplib.WithString("target_id", mcplib.Required(), mcplib.Description("Target artifact ID")),
			mcplib.WithString("link_type", mcplib.Required(), mcplib.Description("Link type to remove")),
		),
		s.handleRemoveLink,
	)
	s.addTool(
		mcplib.NewTool("backlogit_telemetry_harvest",
			mcplib.WithDescription("Parse Copilot CLI logs, correlate token usage by session and tool, write telemetry-sessions.jsonl, and rehydrate telemetry tables"),
			mcplib.WithString("copilot_path", mcplib.Description("Path to the .copilot directory (defaults to auto-detect)")),
			mcplib.WithString("since", mcplib.Description("Exclude log events before this RFC3339 timestamp (e.g. 2026-04-01T00:00:00Z). Defaults to the saved checkpoint.")),
			mcplib.WithBoolean("force", mcplib.Description("Re-process all logs from the beginning, ignoring the saved checkpoint")),
		),
		s.handleTelemetryHarvest,
	)
	s.addTool(
		mcplib.NewTool("backlogit_merge_sync",
			mcplib.WithDescription("Perform an incremental sync of the .backlogit workspace cache. Computes a diff against the in-memory manifest and applies targeted upserts/deletes. Falls back to full rehydration when the delta exceeds the threshold."),
			mcplib.WithBoolean("dry_run", mcplib.Description("When true, compute and return the diff without modifying the database")),
		),
		s.handleMergeSync,
	)
	s.addTool(
		mcplib.NewTool("backlogit_doctor",
			mcplib.WithDescription("Scan the workspace for structural integrity issues such as orphaned artifacts and duplicate IDs. Returns a DoctorReport with findings and checked_at timestamp."),
			mcplib.WithBoolean("check_orphans", mcplib.Description("Enable orphaned-artifact check (default true)")),
			mcplib.WithBoolean("check_duplicates", mcplib.Description("Enable duplicate-ID check (default true)")),
		),
		s.handleDoctor,
	)
	s.addTool(
		mcplib.NewTool("backlogit_get_version",
			mcplib.WithDescription("Return backlogit version, commit SHA, build date, and Go runtime version"),
		),
		s.handleGetVersion,
	)
	s.registerHookTools()
}

func (s *Server) handleListItems(ctx context.Context, request mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	if _, result := s.requireWorkspace(ctx); result != nil {
		return result, nil
	}
	filters := db.QueryFilters{}
	if v, ok := request.Params.Arguments["type"].(string); ok {
		filters.Type = v
	}
	if v, ok := request.Params.Arguments["status"].(string); ok {
		filters.Status = v
	}
	if v, ok := request.Params.Arguments["assigned_to"].(string); ok {
		filters.AssignedTo = v
	}
	if v, ok := request.Params.Arguments["sprint"].(string); ok {
		filters.Sprint = v
	}
	artifacts, err := db.QueryItems(ctx, s.Workspace.DB, filters)
	if err != nil {
		return InternalError(fmt.Sprintf("list items: %v", err)), nil
	}
	return toolResultJSON(artifacts)
}

func (s *Server) handleSearchItems(ctx context.Context, request mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	if _, result := s.requireWorkspace(ctx); result != nil {
		return result, nil
	}
	query, _ := request.Params.Arguments["query"].(string)
	if query == "" {
		return ValidationFailed("query is required"), nil
	}
	limit := 20
	if v, ok := request.Params.Arguments["limit"].(float64); ok && v > 0 {
		limit = int(v)
	}
	artifacts, err := db.SearchItems(ctx, s.Workspace.DB, query, limit)
	if err != nil {
		return InternalError(fmt.Sprintf("search items: %v", err)), nil
	}
	return toolResultJSON(artifacts)
}

func (s *Server) handleMoveItem(ctx context.Context, request mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	if _, result := s.requireWorkspace(ctx); result != nil {
		return result, nil
	}
	id, _ := request.Params.Arguments["id"].(string)
	if id == "" {
		return ValidationFailed("id is required"), nil
	}
	status, _ := request.Params.Arguments["status"].(string)
	if status == "" {
		return ValidationFailed("status is required"), nil
	}
	commitSHA, _ := request.Params.Arguments["commit_sha"].(string)

	// Capture previous status before the update for event delta consistency.
	var previousStatus string
	if commitSHA != "" {
		if existing, getErr := db.GetItem(ctx, s.Workspace.DB, id); getErr == nil {
			previousStatus = string(existing.Status)
		}
	}

	// Check that all children are in terminal statuses before allowing the
	// parent to move to a terminal status. This prevents orphaned in-progress
	// work from being silently buried under a "done" parent.
	terminalSet := make(map[string]bool, len(core.TerminalStatuses))
	for _, ts := range core.TerminalStatuses {
		terminalSet[ts] = true
	}
	if terminalSet[status] {
		if err := core.CheckChildrenTerminal(ctx, s.Workspace.DB, id); err != nil {
			var blockErr *core.ChildBlockingError
			if errors.As(err, &blockErr) {
				return blockingChildrenResult(blockErr.Children), nil
			}
			return InternalError(fmt.Sprintf("check children: %v", err)), nil
		}
	}

	artifact, err := core.UpdateArtifact(ctx, s.Workspace, id, map[string]any{"status": status})
	if err != nil {
		return domainError("move item", err), nil
	}

	// Belt-and-suspenders relocation: UpdateArtifact already relocates the file
	// via persistArtifact, but an explicit call ensures the file is in the
	// registry-mapped directory even if the registry was updated after the write.
	// RelocateArtifactFile is idempotent and validates path containment (F-13).
	// Failure is non-fatal — rehydration will recover consistency (F-8).
	if _, relocErr := core.RelocateArtifactFile(ctx, s.Workspace, artifact.ArtifactType, id, status); relocErr != nil {
		logger.WarnContext(ctx, "move item: file relocation check failed, rehydration will recover",
			"id", id, "status", status, "error", relocErr)
	}

	// Emit a status_changed event with commit traceability when commit_sha is provided.
	// Delta schema matches core setArtifactStatus pattern: {from, to, reason}.
	if commitSHA != "" {
		event := events.Event{
			Timestamp: time.Now(),
			Actor:     "backlogit",
			ItemID:    id,
			EventType: "status_changed",
			Delta: map[string]any{
				"from":   previousStatus,
				"to":     status,
				"reason": "move_item_with_commit_sha",
			},
			CommitSHA: commitSHA,
		}
		if appendErr := s.Events.AppendEvent(ctx, event); appendErr != nil {
			logger.Warn("move item: failed to append commit-traced event", "item_id", id, "commit_sha", commitSHA, "error", appendErr)
		} else if indexErr := db.IndexEvent(ctx, s.Workspace.DB, core.WorkspaceLogsRoot(s.Workspace.RootPath), event); indexErr != nil {
			logger.Warn("move item: failed to index commit-traced event", "item_id", id, "commit_sha", commitSHA, "error", indexErr)
		}
	}

	return toolResultJSON(artifact)
}

func (s *Server) handleDeleteItem(ctx context.Context, request mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	if _, result := s.requireWorkspace(ctx); result != nil {
		return result, nil
	}
	id, _ := request.Params.Arguments["id"].(string)
	if id == "" {
		return ValidationFailed("id is required"), nil
	}
	filePath, err := core.FindArtifactPath(ctx, s.Workspace, id)
	if err != nil {
		return domainError("find artifact", err), nil
	}
	// Delete the file first; only remove from index if file deletion succeeds.
	if err := os.Remove(filePath); err != nil {
		return InternalError(fmt.Sprintf("delete file: %v", err)), nil
	}
	if err := db.DeleteItemCascade(ctx, s.Workspace.DB, id); err != nil {
		return domainError("delete item", err), nil
	}
	return mcplib.NewToolResultText(`{"ok":true}`), nil
}

func (s *Server) handleGetItem(ctx context.Context, request mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	if _, result := s.requireWorkspace(ctx); result != nil {
		return result, nil
	}
	id, _ := request.Params.Arguments["id"].(string)
	if id == "" {
		return ValidationFailed("id is required"), nil
	}

	// If section param is provided, extract the named section from the file.
	if section, ok := request.Params.Arguments["section"].(string); ok && section != "" {
		filePath, err := core.FindArtifactPath(ctx, s.Workspace, id)
		if err != nil {
			return domainError("find artifact", err), nil
		}
		raw, err := os.ReadFile(filePath)
		if err != nil {
			return InternalError(fmt.Sprintf("read artifact: %v", err)), nil
		}
		_, body, err := models.ParseFrontmatter(string(raw))
		if err != nil {
			return InternalError(fmt.Sprintf("parse artifact: %v", err)), nil
		}
		sections, err := parser.ParseSections(body)
		if err != nil {
			return InternalError(fmt.Sprintf("parse sections: %v", err)), nil
		}
		content, ok := sections[section]
		if !ok {
			return ValidationFailed(fmt.Sprintf("section %q not found in artifact %s", section, id)), nil
		}
		return toolResultJSON(map[string]any{"id": id, "section": section, "content": content})
	}

	artifact, err := db.GetItem(ctx, s.Workspace.DB, id)
	if err != nil {
		return domainError("get item", err), nil
	}
	return toolResultJSON(artifact)
}

func (s *Server) handleCreateItem(ctx context.Context, request mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	if _, result := s.requireWorkspace(ctx); result != nil {
		return result, nil
	}
	title, _ := request.Params.Arguments["title"].(string)
	if title == "" {
		return ValidationFailed("title is required"), nil
	}
	artifactType, _ := request.Params.Arguments["artifact_type"].(string)
	if artifactType == "" {
		return ValidationFailed("artifact_type is required"), nil
	}
	var opts []core.Option
	if status, ok := request.Params.Arguments["status"].(string); ok && status != "" {
		opts = append(opts, core.WithStatus(status))
	}
	if desc, ok := request.Params.Arguments["description"].(string); ok && desc != "" {
		opts = append(opts, core.WithDescription(desc))
	}
	if parentID, ok := request.Params.Arguments["parent_id"].(string); ok && parentID != "" {
		opts = append(opts, core.WithParent(parentID))
	}
	if sprint, ok := request.Params.Arguments["sprint"].(string); ok && sprint != "" {
		opts = append(opts, core.WithSprint(sprint))
	}
	if v, ok := request.Params.Arguments["assigned_to"].(string); ok && v != "" {
		opts = append(opts, core.WithAssignedTo(v))
	}
	if v, ok := request.Params.Arguments["owner"].(string); ok && v != "" {
		opts = append(opts, core.WithOwner(v))
	}
	if v, ok := request.Params.Arguments["commit"].(string); ok && v != "" {
		opts = append(opts, core.WithCommit(v))
	}
	if v, ok := request.Params.Arguments["labels"].(string); ok && v != "" {
		opts = append(opts, core.WithLabels(strings.Split(v, ",")))
	}
	if v, ok := request.Params.Arguments["dependencies"].(string); ok && v != "" {
		opts = append(opts, core.WithDependencies(strings.Split(v, ",")))
	}
	if v, ok := request.Params.Arguments["references"].(string); ok && v != "" {
		opts = append(opts, core.WithReferences(strings.Split(v, ",")))
	}
	sections, sectionsErr := ParseSectionsParam(request.Params.Arguments)
	if sectionsErr != nil {
		return ValidationFailed(fmt.Sprintf("invalid sections param: %v", sectionsErr)), nil
	}
	artifact, err := core.CreateArtifact(ctx, s.Workspace, title, artifactType, opts...)
	if err != nil {
		return InternalError(fmt.Sprintf("create artifact: %v", err)), nil
	}

	// Write section content when sections are provided; independent of template service.
	if sections != nil {
		if writeErr := writeSectionsToFile(ctx, s.Workspace, artifact, sections); writeErr != nil {
			return InternalError(fmt.Sprintf("write sections: %v", writeErr)), nil
		}
	}

	return toolResultJSON(artifact)
}

func (s *Server) handleUpdateItem(ctx context.Context, request mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	if _, result := s.requireWorkspace(ctx); result != nil {
		return result, nil
	}
	id, _ := request.Params.Arguments["id"].(string)
	if id == "" {
		return ValidationFailed("id is required"), nil
	}
	updates := make(map[string]any)
	for _, key := range []string{"title", "status", "description", "sprint", "priority", "assigned_to", "owner", "commit"} {
		if v, ok := request.Params.Arguments[key].(string); ok && v != "" {
			updates[key] = v
		}
	}
	if v, ok := request.Params.Arguments["labels"].(string); ok && v != "" {
		updates["labels"] = strings.Split(v, ",")
	}
	sections, sectionsErr := ParseSectionsParam(request.Params.Arguments)
	if sectionsErr != nil {
		return ValidationFailed(fmt.Sprintf("invalid sections param: %v", sectionsErr)), nil
	}
	artifact, err := core.UpdateArtifact(ctx, s.Workspace, id, updates)
	if err != nil {
		return domainError("update artifact", err), nil
	}
	// Write section content when provided. UpdateArtifact already wrote the
	// frontmatter file and upserted the DB index; sections are markdown body
	// only and do not require a follow-up upsert.
	if sections != nil {
		if writeErr := writeSectionsToFile(ctx, s.Workspace, artifact, sections); writeErr != nil {
			return InternalError(fmt.Sprintf("write sections: %v", writeErr)), nil
		}
	}
	return toolResultJSON(artifact)
}

func (s *Server) handleQuerySQL(ctx context.Context, request mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	if _, result := s.requireWorkspace(ctx); result != nil {
		return result, nil
	}
	sqlStr, _ := request.Params.Arguments["sql"].(string)
	if sqlStr == "" {
		return ValidationFailed("sql is required"), nil
	}
	results, err := db.ExecuteGatedQuery(s.Workspace.DB, sqlStr)
	if err != nil {
		return InternalError(fmt.Sprintf("query: %v", err)), nil
	}
	data, err := json.Marshal(results)
	if err != nil {
		return InternalError(fmt.Sprintf("marshal results: %v", err)), nil
	}
	return mcplib.NewToolResultText(string(data)), nil
}

func (s *Server) handleSyncIndex(ctx context.Context, _ mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	if _, result := s.requireWorkspace(ctx); result != nil {
		return result, nil
	}
	count, err := db.Rehydrate(ctx, core.WorkspaceStorageRoot(s.Workspace.RootPath), s.Workspace.DB)
	if err != nil {
		return InternalError(fmt.Sprintf("sync index: %v", err)), nil
	}
	return toolResultJSON(map[string]any{"indexed": count})
}

func (s *Server) handleMergeSync(ctx context.Context, request mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	ws, result := s.requireWorkspace(ctx)
	if result != nil {
		return result, nil
	}

	dryRun, _ := request.Params.Arguments["dry_run"].(bool)

	s.manifestMu.RLock()
	oldManifest := s.manifest
	oldVersion := s.manifestVersion
	s.manifestMu.RUnlock()

	storageRoot := core.WorkspaceStorageRoot(ws.RootPath)
	syncResult, newManifest, err := db.MergeSync(ctx, storageRoot, ws.DB, oldManifest, dryRun)
	if err != nil {
		return InternalError(fmt.Sprintf("merge sync: %v", err)), nil
	}

	if !dryRun {
		s.manifestMu.Lock()
		// Only write when our snapshot has not been superseded by a concurrent
		// call. A lost CAS means another goroutine already wrote a newer manifest;
		// discard ours silently to avoid regressing the baseline.
		if s.manifestVersion == oldVersion {
			s.manifest = newManifest
			s.manifestVersion++
		}
		s.manifestMu.Unlock()
	}

	// Ensure nil slices marshal as [] rather than null.
	if syncResult.Added == nil {
		syncResult.Added = []db.SyncEntry{}
	}
	if syncResult.Changed == nil {
		syncResult.Changed = []db.SyncEntry{}
	}
	if syncResult.Deleted == nil {
		syncResult.Deleted = []db.SyncEntry{}
	}
	if syncResult.Relocated == nil {
		syncResult.Relocated = []db.SyncEntry{}
	}

	return toolResultJSON(syncResult)
}

func (s *Server) handleAppendComment(ctx context.Context, request mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	if _, result := s.requireWorkspace(ctx); result != nil {
		return result, nil
	}
	itemID, _ := request.Params.Arguments["item_id"].(string)
	if itemID == "" {
		return ValidationFailed("item_id is required"), nil
	}
	actor, _ := request.Params.Arguments["actor"].(string)
	comment, _ := request.Params.Arguments["comment"].(string)
	commitSHA, _ := request.Params.Arguments["commit_sha"].(string)
	event := events.Event{
		Actor:     actor,
		ItemID:    itemID,
		EventType: "comment",
		Delta:     map[string]any{"comment": comment},
		CommitSHA: commitSHA,
	}
	if err := s.Events.AppendEvent(ctx, event); err != nil {
		return InternalError(fmt.Sprintf("append comment: %v", err)), nil
	}
	if err := db.IndexEvent(ctx, s.Workspace.DB, core.WorkspaceLogsRoot(s.Workspace.RootPath), event); err != nil {
		return InternalError(fmt.Sprintf("index comment log: %v", err)), nil
	}
	return mcplib.NewToolResultText(`{"ok":true}`), nil
}

func (s *Server) handleLogTelemetry(ctx context.Context, request mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	if _, result := s.requireWorkspace(ctx); result != nil {
		return result, nil
	}
	eventType, _ := request.Params.Arguments["event_type"].(string)
	if eventType == "" {
		return ValidationFailed("event_type is required"), nil
	}
	payload := make(map[string]any)
	for k, v := range request.Params.Arguments {
		if k != "event_type" {
			payload[k] = v
		}
	}
	entry := events.TelemetryEntry{
		EventType: eventType,
		Payload:   payload,
	}
	if err := s.Telemetry.LogTelemetry(ctx, entry); err != nil {
		return InternalError(fmt.Sprintf("log telemetry: %v", err)), nil
	}
	return mcplib.NewToolResultText(`{"ok":true}`), nil
}

func (s *Server) handleSaveMemory(ctx context.Context, request mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	if _, result := s.requireWorkspace(ctx); result != nil {
		return result, nil
	}
	key, _ := request.Params.Arguments["key"].(string)
	if key == "" {
		return ValidationFailed("key is required"), nil
	}
	summary, _ := request.Params.Arguments["summary"].(string)
	memoriesPath := filepath.Join(s.Workspace.RootPath, ".backlogit", "memories.json")
	if err := events.SaveMemory(ctx, memoriesPath, key, summary); err != nil {
		return InternalError(fmt.Sprintf("save memory: %v", err)), nil
	}
	return mcplib.NewToolResultText(`{"ok":true}`), nil
}

func (s *Server) handleCreateCheckpoint(ctx context.Context, request mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	if _, result := s.requireWorkspace(ctx); result != nil {
		return result, nil
	}
	stateDump, _ := request.Params.Arguments["state_dump"].(string)
	if stateDump == "" {
		return ValidationFailed("state_dump is required"), nil
	}
	checkpointDir := filepath.Join(s.Workspace.RootPath, ".backlogit", "checkpoints")
	path, err := events.CreateCheckpoint(ctx, checkpointDir, stateDump)
	if err != nil {
		return domainError("create checkpoint", err), nil
	}
	return toolResultJSON(map[string]string{"path": path})
}

func (s *Server) handleListCheckpoints(ctx context.Context, request mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	if _, result := s.requireWorkspace(ctx); result != nil {
		return result, nil
	}
	checkpointDir := filepath.Join(s.Workspace.RootPath, ".backlogit", "checkpoints")

	filter := events.CheckpointFilter{}
	if v, ok := request.Params.Arguments["consumer_id"].(string); ok && v != "" {
		filter.Agent = v
	}
	if v, ok := request.Params.Arguments["status"].(string); ok && v != "" {
		filter.Status = v
	}
	if v, ok := request.Params.Arguments["shipment_id"].(string); ok && v != "" {
		filter.ShipmentID = v
	}
	if v, ok := request.Params.Arguments["feature_id"].(string); ok && v != "" {
		filter.FeatureID = v
	}
	if v, ok := request.Params.Arguments["max_age_hours"].(float64); ok && v > 0 {
		filter.MaxAge = time.Duration(v * float64(time.Hour))
	}

	summaries, err := events.ListCheckpoints(ctx, checkpointDir, filter)
	if err != nil {
		return InternalError(fmt.Sprintf("list checkpoints: %v", err)), nil
	}

	quarantined := 0
	for _, sm := range summaries {
		if sm.Quarantined {
			quarantined++
		}
	}

	return toolResultJSON(map[string]any{
		"checkpoints": summaries,
		"total":       len(summaries),
		"quarantined": quarantined,
	})
}

func (s *Server) handleGetCheckpoint(ctx context.Context, request mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	if _, result := s.requireWorkspace(ctx); result != nil {
		return result, nil
	}
	filename, _ := request.Params.Arguments["filename"].(string)
	if filename == "" {
		return ValidationFailed("filename is required"), nil
	}
	checkpointDir := filepath.Join(s.Workspace.RootPath, ".backlogit", "checkpoints")
	cp, err := events.GetCheckpoint(ctx, checkpointDir, filename)
	if err != nil {
		return domainError("get checkpoint", err), nil
	}
	return toolResultJSON(map[string]any{
		"checkpoint": cp,
		"filename":   filename,
		"valid":      true,
	})
}

func (s *Server) handleResolveCheckpoint(ctx context.Context, request mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	if _, result := s.requireWorkspace(ctx); result != nil {
		return result, nil
	}
	filename, _ := request.Params.Arguments["filename"].(string)
	if filename == "" {
		return ValidationFailed("filename is required"), nil
	}
	checkpointDir := filepath.Join(s.Workspace.RootPath, ".backlogit", "checkpoints")
	if err := events.ResolveCheckpoint(ctx, checkpointDir, filename); err != nil {
		return domainError("resolve checkpoint", err), nil
	}
	return toolResultJSON(map[string]any{
		"ok":          true,
		"filename":    filename,
		"status":      "resolved",
		"resolved_at": time.Now().UTC(),
	})
}

func (s *Server) handleCleanupCheckpoints(ctx context.Context, request mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	ws, result := s.requireWorkspace(ctx)
	if result != nil {
		return result, nil
	}
	checkpointDir := filepath.Join(ws.RootPath, ".backlogit", "checkpoints")

	retentionDays := 7 // default
	if v, ok := request.Params.Arguments["retention_days"].(float64); ok {
		if v < 1 || v != float64(int(v)) {
			return ValidationFailed("retention_days must be an integer >= 1"), nil
		}
		retentionDays = int(v)
	} else if ws.Config != nil && ws.Config.CheckpointRetention.RetentionDays > 0 {
		retentionDays = ws.Config.CheckpointRetention.RetentionDays
	}

	cleanupResult, err := events.CleanupCheckpoints(ctx, checkpointDir, retentionDays)
	if err != nil {
		return InternalError(fmt.Sprintf("cleanup checkpoints: %v", err)), nil
	}
	return toolResultJSON(cleanupResult)
}

// validateSectionName rejects section names that would produce malformed HTML
// comment markers or be unparseable by the section parser. Section names must
// be non-empty, contain no whitespace (the parser regex requires \S+), no
// "-->" sequences, and no newlines.
func validateSectionName(name string) error {
	if name == "" {
		return fmt.Errorf("section name must not be empty")
	}
	if strings.Contains(name, "-->") {
		return fmt.Errorf("section name %q contains invalid sequence \"-->\"", name)
	}
	if strings.ContainsAny(name, "\n\r") {
		return fmt.Errorf("section name %q must not contain newlines", name)
	}
	if strings.ContainsAny(name, " \t") {
		return fmt.Errorf("section name %q must not contain whitespace; the section parser requires contiguous non-whitespace names", name)
	}
	return nil
}

// writeSectionsToFile appends named section content to an artifact's Markdown body
// using BEGIN/END markers. It reads the existing file, processes each section
// individually to prevent duplication, and atomically rewrites the file.
func writeSectionsToFile(ctx context.Context, ws *core.Workspace, artifact *models.Artifact, sections map[string]string) error {
	// Validate all section names before any I/O.
	for name := range sections {
		if err := validateSectionName(name); err != nil {
			return err
		}
	}
	filePath, err := core.FindArtifactPath(ctx, ws, artifact.ID)
	if err != nil {
		return fmt.Errorf("find artifact path: %w", err)
	}
	raw, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("read artifact file: %w", err)
	}
	fm, body, err := models.ParseFrontmatter(string(raw))
	if err != nil {
		return fmt.Errorf("parse artifact: %w", err)
	}

	// Process each section individually to avoid the batch duplication bug:
	// if one section is missing and another exists, only the missing one should
	// be appended. Structural errors (not "section not found") propagate immediately.
	// Iterate in sorted key order to produce deterministic output across runs.
	sectionNames := make([]string, 0, len(sections))
	for name := range sections {
		sectionNames = append(sectionNames, name)
	}
	sort.Strings(sectionNames)
	for _, name := range sectionNames {
		value := sections[name]
		singleSection := map[string]string{name: value}
		updated, writeErr := parser.WriteSections(body, singleSection)
		if writeErr != nil {
			// Distinguish "not found" from structural errors. WriteSections returns
			// an error when the section markers are not present in the body.
			// For missing sections, append the markers. For other errors, propagate.
			if strings.Contains(writeErr.Error(), "not found") || strings.Contains(writeErr.Error(), "no section") {
				body += "\n\n<!-- BEGIN:" + name + " -->\n" + value + "\n<!-- END:" + name + " -->"
			} else {
				return fmt.Errorf("write section %q: %w", name, writeErr)
			}
		} else {
			body = updated
		}
	}

	newContent := models.SerializeFrontmatter(fm, body)
	tmp := filePath + ".tmp"
	if err := os.WriteFile(tmp, []byte(newContent), 0o644); err != nil {
		return fmt.Errorf("write artifact: %w", err)
	}
	if err := os.Rename(tmp, filePath); err != nil {
		os.Remove(tmp) //nolint:errcheck
		return fmt.Errorf("rename artifact: %w", err)
	}
	return nil
}

// queueLayout returns the configured QueueLayoutConfig for the workspace,
// falling back to a sensible default when none is configured.
func (s *Server) queueLayout() *core.QueueLayoutConfig {
	if s.Workspace != nil && s.Workspace.Config != nil && s.Workspace.Config.QueueLayout != nil {
		return s.Workspace.Config.QueueLayout
	}
	return &core.QueueLayoutConfig{
		RootDir: "queue",
		Levels: []core.HierarchyLevel{
			{Level: 1, Types: []string{"feature"}},
			{Level: 2, Types: []string{"task"}},
			{Level: 3, Types: []string{"subtask"}},
		},
	}
}

func (s *Server) handleGetWITMetadata(ctx context.Context, request mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	if _, result := s.requireWorkspace(ctx); result != nil {
		return result, nil
	}
	backlogitDir := s.backlogitDir()
	artifactType, _ := request.Params.Arguments["type"].(string)
	if artifactType == "" {
		return ValidationFailed("type is required"), nil
	}
	headerDef, err := config.LoadHeaderDef(backlogitDir)
	if err != nil {
		return InternalError(fmt.Sprintf("load header-def: %v", err)), nil
	}
	templates, err := config.LoadTemplates(filepath.Join(backlogitDir, "templates"))
	if err != nil {
		return InternalError(fmt.Sprintf("load templates: %v", err)), nil
	}
	layout := s.queueLayout()
	metadata, err := core.DescribeType(artifactType, headerDef, templates, layout)
	if err != nil {
		return InternalError(fmt.Sprintf("describe type: %v", err)), nil
	}
	return toolResultJSON(metadata)
}

func (s *Server) handleListTypes(ctx context.Context, _ mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	if _, result := s.requireWorkspace(ctx); result != nil {
		return result, nil
	}
	backlogitDir := s.backlogitDir()
	headerDef, err := config.LoadHeaderDef(backlogitDir)
	if err != nil {
		return InternalError(fmt.Sprintf("load header-def: %v", err)), nil
	}
	templates, err := config.LoadTemplates(filepath.Join(backlogitDir, "templates"))
	if err != nil {
		return InternalError(fmt.Sprintf("load templates: %v", err)), nil
	}
	layout := s.queueLayout()
	types, err := core.ListTypes(headerDef, templates, layout)
	if err != nil {
		return InternalError(fmt.Sprintf("list types: %v", err)), nil
	}
	return toolResultJSON(types)
}

func (s *Server) handleAddDependency(ctx context.Context, request mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	if _, result := s.requireWorkspace(ctx); result != nil {
		return result, nil
	}
	itemID, _ := request.Params.Arguments["item_id"].(string)
	if itemID == "" {
		return ValidationFailed("item_id is required"), nil
	}
	dependsOn, _ := request.Params.Arguments["depends_on"].(string)
	if dependsOn == "" {
		return ValidationFailed("depends_on is required"), nil
	}
	depType := "blocks"
	if v, ok := request.Params.Arguments["dep_type"].(string); ok && v != "" {
		depType = v
	}
	if err := db.AddDependencyChecked(ctx, s.Workspace.DB, itemID, dependsOn, depType); err != nil {
		return InternalError(fmt.Sprintf("add dependency: %v", err)), nil
	}
	return toolResultJSON(map[string]string{
		"item_id":    itemID,
		"depends_on": dependsOn,
		"dep_type":   depType,
		"status":     "added",
	})
}

func (s *Server) handleRemoveDependency(ctx context.Context, request mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	if _, result := s.requireWorkspace(ctx); result != nil {
		return result, nil
	}
	itemID, _ := request.Params.Arguments["item_id"].(string)
	if itemID == "" {
		return ValidationFailed("item_id is required"), nil
	}
	dependsOn, _ := request.Params.Arguments["depends_on"].(string)
	if dependsOn == "" {
		return ValidationFailed("depends_on is required"), nil
	}
	if err := db.DeleteDependency(ctx, s.Workspace.DB, itemID, dependsOn); err != nil {
		return InternalError(fmt.Sprintf("remove dependency: %v", err)), nil
	}
	return toolResultJSON(map[string]string{
		"item_id":    itemID,
		"depends_on": dependsOn,
		"status":     "removed",
	})
}

func (s *Server) handleGetDependencies(ctx context.Context, request mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	if _, result := s.requireWorkspace(ctx); result != nil {
		return result, nil
	}
	id, _ := request.Params.Arguments["id"].(string)
	if id == "" {
		return ValidationFailed("id is required"), nil
	}
	reverse, _ := request.Params.Arguments["reverse"].(bool)
	if reverse {
		edges, err := db.GetDependents(ctx, s.Workspace.DB, id)
		if err != nil {
			return InternalError(fmt.Sprintf("get dependents: %v", err)), nil
		}
		return toolResultJSON(edges)
	}
	edges, err := db.GetDependencies(ctx, s.Workspace.DB, id)
	if err != nil {
		return InternalError(fmt.Sprintf("get dependencies: %v", err)), nil
	}
	return toolResultJSON(edges)
}

func (s *Server) handleArchiveItem(ctx context.Context, request mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	if _, result := s.requireWorkspace(ctx); result != nil {
		return result, nil
	}
	id, _ := request.Params.Arguments["id"].(string)
	if id == "" {
		return ValidationFailed("id is required"), nil
	}
	var opts []core.ArchiveOpt
	if sha, _ := request.Params.Arguments["commit_sha"].(string); sha != "" {
		opts = append(opts, core.WithCommitSHA(sha))
	}
	record, err := core.ArchiveItem(ctx, s.Workspace.DB, s.Workspace, id, opts...)
	if err != nil {
		return domainError("archive item", err), nil
	}
	return toolResultJSON(record)
}

func (s *Server) handleGetQueue(ctx context.Context, request mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	if _, result := s.requireWorkspace(ctx); result != nil {
		return result, nil
	}
	filter := &core.QueueFilter{}
	if v, ok := request.Params.Arguments["type"].(string); ok && v != "" {
		filter.Types = []string{v}
	}
	if v, ok := request.Params.Arguments["status"].(string); ok && v != "" {
		filter.Statuses = []string{v}
	}
	if v, ok := request.Params.Arguments["assigned_to"].(string); ok && v != "" {
		filter.AssignedTo = v
	}
	if v, ok := request.Params.Arguments["limit"].(float64); ok && v > 0 {
		filter.Limit = int(v)
	}
	if v, ok := request.Params.Arguments["offset"].(float64); ok && v > 0 {
		filter.Offset = int(v)
	}
	if v, ok := request.Params.Arguments["group_by"].(string); ok && v != "" {
		filter.GroupBy = v
	}
	view, err := core.QueryQueue(ctx, s.Workspace.DB, filter)
	if err != nil {
		return InternalError(fmt.Sprintf("get queue: %v", err)), nil
	}
	return toolResultJSON(view)
}

func (s *Server) handleTrackCommit(ctx context.Context, request mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	if _, result := s.requireWorkspace(ctx); result != nil {
		return result, nil
	}
	itemID, _ := request.Params.Arguments["item_id"].(string)
	if itemID == "" {
		return ValidationFailed("item_id is required"), nil
	}
	sha, _ := request.Params.Arguments["sha"].(string)
	if sha == "" {
		return ValidationFailed("sha is required"), nil
	}
	message, _ := request.Params.Arguments["message"].(string)
	author, _ := request.Params.Arguments["author"].(string)
	if err := core.LinkCommit(ctx, s.Workspace.DB, s.Workspace, itemID, sha, message, author); err != nil {
		return InternalError(fmt.Sprintf("track commit: %v", err)), nil
	}
	return toolResultJSON(map[string]string{
		"item_id": itemID,
		"sha":     sha,
		"status":  "linked",
	})
}

func (s *Server) handleFetchStash(ctx context.Context, request mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	if _, result := s.requireWorkspace(ctx); result != nil {
		return result, nil
	}
	priority, _ := request.Params.Arguments["priority"].(string)
	kind, _ := request.Params.Arguments["kind"].(string)
	groupByPriority, _ := request.Params.Arguments["group_by_priority"].(bool)
	entries, err := core.FetchStash(ctx, s.Workspace, core.FetchStashOptions{
		Priority:        priority,
		Kind:            kind,
		GroupByPriority: groupByPriority,
	})
	if err != nil {
		return domainError("fetch stash", err), nil
	}
	return toolResultJSON(entries)
}

func (s *Server) handleStash(ctx context.Context, request mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	if _, result := s.requireWorkspace(ctx); result != nil {
		return result, nil
	}
	kind, _ := request.Params.Arguments["kind"].(string)
	if kind == "" {
		return ValidationFailed("kind is required"), nil
	}
	priority, _ := request.Params.Arguments["priority"].(string)
	text, _ := request.Params.Arguments["text"].(string)
	if text == "" {
		return ValidationFailed("text is required"), nil
	}
	entry, err := core.AddStashEntry(ctx, s.Workspace, kind, priority, text)
	if err != nil {
		return domainError("stash item", err), nil
	}
	return toolResultJSON(entry)
}

func (s *Server) handleHarvestStash(ctx context.Context, request mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	if _, result := s.requireWorkspace(ctx); result != nil {
		return result, nil
	}
	stashID, _ := request.Params.Arguments["stash_id"].(string)
	priority, _ := request.Params.Arguments["priority"].(string)
	artifactType, _ := request.Params.Arguments["artifact_type"].(string)
	if artifactType == "" {
		return ValidationFailed("artifact_type is required"), nil
	}
	if stashID == "" && priority == "" {
		return ValidationFailed("stash_id or priority is required"), nil
	}
	if stashID != "" && priority != "" {
		return ValidationFailed("stash_id and priority are mutually exclusive"), nil
	}
	title, _ := request.Params.Arguments["title"].(string)
	description, _ := request.Params.Arguments["description"].(string)
	status, _ := request.Params.Arguments["status"].(string)
	parentID, _ := request.Params.Arguments["parent_id"].(string)
	if priority != "" {
		result, err := core.HarvestStashByPriority(ctx, s.Workspace, core.HarvestStashOptions{
			Priority:     priority,
			ArtifactType: artifactType,
			Title:        title,
			Description:  description,
			Status:       status,
			ParentID:     parentID,
		})
		if err != nil {
			return domainError("harvest stash by priority", err), nil
		}
		return toolResultJSON(result)
	}
	result, err := core.HarvestStashEntry(ctx, s.Workspace, core.HarvestStashOptions{
		StashID:      stashID,
		ArtifactType: artifactType,
		Title:        title,
		Description:  description,
		Status:       status,
		ParentID:     parentID,
	})
	if err != nil {
		return domainError("harvest stash", err), nil
	}
	return toolResultJSON(result)
}

func (s *Server) handleStashGet(ctx context.Context, request mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	if _, result := s.requireWorkspace(ctx); result != nil {
		return result, nil
	}
	stashID, _ := request.Params.Arguments["stash_id"].(string)
	if stashID == "" {
		return ValidationFailed("stash_id is required"), nil
	}
	entry, err := core.GetStashEntry(ctx, s.Workspace, stashID)
	if err != nil {
		return domainError("get stash entry", err), nil
	}
	return toolResultJSON(entry)
}

func (s *Server) handleStashEdit(ctx context.Context, request mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	if _, result := s.requireWorkspace(ctx); result != nil {
		return result, nil
	}
	stashID, _ := request.Params.Arguments["stash_id"].(string)
	if stashID == "" {
		return ValidationFailed("stash_id is required"), nil
	}
	text, _ := request.Params.Arguments["text"].(string)
	kind, _ := request.Params.Arguments["kind"].(string)
	priority, _ := request.Params.Arguments["priority"].(string)
	if text == "" && kind == "" && priority == "" {
		return ValidationFailed("at least one of text, kind, or priority must be provided"), nil
	}
	entry, err := core.EditStashEntry(ctx, s.Workspace, stashID, core.EditStashOptions{
		Text:     text,
		Kind:     kind,
		Priority: priority,
	})
	if err != nil {
		return domainError("edit stash entry", err), nil
	}
	return toolResultJSON(entry)
}

func (s *Server) handleStashRemove(ctx context.Context, request mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	if _, result := s.requireWorkspace(ctx); result != nil {
		return result, nil
	}
	stashID, _ := request.Params.Arguments["stash_id"].(string)
	if stashID == "" {
		return ValidationFailed("stash_id is required"), nil
	}
	entry, err := core.RemoveStashEntry(ctx, s.Workspace, stashID)
	if err != nil {
		return domainError("remove stash entry", err), nil
	}
	return toolResultJSON(map[string]any{
		"id":     entry.ID,
		"status": "removed",
	})
}

func (s *Server) handleDeliberate(ctx context.Context, request mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	if _, result := s.requireWorkspace(ctx); result != nil {
		return result, nil
	}
	if s.templateSvc == nil {
		return InternalError("template service is unavailable"), nil
	}
	stashID, _ := request.Params.Arguments["stash_id"].(string)
	if stashID == "" {
		return ValidationFailed("stash_id is required"), nil
	}
	title, _ := request.Params.Arguments["title"].(string)
	problemFrame, _ := request.Params.Arguments["problem_frame"].(string)
	options, _ := request.Params.Arguments["options"].(string)
	chosenDirection, _ := request.Params.Arguments["chosen_direction"].(string)
	openQuestions, _ := request.Params.Arguments["open_questions"].(string)
	notes, _ := request.Params.Arguments["notes"].(string)

	result, err := templates.CreateDeliberationFromStash(ctx, s.Workspace, s.templateSvc, templates.DeliberationInput{
		StashID:         stashID,
		Title:           title,
		ProblemFrame:    problemFrame,
		Options:         options,
		ChosenDirection: chosenDirection,
		OpenQuestions:   openQuestions,
		Notes:           notes,
	})
	if err != nil {
		return domainError("create deliberation", err), nil
	}
	return toolResultJSON(result)
}

func (s *Server) handleCreateShipment(ctx context.Context, request mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	if _, result := s.requireWorkspace(ctx); result != nil {
		return result, nil
	}

	title, _ := request.Params.Arguments["title"].(string)
	if title == "" {
		return ValidationFailed("title is required"), nil
	}

	items, _ := request.Params.Arguments["items"].(string)
	logger.Info("shipment tool invoked", "tool", "backlogit_create_shipment", "title", title)

	shipment, err := core.CreateShipment(ctx, s.Workspace, title, splitCommaSeparated(items))
	if err != nil {
		return domainError("create shipment", err), nil
	}
	return toolResultJSON(shipment)
}

func (s *Server) handleGetShipment(ctx context.Context, request mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	if _, result := s.requireWorkspace(ctx); result != nil {
		return result, nil
	}

	id, _ := request.Params.Arguments["id"].(string)
	if id == "" {
		return ValidationFailed("id is required"), nil
	}

	logger.Info("shipment tool invoked", "tool", "backlogit_get_shipment", "shipment_id", id)

	shipment, err := core.GetShipment(ctx, s.Workspace, id)
	if err != nil {
		return domainError("get shipment", err), nil
	}
	return toolResultJSON(shipment)
}

// normalizeShipmentItems ensures that the custom_fields["items"] field of a
// shipment artifact is always a []string, matching the canonical shape produced
// by core.GetShipment. Both []string and []any source values are handled, and a
// nil CustomFields map is initialised before writing.
func normalizeShipmentItems(shipment *models.Artifact) {
	if shipment.CustomFields == nil {
		shipment.CustomFields = map[string]any{}
	}
	raw, ok := shipment.CustomFields["items"]
	if !ok || raw == nil {
		shipment.CustomFields["items"] = []string{}
		return
	}
	switch items := raw.(type) {
	case []string:
		// Already the correct type — clone to avoid aliasing.
		out := make([]string, len(items))
		copy(out, items)
		shipment.CustomFields["items"] = out
	case []any:
		out := make([]string, 0, len(items))
		for _, item := range items {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		shipment.CustomFields["items"] = out
	default:
		shipment.CustomFields["items"] = []string{}
	}
}

func (s *Server) handleListShipments(ctx context.Context, request mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	if _, result := s.requireWorkspace(ctx); result != nil {
		return result, nil
	}

	status, _ := request.Params.Arguments["status"].(string)
	logger.Info("shipment tool invoked", "tool", "backlogit_list_shipments", "status", status)

	shipments, err := db.QueryItems(ctx, s.Workspace.DB, db.QueryFilters{
		Type:   "shipment",
		Status: status,
	})
	if err != nil {
		return domainError("list shipments", err), nil
	}
	for _, shipment := range shipments {
		normalizeShipmentItems(shipment)
	}
	return toolResultJSON(shipments)
}

func (s *Server) handleClaimShipment(ctx context.Context, request mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	if _, result := s.requireWorkspace(ctx); result != nil {
		return result, nil
	}

	id, _ := request.Params.Arguments["id"].(string)
	if id == "" {
		return ValidationFailed("id is required"), nil
	}

	logger.Info("shipment tool invoked", "tool", "backlogit_claim_shipment", "shipment_id", id)

	shipment, err := core.ClaimShipment(ctx, s.Workspace, id)
	if err != nil {
		return domainError("claim shipment", err), nil
	}
	return toolResultJSON(shipment)
}

func (s *Server) handleShipShipment(ctx context.Context, request mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	if _, result := s.requireWorkspace(ctx); result != nil {
		return result, nil
	}

	id, _ := request.Params.Arguments["id"].(string)
	if id == "" {
		return ValidationFailed("id is required"), nil
	}
	sha, _ := request.Params.Arguments["sha"].(string)
	message, _ := request.Params.Arguments["message"].(string)
	author, _ := request.Params.Arguments["author"].(string)

	logger.Info("shipment tool invoked", "tool", "backlogit_ship_shipment", "shipment_id", id)

	result, err := core.ShipShipment(ctx, s.Workspace, id, &core.CommitMetadata{
		SHA:     sha,
		Message: message,
		Author:  author,
	})
	if err != nil {
		return domainError("ship shipment", err), nil
	}
	return toolResultJSON(result)
}

func (s *Server) handleReturnBlocked(ctx context.Context, request mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	if _, result := s.requireWorkspace(ctx); result != nil {
		return result, nil
	}

	shipmentID, _ := request.Params.Arguments["shipment_id"].(string)
	if shipmentID == "" {
		return ValidationFailed("shipment_id is required"), nil
	}
	itemID, _ := request.Params.Arguments["item_id"].(string)
	if itemID == "" {
		return ValidationFailed("item_id is required"), nil
	}
	reason, _ := request.Params.Arguments["reason"].(string)
	if reason == "" {
		return ValidationFailed("reason is required"), nil
	}

	logger.Info(
		"shipment tool invoked",
		"tool", "backlogit_return_blocked",
		"shipment_id", shipmentID,
		"item_id", itemID,
	)

	if err := core.ReturnBlockedItem(ctx, s.Workspace, shipmentID, itemID, reason); err != nil {
		return domainError("return blocked item", err), nil
	}
	return toolResultJSON(map[string]any{
		"shipment_id": shipmentID,
		"item_id":     itemID,
		"item_status": "blocked",
		"reason":      reason,
	})
}

func (s *Server) handleAddToShipment(ctx context.Context, request mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	if _, result := s.requireWorkspace(ctx); result != nil {
		return result, nil
	}

	shipmentID, _ := request.Params.Arguments["shipment_id"].(string)
	if shipmentID == "" {
		return ValidationFailed("shipment_id is required"), nil
	}
	itemID, _ := request.Params.Arguments["item_id"].(string)
	if itemID == "" {
		return ValidationFailed("item_id is required"), nil
	}

	logger.Info(
		"shipment tool invoked",
		"tool", "backlogit_add_to_shipment",
		"shipment_id", shipmentID,
		"item_id", itemID,
	)

	if err := core.AddItemToShipment(ctx, s.Workspace, shipmentID, itemID); err != nil {
		return domainError("add item to shipment", err), nil
	}
	return toolResultJSON(map[string]any{
		"shipment_id": shipmentID,
		"item_id":     itemID,
		"status":      "added",
	})
}

func splitCommaSeparated(value string) []string {
	if value == "" {
		return nil
	}

	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			result = append(result, part)
		}
	}
	return result
}

func (s *Server) handleAdoptItem(ctx context.Context, request mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	if _, result := s.requireWorkspace(ctx); result != nil {
		return result, nil
	}

	itemID, _ := request.Params.Arguments["item_id"].(string)
	if itemID == "" {
		return ValidationFailed("item_id is required"), nil
	}
	newParentID, _ := request.Params.Arguments["new_parent_id"].(string)
	if newParentID == "" {
		return ValidationFailed("new_parent_id is required"), nil
	}

	logger.Info("adopt tool invoked", "tool", "backlogit_adopt_item", "item_id", itemID, "new_parent_id", newParentID)

	result, err := core.AdoptItem(ctx, s.Workspace, itemID, newParentID)
	if err != nil {
		return domainError("adopt item", err), nil
	}
	return toolResultJSON(result)
}

// handleTelemetryHarvest implements the backlogit_telemetry_harvest MCP tool.
// It parses Copilot CLI process logs and session events, correlates them into
// per-session summaries, writes telemetry-sessions.jsonl, and rehydrates the
// telemetry tables in the SQLite index.
func (s *Server) handleTelemetryHarvest(ctx context.Context, request mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	ws, result := s.requireWorkspace(ctx)
	if result != nil {
		return result, nil
	}

	copilotPath, _ := request.Params.Arguments["copilot_path"].(string)
	if copilotPath == "" {
		copilotPath = filepath.Join(ws.RootPath, ".copilot")
	}

	// Parse optional since / force params.
	opts := telemetry.HarvestOptions{}
	if sinceStr, _ := request.Params.Arguments["since"].(string); sinceStr != "" {
		t, err := time.Parse(time.RFC3339, sinceStr)
		if err != nil {
			return ValidationFailed(fmt.Sprintf("invalid since timestamp %q: %v", sinceStr, err)), nil
		}
		opts.Since = &t
	}
	if force, _ := request.Params.Arguments["force"].(bool); force {
		opts.Force = true
	}

	hr, err := telemetry.HarvestTelemetry(ctx, ws.RootPath, copilotPath, ws.DB, opts)
	if err != nil {
		if errors.Is(err, backlogiterrors.ErrTelemetrySourceMissing) {
			return ValidationFailed(fmt.Sprintf("telemetry source missing — run 'backlogit mcp' from a workspace that contains a .copilot directory: %v", err)), nil
		}
		return domainError("harvest telemetry", err), nil
	}

	return toolResultJSON(map[string]any{
		"sessions_harvested": hr.SessionsHarvested,
		"tool_calls_indexed": hr.ToolCallsIndexed,
		"total_tokens":       hr.TotalTokens,
	})
}

// handleDoctor implements the backlogit_doctor MCP tool.
// It scans the workspace for structural integrity issues (orphaned artifacts,
// duplicate IDs) and returns a compact JSON DoctorReport.
func (s *Server) handleDoctor(ctx context.Context, request mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	ws, result := s.requireWorkspace(ctx)
	if result != nil {
		return result, nil
	}

	checkOrphans := true
	checkDuplicates := true
	if v, ok := request.Params.Arguments["check_orphans"].(bool); ok {
		checkOrphans = v
	}
	if v, ok := request.Params.Arguments["check_duplicates"].(bool); ok {
		checkDuplicates = v
	}

	opts := &core.DoctorOptions{
		CheckOrphans:    checkOrphans,
		CheckDuplicates: checkDuplicates,
	}
	report, err := core.Doctor(ctx, ws, opts)
	if err != nil {
		return InternalError(fmt.Sprintf("doctor: %v", err)), nil
	}
	return toolResultJSON(report)
}
