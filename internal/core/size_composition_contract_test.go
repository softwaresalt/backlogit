package core_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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

// TestAttachSizeComposition asserts the shared projection helper embeds the
// source value unchanged and attaches the rollup under SizeCompositionKey without
// mutating the input. Both the CLI and MCP read surfaces route through this
// helper, so pinning its shape guards against transport drift (114-F).
func TestAttachSizeComposition(t *testing.T) {
	type shipmentEnvelope struct {
		ID    string `json:"id"`
		Title string `json:"title"`
	}
	src := shipmentEnvelope{ID: "001-S", Title: "Parity shipment"}
	ruleset := "v1"
	comp := &core.SizeCompositionResult{
		Histogram:      map[string]int{"XL": 1, "M": 2},
		Unsized:        1,
		Members:        []core.SizeCompositionMember{{ID: "001.001-T", ArtifactType: "task", Size: "XL"}},
		RulesetVersion: &ruleset,
	}

	payload, err := core.AttachSizeComposition(src, comp)
	require.NoError(t, err)

	// Source fields survive the round-trip unchanged.
	assert.Equal(t, "001-S", payload["id"])
	assert.Equal(t, "Parity shipment", payload["title"])

	// The rollup is attached under the shared key and preserves its shape.
	attached, ok := payload[core.SizeCompositionKey].(*core.SizeCompositionResult)
	require.True(t, ok, "rollup must attach as *SizeCompositionResult")
	assert.Equal(t, comp, attached)

	// The helper must not mutate the caller's value.
	assert.Equal(t, shipmentEnvelope{ID: "001-S", Title: "Parity shipment"}, src)
}

// TestAttachSizeComposition_MarshalError asserts a value that cannot be marshaled
// to JSON surfaces an error rather than silently dropping the rollup.
func TestAttachSizeComposition_MarshalError(t *testing.T) {
	_, err := core.AttachSizeComposition(make(chan int), &core.SizeCompositionResult{})
	require.Error(t, err)
}
