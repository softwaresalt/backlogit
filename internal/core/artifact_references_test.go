package core

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	bldb "github.com/softwaresalt/backlogit/internal/db"
	"github.com/softwaresalt/backlogit/internal/models"
)

// ── findCrossArtifactReferences ──────────────────────────────────────────────

func TestFindCrossArtifactReferences_ParentIDRewrite(t *testing.T) {
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()

	feat, err := CreateArtifact(ctx, ws, "Root feature", "feature")
	require.NoError(t, err)
	t1, err := CreateArtifact(ctx, ws, "Task to adopt", "task", WithParent(feat.ID))
	require.NoError(t, err)
	// Subtask whose parent_id references t1 — should be collected and rewritten.
	st1, err := CreateArtifact(ctx, ws, "Subtask", "subtask", WithParent(t1.ID))
	require.NoError(t, err)

	newID := feat.ID + ".099-T"

	updates, err := findCrossArtifactReferences(ctx, ws, t1.ID, newID)
	require.NoError(t, err)

	require.Len(t, updates, 1, "subtask referencing adopted task as parent should be collected")
	assert.Equal(t, st1.ID, updates[0].artifact.ID)
	assert.Equal(t, newID, updates[0].artifact.ParentID, "parent_id must be rewritten to newID")
	assert.NotEmpty(t, updates[0].snapshotRaw, "snapshot bytes must be captured for rollback")
	assert.NotEmpty(t, updates[0].filePath, "filePath must be populated")
}

func TestFindCrossArtifactReferences_DependenciesRewrite(t *testing.T) {
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()

	feat, err := CreateArtifact(ctx, ws, "Root feature", "feature")
	require.NoError(t, err)
	t1, err := CreateArtifact(ctx, ws, "Task to adopt", "task", WithParent(feat.ID))
	require.NoError(t, err)
	t2, err := CreateArtifact(ctx, ws, "Dependent task", "task",
		WithParent(feat.ID), WithDependencies([]string{t1.ID}))
	require.NoError(t, err)

	newID := feat.ID + ".099-T"

	updates, err := findCrossArtifactReferences(ctx, ws, t1.ID, newID)
	require.NoError(t, err)

	require.Len(t, updates, 1)
	assert.Equal(t, t2.ID, updates[0].artifact.ID)
	assert.Contains(t, updates[0].artifact.Dependencies, newID,
		"dependencies should contain the new ID")
	assert.NotContains(t, updates[0].artifact.Dependencies, t1.ID,
		"oldID should be replaced, not retained")
}

func TestFindCrossArtifactReferences_LinksRewrite(t *testing.T) {
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()

	feat, err := CreateArtifact(ctx, ws, "Root feature", "feature")
	require.NoError(t, err)
	t1, err := CreateArtifact(ctx, ws, "Task to adopt", "task", WithParent(feat.ID))
	require.NoError(t, err)
	t2, err := CreateArtifact(ctx, ws, "Linking task", "task", WithParent(feat.ID))
	require.NoError(t, err)

	// Establish t2 → t1 semantic link via exported helper.
	require.NoError(t, AddArtifactLink(ctx, ws, t2.ID, t1.ID, "related_to"))

	newID := feat.ID + ".099-T"

	updates, err := findCrossArtifactReferences(ctx, ws, t1.ID, newID)
	require.NoError(t, err)

	require.Len(t, updates, 1)
	assert.Equal(t, t2.ID, updates[0].artifact.ID)
	require.Len(t, updates[0].artifact.Links, 1)
	assert.Equal(t, newID, updates[0].artifact.Links[0].TargetID,
		"link target must be rewritten to newID")
}

func TestFindCrossArtifactReferences_NoOp(t *testing.T) {
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()

	feat, err := CreateArtifact(ctx, ws, "Root feature", "feature")
	require.NoError(t, err)
	t1, err := CreateArtifact(ctx, ws, "Task to adopt", "task", WithParent(feat.ID))
	require.NoError(t, err)
	// Unrelated task: no references to t1.
	_, err = CreateArtifact(ctx, ws, "Unrelated task", "task", WithParent(feat.ID))
	require.NoError(t, err)

	newID := feat.ID + ".099-T"

	updates, err := findCrossArtifactReferences(ctx, ws, t1.ID, newID)
	require.NoError(t, err)
	assert.Empty(t, updates, "artifact with no references to oldID should not be collected")
}

func TestFindCrossArtifactReferences_SelfExclusion(t *testing.T) {
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()

	feat, err := CreateArtifact(ctx, ws, "Root feature", "feature")
	require.NoError(t, err)
	t1, err := CreateArtifact(ctx, ws, "Task to adopt", "task", WithParent(feat.ID))
	require.NoError(t, err)
	// Create a referencing artifact so updates is non-empty and we can inspect it.
	_, err = CreateArtifact(ctx, ws, "Dep task", "task",
		WithParent(feat.ID), WithDependencies([]string{t1.ID}))
	require.NoError(t, err)

	newID := feat.ID + ".099-T"

	updates, err := findCrossArtifactReferences(ctx, ws, t1.ID, newID)
	require.NoError(t, err)

	for _, u := range updates {
		assert.NotEqual(t, t1.ID, u.artifact.ID,
			"adopted artifact itself must be excluded from updates")
		assert.NotEqual(t, newID, u.artifact.ID,
			"newID artifact must be excluded from updates")
	}
}

func TestFindCrossArtifactReferences_ExactMatchOnly(t *testing.T) {
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()

	feat, err := CreateArtifact(ctx, ws, "Root feature", "feature")
	require.NoError(t, err)
	t1, err := CreateArtifact(ctx, ws, "Task to adopt", "task", WithParent(feat.ID))
	require.NoError(t, err)

	// t2: dependency contains t1.ID as a substring but is not an exact match.
	t2, err := CreateArtifact(ctx, ws, "Near-miss task", "task", WithParent(feat.ID))
	require.NoError(t, err)
	t2.Dependencies = []string{t1.ID + "-ext"}
	t2Path := findArtifactPathDirect(ws, t2.ID)
	require.NotEmpty(t, t2Path)
	require.NoError(t, WriteArtifactFile(t2, t2Path))
	require.NoError(t, bldb.UpsertItem(ctx, ws.DB, t2))

	// t3: exact match — should be collected.
	t3, err := CreateArtifact(ctx, ws, "Exact-match task", "task",
		WithParent(feat.ID), WithDependencies([]string{t1.ID}))
	require.NoError(t, err)

	newID := feat.ID + ".099-T"

	updates, err := findCrossArtifactReferences(ctx, ws, t1.ID, newID)
	require.NoError(t, err)

	ids := make(map[string]bool, len(updates))
	for _, u := range updates {
		ids[u.artifact.ID] = true
	}
	assert.True(t, ids[t3.ID], "exact-match artifact should be collected")
	assert.False(t, ids[t2.ID], "partial-match artifact should not be collected")
}

func TestFindCrossArtifactReferences_MultipleRefs(t *testing.T) {
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()

	feat, err := CreateArtifact(ctx, ws, "Root feature", "feature")
	require.NoError(t, err)
	t1, err := CreateArtifact(ctx, ws, "Task to adopt", "task", WithParent(feat.ID))
	require.NoError(t, err)

	// t2 has t1.ID listed twice in its dependency list.
	t2, err := CreateArtifact(ctx, ws, "Multi-ref task", "task", WithParent(feat.ID))
	require.NoError(t, err)
	t2.Dependencies = []string{t1.ID, "other-dep", t1.ID}
	t2Path := findArtifactPathDirect(ws, t2.ID)
	require.NotEmpty(t, t2Path)
	require.NoError(t, WriteArtifactFile(t2, t2Path))
	require.NoError(t, bldb.UpsertItem(ctx, ws.DB, t2))

	newID := feat.ID + ".099-T"

	updates, err := findCrossArtifactReferences(ctx, ws, t1.ID, newID)
	require.NoError(t, err)

	require.Len(t, updates, 1)
	for _, dep := range updates[0].artifact.Dependencies {
		assert.NotEqual(t, t1.ID, dep,
			"no occurrence of oldID should remain after rewrite")
	}
}

func TestFindCrossArtifactReferences_SameID_ReturnsEmpty(t *testing.T) {
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()

	feat, err := CreateArtifact(ctx, ws, "Root feature", "feature")
	require.NoError(t, err)
	t1, err := CreateArtifact(ctx, ws, "Task", "task", WithParent(feat.ID))
	require.NoError(t, err)

	// oldID == newID: no rewrite is meaningful; expect empty result.
	updates, err := findCrossArtifactReferences(ctx, ws, t1.ID, t1.ID)
	require.NoError(t, err)
	assert.Empty(t, updates, "identical oldID and newID should yield no updates")
}

// ── applyCrossArtifactRewrites ───────────────────────────────────────────────

func TestApplyCrossArtifactRewrites_WritesFile(t *testing.T) {
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()

	const newDepID = "apply-new-dep-id"
	const oldDepID = "apply-old-dep-id"

	feat, err := CreateArtifact(ctx, ws, "Feature", "feature")
	require.NoError(t, err)
	ref, err := CreateArtifact(ctx, ws, "Referencing artifact", "task", WithParent(feat.ID))
	require.NoError(t, err)

	// Overwrite the file with oldDepID in the dependency list.
	ref.Dependencies = []string{oldDepID}
	refPath := findArtifactPathDirect(ws, ref.ID)
	require.NotEmpty(t, refPath)
	require.NoError(t, WriteArtifactFile(ref, refPath))

	original, err := os.ReadFile(refPath)
	require.NoError(t, err)

	updated := *ref
	updated.Dependencies = []string{newDepID}

	tx, err := ws.DB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback() //nolint:errcheck

	require.NoError(t, applyCrossArtifactRewrites(ctx, tx, ws, []crossRefUpdate{
		{artifact: &updated, filePath: refPath, snapshotRaw: original},
	}))
	require.NoError(t, tx.Commit())

	written, err := os.ReadFile(refPath)
	require.NoError(t, err)
	assert.NotEqual(t, string(original), string(written), "file should be rewritten")
	assert.Contains(t, string(written), newDepID,
		"new dep ID should appear in the rewritten file")
	assert.NotContains(t, string(written), oldDepID,
		"old dep ID should not appear in the rewritten file")
}

func TestApplyCrossArtifactRewrites_RefreshesDepsLinks(t *testing.T) {
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()

	const newDepTarget = "refresh-new-dep-target"
	const newLinkTarget = "refresh-new-link-target"
	const oldDep = "refresh-old-dep"
	const oldLink = "refresh-old-link"

	feat, err := CreateArtifact(ctx, ws, "Feature", "feature")
	require.NoError(t, err)
	ref, err := CreateArtifact(ctx, ws, "Referencing artifact", "task", WithParent(feat.ID))
	require.NoError(t, err)
	refPath := findArtifactPathDirect(ws, ref.ID)
	require.NotEmpty(t, refPath)

	// Pre-insert stale dep and link rows that the apply should erase.
	_, err = ws.DB.ExecContext(ctx,
		`INSERT OR REPLACE INTO item_deps (item_id, depends_on, dep_type) VALUES (?, ?, 'blocks')`,
		ref.ID, oldDep)
	require.NoError(t, err)
	_, err = ws.DB.ExecContext(ctx,
		`INSERT OR IGNORE INTO item_links (source_id, target_id, link_type) VALUES (?, ?, 'informs')`,
		ref.ID, oldLink)
	require.NoError(t, err)

	snapshot, err := os.ReadFile(refPath)
	require.NoError(t, err)

	updated := *ref
	updated.Dependencies = []string{newDepTarget}
	updated.Links = []models.ArtifactLink{{TargetID: newLinkTarget, LinkType: "informs"}}

	tx, err := ws.DB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback() //nolint:errcheck

	require.NoError(t, applyCrossArtifactRewrites(ctx, tx, ws, []crossRefUpdate{
		{artifact: &updated, filePath: refPath, snapshotRaw: snapshot},
	}))
	require.NoError(t, tx.Commit())

	// Old dep row must be gone; new dep row must exist.
	deps, err := bldb.GetDependencies(ctx, ws.DB, ref.ID)
	require.NoError(t, err)
	require.Len(t, deps, 1, "exactly one dep row should exist after refresh")
	assert.Equal(t, newDepTarget, deps[0].DependsOn)

	var oldDepCount int
	require.NoError(t, ws.DB.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM item_deps WHERE item_id = ? AND depends_on = ?`,
		ref.ID, oldDep).Scan(&oldDepCount))
	assert.Equal(t, 0, oldDepCount, "stale dep row should be deleted")

	// Old link row must be gone; new link row must exist.
	var newLinkCount int
	require.NoError(t, ws.DB.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM item_links WHERE source_id = ? AND target_id = ?`,
		ref.ID, newLinkTarget).Scan(&newLinkCount))
	assert.Equal(t, 1, newLinkCount, "new link row should be inserted")

	var oldLinkCount int
	require.NoError(t, ws.DB.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM item_links WHERE source_id = ? AND target_id = ?`,
		ref.ID, oldLink).Scan(&oldLinkCount))
	assert.Equal(t, 0, oldLinkCount, "stale link row should be deleted")
}

func TestApplyCrossArtifactRewrites_RollbackOnWriteFailure(t *testing.T) {
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()

	const newDep = "rollback-new-dep"

	feat, err := CreateArtifact(ctx, ws, "Feature", "feature")
	require.NoError(t, err)

	// ref1: first update — succeeds before the failure is triggered.
	ref1, err := CreateArtifact(ctx, ws, "First artifact", "task", WithParent(feat.ID))
	require.NoError(t, err)
	ref1Path := findArtifactPathDirect(ws, ref1.ID)
	require.NotEmpty(t, ref1Path)
	snapshot1, err := os.ReadFile(ref1Path)
	require.NoError(t, err)
	updated1 := *ref1
	updated1.Dependencies = []string{newDep}

	// ref2: second update — use a directory as the target path so the rename fails.
	ref2, err := CreateArtifact(ctx, ws, "Second artifact", "task", WithParent(feat.ID))
	require.NoError(t, err)
	ref2ActualPath := findArtifactPathDirect(ws, ref2.ID)
	require.NotEmpty(t, ref2ActualPath)
	badPath := filepath.Join(filepath.Dir(ref2ActualPath), ref2.ID+"-dir.md")
	require.NoError(t, os.MkdirAll(badPath, 0o755))
	snapshot2, err := os.ReadFile(ref2ActualPath)
	require.NoError(t, err)
	updated2 := *ref2
	updated2.Dependencies = []string{newDep}

	tx, err := ws.DB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback() //nolint:errcheck

	applyErr := applyCrossArtifactRewrites(ctx, tx, ws, []crossRefUpdate{
		{artifact: &updated1, filePath: ref1Path, snapshotRaw: snapshot1},
		{artifact: &updated2, filePath: badPath, snapshotRaw: snapshot2},
	})
	require.Error(t, applyErr, "apply should fail when a file write fails")

	// ref1 must be restored to its pre-apply state.
	restored, err := os.ReadFile(ref1Path)
	require.NoError(t, err)
	assert.Equal(t, string(snapshot1), string(restored),
		"first artifact must be restored from snapshot after downstream write failure")
}
