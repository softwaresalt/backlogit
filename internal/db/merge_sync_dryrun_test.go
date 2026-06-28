package db_test

// 062.005-T: Enforce MergeSync dry-run purity (stash EE33B6ED).
//
// MergeSync(dryRun=true) must perform ZERO database writes, even when the diff
// is large enough that ShouldFallback would otherwise trigger a full rehydrate.
// Before the fix, the fallback branch called RehydrateWithManifest (which
// clears and repopulates every table) BEFORE the dry-run guard, so a dry run
// that crossed the fallback threshold mutated the database.

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/db"
)

func countItems(t *testing.T, database *sql.DB) int {
	t.Helper()
	var n int
	require.NoError(t, database.QueryRow("SELECT COUNT(*) FROM items").Scan(&n))
	return n
}

// A dry run that crosses the fallback threshold must still not write anything to
// the database, while still reporting that a fallback would be used.
func TestMergeSync_DryRunWithFallback_DoesNotWrite(t *testing.T) {
	ws, database, manifest := setupSyncWorkspace(t)
	ctx := context.Background()

	baseline := countItems(t, database)

	// Add enough NEW, uniquely-identified artifacts to exceed the fallback
	// threshold. Unique ids ensure a real rehydrate would grow the items table.
	const extra = 60
	for i := 0; i < extra; i++ {
		content := fmt.Sprintf(`---
id: 9%02d-F
title: Extra Feature %d
artifact_type: feature
status: queued
---
Body %d.
`, i, i, i)
		name := filepath.Join(ws, "queue", fmt.Sprintf("9%02d-F.md", i))
		require.NoError(t, os.WriteFile(name, []byte(content), 0o644))
	}

	result, _, err := db.MergeSync(ctx, ws, database, manifest, true)
	require.NoError(t, err)

	// Dry run must report the fallback decision...
	assert.True(t, result.DryRun, "result must record dry_run")
	assert.True(t, result.FallbackUsed, "a large dry-run delta must report that fallback would be used")

	// ...but must NOT have written any of the new artifacts to the database.
	after := countItems(t, database)
	assert.Equal(t, baseline, after,
		"dry-run fallback must not write to the database (items count must be unchanged)")

	_, getErr := db.GetItem(ctx, database, "900-F")
	assert.Error(t, getErr, "dry-run fallback must not insert new items into the database")
}
