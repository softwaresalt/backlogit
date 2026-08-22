package events_test

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/events"
)

// TestCheckpointContext_UnicodeFoldCollision_StableAcrossReparse is a
// regression for the Unicode case-folding gap Copilot review found on PR
// #373 (deferred past the review-fix circuit breaker as stash 6D03554D, now
// fixed under an operator-authorized exceptional review-fix cycle):
// isModeledContextKey compared keys via strings.ToLower, which is NOT the
// case-folding equivalence relation encoding/json itself uses for its own
// struct field matching (Unicode simple case folding -- the same algorithm
// strings.EqualFold implements). U+017F LATIN SMALL LETTER LONG S ("ſ")
// simple-case-folds to "s"/"S" under encoding/json's matching, but
// unicode.ToLower leaves it unchanged (it is already its own lowercase
// form), so "ſhipment_id" fold-matched CheckpointContext.ShipmentID under
// pass 1's real encoding/json decode (the plainContext shadow) while
// isModeledContextKey's ToLower comparison in pass 2 (Extra routing)
// disagreed and treated it as an unmodeled key. That let a single
// fold-duplicate top-level context key survive into Extra, get re-emitted
// AFTER the modeled field on disk, and change which occurrence wins the
// NEXT time the same bytes are decoded -- a checkpoint's own ShipmentID
// could silently flip on a second round trip through disk. Both key orders
// are exercised because the flip depends on source order, not on which
// order is "canonical".
func TestCheckpointContext_UnicodeFoldCollision_StableAcrossReparse(t *testing.T) {
	cases := []struct {
		name    string
		context string
	}{
		{"long_s_variant_first", `{"ſhipment_id":"old","shipment_id":"new"}`},
		{"canonical_variant_first", `{"shipment_id":"new","ſhipment_id":"old"}`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			stateDump := `{"schema_version":1,"agent":"ship","session_id":"s1","phase":"build","context":` + tc.context + `}`

			result, err := events.CreateCheckpoint(context.Background(), dir, stateDump)
			require.NoError(t, err)

			raw1, err := readCheckpointFile(t, result.Path)
			require.NoError(t, err)
			cp1, err := events.ParseCheckpoint(raw1)
			require.NoError(t, err)
			gen1 := cp1.Context.ShipmentID

			// A fold-duplicate modeled key must never survive into Extra: at
			// most one "shipment_id" spelling (canonical or fold variant)
			// may ever be emitted, because the fold variant IS the modeled
			// field as far as encoding/json (and therefore this schema) is
			// concerned.
			occurrences := strings.Count(string(raw1), `"shipment_id"`) + strings.Count(string(raw1), `"ſhipment_id"`)
			assert.Equal(t, 1, occurrences,
				"a fold-duplicate modeled key must collapse to exactly one emitted occurrence, never both")

			// Re-submit the written bytes through CreateCheckpoint a second
			// time (simulating a reparse/re-persist cycle) and confirm the
			// winner is stable, not flipped by the reordering that emitting
			// modeled-fields-before-Extra would otherwise introduce.
			result2, err := events.CreateCheckpoint(context.Background(), dir, string(raw1))
			require.NoError(t, err)
			raw2, err := readCheckpointFile(t, result2.Path)
			require.NoError(t, err)
			cp2, err := events.ParseCheckpoint(raw2)
			require.NoError(t, err)

			assert.Equal(t, gen1, cp2.Context.ShipmentID,
				"ShipmentID must be stable across a second create/reparse round trip, not flip due to fold-duplicate key leakage")
		})
	}
}

// TestCreateCheckpoint_UnicodeFoldVariantTopLevelKeyAccepted is a companion
// regression proving the closed top-level namespace check
// (checkClosedSchemaNamespace) shares the same fold matcher as context-key
// routing: a top-level key spelling that fold-matches a modeled tag under
// encoding/json's own struct decode (pass 1, ParseCheckpoint) must also be
// accepted by the closed-namespace check (pass 2), instead of being
// misclassified as an "unknown field" purely because of the ToLower vs.
// Unicode-simple-fold mismatch. Before the fix this create is wrongly
// rejected even though pass 1 already accepted the same bytes.
func TestCreateCheckpoint_UnicodeFoldVariantTopLevelKeyAccepted(t *testing.T) {
	dir := t.TempDir()
	// "ſession_id" (U+017F LATIN SMALL LETTER LONG S) fold-matches the
	// modeled "session_id" top-level tag under encoding/json's own field
	// matching, exactly as a differently-cased alias would -- but
	// unicode.ToLower leaves "ſ" unchanged, so a ToLower-only membership
	// check misclassifies it as unknown.
	stateDump := `{"schema_version":1,"agent":"ship","ſession_id":"s-fold","phase":"build"}`

	result, err := events.CreateCheckpoint(context.Background(), dir, stateDump)
	require.NoError(t, err, "a top-level key that fold-matches a modeled field must be accepted, matching pass 1's own decode")

	raw, err := readCheckpointFile(t, result.Path)
	require.NoError(t, err)
	cp, err := events.ParseCheckpoint(raw)
	require.NoError(t, err)
	assert.Equal(t, "s-fold", cp.SessionID)
}
