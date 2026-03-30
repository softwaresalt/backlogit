package config

// TemplateConfig represents a Markdown template with named sections.
type TemplateConfig struct {
	Name         string           `yaml:"name" validate:"required"`
	ArtifactType string           `yaml:"type" validate:"required"`
	Sections     []SectionDef     `yaml:"sections" validate:"required,min=1,dive"`
	Body         string           `yaml:"-"`
}

// SectionDef describes a single template section.
type SectionDef struct {
	Name     string `yaml:"name" validate:"required"`
	Flag     string `yaml:"flag" validate:"required"`
	Required bool   `yaml:"required"`
}

// LoadTemplates discovers and parses all template files from the templates directory.
func LoadTemplates(templatesDir string) ([]*TemplateConfig, error) {
	panic("not implemented: Worker: Walk templatesDir, parse each .md file's YAML frontmatter into TemplateConfig, validate section names are unique within each template, validate flags are lowercase-hyphenated, validate BEGIN/END tags are matched in the body, return slice of validated configs or descriptive error.")
}

// GetTemplateForType returns the template config for a given artifact type.
func GetTemplateForType(templates []*TemplateConfig, artifactType string) *TemplateConfig {
	panic("not implemented: Worker: Iterate templates, return the first matching artifactType. Return nil if no match found.")
}
