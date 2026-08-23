package events_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	backlogiterrors "github.com/softwaresalt/backlogit/internal/errors"
	"github.com/softwaresalt/backlogit/internal/events"
)

// TestParseCheckpoint_DegenerateContextShapes is scenario 1 of 146.008-T
// (U3b): "context": null, absent context, context as a JSON string, and
// context as a number each parse with exactly the outcome they produce
// today. This is a PRE-U2 golden baseline, captured once before 146.006-T
// (U2) begins: null and absent are a no-op (zero-value Context, no error);
// a string or number value fails to unmarshal into the CheckpointContext
// struct field and returns ErrCheckpointCorrupt. Writing these expectations
// after U2 lands would encode post-change behavior as "today" and make the
// guard pass vacuously.
func TestParseCheckpoint_DegenerateContextShapes(t *testing.T) {
	base := `{"schema_version":1,"agent":"ship","session_id":"s1","phase":"build","status":"active","created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z"`

	t.Run("null", func(t *testing.T) {
		cp, err := events.ParseCheckpoint([]byte(base + `,"context":null}`))
		require.NoError(t, err)
		assert.Equal(t, events.CheckpointContext{}, cp.Context)
	})

	t.Run("absent", func(t *testing.T) {
		cp, err := events.ParseCheckpoint([]byte(base + `}`))
		require.NoError(t, err)
		assert.Equal(t, events.CheckpointContext{}, cp.Context)
	})

	t.Run("string", func(t *testing.T) {
		_, err := events.ParseCheckpoint([]byte(base + `,"context":"oops"}`))
		require.Error(t, err)
		assert.ErrorIs(t, err, backlogiterrors.ErrCheckpointCorrupt)
	})

	t.Run("number", func(t *testing.T) {
		_, err := events.ParseCheckpoint([]byte(base + `,"context":42}`))
		require.Error(t, err)
		assert.ErrorIs(t, err, backlogiterrors.ErrCheckpointCorrupt)
	})
}

// TestParseCheckpoint_DuplicateContextKeyAliases is scenario 2 of 146.008-T
// (U3b): duplicate-key context objects parse with the same last-wins outcome
// as today, over a table that includes both the exact-duplicate case and
// both mixed-case alias orders. encoding/json resolves all three in SOURCE
// order (the second/last-declared key always wins), which is what makes the
// pre-U2 golden table meaningful: a source-order-faithful decoder (146.006-T,
// U2) satisfies all three deterministically, while a decoder that resolved
// modeled aliases by iterating map[string]json.RawMessage could satisfy them
// only by chance and would flake, because Go randomizes map iteration order.
// No row is "expected: either value".
func TestParseCheckpoint_DuplicateContextKeyAliases(t *testing.T) {
	base := `{"schema_version":1,"agent":"ship","session_id":"s1","phase":"build","status":"active","created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z"`

	tests := []struct {
		name string
		ctx  string
	}{
		{"exact_duplicate", `{"shipment_id":"A","shipment_id":"B"}`},
		{"alias_order_lower_then_upper", `{"shipment_id":"A","Shipment_ID":"B"}`},
		{"alias_order_upper_then_lower", `{"Shipment_ID":"A","shipment_id":"B"}`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cp, err := events.ParseCheckpoint([]byte(base + `,"context":` + tc.ctx + `}`))
			require.NoError(t, err)
			assert.Equal(t, "B", cp.Context.ShipmentID, "the last-declared key in source order must win deterministically")
		})
	}
}

// TestCreateCheckpoint_LegacyDumpWrittenVerbatim is scenario 3 of 146.008-T
// (U3b): a dump with no schema_version is written verbatim and is never
// subjected to the unknown-key probe (which does not exist before 146.011-T,
// U4, and even once it exists only applies to the V1 path).
func TestCreateCheckpoint_LegacyDumpWrittenVerbatim(t *testing.T) {
	dir := t.TempDir()
	stateDump := `{"anything_at_all":"x","nested":{"a":1}}`

	result, err := events.CreateCheckpoint(context.Background(), dir, stateDump)
	require.NoError(t, err)

	raw, err := readCheckpointFile(t, result.Path)
	require.NoError(t, err)
	assert.Equal(t, stateDump, string(raw), "a legacy (non-V1) dump must be written byte-for-byte verbatim")
}
