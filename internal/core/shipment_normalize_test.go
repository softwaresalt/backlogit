package core

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/softwaresalt/backlogit/internal/models"
)

// buildShipmentItemsArtifact constructs a minimal models.Artifact with
// custom_fields["items"] set to raw. When raw is nil the CustomFields map
// itself is left nil to exercise the nil-map guard.
func buildShipmentItemsArtifact(raw any) *models.Artifact {
	a := &models.Artifact{ArtifactType: "shipment"}
	if raw == nil {
		return a
	}
	a.CustomFields = map[string]any{"items": raw}
	return a
}

// TestNormalizeShipmentItems_AllCases unit-tests the single source of truth,
// core.NormalizeShipmentItems, verifying every source-type branch produces a
// non-nil []string. Relocated from internal/mcp as part of consolidating the
// duplicate shipment-items normalizer into core (stash 17D29DDC).
func TestNormalizeShipmentItems_AllCases(t *testing.T) {
	tests := []struct {
		name  string
		input any // value for custom_fields["items"], or nil for a nil CustomFields map
		want  []string
	}{
		{name: "nil custom_fields map", input: nil, want: []string{}},
		{name: "[]string already typed", input: []string{"a", "b"}, want: []string{"a", "b"}},
		{name: "[]any with strings", input: []any{"x", "y"}, want: []string{"x", "y"}},
		{name: "[]any with non-string element", input: []any{"ok", 42}, want: []string{"ok"}},
		{name: "unknown type falls back", input: 123, want: []string{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizeShipmentItems(buildShipmentItemsArtifact(tt.input))
			assert.Equal(t, tt.want, got,
				"NormalizeShipmentItems produced unexpected items for input %v (type %T)", tt.input, tt.input)
		})
	}
}

// TestNormalizeShipmentItems_EmptyStringSliceNeverNil pins the reconciled
// behavioral edge: an empty []string input must return a NON-NIL empty slice
// (which marshals to [], not null). This converges on the superset guarantee
// formerly enforced by the deleted internal/mcp mutator and keeps the never-null
// JSON wire-shape contract at the single source of truth. See the end-to-end
// guard TestListShipments_EmptyItems_NeverNull (internal/mcp).
func TestNormalizeShipmentItems_EmptyStringSliceNeverNil(t *testing.T) {
	got := NormalizeShipmentItems(buildShipmentItemsArtifact([]string{}))
	assert.NotNil(t, got, "empty []string input must yield a non-nil slice (never null on the wire)")
	assert.Len(t, got, 0, "empty []string input must yield a zero-length slice")
}
