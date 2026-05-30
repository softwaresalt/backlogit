package core_test

// 060.004-T: Restore AdoptItem identity on rollback
//
// Rollback identity contract after fix:
//   - When AdoptItem fails after renaming the artifact's .md file (e.g. because
//     applyCrossArtifactRewrites cannot write a dependent artifact's file),
//     the file at the original path must be restored with the ORIGINAL ID in
//     its frontmatter — not the new hierarchical ID that was written to the
//     new path before removal.
//   - Before the fix, rollback uses os.Rename(newMDPath, oldMDPath), which
//     places a file with frontmatter "id: newID" at the old path. FindArtifactPath
//     then cannot find the original item by its old ID.
//   - After the fix, rollback restores the exact original file content so the
//     item remains discoverable by its old ID.

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/core"
)

// TestAdoptItem_RollbackRestoresOriginalID verifies that when AdoptItem fails
// mid-operation, the artifact at the original path retains its original ID in
// frontmatter so it remains discoverable after the failed adoption.
//
// Failure is injected by making a dependent artifact's file read-only, causing
// applyCrossArtifactRewrites to fail after the new .md file has been written
// and the old one removed — the exact rollback scenario described in 060.004-T.
func TestAdoptItem_RollbackRestoresOriginalID(t *testing.T) {
	ws := setupTestWorkspace(t)
	ctx := context.Background()

	feat1, err := core.CreateArtifact(ctx, ws, "Origin feature rollback", "feature")
	require.NoError(t, err)
	feat2, err := core.CreateArtifact(ctx, ws, "Destination feature rollback", "feature")
	require.NoError(t, err)

	// t1 will be adopted; t2 depends on t1 so it is a cross-reference target.
	t1, err := core.CreateArtifact(ctx, ws, "Task to adopt rollback", "task", core.WithParent(feat1.ID))
	require.NoError(t, err)
	t2, err := core.CreateArtifact(ctx, ws, "Dependent task rollback", "task",
		core.WithParent(feat1.ID), core.WithDependencies([]string{t1.ID}))
	require.NoError(t, err)

	// Locate t2's file so we can make it read-only before adopt.
	t2Path, findErr := core.FindArtifactPath(ctx, ws, t2.ID)
	require.NoError(t, findErr)

	// Record the original path of t1 before adoption.
	t1OrigPath, findErr := core.FindArtifactPath(ctx, ws, t1.ID)
	require.NoError(t, findErr)

	// Ensure the directory containing t1 and t2 are writable (needed for rename).
	// Make only t2.md read-only to block applyCrossArtifactRewrites.
	require.NoError(t, os.Chmod(t2Path, 0o444))
	t.Cleanup(func() { _ = os.Chmod(t2Path, 0o644) })

	// Adopt must fail because t2.md is read-only.
	_, adoptErr := core.AdoptItem(ctx, ws, t1.ID, feat2.ID)
	require.Error(t, adoptErr, "AdoptItem must return an error when cross-ref rewrite fails")

	// After the failed adoption, t1 must be findable by its ORIGINAL ID.
	restoredPath, findErr2 := core.FindArtifactPath(ctx, ws, t1.ID)
	require.NoError(t, findErr2,
		"original artifact must be discoverable by old ID after failed adopt rollback")

	// The restored file must be at the original path (or at least discoverable).
	assert.NotEmpty(t, restoredPath)

	// Read the restored file and verify the ID field matches the old ID.
	raw, readErr := os.ReadFile(restoredPath)
	require.NoError(t, readErr)
	assert.Contains(t, string(raw), "id: "+t1.ID,
		"restored file at %q must contain original ID %q\n\nfile content:\n%s",
		restoredPath, t1.ID, string(raw))

	// The file must NOT contain the new (post-adopt) ID.
	// We don't know the exact new ID, but it won't equal the old ID, so if
	// the file contains the old ID and not a mismatched ID, the rollback is correct.
	// Also verify the original path is consistent.
	assert.Equal(t, t1OrigPath, restoredPath,
		"restored file must be at the original path after rollback")

	// Reset t2 permissions before workspace cleanup.
	_ = os.Chmod(t2Path, 0o644)
}
