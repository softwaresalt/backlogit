package core_test

// 066.003-T (Unit 3) harness: ArchiveItem refuses to overwrite a distinct
// occupied destination. RED until ArchiveItem guards the destination by path
// equality and returns ErrArchiveDestinationOccupied for a foreign occupant.

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/core"
	bldb "github.com/softwaresalt/backlogit/internal/db"
	blerrors "github.com/softwaresalt/backlogit/internal/errors"
)

func TestArchiveItem_RefusesDistinctOccupiedDestination(t *testing.T) {
	ws := setupTestWorkspace(t)
	ctx := context.Background()

	feature, err := core.CreateArtifact(ctx, ws, "Queue feature to archive", "feature")
	require.NoError(t, err)
	require.NoError(t, bldb.UpsertItem(ctx, ws.DB, feature))

	// A DIFFERENT item already occupies the archive destination: same root ID,
	// different title — the 066-F silent-overwrite data-loss scenario.
	archiveDir := filepath.Join(core.WorkspaceStorageRoot(ws.RootPath), "archive")
	require.NoError(t, os.MkdirAll(archiveDir, 0o755))
	foreignPath := filepath.Join(archiveDir, feature.ID+".md")
	foreign := "---\n" +
		"id: \"" + feature.ID + "\"\n" +
		"title: \"Pre-existing DIFFERENT archived item\"\n" +
		"status: archived\n" +
		"artifact_type: feature\n" +
		"level: 1\n" +
		"---\nForeign archived body.\n"
	require.NoError(t, os.WriteFile(foreignPath, []byte(foreign), 0o644))

	_, err = core.ArchiveItem(ctx, ws.DB, ws, feature.ID)
	require.Error(t, err, "archiving onto a distinct occupied destination must fail, not silently overwrite")
	assert.True(t, errors.Is(err, blerrors.ErrArchiveDestinationOccupied),
		"archive must refuse with ErrArchiveDestinationOccupied; got: %v", err)

	// The existing archived item must remain intact (no data loss).
	got, err := os.ReadFile(foreignPath)
	require.NoError(t, err)
	assert.Contains(t, string(got), "Pre-existing DIFFERENT archived item",
		"the foreign archived item must be left untouched")
	assert.Contains(t, string(got), "Foreign archived body.")

	// The source queue copy must remain in place (archive refused, nothing moved).
	assert.FileExists(t, filepath.Join(core.WorkspaceStorageRoot(ws.RootPath), "queue", feature.ID+".md"))
}

// TestArchiveItem_SameItemHalfArchiveRecoverySucceeds is a regression guard:
// the legitimate 060.002-T half-archive recovery (the SAME logical item already
// has an archive copy while the queue copy survives) must still succeed and must
// NOT be misclassified as a foreign-destination collision.
func TestArchiveItem_SameItemHalfArchiveRecoverySucceeds(t *testing.T) {
	ws := setupTestWorkspace(t)
	ctx := context.Background()

	feature, err := core.CreateArtifact(ctx, ws, "Recoverable feature", "feature")
	require.NoError(t, err)
	require.NoError(t, bldb.UpsertItem(ctx, ws.DB, feature))

	queuePath, err := core.FindArtifactPath(ctx, ws, feature.ID)
	require.NoError(t, err)
	content, err := os.ReadFile(queuePath)
	require.NoError(t, err)

	// Same logical item already copied into archive (same id AND same title).
	archiveDir := filepath.Join(core.WorkspaceStorageRoot(ws.RootPath), "archive")
	require.NoError(t, os.MkdirAll(archiveDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(archiveDir, feature.ID+".md"), content, 0o644))

	rec, err := core.ArchiveItem(ctx, ws.DB, ws, feature.ID)
	require.NoError(t, err, "half-archive recovery of the SAME item must not be refused")
	require.NotNil(t, rec)
	assert.FileExists(t, filepath.Join(archiveDir, feature.ID+".md"))
	assert.NoFileExists(t, queuePath, "queue copy should be drained after successful recovery")
}
