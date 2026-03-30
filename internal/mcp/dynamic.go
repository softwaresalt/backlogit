package mcp

// RegisterDynamicTools generates and registers MCP tools from template definitions.
// For each registered template, it generates:
//   - backlogit_create_{type}: pre-fills artifact type with section-specific string parameters
//   - backlogit_update_{type}_section: accepts id, section_name, content to update a section
//
// Each dynamic tool handler delegates to core create/update paths with template-aware section writing.
// Naming collisions with static tools are detected and rejected.
func RegisterDynamicTools(s *Server, templates []DynamicTemplateInput) error {
	panic("not implemented: Worker: Iterate templates, generate create_{type} and update_{type}_section tools for each. Each create tool pre-fills artifact_type and exposes section parameters from template section definitions. Each update tool accepts id, section_name, content. Detect and reject naming collisions with static tools registered in RegisterTools. Call from NewServer after loading templates.")
}

// DynamicTemplateInput holds the minimal template data needed for dynamic tool generation.
type DynamicTemplateInput struct {
	Name         string
	ArtifactType string
	Sections     []DynamicSectionInput
}

// DynamicSectionInput describes a section for dynamic tool parameter generation.
type DynamicSectionInput struct {
	Name     string
	Flag     string
	Required bool
}

// ListTools returns the names of all registered MCP tools.
func (s *Server) ListTools() []string {
	panic("not implemented: Worker: Return a string slice of all registered tool names from the internal mcp-go server. This is used by contract tests to verify tool registration.")
}
