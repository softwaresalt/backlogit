package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	mcplib "github.com/mark3labs/mcp-go/mcp"

	"github.com/backlogit/backlogit/internal/config"
	"github.com/backlogit/backlogit/internal/core"
	"github.com/backlogit/backlogit/internal/db"
	"github.com/backlogit/backlogit/internal/events"
	"github.com/backlogit/backlogit/internal/models"
	"github.com/backlogit/backlogit/internal/parser"
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
			mcplib.WithString("artifact_type", mcplib.Required(), mcplib.Description("Artifact type (task, story, bug, epic)")),
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
			mcplib.WithDescription("Append a comment event to events.jsonl"),
			mcplib.WithString("item_id", mcplib.Required(), mcplib.Description("Item ID")),
			mcplib.WithString("actor", mcplib.Required(), mcplib.Description("Actor name")),
			mcplib.WithString("comment", mcplib.Required(), mcplib.Description("Comment text")),
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
		mcplib.NewTool("backlogit_get_wit_metadata",
			mcplib.WithDescription("Get complete WIT metadata for an artifact type including fields, sections, and relationships"),
			mcplib.WithString("type", mcplib.Required(), mcplib.Description("Artifact type (task, bug, epic, feature)")),
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
		),
		s.handleArchiveItem,
	)
	s.addTool(
		mcplib.NewTool("backlogit_get_queue",
			mcplib.WithDescription("Get prioritized work queue items respecting dependency constraints"),
			mcplib.WithString("type", mcplib.Description("Filter by artifact type")),
			mcplib.WithString("status", mcplib.Description("Filter by status")),
			mcplib.WithString("assignee", mcplib.Description("Filter by assignee")),
			mcplib.WithNumber("limit", mcplib.Description("Maximum results")),
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
}

func (s *Server) handleListItems(ctx context.Context, request mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	backlogitDir := filepath.Join(s.Workspace.RootPath, ".backlogit")
	if !dirExists(backlogitDir) {
		return WorkspaceNotInitialized(), nil
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
	backlogitDir := filepath.Join(s.Workspace.RootPath, ".backlogit")
	if !dirExists(backlogitDir) {
		return WorkspaceNotInitialized(), nil
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
	backlogitDir := filepath.Join(s.Workspace.RootPath, ".backlogit")
	if !dirExists(backlogitDir) {
		return WorkspaceNotInitialized(), nil
	}
	id, _ := request.Params.Arguments["id"].(string)
	if id == "" {
		return ValidationFailed("id is required"), nil
	}
	status, _ := request.Params.Arguments["status"].(string)
	if status == "" {
		return ValidationFailed("status is required"), nil
	}
	artifact, err := core.UpdateArtifact(ctx, s.Workspace, id, map[string]any{"status": status})
	if err != nil {
		return InternalError(fmt.Sprintf("move item: %v", err)), nil
	}
	filePath, err := core.FindArtifactPath(ctx, s.Workspace, id)
	if err != nil {
		return InternalError(fmt.Sprintf("find artifact: %v", err)), nil
	}
	if err := core.WriteArtifactFile(artifact, filePath); err != nil {
		return InternalError(fmt.Sprintf("write artifact: %v", err)), nil
	}
	if err := db.UpsertItem(ctx, s.Workspace.DB, artifact); err != nil {
		return InternalError(fmt.Sprintf("upsert item: %v", err)), nil
	}
	return toolResultJSON(artifact)
}

func (s *Server) handleDeleteItem(ctx context.Context, request mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	backlogitDir := filepath.Join(s.Workspace.RootPath, ".backlogit")
	if !dirExists(backlogitDir) {
		return WorkspaceNotInitialized(), nil
	}
	id, _ := request.Params.Arguments["id"].(string)
	if id == "" {
		return ValidationFailed("id is required"), nil
	}
	filePath, err := core.FindArtifactPath(ctx, s.Workspace, id)
	if err != nil {
		return InternalError(fmt.Sprintf("find artifact: %v", err)), nil
	}
	// Delete the file first; only remove from index if file deletion succeeds.
	if err := os.Remove(filePath); err != nil {
		return InternalError(fmt.Sprintf("delete file: %v", err)), nil
	}
	if err := db.DeleteItem(ctx, s.Workspace.DB, id); err != nil {
		return InternalError(fmt.Sprintf("delete from index: %v", err)), nil
	}
	return mcplib.NewToolResultText(`{"ok":true}`), nil
}

func (s *Server) handleGetItem(ctx context.Context, request mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	backlogitDir := filepath.Join(s.Workspace.RootPath, ".backlogit")
	if !dirExists(backlogitDir) {
		return WorkspaceNotInitialized(), nil
	}
	id, _ := request.Params.Arguments["id"].(string)
	if id == "" {
		return ValidationFailed("id is required"), nil
	}

	// If section param is provided, extract the named section from the file.
	if section, ok := request.Params.Arguments["section"].(string); ok && section != "" {
		filePath, err := core.FindArtifactPath(ctx, s.Workspace, id)
		if err != nil {
			return InternalError(fmt.Sprintf("find artifact: %v", err)), nil
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
		return InternalError(fmt.Sprintf("get item: %v", err)), nil
	}
	return toolResultJSON(artifact)
}

func (s *Server) handleCreateItem(ctx context.Context, request mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	backlogitDir := filepath.Join(s.Workspace.RootPath, ".backlogit")
	if !dirExists(backlogitDir) {
		return WorkspaceNotInitialized(), nil
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

	if err := db.UpsertItem(ctx, s.Workspace.DB, artifact); err != nil {
		return InternalError(fmt.Sprintf("index artifact: %v", err)), nil
	}
	return toolResultJSON(artifact)
}

func (s *Server) handleUpdateItem(ctx context.Context, request mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	backlogitDir := filepath.Join(s.Workspace.RootPath, ".backlogit")
	if !dirExists(backlogitDir) {
		return WorkspaceNotInitialized(), nil
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
		return InternalError(fmt.Sprintf("update artifact: %v", err)), nil
	}
	filePath, err := core.FindArtifactPath(ctx, s.Workspace, id)
	if err != nil {
		return InternalError(fmt.Sprintf("find artifact: %v", err)), nil
	}
	if err := core.WriteArtifactFile(artifact, filePath); err != nil {
		return InternalError(fmt.Sprintf("write artifact: %v", err)), nil
	}
	// Write section content when sections are provided.
	if sections != nil {
		if writeErr := writeSectionsToFile(ctx, s.Workspace, artifact, sections); writeErr != nil {
			return InternalError(fmt.Sprintf("write sections: %v", writeErr)), nil
		}
	}
	if err := db.UpsertItem(ctx, s.Workspace.DB, artifact); err != nil {
		return InternalError(fmt.Sprintf("upsert item: %v", err)), nil
	}
	return toolResultJSON(artifact)
}

func (s *Server) handleQuerySQL(_ context.Context, request mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	backlogitDir := filepath.Join(s.Workspace.RootPath, ".backlogit")
	if !dirExists(backlogitDir) {
		return WorkspaceNotInitialized(), nil
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
	backlogitDir := filepath.Join(s.Workspace.RootPath, ".backlogit")
	if !dirExists(backlogitDir) {
		return WorkspaceNotInitialized(), nil
	}
	count, err := db.Rehydrate(ctx, s.Workspace.RootPath, s.Workspace.DB)
	if err != nil {
		return InternalError(fmt.Sprintf("sync index: %v", err)), nil
	}
	return toolResultJSON(map[string]any{"indexed": count})
}

func (s *Server) handleAppendComment(ctx context.Context, request mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	backlogitDir := filepath.Join(s.Workspace.RootPath, ".backlogit")
	if !dirExists(backlogitDir) {
		return WorkspaceNotInitialized(), nil
	}
	itemID, _ := request.Params.Arguments["item_id"].(string)
	if itemID == "" {
		return ValidationFailed("item_id is required"), nil
	}
	actor, _ := request.Params.Arguments["actor"].(string)
	comment, _ := request.Params.Arguments["comment"].(string)
	event := events.Event{
		Actor:     actor,
		ItemID:    itemID,
		EventType: "comment",
		Delta:     map[string]any{"comment": comment},
	}
	if err := s.Events.AppendEvent(ctx, event); err != nil {
		return InternalError(fmt.Sprintf("append comment: %v", err)), nil
	}
	return mcplib.NewToolResultText(`{"ok":true}`), nil
}

func (s *Server) handleLogTelemetry(ctx context.Context, request mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	backlogitDir := filepath.Join(s.Workspace.RootPath, ".backlogit")
	if !dirExists(backlogitDir) {
		return WorkspaceNotInitialized(), nil
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
	backlogitDir := filepath.Join(s.Workspace.RootPath, ".backlogit")
	if !dirExists(backlogitDir) {
		return WorkspaceNotInitialized(), nil
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
	backlogitDir := filepath.Join(s.Workspace.RootPath, ".backlogit")
	if !dirExists(backlogitDir) {
		return WorkspaceNotInitialized(), nil
	}
	stateDump, _ := request.Params.Arguments["state_dump"].(string)
	if stateDump == "" {
		return ValidationFailed("state_dump is required"), nil
	}
	checkpointDir := filepath.Join(s.Workspace.RootPath, ".backlogit", "checkpoints")
	path, err := events.CreateCheckpoint(ctx, checkpointDir, stateDump)
	if err != nil {
		return InternalError(fmt.Sprintf("create checkpoint: %v", err)), nil
	}
	return toolResultJSON(map[string]string{"path": path})
}

// validateSectionName rejects section names that would produce malformed HTML comment markers.
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
	return nil
}

// writeSectionsToFile appends named section content to an artifact's Markdown body
// using BEGIN/END markers. It reads the existing file, appends each section that is
// not already present, and atomically rewrites the file.
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

	newBody, writeErr := parser.WriteSections(body, sections)
	if writeErr != nil {
		// Section tags not in file — append them.
		for name, value := range sections {
			body += "\n\n<!-- BEGIN:" + name + " -->\n" + value + "\n<!-- END:" + name + " -->"
		}
		newBody = body
	}

	newContent := models.SerializeFrontmatter(fm, newBody)
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
	if s.Workspace.Config != nil && s.Workspace.Config.QueueLayout != nil {
		return s.Workspace.Config.QueueLayout
	}
	return &core.QueueLayoutConfig{
		RootDir: "queue",
		Levels: []core.HierarchyLevel{
			{Level: 1, Types: []string{"feature", "epic"}},
			{Level: 2, Types: []string{"task", "story", "bug"}},
			{Level: 3, Types: []string{"sub-task"}},
		},
	}
}

func (s *Server) handleGetWITMetadata(ctx context.Context, request mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	backlogitDir := filepath.Join(s.Workspace.RootPath, ".backlogit")
	if !dirExists(backlogitDir) {
		return WorkspaceNotInitialized(), nil
	}
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
	backlogitDir := filepath.Join(s.Workspace.RootPath, ".backlogit")
	if !dirExists(backlogitDir) {
		return WorkspaceNotInitialized(), nil
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
	types, err := core.ListTypes(headerDef, templates, layout)
	if err != nil {
		return InternalError(fmt.Sprintf("list types: %v", err)), nil
	}
	return toolResultJSON(types)
}

func (s *Server) handleAddDependency(ctx context.Context, request mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	backlogitDir := filepath.Join(s.Workspace.RootPath, ".backlogit")
	if !dirExists(backlogitDir) {
		return WorkspaceNotInitialized(), nil
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
	backlogitDir := filepath.Join(s.Workspace.RootPath, ".backlogit")
	if !dirExists(backlogitDir) {
		return WorkspaceNotInitialized(), nil
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
	backlogitDir := filepath.Join(s.Workspace.RootPath, ".backlogit")
	if !dirExists(backlogitDir) {
		return WorkspaceNotInitialized(), nil
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
	backlogitDir := filepath.Join(s.Workspace.RootPath, ".backlogit")
	if !dirExists(backlogitDir) {
		return WorkspaceNotInitialized(), nil
	}
	id, _ := request.Params.Arguments["id"].(string)
	if id == "" {
		return ValidationFailed("id is required"), nil
	}
	record, err := core.ArchiveItem(ctx, s.Workspace.DB, s.Workspace, id)
	if err != nil {
		return InternalError(fmt.Sprintf("archive item: %v", err)), nil
	}
	return toolResultJSON(record)
}

func (s *Server) handleGetQueue(ctx context.Context, request mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	backlogitDir := filepath.Join(s.Workspace.RootPath, ".backlogit")
	if !dirExists(backlogitDir) {
		return WorkspaceNotInitialized(), nil
	}
	filter := &core.QueueFilter{}
	if v, ok := request.Params.Arguments["type"].(string); ok && v != "" {
		filter.Types = []string{v}
	}
	if v, ok := request.Params.Arguments["status"].(string); ok && v != "" {
		filter.Statuses = []string{v}
	}
	if v, ok := request.Params.Arguments["assignee"].(string); ok && v != "" {
		filter.AssignedTo = v
	}
	if v, ok := request.Params.Arguments["limit"].(float64); ok && v > 0 {
		filter.Limit = int(v)
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
	backlogitDir := filepath.Join(s.Workspace.RootPath, ".backlogit")
	if !dirExists(backlogitDir) {
		return WorkspaceNotInitialized(), nil
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
