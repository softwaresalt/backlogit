package templates

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/backlogit/backlogit/internal/config"
	"github.com/backlogit/backlogit/internal/core"
	"github.com/backlogit/backlogit/internal/db"
	"github.com/backlogit/backlogit/internal/models"
	"github.com/backlogit/backlogit/internal/parser"
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
type Service struct {
	templatesDir string
	templates    []*config.TemplateConfig
}

// NewService creates a template service from the workspace templates directory.
func NewService(_ context.Context, templatesDir string) (*Service, error) {
	loaded, err := config.LoadTemplates(templatesDir)
	if err != nil {
		return nil, fmt.Errorf("load templates: %w", err)
	}
	return &Service{
		templatesDir: templatesDir,
		templates:    loaded,
	}, nil
}

// Resolve returns the TemplateConfig for the given artifact type.
func (s *Service) Resolve(artifactType string) (*config.TemplateConfig, error) {
	tmpl := config.GetTemplateForType(s.templates, artifactType)
	if tmpl == nil {
		return nil, fmt.Errorf("template not found for type %q", artifactType)
	}
	return tmpl, nil
}

// Create produces a new artifact with template-driven section content.
func (s *Service) Create(ctx context.Context, ws *core.Workspace, title string, artifactType string, sections map[string]string, opts ...core.Option) (*models.Artifact, error) {
	tmpl, err := s.Resolve(artifactType)
	if err != nil {
		return nil, err
	}

	if err := validateSectionNames(tmpl, sections); err != nil {
		return nil, err
	}

	body := renderTemplateBody(tmpl.Body, title)
	if len(sections) > 0 {
		newBody, writeErr := parser.WriteSections(body, sections)
		if writeErr != nil {
			return nil, fmt.Errorf("write sections: %w", writeErr)
		}
		body = newBody
	}

	allOpts := append([]core.Option{core.WithDescription(body)}, opts...)
	artifact, err := core.CreateArtifact(ctx, ws, title, artifactType, allOpts...)
	if err != nil {
		return nil, err
	}
	if err := db.UpsertItem(ctx, ws.DB, artifact); err != nil {
		return nil, fmt.Errorf("sync index after create: %w", err)
	}
	return artifact, nil
}

// Update applies section-level changes to an existing artifact.
func (s *Service) Update(ctx context.Context, ws *core.Workspace, id string, sections map[string]string) (*models.Artifact, error) {
	filePath, err := core.FindArtifactPath(ctx, ws, id)
	if err != nil {
		return nil, err
	}

	raw, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("read artifact: %w", err)
	}

	fm, body, err := models.ParseFrontmatter(string(raw))
	if err != nil {
		return nil, fmt.Errorf("parse artifact: %w", err)
	}

	artifact, err := models.ArtifactFromFrontmatter(fm, body)
	if err != nil {
		return nil, fmt.Errorf("parse artifact fields: %w", err)
	}

	tmpl, err := s.Resolve(artifact.ArtifactType)
	if err != nil {
		return nil, err
	}

	if err := validateSectionNames(tmpl, sections); err != nil {
		return nil, err
	}

	newBody, err := parser.WriteSections(renderTemplateBody(body, artifact.Title), sections)
	if err != nil {
		return nil, fmt.Errorf("write sections: %w", err)
	}

	now := time.Now()
	artifact.Description = newBody
	artifact.UpdatedAt = now
	fm["updated_at"] = now

	newContent := models.SerializeFrontmatter(fm, newBody)
	tmp := filePath + ".tmp"
	if err := os.WriteFile(tmp, []byte(newContent), 0o644); err != nil {
		return nil, fmt.Errorf("write artifact: %w", err)
	}
	if err := os.Rename(tmp, filePath); err != nil {
		os.Remove(tmp) //nolint:errcheck
		return nil, fmt.Errorf("rename artifact: %w", err)
	}

	if err := db.UpsertItem(ctx, ws.DB, artifact); err != nil {
		return nil, fmt.Errorf("sync index after section update: %w", err)
	}

	return artifact, nil
}

// GetSection extracts the content of a named section from an artifact.
func (s *Service) GetSection(ctx context.Context, ws *core.Workspace, id string, sectionName string) (string, error) {
	filePath, err := core.FindArtifactPath(ctx, ws, id)
	if err != nil {
		return "", err
	}

	raw, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("read artifact: %w", err)
	}

	_, body, err := models.ParseFrontmatter(string(raw))
	if err != nil {
		return "", fmt.Errorf("parse artifact: %w", err)
	}

	sections, err := parser.ParseSections(body)
	if err != nil {
		return "", fmt.Errorf("parse sections: %w", err)
	}

	content, ok := sections[sectionName]
	if !ok {
		return "", fmt.Errorf("section %q not found", sectionName)
	}
	return content, nil
}

// ListTemplates returns metadata for all registered templates and their sections.
func (s *Service) ListTemplates() []TemplateInfo {
	infos := make([]TemplateInfo, 0, len(s.templates))
	for _, t := range s.templates {
		sections := make([]SectionInfo, len(t.Sections))
		for i, sec := range t.Sections {
			sections[i] = SectionInfo{Name: sec.Name, Required: sec.Required}
		}
		infos = append(infos, TemplateInfo{
			TypeName:    t.ArtifactType,
			DisplayName: t.Name,
			Sections:    sections,
		})
	}
	return infos
}

// validateSectionNames checks that all keys in sections exist in the template.
func validateSectionNames(tmpl *config.TemplateConfig, sections map[string]string) error {
	valid := make(map[string]struct{}, len(tmpl.Sections))
	for _, sec := range tmpl.Sections {
		valid[sec.Name] = struct{}{}
	}
	for name := range sections {
		if _, ok := valid[name]; !ok {
			return fmt.Errorf("unknown section %q in template %q", name, tmpl.ArtifactType)
		}
	}
	return nil
}

func renderTemplateBody(body string, title string) string {
	return strings.ReplaceAll(body, "{title}", title)
}
