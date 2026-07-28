package core

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/atomicfile"
	blerrors "github.com/softwaresalt/backlogit/internal/errors"
	"github.com/softwaresalt/backlogit/internal/events"
	"github.com/softwaresalt/backlogit/internal/models"
)

// countEstimateHistoryEvents returns the number of estimate_history audit lines
// in the item's JSONL log.
func countEstimateHistoryEvents(t *testing.T, ws *Workspace, id string) int {
	t.Helper()
	logPath := events.LogPathForItem(WorkspaceLogsRoot(ws.RootPath), id)
	raw, err := os.ReadFile(logPath)
	require.NoError(t, err)
	count := 0
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		if strings.Contains(line, events.EventEstimateHistory) {
			count++
		}
	}
	return count
}

// TestSizeSeam_NotAppliedRetriesAtomicWriteOnlyEventCountStaysOne asserts that an
// ErrWriteNotApplied on the first atomic write is retried (scoped to the write)
// while the estimate-history audit event — appended BEFORE the write — stays at
// exactly one, proving the composite op is not blindly re-run.
func TestSizeSeam_NotAppliedRetriesAtomicWriteOnlyEventCountStaysOne(t *testing.T) {
	ws, backlogitDir := newSizeEstimationHarnessWorkspace(t)
	seedSizingHarnessArtifact(t, ws, backlogitDir, &models.Artifact{
		ID:           "940.001-T",
		Title:        "Retry-scoped task",
		Status:       models.StatusActive,
		ArtifactType: "task",
		ParentID:     "940-F",
		Priority:     "medium",
	})

	writeAttempts := 0
	previous := sizeSeamAtomicWrite
	sizeSeamAtomicWrite = func(path string, data []byte, durable bool) error {
		writeAttempts++
		if writeAttempts == 1 {
			// Definitely-not-applied: the destination is untouched, safe to retry.
			return blerrors.ErrWriteNotApplied
		}
		return atomicfile.WriteFileAtomicWithOptions(path, data, atomicfile.Options{DurableWrites: durable})
	}
	t.Cleanup(func() { sizeSeamAtomicWrite = previous })

	_, err := SetArtifactSizeWithProvenance(context.Background(), ws, "940.001-T", SizeMutation{
		Size:  stringPtr("M"),
		Actor: ActorContextHuman,
	})
	require.NoError(t, err, "a not-applied first attempt must succeed after the scoped retry")
	assert.Equal(t, 2, writeAttempts, "the atomic write must be retried exactly once")
	assert.Equal(t, 1, countEstimateHistoryEvents(t, ws, "940.001-T"),
		"the audit event must stay exactly one across the write retry")
}

// TestSizeSeam_IndeterminateNotRetriedNoDuplicateEvent asserts an
// ErrWriteIndeterminate is surfaced (not retried) with no duplicate audit event.
func TestSizeSeam_IndeterminateNotRetriedNoDuplicateEvent(t *testing.T) {
	ws, backlogitDir := newSizeEstimationHarnessWorkspace(t)
	seedSizingHarnessArtifact(t, ws, backlogitDir, &models.Artifact{
		ID:           "941.001-T",
		Title:        "Indeterminate task",
		Status:       models.StatusActive,
		ArtifactType: "task",
		ParentID:     "941-F",
		Priority:     "medium",
	})

	writeAttempts := 0
	previous := sizeSeamAtomicWrite
	sizeSeamAtomicWrite = func(string, []byte, bool) error {
		writeAttempts++
		return blerrors.ErrWriteIndeterminate
	}
	t.Cleanup(func() { sizeSeamAtomicWrite = previous })

	_, err := SetArtifactSizeWithProvenance(context.Background(), ws, "941.001-T", SizeMutation{
		Size:  stringPtr("L"),
		Actor: ActorContextHuman,
	})
	require.Error(t, err)
	assert.True(t, blerrors.IsWriteIndeterminate(err), "an indeterminate write must be surfaced to the caller")
	assert.Equal(t, 1, writeAttempts, "an indeterminate write must NOT be retried")
	assert.Equal(t, 1, countEstimateHistoryEvents(t, ws, "941.001-T"),
		"no duplicate audit event on the indeterminate path")
}

// TestSizeSeam_DurableOffPathUnchanged asserts the default (durable-off) path
// still succeeds and appends exactly one audit event.
func TestSizeSeam_DurableOffPathUnchanged(t *testing.T) {
	ws, backlogitDir := newSizeEstimationHarnessWorkspace(t)
	seedSizingHarnessArtifact(t, ws, backlogitDir, &models.Artifact{
		ID:           "942.001-T",
		Title:        "Durable-off task",
		Status:       models.StatusActive,
		ArtifactType: "task",
		ParentID:     "942-F",
		Priority:     "medium",
	})

	_, err := SetArtifactSizeWithProvenance(context.Background(), ws, "942.001-T", SizeMutation{
		Size:  stringPtr("S"),
		Actor: ActorContextHuman,
	})
	require.NoError(t, err)
	assert.Equal(t, 1, countEstimateHistoryEvents(t, ws, "942.001-T"))
}
