package integration_test

// 066.005-T (Unit 6) harness: end-to-end reproduction of the root-ID conflict.
// Two DIFFERENT top-level items share one root ID — one in queue, one in
// archive — and all three integrity surfaces must engage:
//   (a) doctor reports the root-ID collision (066.001-T / U1)
//   (b) CreateArtifact refuses the colliding allocation (066.002-T / U2)
//   (c) ArchiveItem refuses to overwrite the distinct destination (066.003-T / U3)
//
// RED on pre-fix main; green after U1-U3.

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/config"
	"github.com/softwaresalt/backlogit/internal/core"
	"github.com/softwaresalt/backlogit/internal/db"
	blerrors "github.com/softwaresalt/backlogit/internal/errors"
)

func TestRootIDConflict_EndToEndIntegrity(t *testing.T) {
	ctx := context.Background()

	// Hermetic workspace (no reliance on repo state).
	root := t.TempDir()
	backlogitDir := filepath.Join(root, ".backlogit")
	require.NoError(t, os.MkdirAll(backlogitDir, 0o755))
	require.NoError(t, config.WriteDefaults(backlogitDir))

	ws, err := core.NewWorkspace(ctx, root)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, ws.Close()) })

	// 1. Create feature A and archive it -> archive/<id>.md.
	featureA, err := core.CreateArtifact(ctx, ws, "Original feature A", "feature")
	require.NoError(t, err)
	require.NoError(t, db.UpsertItem(ctx, ws.DB, featureA))
	rootID := featureA.ID

	_, err = core.ArchiveItem(ctx, ws.DB, ws, rootID)
	require.NoError(t, err)

	// 2. Materialize a DIFFERENT feature B in the queue sharing the same root ID.
	queueDir := filepath.Join(backlogitDir, "queue")
	require.NoError(t, os.MkdirAll(queueDir, 0o755))
	featureB := "---\n" +
		"id: \"" + rootID + "\"\n" +
		"title: \"Conflicting feature B\"\n" +
		"status: active\n" +
		"artifact_type: feature\n" +
		"level: 1\n" +
		"---\nFeature B body.\n"
	require.NoError(t, os.WriteFile(filepath.Join(queueDir, rootID+".md"), []byte(featureB), 0o644))

	// 3. Sync the index from the canonical files (PK-collapse keeps one row).
	_, err = db.Rehydrate(ctx, backlogitDir, ws.DB)
	require.NoError(t, err)

	// (a) Doctor must flag the root-ID collision.
	report, err := core.Doctor(ctx, ws, &core.DoctorOptions{CheckOrphans: true, CheckDuplicates: true})
	require.NoError(t, err)
	var rootCollision bool
	for _, f := range report.Findings {
		if f.ArtifactID == rootID && f.Type == core.FindingRootIDCollision {
			rootCollision = true
		}
	}
	assert.True(t, rootCollision, "doctor must report a root-ID collision for %s", rootID)

	// Simulate the stale-cache window (Finding 2): the allocator no longer sees
	// the archived ordinal and will re-mint the same root ID.
	_, err = ws.DB.ExecContext(ctx, "DELETE FROM items WHERE id = ?", rootID)
	require.NoError(t, err)

	// (b) Creation must refuse to reuse the colliding ID.
	_, err = core.CreateArtifact(ctx, ws, "Re-minted feature", "feature")
	require.Error(t, err)
	assert.True(t, errors.Is(err, blerrors.ErrIDCollision),
		"create must fail loud with ErrIDCollision; got: %v", err)

	// (c) Archiving feature B must refuse to overwrite archived feature A.
	_, err = core.ArchiveItem(ctx, ws.DB, ws, rootID)
	require.Error(t, err)
	assert.True(t, errors.Is(err, blerrors.ErrArchiveDestinationOccupied),
		"archive must fail loud with ErrArchiveDestinationOccupied; got: %v", err)

	// The archived feature A must remain intact (no silent overwrite / data loss).
	archived, err := os.ReadFile(filepath.Join(backlogitDir, "archive", rootID+".md"))
	require.NoError(t, err)
	assert.Contains(t, string(archived), "Original feature A",
		"archived feature A must not be overwritten by feature B")
}
