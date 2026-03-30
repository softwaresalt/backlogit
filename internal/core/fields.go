package core

import (
	"fmt"

	"github.com/backlogit/backlogit/internal/config"
)

// ValidateFields checks custom field values against their configured types.
func ValidateFields(fieldConfigs map[string]*config.FieldConfig, fields map[string]any) error {
	for key, value := range fields {
		fc, ok := fieldConfigs[key]
		if !ok {
			continue
		}
		switch fc.Type {
		case "enum":
			strVal := fmt.Sprintf("%v", value)
			valid := false
			for _, v := range fc.Values {
				if v == strVal {
					valid = true
					break
				}
			}
			if !valid {
				return fmt.Errorf("field %q value %q not in allowed values %v", key, strVal, fc.Values)
			}
		case "int":
			switch value.(type) {
			case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64:
				// acceptable numeric types
			default:
				return fmt.Errorf("field %q expected int, got %T", key, value)
			}
		}
	}
	return nil
}

// TranslateExternalMap converts a local field value to its external representation.
func TranslateExternalMap(fieldConfig *config.FieldConfig, value any) (any, error) {
	if fieldConfig.ExternalMap == nil {
		return value, nil
	}
	key := fmt.Sprintf("%v", value)
	if mapped, ok := fieldConfig.ExternalMap[key]; ok {
		return mapped, nil
	}
	return value, nil
}
