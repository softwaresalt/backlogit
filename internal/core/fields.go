package core

import "github.com/backlogit/backlogit/internal/config"

// ValidateFields checks custom field values against their configured types.
//
// Worker: Implement field validation against config schemas.
func ValidateFields(fieldConfigs map[string]*config.FieldConfig, fields map[string]any) error {
	panic("not implemented: Worker: Implement custom field validation")
}

// TranslateExternalMap converts a local field value to its external representation.
//
// Worker: Implement external_map translation for Jira/ADO sync.
func TranslateExternalMap(fieldConfig *config.FieldConfig, value any) (any, error) {
	panic("not implemented: Worker: Implement external map translation")
}
