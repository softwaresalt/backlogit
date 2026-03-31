package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	mcplib "github.com/mark3labs/mcp-go/mcp"

	"github.com/backlogit/backlogit/internal/core/templates"
)

// RegisterSectionAwareTools registers the fixed MCP tools that support section-aware
// operations and template discovery. All tools are registered unconditionally regardless
// of workspace state, satisfying the constitutional mandate.
func RegisterSectionAwareTools(s *Server, templateSvc *templates.Service) {
	s.addTool(
		mcplib.NewTool("backlogit_list_templates",
			mcplib.WithDescription("List registered template types and their section definitions"),
		),
		func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
			return handleListTemplates(ctx, s, templateSvc)
		},
	)
}

// handleListTemplates handles the backlogit_list_templates MCP tool call.
func handleListTemplates(_ context.Context, _ *Server, templateSvc *templates.Service) (*mcplib.CallToolResult, error) {
	if templateSvc == nil {
		return mcplib.NewToolResultText("[]"), nil
	}
	infos := templateSvc.ListTemplates()
	data, err := json.Marshal(infos)
	if err != nil {
		return nil, fmt.Errorf("marshal templates: %w", err)
	}
	return mcplib.NewToolResultText(string(data)), nil
}

// ListTools returns the names of all registered MCP tools.
// Used by contract tests to verify the fixed tool surface.
func (s *Server) ListTools() []string {
	result := make([]string, len(s.toolNames))
	copy(result, s.toolNames)
	return result
}

// ParseSectionsParam extracts and validates the sections parameter from an MCP request.
// Exported for test access. Internal handlers call this directly.
func ParseSectionsParam(args map[string]any) (map[string]string, error) {
	raw, ok := args["sections"]
	if !ok || raw == nil {
		return nil, nil
	}
	switch v := raw.(type) {
	case string:
		var sections map[string]string
		if err := json.Unmarshal([]byte(v), &sections); err != nil {
			return nil, fmt.Errorf("parse sections JSON: %w", err)
		}
		return sections, nil
	case map[string]any:
		sections := make(map[string]string, len(v))
		for k, val := range v {
			sections[k] = fmt.Sprintf("%v", val)
		}
		return sections, nil
	default:
		return nil, fmt.Errorf("sections must be a JSON object, got %T", raw)
	}
}
