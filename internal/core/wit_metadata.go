package core

import (
	"github.com/backlogit/backlogit/internal/config"
)

// WITMetadata represents complete metadata for a work item type, merging data
// from header-def.yaml, template config, and workspace config.
type WITMetadata struct {
	TypeName       string                    `json:"type"`
	Description    string                    `json:"description"`
	HierarchyLevel int                       `json:"hierarchy_level"`
	IDFormat       string                    `json:"id_format"`
	Fields         map[string]WITFieldMeta   `json:"fields"`
	Sections       []WITSectionMeta          `json:"sections"`
	Relationships  []WITRelationship         `json:"relationships"`
	Directories    WITDirectoryMeta          `json:"directories"`
}

// WITFieldMeta describes a single field's metadata for agent consumption.
type WITFieldMeta struct {
	Type     string   `json:"type"`
	Values   []string `json:"values,omitempty"`
	Required bool     `json:"required"`
	Default  string   `json:"default,omitempty"`
}

// WITSectionMeta describes a template section.
type WITSectionMeta struct {
	Name        string `json:"name"`
	Required    bool   `json:"required"`
	Description string `json:"description"`
}

// WITRelationship describes a relationship between WIT types.
type WITRelationship struct {
	RelatedType string `json:"related_type"`
	Relation    string `json:"relation"`
	Description string `json:"description"`
}

// WITDirectoryMeta describes the active and archive directories for a type.
type WITDirectoryMeta struct {
	Active  string `json:"active"`
	Archive string `json:"archive"`
}

// DescribeType merges header-def, template, and workspace config to produce
// complete WIT metadata for a given artifact type.
//
// Worker: Look up the type in headerDef.Types, find the matching template in
// templates, resolve hierarchy level from layout config, merge all three into
// a WITMetadata struct. Return error for unknown types.
func DescribeType(
	artifactType string,
	headerDef *config.HeaderDefConfig,
	templates []*config.TemplateConfig,
	layout *QueueLayoutConfig,
) (*WITMetadata, error) {
	panic("not implemented: Worker: Merge header-def, template, and layout config into WITMetadata for the given artifact type")
}

// ListTypes returns a summary of all configured WIT types with their
// hierarchy levels and descriptions (lightweight discovery).
//
// Worker: Iterate headerDef.Types and templates, extract type name, description,
// and hierarchy level for each. Return as a slice sorted by hierarchy level.
func ListTypes(
	headerDef *config.HeaderDefConfig,
	templates []*config.TemplateConfig,
	layout *QueueLayoutConfig,
) ([]WITMetadata, error) {
	panic("not implemented: Worker: Enumerate all configured WIT types with descriptions and hierarchy levels")
}
