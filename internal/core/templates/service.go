package templates

import (
	"context"

	"github.com/backlogit/backlogit/internal/config"
	"github.com/backlogit/backlogit/internal/models"
)

// TemplateInfo describes a registered template and its section metadata for agent discovery.
type TemplateInfo struct {
	TypeName    string        `json:"type_name"`
	DisplayName string        `json:"display_name"`
	Sections    []SectionInfo `json:"sections"`
}

// SectionInfo describes a single section within a template.
type SectionInfo struct {
	Name     string `json:"name"`
	Required bool   `json:"required"`
}

// Service provides the application-layer boundary for template-aware operations.
// Both CLI commands and MCP tool handlers call this service for template resolution,
// section-aware CRUD, and template discovery.
type Service struct {
	templatesDir string
	templates    []*config.TemplateConfig
}

// NewService creates a template service from the workspace templates directory.
//
// Worker: Load templates from templatesDir via config.LoadTemplates. Store the
// loaded templates for use by Resolve, Create, Update, GetSection, and ListTemplates.
// Return descriptive error if template loading fails.
func NewService(ctx context.Context, templatesDir string) (*Service, error) {
	panic("not implemented: Worker: Call config.LoadTemplates(templatesDir), store result in Service.templates, return error if loading fails. The ctx parameter supports cancellation for I/O-bound template loading.")
}

// Resolve returns the TemplateConfig for the given artifact type.
//
// Worker: Look up the template by artifact type from s.templates using
// config.GetTemplateForType. Return ErrTypeNotFound if no template matches.
func (s *Service) Resolve(artifactType string) (*config.TemplateConfig, error) {
	panic("not implemented: Worker: Call config.GetTemplateForType(s.templates, artifactType). If nil, return fmt.Errorf wrapping errors.ErrTypeNotFound. Otherwise return the template.")
}

// Create produces a new artifact with template-driven section content.
// The sections map keys must match section names defined in the template.
//
// Worker: Resolve the template for artifactType. Validate that all keys in
// sections exist in the template definition. Construct the markdown body by
// populating BEGIN/END tags with section content via parser.WriteSections.
// Delegate file creation to core.CreateArtifact with the composed body.
// Return the created artifact or a descriptive error.
func (s *Service) Create(ctx context.Context, ws interface{}, title string, artifactType string, sections map[string]string, opts ...interface{}) (*models.Artifact, error) {
	panic("not implemented: Worker: 1) Resolve template for artifactType. 2) Validate section names against template. 3) Build body from template Body with parser.WriteSections(templateBody, sections). 4) Call core.CreateArtifact with WithDescription(composedBody). 5) Return artifact. Use core.Workspace as the ws type after removing the interface{} placeholder.")
}

// Update applies section-level changes to an existing artifact.
// Only specified sections are modified; omitted sections remain unchanged.
//
// Worker: Read the existing artifact file. Parse its body to extract current
// sections via parser.ParseSections. Validate that all keys in updates exist
// in the template definition. Apply updates via parser.WriteSections. Write
// the updated content back via core.UpdateArtifact.
func (s *Service) Update(ctx context.Context, ws interface{}, id string, sections map[string]string) (*models.Artifact, error) {
	panic("not implemented: Worker: 1) Find artifact by id via ws. 2) Resolve template for artifact's type. 3) Validate section names. 4) Read file, apply parser.WriteSections with updates. 5) Write back via core.UpdateArtifact. Use core.Workspace as the ws type.")
}

// GetSection extracts the content of a named section from an artifact.
//
// Worker: Find the artifact by id. Read its markdown file. Parse sections via
// parser.ParseSections. Return the content of the named section, or
// ErrSectionNotFound if the section does not exist in the document.
func (s *Service) GetSection(ctx context.Context, ws interface{}, id string, sectionName string) (string, error) {
	panic("not implemented: Worker: 1) Find artifact file by id. 2) Read content. 3) Call parser.ParseSections. 4) Look up sectionName in result map. 5) Return content or ErrSectionNotFound. Use core.Workspace as the ws type.")
}

// ListTemplates returns metadata for all registered templates and their sections.
// This enables agent discovery of available template types and section names.
//
// Worker: Iterate s.templates and build a []TemplateInfo with type name,
// display name (from template Name field), and section metadata. Return the
// slice. If no templates are loaded, return an empty slice (not nil).
func (s *Service) ListTemplates() []TemplateInfo {
	panic("not implemented: Worker: Iterate s.templates. For each, create TemplateInfo with TypeName=t.ArtifactType, DisplayName=t.Name, and Sections mapped from t.Sections. Return the collected slice. Return empty slice if s.templates is nil or empty.")
}
