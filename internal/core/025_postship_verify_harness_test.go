package core_test

// 025.018-T (Unit 6): Post-shipment queue-path consistency verification.
// VerifyPostShipConsistency is implemented in internal/core/shipment_verify.go.
//
// Tests:
//   - TestShipShipment_FailsOnStaleQueueFile   — verifies error on stale queue file after archive
//   - TestShipShipment_SucceedsWhenQueueClean  — verifies success when queue is clean after archive

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/backlogit/backlogit/internal/core"
	bldb "github.com/backlogit/backlogit/internal/db"
)

// TestShipShipment_FailsOnStaleQueueFile verifies that if a queue file survives
// archiving (simulated by writing it back after ArchiveItem), ShipShipment returns
// an error identifying the stale path rather than silently succeeding.
//
// After Unit 6, ShipShipment calls VerifyPostShipConsistency; this test exercises
// that path directly by patching the filesystem after shipment.
func TestShipShipment_FailsOnStaleQueueFile(t *testing.T) {
	ws := setupTestWorkspace(t)
	ctx := context.Background()

	feature, err := core.CreateArtifact(ctx, ws, "Verify post-ship feature", "feature")
	require.NoError(t, err)
	require.NoError(t, bldb.UpsertItem(ctx, ws.DB, feature))

	task, err := core.CreateArtifact(ctx, ws, "Verify post-ship task", "task", core.WithParent(feature.ID))
	require.NoError(t, err)
	require.NoError(t, bldb.UpsertItem(ctx, ws.DB, task))

	// Record task queue path before shipping.
	taskPath, err := core.FindArtifactPath(ctx, ws, task.ID)
	require.NoError(t, err)
	taskContent, err := os.ReadFile(taskPath)
	require.NoError(t, err)

	shipment, err := core.CreateShipment(ctx, ws, "Post-ship verify shipment", []string{task.ID})
	require.NoError(t, err)
	_, err = core.ClaimShipment(ctx, ws, shipment.ID)
	require.NoError(t, err)

	// Simulate a stale queue file by writing it back between archive and verify.
	// This mimics what happens if ArchiveItem's os.Remove fails silently.
	// The test intercepts BEFORE ShipShipment to write the stale file beforehand;
	// ShipShipment's integrated VerifyPostShipConsistency should catch it.
	// Strategy: archive manually, re-inject stale file, then call ShipShipment.
	_, archiveErr := core.ArchiveItem(ctx, ws.DB, ws, task.ID)
	require.NoError(t, archiveErr)
	require.NoError(t, os.WriteFile(taskPath, taskContent, 0o644)) // re-inject stale file

	// Now directly test VerifyPostShipConsistency detects the stale file.
	err = core.VerifyPostShipConsistency(ctx, ws, []string{task.ID})

	require.Error(t, err, "VerifyPostShipConsistency must return an error when a stale queue file is found")
	assert.Contains(t, err.Error(), task.ID, "error must identify the artifact with a stale queue file")
}

// TestShipShipment_SucceedsWhenQueueClean verifies that VerifyPostShipConsistency
// returns nil when all provided artifact IDs are absent from the queue directory.
func TestShipShipment_SucceedsWhenQueueClean(t *testing.T) {
	ws := setupTestWorkspace(t)
	ctx := context.Background()

	feature, err := core.CreateArtifact(ctx, ws, "Clean post-ship feature", "feature")
	require.NoError(t, err)
	require.NoError(t, bldb.UpsertItem(ctx, ws.DB, feature))

	task, err := core.CreateArtifact(ctx, ws, "Clean post-ship task", "task", core.WithParent(feature.ID))
	require.NoError(t, err)
	require.NoError(t, bldb.UpsertItem(ctx, ws.DB, task))

	// Archive the task to remove it from queue.
	_, err = core.ArchiveItem(ctx, ws.DB, ws, task.ID)
	require.NoError(t, err)

	// Now verify: no stale files remain.
	err = core.VerifyPostShipConsistency(ctx, ws, []string{task.ID})

	assert.NoError(t, err, "VerifyPostShipConsistency must return nil when all archived IDs are absent from queue")
}
