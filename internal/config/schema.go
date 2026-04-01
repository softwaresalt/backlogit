package config

import "github.com/go-playground/validator/v10"

var validate = validator.New()

// Validate checks all struct tags and returns a descriptive error on failure.
func (c *WorkspaceConfig) Validate() error {
	return validate.Struct(c)
}

// WorkspaceConfig holds the parsed workspace configuration.
type WorkspaceConfig struct {
	ArtifactTypes map[string]*ArtifactTypeConfig `yaml:"artifact_types" validate:"required,min=1"`
	Fields        map[string]*FieldConfig        `yaml:"fields"`
	MaxSlugLength int                            `yaml:"max_slug_length" validate:"gte=10,lte=200"`
	QueueLayout   *QueueLayoutConfig             `yaml:"queue_layout"`
}

// ArtifactTypeConfig defines an artifact type's behavior.
type ArtifactTypeConfig struct {
	Prefix          string   `yaml:"prefix" validate:"required"`
	NameFormat      string   `yaml:"name_format" validate:"required"`
	AllowedChildren []string `yaml:"allowed_children"`
}

// FieldConfig defines a custom field's schema.
type FieldConfig struct {
	Type     string   `yaml:"type" validate:"required,oneof=enum string int"`
	Values   []string `yaml:"values"`
	Default  string   `yaml:"default"`
	Optional bool     `yaml:"optional"`
	// ExternalMap holds translation rules for external systems (e.g., Jira, ADO).
	// Uses map[string]any because external system payloads have heterogeneous value types.
	ExternalMap map[string]any `yaml:"external_map"`
}

// RegistryConfig holds directory routing rules.
type RegistryConfig struct {
	Directories []DirectoryRule `yaml:"directories" validate:"required"`
}

// DirectoryRule maps conditions to a target directory path.
type DirectoryRule struct {
	Path      string             `yaml:"path" validate:"required"`
	Condition DirectoryCondition `yaml:"condition"`
}

// DirectoryCondition specifies when a directory rule applies.
type DirectoryCondition struct {
	Status []string `yaml:"status"`
	Type   []string `yaml:"type"`
}

// QueueLayoutConfig defines the hierarchical file organization for .backlogit/queue/.
type QueueLayoutConfig struct {
	RootDir    string           `yaml:"root_dir" validate:"required"`
	Levels     []HierarchyLevel `yaml:"levels" validate:"required,min=1,dive"`
	NameFormat string           `yaml:"name_format"`
}

// HierarchyLevel maps a hierarchy depth to one or more artifact types.
type HierarchyLevel struct {
	Level int      `yaml:"level" validate:"required,gte=1,lte=5"`
	Types []string `yaml:"types" validate:"required,min=1"`
}

// HooksConfig is a minimal stub for external integration hooks (deferred scope).
type HooksConfig struct {
	Enabled bool `yaml:"enabled"`
}
