package events_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	backlogiterrors "github.com/softwaresalt/backlogit/internal/errors"
	"github.com/softwaresalt/backlogit/internal/events"
)

// TestCheckpointContext_UnicodeFoldCollision_StableAcrossReparse updated
// for 148-F / U2: fold-duplicate modeled context keys are now REJECTED at
// the create boundary with ErrCheckpointDuplicateContextKey rather than
// silently collapsed. The pre-U2 stable-collapse round-trip analysis is
// superseded by the reject-before-write invariant.
func TestCheckpointContext_UnicodeFoldCollision_StableAcrossReparse(t *testing.T) {
	// U+017F LATIN SMALL LETTER LONG S fold-aliases "s"/"S" under Unicode
	// simple case folding (encoding/json's own field-matching algorithm),
	// so a context with both shipment_id and its long-s variant carries a
	// fold-duplicate modeled key that U2 must reject.
	longSShipmentKey := "\u017fhipment_id"

	cases := []struct {
		name    string
		context string
	}{
		{"long_s_variant_first", `{"` + longSShipmentKey + `":"old","shipment_id":"new"}`},
		{"canonical_variant_first", `{"shipment_id":"new","` + longSShipmentKey + `":"old"}`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			stateDump := `{"schema_version":1,"agent":"ship","session_id":"s1","phase":"build","context":` + tc.context + `}`

			// 148-F / U2: fold-duplicate modeled context keys are REJECTED at
			// the create boundary. A context carrying both shipment_id and its
			// long-s fold-alias must return ErrCheckpointDuplicateContextKey
			// and write no file.
			_, err := events.CreateCheckpoint(context.Background(), dir, stateDump)
			require.Error(t, err, "context with fold-duplicate modeled key must be rejected by U2 gate")
			require.True(t, errors.Is(err, backlogiterrors.ErrCheckpointDuplicateContextKey),
				"error must satisfy errors.Is(err, ErrCheckpointDuplicateContextKey), got: %v", err)
			assertNoCheckpointWritten(t, dir)
		})
	}
}

// TestCreateCheckpoint_UnicodeFoldVariantTopLevelKeyAccepted is a companion
// regression proving the closed top-level namespace check
// (checkClosedSchemaNamespace) shares the same fold matcher as context-key
// routing: a top-level key spelling that fold-matches a modeled tag must be
// accepted by the closed-namespace check, not misclassified as unknown.
func TestCreateCheckpoint_UnicodeFoldVariantTopLevelKeyAccepted(t *testing.T) {
	dir := t.TempDir()
	// "ſession_id" (U+017F LATIN SMALL LETTER LONG S) fold-matches the
	// modeled "session_id" top-level tag. A top-level fold variant must
	// still be accepted (not rejected as unknown).
	longSSessionKey := "\u017fession_id"
	stateDump := `{"schema_version":1,"agent":"ship","` + longSSessionKey + `":"s-fold","phase":"build"}`

	result, err := events.CreateCheckpoint(context.Background(), dir, stateDump)
	require.NoError(t, err, "a top-level key that fold-matches a modeled field must be accepted")

	raw, err := readCheckpointFile(t, result.Path)
	require.NoError(t, err)
	cp, err := events.ParseCheckpoint(raw)
	require.NoError(t, err)
	assert.Equal(t, "s-fold", cp.SessionID)
}
