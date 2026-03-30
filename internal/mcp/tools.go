package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"

	mcplib "github.com/mark3labs/mcp-go/mcp"

	"github.com/backlogit/backlogit/internal/core"
	"github.com/backlogit/backlogit/internal/db"
	"github.com/backlogit/backlogit/internal/events"
)

// RegisterTools adds all backlogit tools to the MCP server.
func (s *Server) RegisterTools() {
	s.mcp.AddTool(
		mcplib.NewTool("backlogit_get_item",
			mcplib.WithDescription("Get a backlogit item by ID"),
			mcplib.WithString("id", mcplib.Required(), mcplib.Description("Item ID")),
		),
		s.handleGetItem,
	)
	s.mcp.AddTool(
		mcplib.NewTool("backlogit_create_item",
			mcplib.WithDescription("Create a new backlogit artifact"),
			mcplib.WithString("title", mcplib.Required(), mcplib.Description("Artifact title")),
			mcplib.WithString("artifact_type", mcplib.Required(), mcplib.Description("Artifact type (task, story, bug, epic)")),
			mcplib.WithString("status", mcplib.Description("Initial status"), mcplib.DefaultString("queued")),
			mcplib.WithString("description", mcplib.Description("Artifact description")),
			mcplib.WithString("parent_id", mcplib.Description("Parent artifact ID")),
			mcplib.WithString("sprint", mcplib.Description("Sprint ID")),
		),
		s.handleCreateItem,
	)
	s.mcp.AddTool(
		mcplib.NewTool("backlogit_update_item",
			mcplib.WithDescription("Update an existing backlogit artifact"),
			mcplib.WithString("id", mcplib.Required(), mcplib.Description("Item ID")),
			mcplib.WithString("title", mcplib.Description("New title")),
			mcplib.WithString("status", mcplib.Description("New status")),
			mcplib.WithString("description", mcplib.Description("New description")),
			mcplib.WithString("sprint", mcplib.Description("Sprint ID")),
			mcplib.WithString("priority", mcplib.Description("Priority")),
		),
		s.handleUpdateItem,
	)
	s.mcp.AddTool(
		mcplib.NewTool("backlogit_query_sql",
			mcplib.WithDescription("Execute a read-only SQL query against the backlogit index"),
			mcplib.WithString("sql", mcplib.Required(), mcplib.Description("SELECT statement to execute")),
		),
		s.handleQuerySQL,
	)
	s.mcp.AddTool(
		mcplib.NewTool("backlogit_sync_index",
			mcplib.WithDescription("Rehydrate the SQLite index from Markdown source files"),
		),
		s.handleSyncIndex,
	)
	s.mcp.AddTool(
		mcplib.NewTool("backlogit_append_comment",
			mcplib.WithDescription("Append a comment event to events.jsonl"),
			mcplib.WithString("item_id", mcplib.Required(), mcplib.Description("Item ID")),
			mcplib.WithString("actor", mcplib.Required(), mcplib.Description("Actor name")),
			mcplib.WithString("comment", mcplib.Required(), mcplib.Description("Comment text")),
		),
		s.handleAppendComment,
	)
	s.mcp.AddTool(
		mcplib.NewTool("backlogit_log_telemetry",
			mcplib.WithDescription("Write agent telemetry to telemetry.jsonl"),
			mcplib.WithString("event_type", mcplib.Required(), mcplib.Description("Telemetry event type")),
		),
		s.handleLogTelemetry,
	)
	s.mcp.AddTool(
		mcplib.NewTool("backlogit_save_memory",
			mcplib.WithDescription("Save a key-value pair to agent memories"),
			mcplib.WithString("key", mcplib.Required(), mcplib.Description("Memory key")),
			mcplib.WithString("summary", mcplib.Required(), mcplib.Description("Memory summary")),
		),
		s.handleSaveMemory,
	)
	s.mcp.AddTool(
		mcplib.NewTool("backlogit_create_checkpoint",
			mcplib.WithDescription("Save a session state checkpoint"),
			mcplib.WithString("state_dump", mcplib.Required(), mcplib.Description("JSON state dump to persist")),
		),
		s.handleCreateCheckpoint,
	)
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
	artifact, err := core.CreateArtifact(ctx, s.Workspace, title, artifactType, opts...)
	if err != nil {
		return InternalError(fmt.Sprintf("create artifact: %v", err)), nil
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
	for _, key := range []string{"title", "status", "description", "sprint", "priority"} {
		if v, ok := request.Params.Arguments[key].(string); ok && v != "" {
			updates[key] = v
		}
	}
	artifact, err := core.UpdateArtifact(ctx, s.Workspace, id, updates)
	if err != nil {
		return InternalError(fmt.Sprintf("update artifact: %v", err)), nil
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
