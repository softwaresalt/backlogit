package events_test

// U3b — Resolve-verb conformance contract and named already-resolved
// residual (147-F / 147.007-T, harness-exempt: verification-only). No
// production change: both scenarios pin behaviour already shipped by
// 147.037-T / U14 (guarded seam migration) and 147.011-T / U6 (conformance
// projection on GetCheckpointResult). Deliberately a distinct file from
// checkpoint_lifecycle_conformance_test.go (147.042-T / U3c's red harness)
// per build-feature Step 5's verification-only exception.

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	backlogiterrors "github.com/softwaresalt/backlogit/internal/errors"
	"github.com/softwaresalt/backlogit/internal/events"
)

// TestU3bGuard_AlreadyResolvedNonConformingDocumentIsNoOpButFlaggedByGet
// asserts the named residual: a document whose status is already
// "resolved" short-circuits ResolveCheckpoint as an idempotent no-op (nil,
// bytes unchanged) even when it is non-conforming — the conformance gate
// never runs on an already-resolved document. Discovery is left to U6:
// GetCheckpointResult on the same file reports NeedsQuarantine: true.
func TestU3bGuard_AlreadyResolvedNonConformingDocumentIsNoOpButFlaggedByGet(t *testing.T) {
	dir := t.TempDir()
	name := "checkpoint-u3b-resolved-nonconforming.json"
	path := filepath.Join(dir, name)
	body := []byte(`{"schema_version":1,"agent":"ship","session_id":"s1","phase":"build","status":"resolved",` +
		`"created_at":"2026-08-24T00:00:00Z","updated_at":"2026-08-24T00:00:00Z","extra_key":"x"}`)
	require.NoError(t, os.WriteFile(path, body, 0o644))

	err := events.ResolveCheckpoint(context.Background(), dir, name)
	require.NoError(t, err, "an already-resolved document is an idempotent no-op regardless of conformance")

	after, readErr := os.ReadFile(path)
	require.NoError(t, readErr)
	assert.Equal(t, body, after, "resolve must not rewrite an already-resolved document")

	result, getErr := events.GetCheckpointResult(context.Background(), dir, name)
	require.NoError(t, getErr)
	assert.True(t, result.Valid)
	assert.False(t, result.Conforming)
	assert.True(t, result.NeedsQuarantine, "GetCheckpointResult (U6) is the discovery path resolve intentionally bypasses")
	assert.Contains(t, result.NonConformingFields.Paths, "extra_key")
}

// TestU3bGuard_AbandonedDocumentStillRefusedByResolve pins the existing,
// unchanged ResolveCheckpoint guard: a document with disposition:"abandoned"
// is refused with ErrCheckpointCannotResolveAbandoned regardless of any
// other change in this feature.
func TestU3bGuard_AbandonedDocumentStillRefusedByResolve(t *testing.T) {
	dir := t.TempDir()
	name := "checkpoint-u3b-abandoned.json"
	path := filepath.Join(dir, name)
	body := []byte(`{"schema_version":1,"agent":"ship","session_id":"s1","phase":"build","status":"abandoned",` +
		`"disposition":"abandoned","disposition_reason":"stale","disposition_operator":"ship",` +
		`"disposition_at":"2026-08-24T00:00:00Z",` +
		`"created_at":"2026-08-24T00:00:00Z","updated_at":"2026-08-24T00:00:00Z"}`)
	require.NoError(t, os.WriteFile(path, body, 0o644))

	err := events.ResolveCheckpoint(context.Background(), dir, name)
	require.Error(t, err)
	assert.ErrorIs(t, err, backlogiterrors.ErrCheckpointCannotResolveAbandoned)

	after, readErr := os.ReadFile(path)
	require.NoError(t, readErr)
	assert.Equal(t, body, after, "a refused resolve must not change the file's bytes")
}
