package core_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/softwaresalt/backlogit/internal/core"
)

// TestIsSizeCompositionAggregate locks the shared read-surface contract for which
// artifact types carry a computed-on-read size rollup. Size estimation is
// task-only, so only the rollup-parent types (feature, shipment) are projectable.
// Both the CLI and MCP transports consume this predicate so the two surfaces
// cannot drift on which types expose size_composition (114-F / 387DE4BF).
func TestIsSizeCompositionAggregate(t *testing.T) {
	cases := []struct {
		artifactType string
		want         bool
	}{
		{"feature", true},
		{"shipment", true},
		{"task", false},
		{"subtask", false},
		{"deliberation", false},
		{"", false},
	}
	for _, tc := range cases {
		t.Run(tc.artifactType, func(t *testing.T) {
			assert.Equal(t, tc.want, core.IsSizeCompositionAggregate(tc.artifactType))
		})
	}
}

// TestSizeCompositionKey pins the shared JSON key so CLI and MCP read surfaces
// attach the rollup under the identical field name.
func TestSizeCompositionKey(t *testing.T) {
	assert.Equal(t, "size_composition", core.SizeCompositionKey)
}
