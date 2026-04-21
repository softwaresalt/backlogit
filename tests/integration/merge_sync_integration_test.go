package integration_test

// 037.005-T (integration tier): end-to-end MergeSync scenarios
//
// These integration tests verify the full data path from filesystem change
// through MergeSync to a queryable SQLite index:
//   - Added artifact indexed after sync
//   - Deleted artifact removed from index after sync
//   - Relocated artifact path updated without spurious delete+add
//   - stash.jsonl change triggers stash table refresh
//   - Delta beyond threshold falls back to full rehydrate
//   - dry_run=true leaves the database unchanged

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/config"
	"github.com/softwaresalt/backlogit/internal/core"
	"github.com/softwaresalt/backlogit/internal/db"
)

const integArtifactA = `---
id: 050-F
title: Integration Feature A
artifact_type: feature
status: queued
---
Body.
`

const integArtifactB = `---
id: 051-T
title: Integration Task B
artifact_type: task
status: queued
parent_id: 050-F
---
Body.
`

// setupMergeSyncWorkspace initialises a full workspace, writes artifact A, runs
// an initial RehydrateWithManifest to capture the baseline, and returns the
// workspace root, database, and manifest.
func setupMergeSyncWorkspace(t *testing.T) (root string, database *sql.DB, manifest map[string]db.FileEntry) {
	t.Helper()
	root = t.TempDir()
	backlogitDir := filepath.Join(root, ".backlogit")
	require.NoError(t, os.MkdirAll(backlogitDir, 0o755))
	require.NoError(t, config.WriteDefaults(backlogitDir))

	ctx := context.Background()
	ws, err := core.NewWorkspace(ctx, root)
	require.NoError(t, err)
	t.Cleanup(func() { ws.Close() })

	queueDir := filepath.Join(backlogitDir, "queue")
	require.NoError(t, os.MkdirAll(queueDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(queueDir, "050-F.md"), []byte(integArtifactA), 0o644))

	storageRoot := core.WorkspaceStorageRoot(root)
	_, manifest, err = db.RehydrateWithManifest(ctx, storageRoot, ws.DB)
	require.NoError(t, err)

	return root, ws.DB, manifest}

func TestMergeSyncIntegration_AddedArtifactIndexed(t *testing.T) {
	root, database, manifest := setupMergeSyncWorkspace(t)
	ctx := context.Background()

	queueDir := filepath.Join(root, ".backlogit", "queue")
	require.NoError(t, os.WriteFile(filepath.Join(queueDir, "051-T.md"), []byte(integArtifactB), 0o644))

	storageRoot := core.WorkspaceStorageRoot(root)
	result, _, err := db.MergeSync(ctx, storageRoot, database, manifest, false)
	require.NoError(t, err)

	require.Len(t, result.Added, 1)
	assert.Equal(t, "051-T", result.Added[0].ID)
	assert.False(t, result.FallbackUsed)

	got, err := db.GetItem(ctx, database, "051-T")
	require.NoError(t, err)
	assert.Equal(t, "Integration Task B", got.Title)
}

func TestMergeSyncIntegration_DeletedArtifactRemovedFromIndex(t *testing.T) {
	root, database, manifest := setupMergeSyncWorkspace(t)
	ctx := context.Background()

	require.NoError(t, os.Remove(filepath.Join(root, ".backlogit", "queue", "050-F.md")))

	storageRoot := core.WorkspaceStorageRoot(root)
	result, _, err := db.MergeSync(ctx, storageRoot, database, manifest, false)
	require.NoError(t, err)

	require.Len(t, result.Deleted, 1)
	assert.Equal(t, "050-F", result.Deleted[0].ID)

	_, err = db.GetItem(ctx, database, "050-F")
	assert.Error(t, err, "deleted artifact must be absent from the index")
}

func TestMergeSyncIntegration_RelocatedArtifactUpdatesPath(t *testing.T) {
	root, database, manifest := setupMergeSyncWorkspace(t)
	ctx := context.Background()

	doneDir := filepath.Join(root, ".backlogit", "done")
	require.NoError(t, os.MkdirAll(doneDir, 0o755))
	require.NoError(t, os.Rename(
		filepath.Join(root, ".backlogit", "queue", "050-F.md"),
		filepath.Join(doneDir, "050-F.md"),
	))

	storageRoot := core.WorkspaceStorageRoot(root)
	result, _, err := db.MergeSync(ctx, storageRoot, database, manifest, false)
	require.NoError(t, err)

	assert.Empty(t, result.Added, "relocation must not appear in Added")
	assert.Empty(t, result.Deleted, "relocation must not appear in Deleted")
	require.Len(t, result.Relocated, 1)
	assert.Equal(t, "050-F", result.Relocated[0].ID)
}

func TestMergeSyncIntegration_StashChangeTriggersRefresh(t *testing.T) {
	root, database, manifest := setupMergeSyncWorkspace(t)
	ctx := context.Background()

	stashLine := `{"id":"AABBCCDD","text":"integration stash entry","priority":"high","kind":"task","state":"active"}` + "\n"
	require.NoError(t, os.WriteFile(
		filepath.Join(root, ".backlogit", "stash.jsonl"),
		[]byte(stashLine),
		0o644,
	))

	storageRoot := core.WorkspaceStorageRoot(root)
	result, _, err := db.MergeSync(ctx, storageRoot, database, manifest, false)
	require.NoError(t, err)

	assert.True(t, result.StashRefreshed)
}

func TestMergeSyncIntegration_LargeDeltaTriggersFallback(t *testing.T) {
	root, database, manifest := setupMergeSyncWorkspace(t)
	ctx := context.Background()

	queueDir := filepath.Join(root, ".backlogit", "queue")
	for i := 0; i < 60; i++ {
		content := integArtifactA
		name := filepath.Join(queueDir, filepath.Base(filepath.Join("bulk-"+string(rune('a'+i%26))+".md")))
		require.NoError(t, os.WriteFile(name, []byte(content), 0o644))
	}

	storageRoot := core.WorkspaceStorageRoot(root)
	result, _, err := db.MergeSync(ctx, storageRoot, database, manifest, false)
	require.NoError(t, err)

	assert.True(t, result.FallbackUsed)
	assert.NotEmpty(t, result.FallbackReason)
}

func TestMergeSyncIntegration_DryRunDoesNotWriteDatabase(t *testing.T) {
	root, database, manifest := setupMergeSyncWorkspace(t)
	ctx := context.Background()

	queueDir := filepath.Join(root, ".backlogit", "queue")
	require.NoError(t, os.WriteFile(filepath.Join(queueDir, "051-T.md"), []byte(integArtifactB), 0o644))

	storageRoot := core.WorkspaceStorageRoot(root)
	result, _, err := db.MergeSync(ctx, storageRoot, database, manifest, true)
	require.NoError(t, err)

	assert.True(t, result.DryRun)
	require.Len(t, result.Added, 1, "dry-run must report the diff")

	_, err = db.GetItem(ctx, database, "051-T")
	assert.Error(t, err, "dry-run must not persist the added artifact")
}

func TestMergeSyncIntegration_ManifestUpdatedAfterSync(t *testing.T) {
	root, database, manifest := setupMergeSyncWorkspace(t)
	ctx := context.Background()

	queueDir := filepath.Join(root, ".backlogit", "queue")
	require.NoError(t, os.WriteFile(filepath.Join(queueDir, "051-T.md"), []byte(integArtifactB), 0o644))

	storageRoot := core.WorkspaceStorageRoot(root)
	_, newManifest, err := db.MergeSync(ctx, storageRoot, database, manifest, false)
	require.NoError(t, err)

	_, ok := newManifest["queue/051-T.md"]
	assert.True(t, ok, "updated manifest must include the newly added file")
}
