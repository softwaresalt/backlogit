package core

import (
	"github.com/backlogit/backlogit/internal/config"
	"github.com/backlogit/backlogit/internal/models"
)

// ValidateArtifactFields checks that all required fields for the artifact's type
// are present and valid according to the header-def.yaml field definitions.
//
// Worker: For each field in the type's schema where Optional == false, verify the
// artifact has a non-zero value. Return a structured error listing all missing
// required fields. Apply default values for optional fields when omitted.
func ValidateArtifactFields(artifact *models.Artifact, headerDef *config.HeaderDefConfig) error {
	panic("not implemented: Worker: Validate required fields per type schema, return descriptive error listing all missing fields")
}

// ApplyFieldDefaults sets default values on artifact fields where the field is
// optional and the current value is the zero value.
//
// Worker: Iterate the type's FieldDef entries. For each optional field with a
// Default value, if the artifact's corresponding field is empty/zero, set it.
func ApplyFieldDefaults(artifact *models.Artifact, headerDef *config.HeaderDefConfig) error {
	panic("not implemented: Worker: Apply default values from FieldDef to empty optional artifact fields")
}
