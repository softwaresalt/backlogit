package config

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-playground/validator/v10"
	"gopkg.in/yaml.v3"
)

// TemplateConfig represents a Markdown template with named sections.
type TemplateConfig struct {
	Name         string       `yaml:"name" validate:"required"`
	ArtifactType string       `yaml:"type" validate:"required"`
	Sections     []SectionDef `yaml:"sections" validate:"required,min=1,dive"`
	Body         string       `yaml:"-"`
}

// SectionDef describes a single template section.
type SectionDef struct {
	Name     string `yaml:"name" validate:"required"`
	Required bool   `yaml:"required"`
}

// LoadTemplates discovers and parses all .md template files from templatesDir.
// If templatesDir does not exist, it returns nil, nil (missing directory is not an error).
// Each file must contain YAML frontmatter followed by a Markdown body with matching
// <!-- BEGIN:{name} --> / <!-- END:{name} --> section tags for every declared section.
func LoadTemplates(templatesDir string) ([]*TemplateConfig, error) {
	if _, err := os.Stat(templatesDir); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("stat templates dir %q: %w", templatesDir, err)
	}

	var templates []*TemplateConfig

	walkErr := filepath.WalkDir(templatesDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		// Skip the root itself; skip any nested sub-directories entirely.
		if d.IsDir() {
			if path == templatesDir {
				return nil
			}
			return filepath.SkipDir
		}
		if !strings.HasSuffix(d.Name(), ".md") {
			return nil
		}

		tmpl, parseErr := parseTemplateFile(path)
		if parseErr != nil {
			return parseErr
		}
		templates = append(templates, tmpl)
		return nil
	})
	if walkErr != nil {
		return nil, walkErr
	}

	return templates, nil
}

// GetTemplateForType returns the first TemplateConfig whose ArtifactType matches
// artifactType, or nil if none is found.
func GetTemplateForType(templates []*TemplateConfig, artifactType string) *TemplateConfig {
	for _, t := range templates {
		if t.ArtifactType == artifactType {
			return t
		}
	}
	return nil
}

// parseTemplateFile reads a single .md template file, extracts its YAML frontmatter
// and body, validates structural invariants, and returns a populated TemplateConfig.
func parseTemplateFile(path string) (*TemplateConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read template file %q: %w", path, err)
	}

	content := string(data)

	// Expect YAML frontmatter delimited by "---\n" ... "\n---\n".
	const opener = "---\n"
	if !strings.HasPrefix(content, opener) {
		return nil, fmt.Errorf("template file %q: missing YAML frontmatter opener", path)
	}

	rest := content[len(opener):]

	// Find the closing delimiter.
	const closingSep = "\n---\n"
	sepIdx := strings.Index(rest, closingSep)
	if sepIdx == -1 {
		return nil, fmt.Errorf("template file %q: unclosed YAML frontmatter", path)
	}

	yamlBlock := rest[:sepIdx]
	body := rest[sepIdx+len(closingSep):]

	var tmpl TemplateConfig
	if err := yaml.Unmarshal([]byte(yamlBlock), &tmpl); err != nil {
		return nil, fmt.Errorf("parse YAML frontmatter in %q: %w", path, err)
	}

	// Validate section names are unique within this template.
	seen := make(map[string]struct{}, len(tmpl.Sections))
	for _, s := range tmpl.Sections {
		if _, exists := seen[s.Name]; exists {
			return nil, fmt.Errorf("template %q: duplicate section name %q", tmpl.Name, s.Name)
		}
		seen[s.Name] = struct{}{}
	}

	// Validate that every declared section has matched BEGIN/END tags in the body.
	for _, s := range tmpl.Sections {
		beginTag := "<!-- BEGIN:" + s.Name + " -->"
		endTag := "<!-- END:" + s.Name + " -->"

		beginIdx := strings.Index(body, beginTag)
		if beginIdx == -1 {
			return nil, fmt.Errorf("template %q: section %q has mismatched BEGIN/END tags", tmpl.Name, s.Name)
		}

		afterBegin := beginIdx + len(beginTag)
		if !strings.Contains(body[afterBegin:], endTag) {
			return nil, fmt.Errorf("template %q: section %q has mismatched BEGIN/END tags", tmpl.Name, s.Name)
		}
	}

	tmpl.Body = body

	validate := validator.New()
	if err := validate.Struct(&tmpl); err != nil {
		return nil, fmt.Errorf("validate template %q: %w", tmpl.Name, err)
	}

	return &tmpl, nil
}
