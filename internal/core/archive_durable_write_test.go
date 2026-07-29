package core

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/atomicfile"
	"github.com/softwaresalt/backlogit/internal/config"
	blerrors "github.com/softwaresalt/backlogit/internal/errors"
)

// TestReplaceFileWithOptions_RoutesDurableFlag is the F5 regression: archive and
// restore content writes must route the workspace durable_writes preference into
// the low-level atomic primitive instead of always taking the durable-off path.
//
// Must not run with t.Parallel: this test swaps the package-global
// replaceFileWriteFn seam read on the production write path.
func TestReplaceFileWithOptions_RoutesDurableFlag(t *testing.T) {
	cases := []struct {
		name string
		ws   *Workspace
		want bool
	}{
		{name: "durable workspace routes durable=true", ws: &Workspace{Config: &config.WorkspaceConfig{DurableWrites: true}}, want: true},
		{name: "non-durable workspace routes durable=false", ws: &Workspace{Config: &config.WorkspaceConfig{DurableWrites: false}}, want: false},
		{name: "nil workspace routes durable=false", ws: nil, want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var captured bool
			orig := replaceFileWriteFn
			replaceFileWriteFn = func(path string, data []byte, opts atomicfile.Options) error {
				captured = opts.DurableWrites
				return atomicfile.WriteFileAtomicWithOptions(path, data, opts)
			}
			t.Cleanup(func() { replaceFileWriteFn = orig })

			path := filepath.Join(t.TempDir(), "record.md")
			require.NoError(t, replaceFileWithOptions(tc.ws, path, []byte("content\n")))

			assert.Equal(t, tc.want, captured, "durable flag must be routed from WorkspaceDurableWrites(ws)")
			got, err := os.ReadFile(path)
			require.NoError(t, err)
			assert.Equal(t, "content\n", string(got))
		})
	}
}

// setupDurableArchiveWorkspace creates a workspace with durable_writes enabled for U1 tests.
// It archives a seed item so UnarchiveItem has a valid target.
func setupDurableArchiveWorkspace(t *testing.T) (ws *Workspace, itemID, archivePath string) {
	t.Helper()
	root := t.TempDir()
	backlogitDir := filepath.Join(root, ".backlogit")
	require.NoError(t, os.MkdirAll(backlogitDir, 0o755))
	require.NoError(t, config.WriteDefaults(backlogitDir))

	// Enable durable_writes in config.
	cfgPath := filepath.Join(backlogitDir, "config.yaml")
	cfgData, err := os.ReadFile(cfgPath)
	require.NoError(t, err)
	// Insert durable_writes: true — WriteDefaults does not include it.
	newCfg := string(cfgData) + "\ndurable_writes: true\n"
	require.NoError(t, os.WriteFile(cfgPath, []byte(newCfg), 0o644))

	ctx := context.Background()
	workspace, err := NewWorkspace(ctx, root)
	require.NoError(t, err)
	t.Cleanup(func() { workspace.Close() })

	feat, err := CreateArtifact(ctx, workspace, "Archive feat", "feature")
	require.NoError(t, err)
	task, err := CreateArtifact(ctx, workspace, "To-archive task", "task", WithParent(feat.ID))
	require.NoError(t, err)

	// Move to done so ArchiveItem accepts it.
	task.Status = "done"
	require.NoError(t, persistArtifact(ctx, workspace, task, false))

	record, err := ArchiveItem(ctx, workspace.DB, workspace, task.ID)
	require.NoError(t, err)
	return workspace, task.ID, record.ArchivePath
}

// TestUnarchiveItem_IndeterminateRestoreWrite_CompletesWithoutRollback is the U1
// regression: when the non-git restore write returns ErrWriteIndeterminate (rename
// committed, parent fsync uncertain), UnarchiveItem must proceed (commit-then-surface)
// rather than returning early. After the call: exactly one canonical file remains at
// the restore path, the archive copy is removed, and the DB row is present. The
// function surfaces ErrWriteIndeterminate to the caller.
//
// Seam-fidelity requirement: the seam writes the restored bytes to originalPath
// (mirroring a real indeterminate state) THEN returns ErrWriteIndeterminate, so the
// "no duplicate remains" assertion is non-trivial.
//
// Must not run with t.Parallel: swaps the replaceFileWriteFn package-global seam.
func TestUnarchiveItem_IndeterminateRestoreWrite_CompletesWithoutRollback(t *testing.T) {
	ws, itemID, archPath := setupDurableArchiveWorkspace(t)
	ctx := context.Background()

	origFn := replaceFileWriteFn
	replaceFileWriteFn = func(path string, data []byte, opts atomicfile.Options) error {
		// Real write first (mirrors rename-committed indeterminate state).
		_ = atomicfile.WriteFileAtomicWithOptions(path, data, opts)
		return fmt.Errorf("parent fsync failed: %w", blerrors.ErrWriteIndeterminate)
	}
	t.Cleanup(func() { replaceFileWriteFn = origFn })

	err := UnarchiveItem(ctx, ws.DB, ws, itemID)
	require.Error(t, err)
	assert.True(t, blerrors.IsWriteIndeterminate(err),
		"UnarchiveItem must surface ErrWriteIndeterminate from indeterminate restore write")

	// Exactly one canonical file remains at the restore path.
	restorePath := filepath.Join(ws.RootPath, ".backlogit", "queue", itemID+".md")
	assert.FileExists(t, restorePath, "restored file must exist (commit-then-surface)")

	// Archive copy must be removed.
	assert.NoFileExists(t, archPath, "archive file must be removed after indeterminate-but-committed restore")

	// DB row must be present.
	artifact, findErr := findArtifact(ctx, ws, itemID)
	require.NoError(t, findErr)
	assert.Equal(t, itemID, artifact.ID, "DB row must be present after indeterminate restore")
}

// TestUnarchiveItem_IndeterminateRestoreWrite_DBUpsertFails_NoRollback is the U1
// combined-failure regression (Copilot-hardened): when the restore write was
// indeterminate AND the subsequent DB upsert fails, restoreArchiveAfterUnarchiveFailure
// must NOT be called (it would roll back a possibly-applied write, violating the
// never-roll-back-indeterminate invariant). The function must:
//   - keep the restored file in place
//   - surface errors.Join(ErrWriteIndeterminate, upsertErr)
//
// Must not run with t.Parallel: swaps the replaceFileWriteFn package-global seam.
func TestUnarchiveItem_IndeterminateRestoreWrite_DBUpsertFails_NoRollback(t *testing.T) {
	ws, itemID, archPath := setupDurableArchiveWorkspace(t)
	ctx := context.Background()

	// Seam writes the file (indeterminate state) then returns the sentinel.
	origFn := replaceFileWriteFn
	replaceFileWriteFn = func(path string, data []byte, opts atomicfile.Options) error {
		_ = atomicfile.WriteFileAtomicWithOptions(path, data, opts)
		return fmt.Errorf("parent fsync: %w", blerrors.ErrWriteIndeterminate)
	}
	t.Cleanup(func() { replaceFileWriteFn = origFn })

	// Pass a closed DB so db.UpsertItem fails.
	closedDB, openErr := sql.Open("sqlite", ":memory:")
	require.NoError(t, openErr)
	require.NoError(t, closedDB.Close()) // immediately closed — all operations will fail

	err := UnarchiveItem(ctx, closedDB, ws, itemID)
	require.Error(t, err)
	assert.True(t, blerrors.IsWriteIndeterminate(err),
		"combined-failure error must contain ErrWriteIndeterminate")
	assert.False(t, errors.Is(err, blerrors.ErrWriteNotApplied),
		"combined-failure error must not be misclassified as not-applied")

	// Restored file must remain — rollback must NOT have been called.
	restorePath := filepath.Join(ws.RootPath, ".backlogit", "queue", itemID+".md")
	assert.FileExists(t, restorePath,
		"restored file must be kept when both write and upsert fail (no rollback on indeterminate)")

	// Archive copy must still be removed (the non-git removal ran before upsert).
	assert.NoFileExists(t, archPath,
		"archive copy must be removed even on combined failure (restore already committed)")
}

// TestUnarchiveItem_NotAppliedRestoreWrite_EarlyReturn guards the existing safe-abort
// behavior: when the restore write returns ErrWriteNotApplied (file untouched), the
// function must return early without removing the archive copy, leaving the workspace
// in its pre-unarchive state.
//
// Must not run with t.Parallel: swaps the replaceFileWriteFn package-global seam.
func TestUnarchiveItem_NotAppliedRestoreWrite_EarlyReturn(t *testing.T) {
	ws, itemID, archPath := setupDurableArchiveWorkspace(t)
	ctx := context.Background()

	origFn := replaceFileWriteFn
	replaceFileWriteFn = func(path string, data []byte, opts atomicfile.Options) error {
		// Do NOT write the file — this is the not-applied state.
		return fmt.Errorf("pre-rename write failed: %w", blerrors.ErrWriteNotApplied)
	}
	t.Cleanup(func() { replaceFileWriteFn = origFn })

	err := UnarchiveItem(ctx, ws.DB, ws, itemID)
	require.Error(t, err)
	assert.False(t, blerrors.IsWriteIndeterminate(err), "not-applied error must not be indeterminate")

	// Restore path must NOT exist.
	restorePath := filepath.Join(ws.RootPath, ".backlogit", "queue", itemID+".md")
	assert.NoFileExists(t, restorePath,
		"restore path must not exist when write was not-applied")

	// Archive file must still be present (safe-abort leaves workspace unchanged).
	assert.FileExists(t, archPath,
		"archive file must be retained when write was not-applied (safe-abort)")
}
