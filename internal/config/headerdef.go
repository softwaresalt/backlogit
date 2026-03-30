package config

// HeaderDefConfig holds the parsed header-def.yaml configuration defining
// per-type field schemas with immutable defaults.
type HeaderDefConfig struct {
	Defaults SystemDefaults           `yaml:"defaults" validate:"required"`
	Types    map[string]*TypeDefConfig `yaml:"types" validate:"required,min=1"`
}

// SystemDefaults holds system-managed immutable field definitions.
type SystemDefaults struct {
	ID          FieldDef `yaml:"id"`
	CreatedDate FieldDef `yaml:"created_date"`
	UpdatedDate FieldDef `yaml:"updated_date"`
}

// FieldDef describes a single field's schema in header-def.yaml.
type FieldDef struct {
	Type      string   `yaml:"type" validate:"required"`
	Values    []string `yaml:"values,omitempty"`
	Default   string   `yaml:"default,omitempty"`
	Optional  bool     `yaml:"optional,omitempty"`
	Immutable bool     `yaml:"immutable,omitempty"`
}

// TypeDefConfig defines a single artifact type's field schema.
type TypeDefConfig struct {
	Prefix   string              `yaml:"prefix" validate:"required"`
	IDFormat string              `yaml:"id_format" validate:"required"`
	Fields   map[string]*FieldDef `yaml:"fields" validate:"required"`
}

// LoadHeaderDef reads and validates header-def.yaml from the workspace directory.
func LoadHeaderDef(workspacePath string) (*HeaderDefConfig, error) {
	panic("not implemented: Worker: Read header-def.yaml from workspacePath, unmarshal into HeaderDefConfig, validate with go-playground/validator, return validated config or descriptive error. Mark id/created_date/updated_date as immutable system fields. Support 8 artifact types: Epic, Feature, Sub-Epic, User-Story, Task, Sub-Task, Bug, Decision.")
}

// ResolveFieldSchema returns the field definitions for a given artifact type,
// merged with the system defaults.
func (h *HeaderDefConfig) ResolveFieldSchema(artifactType string) (map[string]*FieldDef, error) {
	panic("not implemented: Worker: Look up the TypeDefConfig for artifactType, merge system defaults from h.Defaults, and return the combined field map. Return descriptive error if type not found.")
}

// IsImmutable returns true if the given field name is a system-managed immutable field.
func (h *HeaderDefConfig) IsImmutable(fieldName string) bool {
	panic("not implemented: Worker: Check whether fieldName matches id, created_date, or updated_date from h.Defaults where Immutable=true. Return boolean.")
}
