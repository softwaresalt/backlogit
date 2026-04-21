package core_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/core"
	bldb "github.com/softwaresalt/backlogit/internal/db"
)

// TestAdoptItem_CrossReferenceRehydrationConsistency verifies that after
// AdoptItem rewrites an artifact's hierarchical ID, every other artifact whose
// frontmatter references the old ID is also updated on disk. When Rehydrate
// rebuilds the SQLite index from those Markdown files, all dependency edges must
// reference the new ID — not the stale old ID.
//
// Without the fix, the dependency edge in the DB would be reverted to the old ID
// on the next sync because T2's Markdown file still references T1's original ID.
func TestAdoptItem_CrossReferenceRehydrationConsistency(t *testing.T) {
	ws := setupTestWorkspace(t)
	ctx := context.Background()

	feat1, err := core.CreateArtifact(ctx, ws, "Origin feature", "feature")
	require.NoError(t, err)
	feat2, err := core.CreateArtifact(ctx, ws, "Destination feature", "feature")
	require.NoError(t, err)

	// T1 will be adopted into feat2.
	t1, err := core.CreateArtifact(ctx, ws, "Task to adopt", "task", core.WithParent(feat1.ID))
	require.NoError(t, err)

	// T2 depends on T1; its Markdown frontmatter holds t1.ID as a dependency.
	t2, err := core.CreateArtifact(ctx, ws, "Dependent task", "task",
		core.WithParent(feat1.ID), core.WithDependencies([]string{t1.ID}))
	require.NoError(t, err)

	// Adopt T1 into feat2.  The ID will change because the feature prefix changes.
	result, err := core.AdoptItem(ctx, ws, t1.ID, feat2.ID)
	require.NoError(t, err)
	require.NotEmpty(t, result.NewID, "adopt must assign a new hierarchical ID")

	newT1ID := result.NewID
	require.NotEqual(t, t1.ID, newT1ID, "ID must differ after cross-feature adoption")

	// Force a full rehydration from Markdown source files.
	backlogitDir := filepath.Join(ws.RootPath, ".backlogit")
	_, err = bldb.Rehydrate(ctx, backlogitDir, ws.DB)
	require.NoError(t, err)

	// After rehydration, T2's dep edge must reference the new ID, not the old one.
	deps, err := bldb.GetDependencies(ctx, ws.DB, t2.ID)
	require.NoError(t, err)

	depTargets := make(map[string]bool, len(deps))
	for _, d := range deps {
		depTargets[d.DependsOn] = true
	}
	assert.True(t, depTargets[newT1ID],
		"T2 should depend on the new adopted ID (%s) after rehydration", newT1ID)
	assert.False(t, depTargets[t1.ID],
		"T2 must not reference the stale old ID (%s) after rehydration", t1.ID)
}

// TestAdoptItem_CrossReference_NoIDChange verifies that when AdoptItem produces
// no ID change (QueueLayout is nil so nextID falls back to oldID), the
// cross-reference scanner is not invoked. This exercises the no-ID-change branch
// of AdoptItem without triggering the panicking stub.
func TestAdoptItem_CrossReference_NoIDChange(t *testing.T) {
	ws := setupTestWorkspace(t)
	ctx := context.Background()

	feat1, err := core.CreateArtifact(ctx, ws, "Origin feature", "feature")
	require.NoError(t, err)
	feat2, err := core.CreateArtifact(ctx, ws, "Destination feature", "feature")
	require.NoError(t, err)

	task, err := core.CreateArtifact(ctx, ws, "Task to re-parent", "task",
		core.WithParent(feat1.ID))
	require.NoError(t, err)

	// Nil-out QueueLayout so the ID generation path is skipped and newID == oldID.
	ws.Config.QueueLayout = nil

	// Should not panic because findCrossArtifactReferences is not reached.
	result, err := core.AdoptItem(ctx, ws, task.ID, feat2.ID)
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, task.ID, result.NewID,
		"NewID must equal the original ID when QueueLayout is nil")
	assert.Equal(t, feat2.ID, result.NewParentID)
}

// TestAdoptItem_CrossReference_ResultPopulated verifies that after a successful
// adoption with cross-reference rewrites, AdoptItemResult.RewrittenArtifactIDs
// is populated with the IDs of every artifact whose Markdown file was updated.
func TestAdoptItem_CrossReference_ResultPopulated(t *testing.T) {
	ws := setupTestWorkspace(t)
	ctx := context.Background()

	feat1, err := core.CreateArtifact(ctx, ws, "Origin feature", "feature")
	require.NoError(t, err)
	feat2, err := core.CreateArtifact(ctx, ws, "Destination feature", "feature")
	require.NoError(t, err)

	t1, err := core.CreateArtifact(ctx, ws, "Task to adopt", "task",
		core.WithParent(feat1.ID))
	require.NoError(t, err)

	// T2 depends on T1 → will be rewritten during adoption.
	t2, err := core.CreateArtifact(ctx, ws, "Referencing task", "task",
		core.WithParent(feat1.ID), core.WithDependencies([]string{t1.ID}))
	require.NoError(t, err)

	result, err := core.AdoptItem(ctx, ws, t1.ID, feat2.ID)
	require.NoError(t, err)

	assert.Contains(t, result.RewrittenArtifactIDs, t2.ID,
		"RewrittenArtifactIDs should include every artifact whose file was updated")
}
