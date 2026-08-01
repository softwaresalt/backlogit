package core_test

// 025.014-T (Unit 1): Harden archive and shipment tests for queue-path absence.
// These tests assert that archive and ship operations do not leave stale queue files.
// They PASS with current code since ArchiveItem already removes the original file,
// but they are absent from the test suite and serve as safety nets for Unit 6.

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/core"
	bldb "github.com/softwaresalt/backlogit/internal/db"
)

// TestArchiveItem_QueuePathAbsentAfterArchive verifies the original queue file
// no longer exists after ArchiveItem completes.
func TestArchiveItem_QueuePathAbsentAfterArchive(t *testing.T) {
	// Arrange
	ws := setupArchiveWorkspace(t)
	ctx := context.Background()
	originalPath, err := core.FindArtifactPath(ctx, ws, "001-T")
	require.NoError(t, err)
	require.FileExists(t, originalPath)

	// Act
	_, err = core.ArchiveItem(ctx, ws.DB, ws, "001-T")

	// Assert
	require.NoError(t, err)
	assert.NoFileExists(t, originalPath, "queue file must be absent after archiving")
}

// TestArchiveItem_NoDuplicateAcrossQueueArchive verifies that after archiving,
// the artifact ID exists in exactly one location (archive), not both.
func TestArchiveItem_NoDuplicateAcrossQueueArchive(t *testing.T) {
	// Arrange
	ws := setupArchiveWorkspace(t)
	ctx := context.Background()

	// Act
	record, err := core.ArchiveItem(ctx, ws.DB, ws, "001-T")
	require.NoError(t, err)

	// Assert: archive file exists
	assert.FileExists(t, record.ArchivePath)

	// Assert: original queue path is gone
	assert.NoFileExists(t, record.OriginalPath, "queue file must be absent after archiving to prevent duplication")

	// Assert: walking the archive dir yields the file, queue dir does not
	queueDir := filepath.Join(ws.RootPath, ".backlogit", "queue")
	archiveDir := filepath.Join(ws.RootPath, ".backlogit", "archive")

	queueCount := countFilesWithPrefix(t, queueDir, "001-T")
	archiveCount := countFilesWithPrefix(t, archiveDir, "001-T")
	assert.Equal(t, 0, queueCount, "artifact ID must not appear in queue dir after archiving")
	assert.Equal(t, 1, archiveCount, "artifact ID must appear exactly once in archive dir")
}

// TestShipShipment_QueuePathAbsentAfterShip verifies that shipping a shipment
// removes all released items from the queue directory.
func TestShipShipment_QueuePathAbsentAfterShip(t *testing.T) {
	// Arrange
	ws := setupTestWorkspace(t)
	ctx := context.Background()

	feature, err := core.CreateArtifact(ctx, ws, "Ship queue test feature", "feature")
	require.NoError(t, err)
	require.NoError(t, bldb.UpsertItem(ctx, ws.DB, feature))

	task, err := core.CreateArtifact(ctx, ws, "Ship queue test task", "task", core.WithParent(feature.ID))
	require.NoError(t, err)
	require.NoError(t, bldb.UpsertItem(ctx, ws.DB, task))

	// Record queue paths before shipping
	featureQueuePath, err := core.FindArtifactPath(ctx, ws, feature.ID)
	require.NoError(t, err)
	taskQueuePath, err := core.FindArtifactPath(ctx, ws, task.ID)
	require.NoError(t, err)

	// Feature-inclusive manifest: this test's regression target is the
	// archive-deletion bug (stale queue file after ship), not covering-feature
	// archival semantics, so the feature is listed as an explicit member to
	// keep its legitimate archival under the membership contract (133-F).
	shipment, err := core.CreateShipment(ctx, ws, "Ship queue test shipment", []string{feature.ID, task.ID})
	require.NoError(t, err)
	_, err = core.ClaimShipment(ctx, ws, shipment.ID)
	require.NoError(t, err)

	// Act
	result, err := core.ShipShipment(ctx, ws, shipment.ID, nil)

	// Assert: queue files removed
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.NoFileExists(t, featureQueuePath, "feature queue file must be absent after shipping")
	assert.NoFileExists(t, taskQueuePath, "task queue file must be absent after shipping")

	// Assert: archive files exist (regression guard for the archive-deletion bug
	// where ArchiveItem deleted the file it just wrote when the item was already
	// routed to archive/ by the done→archive registry mapping).
	archiveDir := filepath.Join(ws.RootPath, ".backlogit", "archive")
	featureArchiveCount := countFilesWithPrefix(t, archiveDir, feature.ID)
	taskArchiveCount := countFilesWithPrefix(t, archiveDir, task.ID)
	assert.Equal(t, 1, featureArchiveCount, "feature must have exactly one archive file after shipping")
	assert.Equal(t, 1, taskArchiveCount, "task must have exactly one archive file after shipping")
}

// countFilesWithPrefix returns how many files in dir have a base name starting with prefix.
func countFilesWithPrefix(t *testing.T, dir, prefix string) int {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return 0
	}
	require.NoError(t, err)
	count := 0
	for _, e := range entries {
		if !e.IsDir() && len(e.Name()) >= len(prefix) && e.Name()[:len(prefix)] == prefix {
			count++
		}
	}
	return count
}
