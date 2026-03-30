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
// of workspace state, satisfying the constitutional mandate. When called before workspace
// initialization, tools return a descriptive error rather than being absent.
//
// Tools registered:
//   - backlogit_list_templates: Returns all registered template types with section definitions
//   - Section-aware parameters are added to backlogit_create_item and backlogit_update_item
//     via their existing handlers (sections JSON object parameter).
//
// Worker: Register backlogit_list_templates tool with the mcp-go server. The handler
// calls templateSvc.ListTemplates() to return template metadata as JSON. If templateSvc
// is nil (workspace uninitialized), return an empty list, not an error.
// Also register section-aware parameters on create_item (sections: JSON object) and
// update_item (sections: JSON object) and get_item (section: string).
func RegisterSectionAwareTools(s *Server, templateSvc *templates.Service) {
	panic("not implemented: Worker: 1) Register backlogit_list_templates tool using s.mcp.AddTool with description 'List registered template types and their section definitions'. Handler calls templateSvc.ListTemplates() and returns JSON. 2) The existing create_item/update_item/get_item handlers in tools.go need sections/section parameters added — coordinate with RegisterTools. If templateSvc is nil, list_templates returns empty JSON array.")
}

// handleListTemplates handles the backlogit_list_templates MCP tool call.
//
// Worker: If s.templateSvc is nil, return empty JSON array []. Otherwise call
// s.templateSvc.ListTemplates(), marshal to JSON, return as CallToolResult.
func handleListTemplates(ctx context.Context, s *Server) (*mcplib.CallToolResult, error) {
	panic("not implemented: Worker: Check if s.templateSvc is nil — return '[]' as tool result text. Otherwise call s.templateSvc.ListTemplates(), json.Marshal the result, return via mcplib.NewToolResultText.")
}

// handleCreateItemSections processes the 'sections' parameter from a create_item call.
// It delegates to the template service to build a section-populated artifact body.
//
// Worker: Extract 'sections' from request arguments as map[string]string. If present,
// call s.templateSvc.Create with the sections. If not present, fall through to the
// standard create path.
func handleCreateItemSections(ctx context.Context, s *Server, artifactType string, sections map[string]string) (string, error) {
	panic("not implemented: Worker: Call s.templateSvc.Resolve(artifactType) to get the template. Build the body using parser.WriteSections with the template body and sections map. Return the composed body string. Return error if sections contain invalid names.")
}

// ListTools returns the names of all registered MCP tools.
// Used by contract tests to verify the fixed tool surface.
func (s *Server) ListTools() []string {
	panic("not implemented: Worker: Return a string slice of all registered tool names from the internal mcp-go server s.mcp. This is used by contract tests to verify tool registration including backlogit_list_templates.")
}

// ParseSectionsParam extracts and validates the sections parameter from an MCP request.
// Exported for test access. Internal handlers call this directly.
//
// Worker: Extract 'sections' from request arguments. If it's a JSON string, unmarshal it.
// If it's a map[string]any, convert values to strings. Return nil map if not present.
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
