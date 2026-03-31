package core

import (
	"fmt"

	"github.com/backlogit/backlogit/internal/config"
)

// WITMetadata represents complete metadata for a work item type, merging data
// from header-def.yaml, template config, and workspace config.
type WITMetadata struct {
	TypeName       string                  `json:"type"`
	Description    string                  `json:"description"`
	HierarchyLevel int                     `json:"hierarchy_level"`
	IDFormat       string                  `json:"id_format"`
	Fields         map[string]WITFieldMeta `json:"fields"`
	Sections       []WITSectionMeta        `json:"sections"`
	Relationships  []WITRelationship       `json:"relationships"`
	Directories    WITDirectoryMeta        `json:"directories"`
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
func DescribeType(
	artifactType string,
	headerDef *config.HeaderDefConfig,
	templates []*config.TemplateConfig,
	layout *QueueLayoutConfig,
) (*WITMetadata, error) {
	typeCfg, ok := headerDef.Types[artifactType]
	if !ok {
		return nil, fmt.Errorf("unknown artifact type %q", artifactType)
	}

	fields := make(map[string]WITFieldMeta, len(typeCfg.Fields))
	for name, def := range typeCfg.Fields {
		fields[name] = WITFieldMeta{
			Type:     def.Type,
			Values:   def.Values,
			Required: !def.Optional,
			Default:  def.Default,
		}
	}

	var sections []WITSectionMeta
	for _, tmpl := range templates {
		if tmpl.ArtifactType == artifactType {
			for _, s := range tmpl.Sections {
				sections = append(sections, WITSectionMeta{
					Name:     s.Name,
					Required: s.Required,
				})
			}
			break
		}
	}

	level, _ := LevelForType(layout, artifactType)

	return &WITMetadata{
		TypeName:       artifactType,
		HierarchyLevel: level,
		IDFormat:       typeCfg.IDFormat,
		Fields:         fields,
		Sections:       sections,
		Directories: WITDirectoryMeta{
			Active:  artifactType + "s",
			Archive: "archive",
		},
	}, nil
}

// ListTypes returns a summary of all configured WIT types with their
// hierarchy levels and descriptions.
func ListTypes(
	headerDef *config.HeaderDefConfig,
	templates []*config.TemplateConfig,
	layout *QueueLayoutConfig,
) ([]WITMetadata, error) {
	var result []WITMetadata
	for typeName := range headerDef.Types {
		meta, err := DescribeType(typeName, headerDef, templates, layout)
		if err != nil {
			return nil, err
		}
		result = append(result, *meta)
	}
	return result, nil
}
