package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// T001 / ST001: Verify shipment type appears in DefaultConfig artifact types.
func TestDefaultConfig_ContainsShipmentType(t *testing.T) {
	// Arrange
	cfg := DefaultConfig()

	// Act
	shipment, exists := cfg.ArtifactTypes["shipment"]

	// Assert
	require.True(t, exists, "shipment must be present in DefaultConfig().ArtifactTypes")
	assert.Equal(t, "S", shipment.Prefix)
	assert.Equal(t, "{prefix}{NNN}", shipment.NameFormat)
}

// T001 / ST001: Verify shipment appears in queue layout at the correct level.
func TestDefaultConfig_ShipmentInQueueLayout(t *testing.T) {
	// Arrange
	cfg := DefaultConfig()

	// Act
	found := false
	for _, level := range cfg.QueueLayout.Levels {
		for _, typ := range level.Types {
			if typ == "shipment" {
				found = true
			}
		}
	}

	// Assert
	assert.True(t, found, "shipment must appear in QueueLayout.Levels")
}

// T001 / ST002: Verify shipment headerdef schema appears in defaults.
func TestDefaultHeaderDef_ContainsShipmentSchema(t *testing.T) {
	// Arrange
	hd := defaultHeaderDef()

	// Act
	shipmentDef, exists := hd.Types["shipment"]

	// Assert
	require.True(t, exists, "shipment must be present in defaultHeaderDef().Types")
	assert.Equal(t, "S", shipmentDef.Prefix)
	require.Contains(t, shipmentDef.Fields, "status")
	require.Contains(t, shipmentDef.Fields, "branch")
	require.Contains(t, shipmentDef.Fields, "items")
}

// T001 / ST003: Verify default template content for shipment is present.
func TestDefaultTemplates_ContainsShipment(t *testing.T) {
	// Arrange
	templates := defaultTemplates()

	// Act
	tmpl, exists := templates["shipment"]

	// Assert
	require.True(t, exists, "shipment template must be present in defaultTemplates()")
	assert.Contains(t, tmpl, "## Description")
	assert.Contains(t, tmpl, "## Items")
	assert.Contains(t, tmpl, "## Blocked Returns")
}

// T001 / ST005: Verify no two artifact types share the same prefix.
func TestDefaultConfig_PrefixUniqueness(t *testing.T) {
	// Arrange
	cfg := DefaultConfig()

	// Act
	seen := make(map[string]string)
	var duplicates []string
	for name, atc := range cfg.ArtifactTypes {
		if existing, ok := seen[atc.Prefix]; ok {
			duplicates = append(duplicates, atc.Prefix+" used by "+existing+" and "+name)
		}
		seen[atc.Prefix] = name
	}

	// Assert
	assert.Empty(t, duplicates, "artifact type prefixes must be unique: %v", duplicates)
}

// T001 / ST005: Verify shipment prefix S does not collide with existing prefixes.
func TestDefaultConfig_ShipmentPrefixIsS(t *testing.T) {
	// Arrange
	cfg := DefaultConfig()

	// Act
	shipment := cfg.ArtifactTypes["shipment"]

	// Assert
	require.NotNil(t, shipment)
	assert.Equal(t, "S", shipment.Prefix, "shipment prefix must be S")
}
