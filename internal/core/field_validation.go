package core

import (
	"fmt"
	"strings"

	"github.com/softwaresalt/backlogit/internal/config"
	"github.com/softwaresalt/backlogit/internal/models"
)

// ValidateArtifactFields checks that all required fields for the artifact's type
// are present and valid according to the header-def.yaml field definitions.
func ValidateArtifactFields(artifact *models.Artifact, headerDef *config.HeaderDefConfig) error {
	schema, err := headerDef.ResolveFieldSchema(artifact.ArtifactType)
	if err != nil {
		// Unknown type — treat as backward-compatible pass.
		return nil
	}

	var missing []string
	for fieldName, def := range schema {
		if def.Optional || def.Immutable {
			continue
		}
		val := artifactFieldValue(artifact, fieldName)
		if val == "" {
			missing = append(missing, fieldName)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing required fields: %s", strings.Join(missing, ", "))
	}
	return nil
}

// ApplyFieldDefaults sets default values on artifact fields where the field is
// optional and the current value is the zero value.
func ApplyFieldDefaults(artifact *models.Artifact, headerDef *config.HeaderDefConfig) error {
	schema, err := headerDef.ResolveFieldSchema(artifact.ArtifactType)
	if err != nil {
		return nil
	}
	for fieldName, def := range schema {
		if def.Default == "" {
			continue
		}
		if artifactFieldValue(artifact, fieldName) == "" {
			setArtifactField(artifact, fieldName, def.Default)
		}
	}
	return nil
}

// artifactFieldValue returns the string value of a named field on an artifact.
func artifactFieldValue(a *models.Artifact, field string) string {
	switch field {
	case "id":
		return a.ID
	case "created_date":
		if a.CreatedAt.IsZero() {
			return ""
		}
		return "set"
	case "updated_date":
		if a.UpdatedAt.IsZero() {
			return ""
		}
		return "set"
	case "status":
		return string(a.Status)
	case "priority":
		return a.Priority
	case "severity":
		if a.CustomFields != nil {
			if v, ok := a.CustomFields["severity"]; ok {
				return fmt.Sprintf("%v", v)
			}
		}
		return ""
	default:
		if a.CustomFields != nil {
			if v, ok := a.CustomFields[field]; ok {
				return fmt.Sprintf("%v", v)
			}
		}
		return ""
	}
}

// setArtifactField applies a default value to a named field on an artifact.
func setArtifactField(a *models.Artifact, field, value string) {
	switch field {
	case "priority":
		if a.Priority == "" {
			a.Priority = value
		}
	default:
		if a.CustomFields == nil {
			a.CustomFields = make(map[string]any)
		}
		if _, exists := a.CustomFields[field]; !exists {
			a.CustomFields[field] = value
		}
	}
}
