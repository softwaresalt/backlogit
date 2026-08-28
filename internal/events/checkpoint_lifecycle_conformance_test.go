package events_test

// U3c — Resolve-verb conformance refusal harness (RED, lands before the
// migration). This file carries NO production change (147-F / 147.042-T,
// red-deliverable). It fails against the pre-migration ResolveCheckpoint,
// which mutates and rewrites a valid-but-non-conforming document with a
// fabricated skeleton and returns nil. U14 (147.037-T) turns this function
// green by routing ResolveCheckpoint through the guarded rewrite seam
// (RewriteCheckpointFile), whose U13 implementation returns the raw
// *CheckpointNonConformingError before any marshal or write.

import (
	"context"
	"crypto/sha256"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	backlogiterrors "github.com/softwaresalt/backlogit/internal/errors"
	"github.com/softwaresalt/backlogit/internal/events"
)

// TestU3c_ResolveRefusesValidNonConformingDocument asserts ResolveCheckpoint
// on a valid-but-non-conforming document (schema-valid, status:"active",
// carrying two unmodeled top-level keys) is refused with an error satisfying
// errors.Is(err, ErrCheckpointNonConforming), errors.As recovers both
// offender keys sorted, and the file's bytes are unchanged.
func TestU3c_ResolveRefusesValidNonConformingDocument(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "checkpoint-u3c.json")
	body := `{"schema_version":1,"agent":"ship","session_id":"s1","phase":"build","status":"active",` +
		`"created_at":"2026-08-24T00:00:00Z","updated_at":"2026-08-24T00:00:00Z",` +
		`"zeta_extra":"x","alpha_extra":"y"}`
	require.NoError(t, os.WriteFile(path, []byte(body), 0o644))
	before := sha256.Sum256([]byte(body))

	err := events.ResolveCheckpoint(context.Background(), dir, "checkpoint-u3c.json")

	require.Error(t, err, "resolve must refuse a valid-but-non-conforming document")
	assert.True(t, errors.Is(err, backlogiterrors.ErrCheckpointNonConforming))
	var typed *backlogiterrors.CheckpointNonConformingError
	if assert.True(t, errors.As(err, &typed)) {
		assert.Equal(t, []string{"alpha_extra", "zeta_extra"}, typed.Fields)
	}

	after, readErr := os.ReadFile(path)
	require.NoError(t, readErr)
	assert.Equal(t, before, sha256.Sum256(after), "a refused resolve must not change the file's bytes")
}
