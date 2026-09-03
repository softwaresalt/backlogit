package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
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
			mcplib.WithString("size", mcplib.Description("T-shirt size (XS, S, M, L, XL); body-preserving, mutually exclusive with other field updates")),
			mcplib.WithString("complexity", mcplib.Description("Optional task-only implementation difficulty/uncertainty (trivial, low, medium, high); distinct from size and priority; no provenance/history fields; explicit empty string clears; body-preserving and mutually exclusive with other field updates; does not affect queue ordering")),
			mcplib.WithString("size_source", mcplib.Description("Size provenance source (human, agent, derived)")),
			mcplib.WithString("size_ruleset_version", mcplib.Description("Size ruleset version")),
			mcplib.WithString("sections", mcplib.Description("Section updates as JSON object {name: content}")),
		),
		s.handleUpdateItem,
	)
	s.addTool(
		mcplib.NewTool("backlogit_list_items",
			mcplib.WithDescription("List artifacts with optional filters. Feature and shipment items include a computed-on-read size_composition rollup (size histogram, unsized count, and de-duplicated members)."),
			mcplib.WithString("type", mcplib.Description("Filter by artifact type")),
			mcplib.WithString("status", mcplib.Description("Filter by status")),
			mcplib.WithString("assigned_to", mcplib.Description("Filter by assignee")),
			mcplib.WithString("sprint", mcplib.Description("Filter by sprint ID")),
			mcplib.WithString("priority", mcplib.Description("Filter by priority")),
			mcplib.WithString("complexity", mcplib.Description("Filter task-only implementation difficulty/uncertainty (trivial, low, medium, high); distinct from size and priority; no default and does not affect queue ordering")),
			mcplib.WithString("owner", mcplib.Description("Filter by owner (distinct from assigned_to)")),
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
			mcplib.WithDescription("Save a session state checkpoint from a JSON state_dump. When state_dump "+
				"declares schema_version=1, the top level and the nested progress object are a CLOSED schema "+
				"namespace: the only legal top-level keys you may supply at create are schema_version, agent, "+
				"session_id, phase, status, created_at, updated_at, context, progress, and resume_hint, and the "+
				"only legal keys inside progress are tasks_completed, tasks_remaining, files_modified, and "+
				"decisions; any other key at either level fails the call as validation_failed with an "+
				"unknown_fields list naming the offending key path(s). The four disposition fields (disposition, "+
				"disposition_reason, disposition_operator, disposition_at) are part of the schema but are "+
				"RESERVED and administrative: they are set only by backlogit_abandon_checkpoint, never at "+
				"create, and supplying one here is rejected as an unknown field. status:\"abandoned\" is ALSO "+
				"rejected even with no disposition fields present, because backlogit_abandon_checkpoint is the "+
				"only governed path to that state; status:\"active\" and status:\"resolved\" remain accepted. "+
				"The context object is the OPEN "+
				"counterpart: shipment_id, feature_id, task_ids, and branch are modeled, but any other key you "+
				"supply there survives the create round-trip unchanged. A legacy state_dump (no schema_version, "+
				"or a value other than 1) is written verbatim with no schema validation. The successful result "+
				"reports context_keys: the exact list of context key names persisted to disk. "+
				"WRITE CONSTRAINTS (153.003-T / S1 U3): the state_dump must not exceed 64 KiB (validation_failed "+
				"on oversize); it must not contain a heuristically detected secret pattern such as a GitHub "+
				"token (ghp_, gho_, ghs_, ghu_, github_pat_, ghr_), AWS key (AKIA), OpenAI key (sk-), or "+
				"PEM-encoded key material (-----BEGIN). These constraints apply to both V1 and legacy paths "+
				"and are fail-closed: a matching dump is rejected without writing. Redact all secrets before "+
				"creating a checkpoint."),
			mcplib.WithString("state_dump", mcplib.Required(), mcplib.Description("JSON state dump to persist (max 64 KiB; must not contain secret material)")),
		),
		s.handleCreateCheckpoint,
	)
	s.addTool(
		mcplib.NewTool("backlogit_list_checkpoints",
			mcplib.WithDescription("List session state checkpoints with optional filters. "+
				"A summary with needs_quarantine true is not safely rewritable; use "+
				"backlogit_quarantine_checkpoint, not backlogit_resolve_checkpoint or "+
				"backlogit_abandon_checkpoint. Such a summary is returned regardless of the "+
				"status, agent, shipment_id, feature_id, and max_age filters, so a filtered "+
				"result can contain rows that do not match the filter. The accompanying "+
				"remediation_intent is a structured record of the required disposition, not "+
				"a runnable command."),
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
			mcplib.WithDescription("Get and validate a specific checkpoint by filename. "+
				"For a schema-valid document, returns conforming false when it carries "+
				"unmodeled top-level keys; such a document cannot be resolved or abandoned. "+
				"non_conforming_fields carries raw offender paths with explicit truncation "+
				"counts. A schema-invalid document is refused before any conformance verdict "+
				"is produced."),
			mcplib.WithString("filename", mcplib.Required(), mcplib.Description("Checkpoint filename (basename only)")),
		),
		s.handleGetCheckpoint,
	)
	s.addTool(
		mcplib.NewTool("backlogit_resolve_checkpoint",
			mcplib.WithDescription("Mark a checkpoint as resolved. Refuses a stored document it cannot safely "+
				"rewrite rather than replacing it: checkpoint_use_quarantine when the document is "+
				"schema-invalid, checkpoint_non_conforming when it carries unmodeled top-level keys. "+
				"Use backlogit_quarantine_checkpoint instead."),
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
		mcplib.NewTool("backlogit_abandon_checkpoint",
			mcplib.WithDescription("Administratively abandon a valid checkpoint. Refuses a malformed target; "+
				"use backlogit_quarantine_checkpoint instead. Also refuses when the document carries "+
				"unmodeled top-level keys."),
			mcplib.WithString("filename", mcplib.Required(), mcplib.Description("Checkpoint filename (basename only)")),
			mcplib.WithString("reason", mcplib.Required(), mcplib.Description("Reason for the disposition")),
			mcplib.WithString("operator", mcplib.Required(), mcplib.Description("Operator identity performing the disposition; never inferred")),
		),
		s.handleAbandonCheckpoint,
	)
	s.addTool(
		mcplib.NewTool("backlogit_quarantine_checkpoint",
			mcplib.WithDescription("Quarantine a checkpoint file that cannot be safely rewritten (malformed, "+
				"schema-invalid, or carrying unmodeled top-level keys) by moving its bytes verbatim to the "+
				"archive. Refuses a schema-valid, conforming target; use backlogit_abandon_checkpoint or "+
				"backlogit_resolve_checkpoint instead."),
			mcplib.WithString("filename", mcplib.Required(), mcplib.Description("Checkpoint filename (basename only)")),
			mcplib.WithString("reason", mcplib.Required(), mcplib.Description("Reason for the disposition")),
			mcplib.WithString("operator", mcplib.Required(), mcplib.Description("Operator identity performing the disposition; never inferred")),
		),
		s.handleQuarantineCheckpoint,
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
			mcplib.WithDescription("Add a dependency between two artifacts with cycle detection. "+
				"When both item_id and depends_on are shipments and dep_type is 'blocks' (default), "+
				"the call is routed through AddShipmentBlock which validates both endpoints are shipments, "+
				"creating a shipment-to-shipment sequencing edge: item_id must be completed after depends_on "+
				"(depends_on must ship before item_id). "+
				"Non-'blocks' dep_types and non-shipment endpoints use the generic path."),
			mcplib.WithString("item_id", mcplib.Required(), mcplib.Description("Source artifact ID (the dependent)")),
			mcplib.WithString("depends_on", mcplib.Required(), mcplib.Description("Target artifact ID (the prerequisite)")),
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
		mcplib.NewTool("backlogit_stash_archive",
			mcplib.WithDescription("Archive an active stash entry"),
			mcplib.WithString("stash_id", mcplib.Required(), mcplib.Description("Stash entry ID")),
		),
		s.handleStashArchive,
	)
	s.addTool(
		mcplib.NewTool("backlogit_stash_remove",
			mcplib.WithDescription("[Deprecated: use backlogit_stash_archive] Remove an active stash entry"),
			mcplib.WithString("stash_id", mcplib.Required(), mcplib.Description("Stash entry ID")),
		),
		s.handleStashArchive,
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
			mcplib.WithString("priority", mcplib.Description("Optional shipment priority (critical, high, medium, low)")),
		),
		s.handleCreateShipment,
	)
	s.addTool(
		mcplib.NewTool("backlogit_get_shipment",
			mcplib.WithDescription("Get a shipment by ID. The response includes a top-level, read-only covering_feature {id, title} derived at render time from the shipment manifest; it is omitted when the shipment has no root covering feature and is never persisted."),
			mcplib.WithString("id", mcplib.Required(), mcplib.Description("Shipment ID")),
		),
		s.handleGetShipment,
	)
	s.addTool(
		mcplib.NewTool("backlogit_list_shipments",
			mcplib.WithDescription("List shipments with an optional status filter. Each shipment includes the same top-level, read-only covering_feature {id, title} projection as get_shipment, derived at render time and omitted when absent."),
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
			mcplib.WithDescription("Scan the workspace for structural integrity issues such as orphaned artifacts and duplicate IDs. Use fix_orphans=true to archive orphaned artifacts automatically. Returns a DoctorReport with findings, fix_actions, and checked_at timestamp."),
			mcplib.WithBoolean("check_orphans", mcplib.Description("Enable orphaned-artifact check (default true)")),
			mcplib.WithBoolean("check_duplicates", mcplib.Description("Enable duplicate-ID check (default true)")),
			mcplib.WithBoolean("check_partial_mutations", mcplib.Description("Enable advisory detection of residual partial commit-association and dependency-linking state (default false)")),
			mcplib.WithBoolean("check_workspace_root_conflict", mcplib.Description("Enable read-only detection of a conflicting .backlog and .backlogit workspace root before workspace initialization (default false)")),
			mcplib.WithBoolean("check_shipped_event_completeness", mcplib.Description("Enable the read-only shipped-event reconciliation audit: archived shipments whose archived_status is shipped but whose item log carries no shipped event, and shipments left shipped but unarchived (default false)")),
			mcplib.WithBoolean("fix_orphans", mcplib.Description("Archive orphaned artifacts instead of just reporting them (default false)")),
			mcplib.WithString("target", mcplib.Description("Validate a single artifact file (path relative to the workspace, confined to the storage root) and return a versioned DoctorTargetResult instead of a full workspace scan")),
		),
		s.handleDoctor,
	)
	s.addTool(
		mcplib.NewTool("backlogit_get_version",
			mcplib.WithDescription("Return backlogit current version, latest release, update availability, commit SHA, build date, and Go runtime version. Performs a bounded remote latest-release check unless no_update_check or BACKLOGIT_NO_UPDATE_CHECK skips it."),
			mcplib.WithBoolean("no_update_check", mcplib.Description("Skip the remote latest-release check for this call")),
		),
		s.handleGetVersion,
	)
	s.registerHookTools()
	s.registerDocsTools()
	s.registerReconcileTools()
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
	if v, ok := request.Params.Arguments["priority"].(string); ok {
		filters.Priority = v
	}
	if raw, exists := request.Params.Arguments["complexity"]; exists {
		v, ok := raw.(string)
		if !ok {
			return ValidationFailed("complexity must be a string"), nil
		}
		if v != "" {
			if err := core.ValidateComplexityValue(s.Workspace, "task", v); err != nil {
				return domainError("list items", err), nil
			}
			filters.Complexity = v
		}
	}
	if v, ok := request.Params.Arguments["owner"].(string); ok {
		filters.Owner = v
	}
	artifacts, err := db.QueryItems(ctx, s.Workspace.DB, filters)
	if err != nil {
		return InternalError(fmt.Sprintf("list items: %v", err)), nil
	}
	// Route through the shared core shaper so list_items attaches the
	// computed-on-read size_composition rollup to aggregate rows at parity with
	// the CLI `list --json` surface (117-F / 60336CC0).
	return toolResultJSON(core.ListWithSizeComposition(ctx, s.Workspace, artifacts))
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
	if core.IsCascadeTerminalStatus(status) {
		if err := core.CheckChildrenTerminal(ctx, s.Workspace.DB, id); err != nil {
			var blockErr *core.ChildBlockingError
			if errors.As(err, &blockErr) {
				return blockingChildrenResult(blockErr.Children), nil
			}
			return InternalError(fmt.Sprintf("check children: %v", err)), nil
		}
	}

	artifact, outcome, err := core.UpdateArtifactWithGate(ctx, s.Workspace, id, map[string]any{"status": status}, core.TransitionOptions{})
	if err != nil {
		if result, handled := gateErrorResult(err, status); handled {
			return result, nil
		}
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
		logsDir := core.WorkspaceLogsRoot(s.Workspace.RootPath)
		lockedCtx, unlockLog, lockErr := events.LockItemLogCrossProcess(ctx, logsDir, id)
		if lockErr != nil {
			logger.Warn("move item: failed to lock commit-traced event log", "item_id", id, "commit_sha", commitSHA, "error", lockErr)
		} else {
			defer unlockLog()
			if appendErr := s.Events.AppendEvent(lockedCtx, event); appendErr != nil {
				logger.Warn("move item: failed to append commit-traced event", "item_id", id, "commit_sha", commitSHA, "error", appendErr)
			} else if indexErr := db.IndexEvent(lockedCtx, s.Workspace.DB, logsDir, event); indexErr != nil {
				logger.Warn("move item: failed to index commit-traced event", "item_id", id, "commit_sha", commitSHA, "error", indexErr)
			}
		}
	}

	if outcome != nil {
		return gatePassResult(artifact, outcome)
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
	// SE-6: project a never-persisted size_composition rollup onto feature and
	// shipment read surfaces so agents can read the aggregate without a separate call.
	if core.IsSizeCompositionAggregate(artifact.ArtifactType) {
		composition, cErr := core.SizeComposition(ctx, s.Workspace, artifact)
		if cErr != nil {
			slog.WarnContext(ctx, "get_item: size composition failed; returning artifact without rollup", "id", id, "error", cErr)
		} else if composition != nil {
			payload, pErr := withSizeComposition(artifact, composition)
			if pErr != nil {
				slog.WarnContext(ctx, "get_item: size composition projection failed; returning plain artifact", "id", id, "error", pErr)
			} else {
				return toolResultJSON(payload)
			}
		}
	}
	return toolResultJSON(artifact)
}

// withSizeComposition marshals any read-surface value into a generic map and
// attaches the computed-on-read size_composition rollup without mutating or
// persisting the underlying artifact. It is shared by the get_item, get_shipment,
// and get_queue MCP read surfaces (108-F SE-6). It delegates to
// core.AttachSizeComposition so the CLI and MCP transports cannot drift on the
// projection shape (114-F).
func withSizeComposition(v any, composition *core.SizeCompositionResult) (map[string]any, error) {
	return core.AttachSizeComposition(v, composition)
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
		return domainError("create artifact", err), nil
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
	for _, key := range []string{"title", "status", "description", "sprint", "priority", "assigned_to", "owner"} {
		if v, ok := request.Params.Arguments[key].(string); ok && v != "" {
			updates[key] = v
		}
	}
	// commit is handled separately via AssociateCommit for all-three-representation parity (F6/U3)
	commitSHA, _ := request.Params.Arguments["commit"].(string)
	if v, ok := request.Params.Arguments["labels"].(string); ok && v != "" {
		updates["labels"] = strings.Split(v, ",")
	}
	sections, sectionsErr := ParseSectionsParam(request.Params.Arguments)
	if sectionsErr != nil {
		return ValidationFailed(fmt.Sprintf("invalid sections param: %v", sectionsErr)), nil
	}
	if hasComplexityMutationArguments(request.Params.Arguments) {
		if len(updates) > 0 || sections != nil || hasSizeMutationArguments(request.Params.Arguments) || commitSHA != "" {
			return ValidationFailed("complexity cannot be combined with other field updates, sections, or size/provenance updates"), nil
		}
		complexity, ok := request.Params.Arguments["complexity"].(string)
		if !ok {
			return ValidationFailed("complexity must be a string"), nil
		}
		artifact, err := core.SetArtifactComplexity(ctx, s.Workspace, id, complexity)
		if err != nil {
			return domainError("set artifact complexity", err), nil
		}
		return toolResultJSON(artifact)
	}
	// size is a single-purpose, body-preserving mutation routed through
	// core.SetArtifactSize. It is mutually exclusive with generic field updates and
	// section writes, which go through the rebuild path; combining them would
	// double-write and negate body preservation.
	if hasSizeMutationArguments(request.Params.Arguments) {
		if len(updates) > 0 || sections != nil || commitSHA != "" {
			return ValidationFailed("size cannot be combined with other field updates or sections"), nil
		}
		size, _ := request.Params.Arguments["size"].(string)
		source, _ := request.Params.Arguments["size_source"].(string)
		rulesetVersion, _ := request.Params.Arguments["size_ruleset_version"].(string)
		// SE-5 trust boundary: an agent transport must not claim human provenance.
		// Reject an explicit size_source: human from the MCP surface regardless of
		// ruleset so masquerade cannot survive the transport boundary.
		if source == "human" {
			return ValidationFailed("size_source: human cannot be set from the MCP (agent) transport"), nil
		}
		var artifact *models.Artifact
		var err error
		if source != "" || rulesetVersion != "" {
			// Provenance-carrying mutations route through the typed seam
			// (SE-3a/SE-5); implemented in the build phase.
			mutation := core.SizeMutation{Actor: core.ActorContextAgent}
			if size != "" {
				mutation.Size = &size
			}
			if source != "" {
				mutation.Source = &source
			}
			if rulesetVersion != "" {
				mutation.RulesetVersion = &rulesetVersion
			}
			artifact, err = core.SetArtifactSizeWithProvenance(ctx, s.Workspace, id, mutation)
		} else {
			// Plain-size mutation over the MCP (agent) transport must attribute the
			// agent actor in the estimate_history event. Routing through the
			// human-hardcoded SetArtifactSize wrapper would forge human provenance
			// into the audit stream, defeating the SE-5 trust boundary above.
			artifact, err = core.SetArtifactSizeWithProvenance(ctx, s.Workspace, id, core.SizeMutation{Size: &size, Actor: core.ActorContextAgent})
		}
		if err != nil {
			return domainError("set artifact size", err), nil
		}
		return toolResultJSON(artifact)
	}

	// Skip UpdateArtifactWithGate when the only mutation is a commit association;
	// calling it with an empty updates map would bump updated_at and fire hooks spuriously.
	var (
		artifact *models.Artifact
		outcome  *core.GateOutcome
	)
	if len(updates) > 0 || sections != nil {
		requested, _ := updates["status"].(string)
		var updErr error
		artifact, outcome, updErr = core.UpdateArtifactWithGate(ctx, s.Workspace, id, updates, core.TransitionOptions{})
		if updErr != nil {
			if result, handled := gateErrorResult(updErr, requested); handled {
				return result, nil
			}
			return domainError("update artifact", updErr), nil
		}
		// Write section content when provided. UpdateArtifact already wrote the
		// frontmatter and upserted the DB with the prior body; writeSectionsToFile
		// rewrites the body and re-upserts so the DB/FTS reflect the new section content.
		if sections != nil {
			if writeErr := writeSectionsToFile(ctx, s.Workspace, artifact, sections); writeErr != nil {
				return InternalError(fmt.Sprintf("write sections: %v", writeErr)), nil
			}
		}
	}
	// Route commit association through governed-operation core (F6/U3).
	// message and author are unavailable in update_item; empty strings documented in governed-operation-parity.md.
	if commitSHA != "" {
		if assocErr := core.AssociateCommit(ctx, s.Workspace, s.Events, id, commitSHA, "", ""); assocErr != nil {
			return domainError("associate commit", assocErr), nil
		}
		// Reload the artifact from the DB so the response reflects the committed SHA.
		// AssociateCommit calls UpdateArtifact which upserts the DB; GetItem returns the fresh state.
		freshArt, freshErr := db.GetItem(ctx, s.Workspace.DB, id)
		if freshErr == nil {
			artifact = freshArt
		}
	}
	if outcome != nil {
		return gatePassResult(artifact, outcome)
	}
	if artifact == nil {
		// commit-only path with no gate: reload from DB for response.
		var loadErr error
		artifact, loadErr = db.GetItem(ctx, s.Workspace.DB, id)
		if loadErr != nil {
			return InternalError(fmt.Sprintf("reload artifact: %v", loadErr)), nil
		}
	}
	return toolResultJSON(artifact)
}

// hasSizeMutationArguments reports whether the request carries any reserved
// sizing argument. It detects PRESENCE (the key was supplied), not non-emptiness,
// so an explicit empty value (e.g. size="") still routes through the audited size
// seam and is rejected with a validation error — matching the CLI — rather than
// silently no-op'ing through the generic update path.
func hasSizeMutationArguments(args map[string]any) bool {
	for _, key := range []string{"size", "size_source", "size_ruleset_version"} {
		if _, ok := args[key]; ok {
			return true
		}
	}
	return false
}

func hasComplexityMutationArguments(args map[string]any) bool {
	_, ok := args["complexity"]
	return ok
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
	if _, migrateErr := core.MigrateDBOnlyLinksBeforeRehydrate(ctx, s.Workspace); migrateErr != nil {
		return InternalError(migrateErr.Error()), nil
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
	if !dryRun {
		if _, migrateErr := core.MigrateDBOnlyLinksBeforeRehydrate(ctx, ws); migrateErr != nil {
			return InternalError(migrateErr.Error()), nil
		}
	}

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

// appendCommentFn is the package-mcp seam wrapping core.AppendComment so that the
// durability outcome mapping and exactly-once retry tests are injectable from within
// package mcp without requiring the unexported events fsync seams.
//
// Must not run with t.Parallel: tests that swap this seam read on the production
// write path.
var appendCommentFn = core.AppendComment

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
	if err := appendCommentFn(ctx, s.Workspace, s.Events, itemID, actor, comment, commitSHA); err != nil {
		if result := durabilityOutcomeResult("append comment", err); result != nil {
			return result, nil
		}
		return InternalError(fmt.Sprintf("append comment: %v", err)), nil
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
	memoriesPath := filepath.Join(s.backlogitDir(), "memories.json")
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
	checkpointDir := filepath.Join(s.backlogitDir(), "checkpoints")
	result, err := events.CreateCheckpoint(ctx, checkpointDir, stateDump)
	if err != nil {
		return domainError("create checkpoint", err), nil
	}
	return toolResultJSON(map[string]any{"path": result.Path, "context_keys": result.ContextKeys})
}

func (s *Server) handleListCheckpoints(ctx context.Context, request mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	if _, result := s.requireWorkspace(ctx); result != nil {
		return result, nil
	}
	checkpointDir := filepath.Join(s.backlogitDir(), "checkpoints")

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
		// 153.001-T: use domainError so ErrCheckpointTargetUnsafe (symlinked
		// or unsafe checkpoint directory) maps to checkpoint_target_unsafe,
		// not internal_error.
		return domainError("list checkpoints", err), nil
	}

	// "quarantined" is fixed at 0: 136-F/U9 made listing strictly read-only,
	// so a mere list call never physically quarantines anything (the field
	// this key used to reflect, CheckpointSummary.Quarantined, is deprecated
	// and permanently false on this path). needsQuarantine is the accurate,
	// actionable signal for malformed checkpoints awaiting an explicit
	// backlogit_quarantine_checkpoint call.
	needsQuarantine := 0
	for _, sm := range summaries {
		if sm.NeedsQuarantine {
			needsQuarantine++
		}
	}

	return toolResultJSON(map[string]any{
		"checkpoints":      summaries,
		"total":            len(summaries),
		"quarantined":      0,
		"needs_quarantine": needsQuarantine,
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
	checkpointDir := filepath.Join(s.backlogitDir(), "checkpoints")
	// 147-F / U6c: project the conformance verdict from events.GetCheckpointResult
	// rather than the shipped events.GetCheckpoint. Schema-invalid documents keep
	// their existing refusal — GetCheckpointResult returns ErrCheckpointInvalid
	// unwrapped, so domainError still maps it to validation_failed, never a
	// disposition code (a read is not a rewrite).
	result, err := events.GetCheckpointResult(ctx, checkpointDir, filename)
	if err != nil {
		return domainError("get checkpoint", err), nil
	}
	return toolResultJSON(map[string]any{
		"checkpoint":            result.Checkpoint,
		"filename":              filename,
		"valid":                 result.Valid,
		"conforming":            result.Conforming,
		"needs_quarantine":      result.NeedsQuarantine,
		"remediation_intent":    result.RemediationIntent,
		"non_conforming_fields": result.NonConformingFields,
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
	checkpointDir := filepath.Join(s.backlogitDir(), "checkpoints")
	if err := events.ResolveCheckpoint(ctx, checkpointDir, filename); err != nil {
		// 147-F / U7d: route by class, not wholesale. The disposition-shaped
		// refusals (ErrCheckpointUseQuarantine from U3's validity gate,
		// ErrCheckpointNonConforming from the guarded seam) and the guarded
		// seam's TOCTOU refusal (ErrCheckpointContentChanged, added for the
		// rewrite compare-and-swap check) go through checkpointDispositionError
		// so `code`, `filename`, and `unknown_fields` are populated with
		// "resolve checkpoint" as the op, letting U7's op-derived remediation
		// name backlogit_resolve_checkpoint. Every other error (not found,
		// corrupt, cannot-resolve-abandoned, partial mutation) keeps its
		// existing domainError mapping.
		if backlogiterrors.QuarantineIsRemedy(err) || errors.Is(err, backlogiterrors.ErrCheckpointContentChanged) {
			return checkpointDispositionError("resolve checkpoint", filename, err), nil
		}
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
	checkpointDir := filepath.Join(s.backlogitDir(), "checkpoints")

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
		// 153.001-T: use domainError so ErrCheckpointTargetUnsafe maps properly.
		return domainError("cleanup checkpoints", err), nil
	}
	return toolResultJSON(cleanupResult)
}

func (s *Server) handleAbandonCheckpoint(ctx context.Context, request mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	ws, result := s.requireWorkspace(ctx)
	if result != nil {
		return result, nil
	}
	filename, _ := request.Params.Arguments["filename"].(string)
	if filename == "" {
		return ValidationFailed("filename is required"), nil
	}
	reason, _ := request.Params.Arguments["reason"].(string)
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return ValidationFailed("reason is required"), nil
	}
	operator, _ := request.Params.Arguments["operator"].(string)
	operator = strings.TrimSpace(operator)
	if operator == "" {
		return ValidationFailed("operator is required; it is never inferred on the MCP surface"), nil
	}

	if err := core.AbandonCheckpoint(ctx, ws, s.Events, filename, reason, operator); err != nil {
		return checkpointDispositionError("abandon checkpoint", filename, err), nil
	}
	return toolResultJSON(map[string]string{
		"filename":    filename,
		"disposition": events.DispositionAbandoned,
		"reason":      reason,
		"operator":    operator,
	})
}

func (s *Server) handleQuarantineCheckpoint(ctx context.Context, request mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	ws, result := s.requireWorkspace(ctx)
	if result != nil {
		return result, nil
	}
	filename, _ := request.Params.Arguments["filename"].(string)
	if filename == "" {
		return ValidationFailed("filename is required"), nil
	}
	reason, _ := request.Params.Arguments["reason"].(string)
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return ValidationFailed("reason is required"), nil
	}
	operator, _ := request.Params.Arguments["operator"].(string)
	operator = strings.TrimSpace(operator)
	if operator == "" {
		return ValidationFailed("operator is required; it is never inferred on the MCP surface"), nil
	}

	if err := core.QuarantineCheckpoint(ctx, ws, s.Events, filename, reason, operator); err != nil {
		return checkpointDispositionError("quarantine checkpoint", filename, err), nil
	}
	return toolResultJSON(map[string]string{
		"filename":    filename,
		"disposition": events.DispositionQuarantined,
		"reason":      reason,
		"operator":    operator,
	})
}

// writeSectionsToFile appends named section content to an artifact's Markdown body
// using BEGIN/END markers. It reads the existing file, processes each section
// individually to prevent duplication, and atomically rewrites the file.
func writeSectionsToFile(ctx context.Context, ws *core.Workspace, artifact *models.Artifact, sections map[string]string) error {
	// Validate all section names before any I/O.
	for name := range sections {
		if err := parser.ValidateSectionName(name); err != nil {
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
			// A genuinely absent section is appended; malformed markers (a
			// BEGIN with no matching END) are surfaced as an error so the write
			// never silently duplicates or masks corruption.
			if errors.Is(writeErr, parser.ErrSectionNotFound) {
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
		os.Remove(tmp) //nolint:errcheck
		return fmt.Errorf("write artifact: %w", err)
	}
	if err := os.Rename(tmp, filePath); err != nil {
		os.Remove(tmp) //nolint:errcheck
		return fmt.Errorf("rename artifact: %w", err)
	}

	// Re-parse the rewritten artifact so the in-memory copy, SQLite
	// (items.description), and the FTS index all reflect the new file body.
	updated, parseErr := models.ArtifactFromFrontmatter(fm, body)
	if parseErr != nil {
		return fmt.Errorf("parse artifact after section write: %w", parseErr)
	}
	// Propagate the rewritten body back to the caller's artifact so the tool
	// response (create-item / update-item) does not echo a stale description.
	artifact.Description = updated.Description
	if ws.DB != nil {
		if upsertErr := db.UpsertItem(ctx, ws.DB, updated); upsertErr != nil {
			return fmt.Errorf("sync index after section write: %w", upsertErr)
		}
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

	// Mirror the CLI routing: when source is a shipment and dep_type is
	// "blocks" (the default), route through AddShipmentBlock so the MCP
	// surface cannot bypass endpoint validation for that edge shape.
	if depType == "blocks" {
		itemArt, e1 := db.GetItem(ctx, s.Workspace.DB, itemID)
		if e1 != nil && !errors.Is(e1, backlogiterrors.ErrNotFound) {
			return InternalError(fmt.Sprintf("look up source artifact: %v", e1)), nil
		}
		if e1 == nil && itemArt.ArtifactType == "shipment" {
			if err := core.AddShipmentBlock(ctx, s.Workspace, itemID, dependsOn); err != nil {
				return domainError("add shipment block", err), nil
			}
			return toolResultJSON(map[string]string{
				"item_id":    itemID,
				"depends_on": dependsOn,
				"dep_type":   depType,
				"status":     "added",
			})
		}
	}

	if err := core.AddDependency(ctx, s.Workspace, itemID, dependsOn, depType); err != nil {
		return domainError("add dependency", err), nil
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
	if err := core.RemoveDependency(ctx, s.Workspace, itemID, dependsOn); err != nil {
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
	// SE-6: project the never-persisted size_composition rollup onto feature and
	// shipment queue items so agents can read aggregates inline. Nothing is
	// persisted; non-aggregate items are left untouched. Shares the core shaper
	// with the CLI `queue view --json` surface so the two cannot drift.
	projected, pErr := core.QueueViewWithSizeComposition(ctx, s.Workspace, view)
	if pErr != nil {
		slog.WarnContext(ctx, "get_queue: size composition projection failed; returning plain queue", "error", pErr)
		return toolResultJSON(view)
	}
	return toolResultJSON(projected)
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
	if err := core.AssociateCommit(ctx, s.Workspace, s.Events, itemID, sha, message, author); err != nil {
		// Check for structured partial mutation result first (F5 envelope) so
		// the completed/failed/compensation payload is not discarded by the
		// durability sentinel unwrap in durabilityOutcomeResult below.
		var partialErr *backlogiterrors.MutationPartialError
		if errors.As(err, &partialErr) {
			return mutationPartialError("track commit", partialErr), nil
		}
		if result := durabilityOutcomeResult("track commit", err); result != nil {
			return result, nil
		}
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

// handleStashArchive implements the backlogit_stash_archive MCP tool.
func (s *Server) handleStashArchive(ctx context.Context, request mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	if _, result := s.requireWorkspace(ctx); result != nil {
		return result, nil
	}
	stashID, _ := request.Params.Arguments["stash_id"].(string)
	if stashID == "" {
		return ValidationFailed("stash_id is required"), nil
	}
	entry, err := core.ArchiveStashEntry(ctx, s.Workspace, stashID)
	if err != nil {
		return domainError("archive stash entry", err), nil
	}
	return toolResultJSON(map[string]any{
		"id":     entry.ID,
		"status": "archived",
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
	priority, _ := request.Params.Arguments["priority"].(string)
	logger.Info("shipment tool invoked", "tool", "backlogit_create_shipment", "title", title)

	var opts []core.Option
	if priority != "" {
		opts = append(opts, core.WithPriority(priority))
	}
	shipment, err := core.CreateShipment(ctx, s.Workspace, title, splitCommaSeparated(items), opts...)
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
	// Wrap in the shared read-only shaper so the response carries the render-time
	// covering_feature projection and the computed-on-read size_composition
	// rollup. The CLI `shipment get` surface delegates to the same core shaper so
	// the two transports cannot drift (114-F parity). Purely additive: nothing is
	// persisted.
	return toolResultJSON(core.ShipmentViewWithComposition(ctx, s.Workspace, shipment))
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
		// The nil-map guard exists solely to provide an assignment target;
		// the never-null VALUE guarantee comes entirely from
		// core.NormalizeShipmentItems (the single source of truth).
		if shipment.CustomFields == nil {
			shipment.CustomFields = map[string]any{}
		}
		shipment.CustomFields["items"] = core.NormalizeShipmentItems(shipment)
	}
	// Share the same read-only shaper as the CLI `shipment list` surface so both
	// transports carry an identical covering_feature projection and
	// size_composition rollup and cannot drift (114-F parity).
	return toolResultJSON(core.ShipmentViewsWithComposition(ctx, s.Workspace, shipments))
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
		// Route through the shared gate-error dispatcher first (106-F F1/U8):
		// a shipment-level gate refusal (blocked/setup/config/timeout, or a
		// formal-gate-evidence refusal) deserves the same structured,
		// machine-readable treatment task-level completions already get,
		// rather than falling straight to the generic domainError.
		if result, handled := gateErrorResult(err, ""); handled {
			return result, nil
		}
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
	if ws.Config != nil && ws.Config.Telemetry != nil {
		opts.AttributionPrefixes = ws.Config.Telemetry.AttributionPrefixes
	}

	hr, err := telemetry.HarvestTelemetry(ctx, ws.StorageRoot, copilotPath, ws.DB, opts)
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
	checkOrphans := true
	checkDuplicates := true
	checkPartialMutations := false
	checkWorkspaceRootConflict := false
	checkShippedEventCompleteness := false
	fixOrphans := false
	if v, ok := request.Params.Arguments["check_orphans"].(bool); ok {
		checkOrphans = v
	}
	if v, ok := request.Params.Arguments["check_duplicates"].(bool); ok {
		checkDuplicates = v
	}
	if v, ok := request.Params.Arguments["check_partial_mutations"].(bool); ok {
		checkPartialMutations = v
	}
	if v, ok := request.Params.Arguments["check_workspace_root_conflict"].(bool); ok {
		checkWorkspaceRootConflict = v
	}
	if v, ok := request.Params.Arguments["check_shipped_event_completeness"].(bool); ok {
		checkShippedEventCompleteness = v
	}
	if v, ok := request.Params.Arguments["fix_orphans"].(bool); ok {
		fixOrphans = v
	}

	preflightFindings := []core.DoctorFinding(nil)
	if checkWorkspaceRootConflict {
		findings, err := core.CheckWorkspaceRootConflict(s.RootPath)
		if err != nil {
			return InternalError(fmt.Sprintf("doctor workspace root conflict: %v", err)), nil
		}
		preflightFindings = findings
	}

	ws, result := s.requireWorkspace(ctx)
	if result != nil {
		if len(preflightFindings) > 0 {
			return toolResultJSON(&core.DoctorReport{
				Findings:  preflightFindings,
				CheckedAt: time.Now().UTC(),
			})
		}
		return result, nil
	}

	// target mode: validate a single artifact file and return a structured,
	// versioned result (MCP surfaces the DoctorTargetResult, not a process exit
	// code — the exit-code table is the CLI's contract).
	if target, ok := request.Params.Arguments["target"].(string); ok && target != "" {
		res, err := core.DoctorTarget(ws, target)
		if err != nil {
			return InternalError(fmt.Sprintf("doctor target: %v", err)), nil
		}
		return toolResultJSON(res)
	}

	opts := &core.DoctorOptions{
		CheckOrphans:                  checkOrphans,
		CheckDuplicates:               checkDuplicates,
		CheckPartialMutations:         checkPartialMutations,
		CheckWorkspaceRootConflict:    checkWorkspaceRootConflict,
		CheckShippedEventCompleteness: checkShippedEventCompleteness,
		FixOrphans:                    fixOrphans,
	}
	report, err := core.Doctor(ctx, ws, opts)
	if err != nil {
		return InternalError(fmt.Sprintf("doctor: %v", err)), nil
	}
	return toolResultJSON(report)
}
