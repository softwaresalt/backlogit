package core

// 060.001-T: Make stash harvest rollback atomic
//
// Atomicity contract after fix:
//   - If writeStashEntries fails AFTER DB stash state has been committed
//     (UpsertStashEntry + LinkStashEntry), the DB stash state must be
//     rolled back so the entry is still visible as "active" in the index.
//   - Before the fix, the DB stash_entries row is left in "harvested" state
//     and the stash_links row persists even though the JSONL was not rewritten.
//   - After the fix, a JSONL write failure reverts both DB rows so the
//     stash entry remains "active" (no orphaned harvested state).

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/config"
	"github.com/softwaresalt/backlogit/internal/db"
)

// setupHarvestRollbackWorkspace builds a workspace with a seed stash entry.
func setupHarvestRollbackWorkspace(t *testing.T) *Workspace {
	t.Helper()
	tmpDir := t.TempDir()
	backlogDir := filepath.Join(tmpDir, ".backlogit")
	require.NoError(t, os.MkdirAll(backlogDir, 0o755))
	require.NoError(t, config.WriteDefaults(backlogDir))

	stashContent := `{"id":"CCCCDDDD","priority":"medium","kind":"task","text":"Harvest rollback atomicity entry"}` + "\n"
	require.NoError(t, os.WriteFile(filepath.Join(backlogDir, "stash.jsonl"), []byte(stashContent), 0o644))

	ws, wsErr := NewWorkspace(context.Background(), tmpDir)
	require.NoError(t, wsErr)
	t.Cleanup(func() { ws.Close() })
	return ws
}

// TestHarvestStashEntry_JSONLWriteFailure_DBStashStateRolledBack verifies that
// when the JSONL rewrite fails after DB state is committed, the DB stash entry
// is reverted to "active" and the stash_link row is removed.
//
// Failure is injected deterministically by pausing the final stash.jsonl rewrite
// inside writeStashEntries, replacing stash.jsonl with a directory, then letting
// the atomic rename proceed. That guarantees the failure occurs after
// UpsertStashEntry and LinkStashEntry have already committed.
func TestHarvestStashEntry_JSONLWriteFailure_DBStashStateRolledBack(t *testing.T) {
	ws := setupHarvestRollbackWorkspace(t)
	ctx := context.Background()

	backlogDir := filepath.Join(ws.RootPath, ".backlogit")
	stashPath := filepath.Join(backlogDir, "stash.jsonl")
	const maxWait = 10 * time.Second

	originalWriter := ws.writeStashEntriesAtomically
	enterWriteCh := make(chan struct{}, 1)
	releaseWriteCh := make(chan struct{})
	writeCalls := 0
	releaseWrite := func() {
		select {
		case <-releaseWriteCh:
		default:
			close(releaseWriteCh)
		}
	}
	ws.writeStashEntriesAtomically = func(path, content string) error {
		if filepath.Clean(path) == filepath.Clean(stashPath) {
			writeCalls++
			if writeCalls == 1 {
				select {
				case enterWriteCh <- struct{}{}:
				default:
				}
				<-releaseWriteCh
			}
		}
		return originalWriter(path, content)
	}
	t.Cleanup(func() {
		ws.writeStashEntriesAtomically = originalWriter
		releaseWrite()
	})

	harvestErrCh := make(chan error, 1)

	go func() {
		_, err := HarvestStashEntry(ctx, ws, HarvestStashOptions{
			StashID:      "CCCCDDDD",
			ArtifactType: "feature",
		})
		harvestErrCh <- err
	}()

	select {
	case <-enterWriteCh:
	case <-time.After(maxWait):
		t.Fatal("HarvestStashEntry did not reach the stash rewrite step within timeout")
	}

	require.NoError(t, os.Remove(stashPath))
	require.NoError(t, os.Mkdir(stashPath, 0o755))
	releaseWrite()

	var harvestErr error
	select {
	case harvestErr = <-harvestErrCh:
	case <-time.After(maxWait):
		t.Fatal("HarvestStashEntry did not complete within timeout")
	}

	require.Error(t, harvestErr, "HarvestStashEntry must return an error when JSONL rename fails")

	// After the failed harvest, stash_entries must reflect "active" state —
	// the DB rollback must have reverted the "harvested" write.
	entries, listErr := db.ListStashEntries(ctx, ws.DB, true)
	require.NoError(t, listErr)

	var found *db.StashRecord
	for i := range entries {
		if entries[i].ID == "CCCCDDDD" {
			found = &entries[i]
			break
		}
	}

	require.NotNil(t, found, "stash entry must still exist in DB index after failed harvest")
	assert.Equal(t, "active", found.State,
		"stash entry state must be reverted to 'active' after JSONL write failure; got %q", found.State)
	assert.Empty(t, found.ItemID,
		"stash_links row must be removed: ItemID must be empty after JSONL write failure; got %q", found.ItemID)
}
