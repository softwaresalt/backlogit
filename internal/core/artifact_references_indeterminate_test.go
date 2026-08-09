package core

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	blerrors "github.com/softwaresalt/backlogit/internal/errors"
	"github.com/softwaresalt/backlogit/internal/models"
)

// TestApplyCrossArtifactRewrites_RestoresIndeterminateFailingWrite is the F6
// regression: when the write of the CURRENT update fails as ErrWriteIndeterminate
// (rename committed, durability flush failed → the file is possibly-mutated), the
// failing update must itself be rolled back to its snapshot, not just the
// previously written updates. Otherwise the markdown stays changed while the SQL
// transaction rolls back — an on-disk/DB inconsistency.
//
// Must not run with t.Parallel: this test swaps the package-global
// applyCrossRefWriteFn seam read on the production write path.
func TestApplyCrossArtifactRewrites_RestoresIndeterminateFailingWrite(t *testing.T) {
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()

	feat, err := CreateArtifact(ctx, ws, "Feature", "feature")
	require.NoError(t, err)
	ref, err := CreateArtifact(ctx, ws, "Indeterminate artifact", "task", WithParent(feat.ID))
	require.NoError(t, err)
	refPath := findArtifactPathDirect(ws, ref.ID)
	require.NotEmpty(t, refPath)
	snapshot, err := os.ReadFile(refPath)
	require.NoError(t, err)

	updated := *ref
	updated.Dependencies = []models.DependencyEdge{{ID: "indeterminate-dep", Type: "blocks"}}

	// Simulate ErrWriteIndeterminate: the rename committed (file mutated on disk)
	// but the post-rename durability flush failed, so the outcome is uncertain.
	orig := applyCrossRefWriteFn
	applyCrossRefWriteFn = func(_ *models.Artifact, filePath string, _ bool) error {
		require.NoError(t, os.WriteFile(filePath, []byte("MUTATED-BY-INDETERMINATE-WRITE\n"), 0o644))
		return fmt.Errorf("apply cross-ref durable write: %w", blerrors.ErrWriteIndeterminate)
	}
	t.Cleanup(func() { applyCrossRefWriteFn = orig })

	tx, err := ws.DB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback() //nolint:errcheck

	applyErr := applyCrossArtifactRewrites(ctx, tx, ws, []crossRefUpdate{
		{artifact: &updated, filePath: refPath, snapshotRaw: snapshot},
	})
	require.Error(t, applyErr, "apply must fail when the durable write is indeterminate")
	assert.True(t, blerrors.IsWriteIndeterminate(applyErr), "the ErrWriteIndeterminate class must be preserved")

	restored, err := os.ReadFile(refPath)
	require.NoError(t, err)
	assert.Equal(t, string(snapshot), string(restored),
		"an indeterminate write that mutated the file must be rolled back to the pre-apply snapshot")
}
