package core

// 153.005-T / S1 U5 — Pin conforming+resolved checkpoint double-refusal
// invariant (6FA45E69, test-only).
//
// This unit pins the I3 row-3 double-refusal class as a tested invariant:
// a conforming, schema-valid checkpoint with status "resolved" is refused by
// BOTH AbandonCheckpoint (ErrCheckpointNotActive — abandon requires active)
// AND QuarantineCheckpoint (ErrCheckpointUseAbandon — valid conforming docs
// must use abandon, not quarantine). No production delta.

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	blerrors "github.com/softwaresalt/backlogit/internal/errors"
	"github.com/softwaresalt/backlogit/internal/events"
)

// TestU5_S1_ConformingResolvedDoubleRefusal (153.005-T / S1 U5) pins the
// I3 row-3 invariant: a conforming, schema-valid checkpoint whose status is
// "resolved" is refused by both disposition verbs.
//
//   - AbandonCheckpoint must return ErrCheckpointNotActive: abandon requires
//     an active checkpoint; a resolved checkpoint is not active.
//   - QuarantineCheckpoint must return ErrCheckpointUseAbandon: quarantine
//     is reserved for malformed/non-conforming targets; a valid conforming
//     document must go through abandon, not quarantine.
func TestU5_S1_ConformingResolvedDoubleRefusal(t *testing.T) {
	ws := newCheckpointTargetTestWorkspace(t)
	cpDir := filepath.Join(ws.RootPath, ".backlogit", checkpointsSubdir)

	// Build a conforming, schema-valid checkpoint whose status is "resolved".
	// Using a value time to satisfy the required-non-zero validator.
	now := time.Date(2026, 9, 3, 0, 0, 0, 0, time.UTC)
	cp := &events.CheckpointV1{
		SchemaVersion: 1,
		Agent:         "ship",
		SessionID:     "u5-s1-double-refusal",
		Phase:         "build",
		Status:        "resolved",
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	data, err := json.Marshal(cp)
	require.NoError(t, err)
	filename := "checkpoint-u5-resolved.json"
	require.NoError(t, os.MkdirAll(cpDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(cpDir, filename), data, 0o644))

	ew := newDispositionEventWriter(t, ws)
	ctx := context.Background()

	// Row 1: AbandonCheckpoint must refuse with ErrCheckpointNotActive.
	// A resolved checkpoint is not "active" — abandon is not the right verb.
	abandonErr := AbandonCheckpoint(ctx, ws, ew, filename, "double-refusal-test", "operator@example.com")
	require.Error(t, abandonErr, "AbandonCheckpoint must refuse a resolved checkpoint")
	assert.True(t, errors.Is(abandonErr, blerrors.ErrCheckpointNotActive),
		"AbandonCheckpoint on a resolved doc must return ErrCheckpointNotActive, got: %v", abandonErr)

	// Confirm the file is unchanged.
	after, readErr := os.ReadFile(filepath.Join(cpDir, filename))
	require.NoError(t, readErr)
	assert.Equal(t, data, after, "file must be byte-unchanged after a refused abandon")

	// Row 2: QuarantineCheckpoint must refuse with ErrCheckpointUseAbandon.
	// A valid conforming document must go through abandon — quarantine is
	// reserved for malformed/non-conforming targets.
	quarantineErr := QuarantineCheckpoint(ctx, ws, ew, filename, "double-refusal-test", "operator@example.com")
	require.Error(t, quarantineErr, "QuarantineCheckpoint must refuse a conforming resolved checkpoint")
	assert.True(t, errors.Is(quarantineErr, blerrors.ErrCheckpointUseAbandon),
		"QuarantineCheckpoint on a conforming resolved doc must return ErrCheckpointUseAbandon, got: %v", quarantineErr)

	// Confirm the file is still unchanged.
	after2, readErr2 := os.ReadFile(filepath.Join(cpDir, filename))
	require.NoError(t, readErr2)
	assert.Equal(t, data, after2, "file must be byte-unchanged after a refused quarantine")
}
