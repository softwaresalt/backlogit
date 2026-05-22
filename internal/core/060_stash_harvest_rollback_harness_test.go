package core_test

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
	"github.com/softwaresalt/backlogit/internal/core"
	"github.com/softwaresalt/backlogit/internal/db"
)

// setupHarvestRollbackWorkspace builds a workspace with a seed stash entry.
func setupHarvestRollbackWorkspace(t *testing.T) *core.Workspace {
	t.Helper()
	tmpDir := t.TempDir()
	backlogDir := filepath.Join(tmpDir, ".backlogit")
	require.NoError(t, os.MkdirAll(backlogDir, 0o755))
	require.NoError(t, config.WriteDefaults(backlogDir))

	stashContent := `{"id":"CCCCDDDD","priority":"medium","kind":"task","text":"Harvest rollback atomicity entry"}` + "\n"
	require.NoError(t, os.WriteFile(filepath.Join(backlogDir, "stash.jsonl"), []byte(stashContent), 0o644))

	ws, wsErr := core.NewWorkspace(context.Background(), tmpDir)
	require.NoError(t, wsErr)
	t.Cleanup(func() { ws.Close() })
	return ws
}

// TestHarvestStashEntry_JSONLWriteFailure_DBStashStateRolledBack verifies that
// when the JSONL rewrite fails after DB state is committed, the DB stash entry
// is reverted to "active" and the stash_link row is removed.
//
// Failure is injected by a background goroutine that watches for an artifact
// file to appear in the queue directory (proof that CreateArtifact succeeded and
// the DB ops are in progress) and then replaces stash.jsonl with a directory.
// The atomic rename in writeStringAtomically (stash.jsonl.tmp → stash.jsonl)
// fails because the destination is now a directory, returning an error only
// after UpsertStashEntry and LinkStashEntry have already committed.
func TestHarvestStashEntry_JSONLWriteFailure_DBStashStateRolledBack(t *testing.T) {
	ws := setupHarvestRollbackWorkspace(t)
	ctx := context.Background()

	backlogDir := filepath.Join(ws.RootPath, ".backlogit")
	queueDir := filepath.Join(backlogDir, "queue")
	stashPath := filepath.Join(backlogDir, "stash.jsonl")

	harvestErrCh := make(chan error, 1)

	// Run the harvest in a background goroutine so we can manipulate the
	// filesystem from the main test goroutine while it executes.
	go func() {
		_, err := core.HarvestStashEntry(ctx, ws, core.HarvestStashOptions{
			StashID:      "CCCCDDDD",
			ArtifactType: "feature",
		})
		harvestErrCh <- err
	}()

	// Poll until an artifact .md file appears in the queue directory tree.
	// This proves CreateArtifact has succeeded and the DB stash ops are
	// in progress (or about to start). Replace stash.jsonl with a directory
	// so the final writeStringAtomically rename fails.
	const pollInterval = 100 * time.Microsecond
	const maxWait = 10 * time.Second
	blocked := false
	deadline := time.Now().Add(maxWait)

	for !blocked && time.Now().Before(deadline) {
		if mdFound(queueDir) {
			// CreateArtifact has run.  Swap stash.jsonl → directory so
			// the rename in writeStringAtomically(stash.jsonl.tmp → stash.jsonl) fails.
			_ = os.Remove(stashPath)
			if err := os.MkdirAll(stashPath, 0o755); err == nil {
				t.Cleanup(func() { _ = os.RemoveAll(stashPath) })
				blocked = true
			}
		}
		if !blocked {
			time.Sleep(pollInterval)
		}
	}

	if !blocked {
		// We could not inject the failure in time — the harvest may have
		// already completed successfully.  Skip rather than report a false result.
		select {
		case <-harvestErrCh:
		case <-time.After(maxWait):
		}
		t.Skip("could not inject JSONL write failure: artifact file appeared too late or harvest already completed")
	}

	// Wait for the harvest to finish.
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

// mdFound returns true if any .md file exists under dir (non-recursive first-level walk).
func mdFound(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if e.IsDir() {
			// Recurse one level only — queue may have subdirectories.
			sub := filepath.Join(dir, e.Name())
			subEntries, _ := os.ReadDir(sub)
			for _, se := range subEntries {
				if !se.IsDir() && filepath.Ext(se.Name()) == ".md" {
					return true
				}
			}
		} else if filepath.Ext(e.Name()) == ".md" {
			return true
		}
	}
	return false
}
