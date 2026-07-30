package config

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"

	"github.com/go-playground/validator/v10"
	"gopkg.in/yaml.v3"
)

// HeaderDefConfig holds the parsed header-def.yaml configuration defining
// per-type field schemas with immutable defaults.
type HeaderDefConfig struct {
	Defaults SystemDefaults            `yaml:"defaults" validate:"required"`
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
	Prefix   string               `yaml:"prefix" validate:"required"`
	Suffix   string               `yaml:"suffix,omitempty"`
	IDFormat string               `yaml:"id_format" validate:"required"`
	Fields   map[string]*FieldDef `yaml:"fields" validate:"required"`
}

// LoadHeaderDef reads and validates header-def.yaml from the workspace directory.
func LoadHeaderDef(workspacePath string) (*HeaderDefConfig, error) {
	path := filepath.Join(workspacePath, "header-def.yaml")

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("load header-def.yaml: %w", err)
	}

	var cfg HeaderDefConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("load header-def.yaml: unmarshal: %w", err)
	}
	upgradeLegacyGeneratedHeaderDef(&cfg)

	if err := validator.New().Struct(cfg); err != nil {
		return nil, fmt.Errorf("load header-def.yaml: validation: %w", err)
	}

	return &cfg, nil
}

// upgradeLegacyGeneratedHeaderDef widens only the generated pre-complexity
// header-def default so existing workspaces receive task complexity without
// overwriting operator-customized schemas.
func upgradeLegacyGeneratedHeaderDef(cfg *HeaderDefConfig) {
	if cfg == nil {
		return
	}
	prior := defaultHeaderDef()
	delete(prior.Types["task"].Fields, "complexity")
	if !reflect.DeepEqual(cfg, prior) {
		return
	}

	current := defaultHeaderDef()
	cfg.Types["task"].Fields["complexity"] = current.Types["task"].Fields["complexity"]
}

// ResolveFieldSchema returns the field definitions for a given artifact type,
// merged with the system defaults.
func (h *HeaderDefConfig) ResolveFieldSchema(artifactType string) (map[string]*FieldDef, error) {
	typeCfg, ok := h.Types[artifactType]
	if !ok {
		return nil, fmt.Errorf("unknown artifact type %q: not defined in header-def.yaml", artifactType)
	}

	result := make(map[string]*FieldDef, len(typeCfg.Fields)+3)
	result["id"] = &h.Defaults.ID
	result["created_date"] = &h.Defaults.CreatedDate
	result["updated_date"] = &h.Defaults.UpdatedDate

	for k, v := range typeCfg.Fields {
		result[k] = v
	}

	return result, nil
}

// IsImmutable returns true if the given field name is a system-managed immutable field.
func (h *HeaderDefConfig) IsImmutable(fieldName string) bool {
	switch fieldName {
	case "id":
		return h.Defaults.ID.Immutable
	case "created_date":
		return h.Defaults.CreatedDate.Immutable
	case "updated_date":
		return h.Defaults.UpdatedDate.Immutable
	default:
		return false
	}
}
