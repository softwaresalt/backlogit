package core

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/db"
	"github.com/softwaresalt/backlogit/internal/models"
)

// TestSizeCompositionResolvesFeatureMembersFromIndex proves the feature rollup
// resolves members from the SQLite index rather than a per-member filesystem
// WalkDir (114-F / 47ED88ED). A member present only in the index (no Markdown
// file on disk) must still be counted; the previous filesystem resolver would
// have warn-skipped it.
func TestSizeCompositionResolvesFeatureMembersFromIndex(t *testing.T) {
	ws, _ := newSizeEstimationHarnessWorkspace(t)
	ctx := context.Background()
	feature := &models.Artifact{
		ID: "970-F", Title: "Index-only feature", Status: models.StatusActive, ArtifactType: "feature",
	}
	require.NoError(t, db.UpsertItem(ctx, ws.DB, feature))
	require.NoError(t, db.UpsertItem(ctx, ws.DB, &models.Artifact{
		ID: "970.001-T", Title: "Index-only sized child", Status: models.StatusActive,
		ArtifactType: "task", ParentID: "970-F", CustomFields: map[string]any{"size": "M"},
	}))

	result, err := SizeComposition(ctx, ws, feature)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 1, result.Histogram["M"], "index-only sized child must be counted")
	assert.Len(t, result.Members, 1, "one resolved task member")
	assert.Empty(t, result.Skipped, "no member should be skipped when present in the index")
}

// TestSizeCompositionResolvesShipmentManifestFromIndex proves the shipment
// manifest member-type resolution (task vs feature expansion) is also
// index-backed, not a per-member filesystem WalkDir.
func TestSizeCompositionResolvesShipmentManifestFromIndex(t *testing.T) {
	ws, _ := newSizeEstimationHarnessWorkspace(t)
	ctx := context.Background()
	require.NoError(t, db.UpsertItem(ctx, ws.DB, &models.Artifact{
		ID: "971-F", Title: "f", Status: models.StatusActive, ArtifactType: "feature",
	}))
	require.NoError(t, db.UpsertItem(ctx, ws.DB, &models.Artifact{
		ID: "971.001-T", Title: "t", Status: models.StatusActive, ArtifactType: "task",
		ParentID: "971-F", CustomFields: map[string]any{"size": "L"},
	}))
	shipment := &models.Artifact{
		ID: "971-S", Title: "s", Status: models.StatusActive, ArtifactType: "shipment",
		CustomFields: map[string]any{"items": []any{"971-F"}},
	}
	require.NoError(t, db.UpsertItem(ctx, ws.DB, shipment))

	result, err := SizeComposition(ctx, ws, shipment)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 1, result.Histogram["L"], "expanded child task counted from index")
	assert.Len(t, result.Members, 1)
	assert.Empty(t, result.Skipped)
}

// TestGetItemsByIDsBatchResolvesAndOmitsMissing verifies the batch index
// resolver returns found artifacts keyed by ID and simply omits IDs with no
// indexed row (a miss is not an error).
func TestGetItemsByIDsBatchResolvesAndOmitsMissing(t *testing.T) {
	ws, _ := newSizeEstimationHarnessWorkspace(t)
	ctx := context.Background()
	require.NoError(t, db.UpsertItem(ctx, ws.DB, &models.Artifact{
		ID: "972.001-T", Title: "a", Status: models.StatusActive, ArtifactType: "task",
		CustomFields: map[string]any{"size": "S"},
	}))
	require.NoError(t, db.UpsertItem(ctx, ws.DB, &models.Artifact{
		ID: "972.002-T", Title: "b", Status: models.StatusActive, ArtifactType: "task",
	}))

	resolved, err := db.GetItemsByIDs(ctx, ws.DB, []string{"972.001-T", "972.002-T", "999.404-T", ""})
	require.NoError(t, err)
	require.Len(t, resolved, 2, "only the two indexed IDs resolve; missing and empty are omitted")
	require.Contains(t, resolved, "972.001-T")
	assert.Equal(t, "S", resolved["972.001-T"].CustomFields["size"])
	require.Contains(t, resolved, "972.002-T")
	assert.NotContains(t, resolved, "999.404-T")
}

// TestGetItemsByIDsConcurrentWithWrites exercises the batch resolver's read path
// while a writer concurrently upserts to the same index. The resolver runs each
// chunk as an implicit deferred read on the pooled handle (never an explicit
// immediate-lock transaction), so it must coexist with writers and always return
// the stable indexed rows without error. This guards against reintroducing an
// explicit ReadOnly transaction, which — because db.Open sets _txlock=immediate —
// would acquire the write lock and serialize every composition read behind
// writers.
func TestGetItemsByIDsConcurrentWithWrites(t *testing.T) {
	ws, _ := newSizeEstimationHarnessWorkspace(t)
	ctx := context.Background()

	stableIDs := []string{"973.001-T", "973.002-T", "973.003-T"}
	for _, id := range stableIDs {
		require.NoError(t, db.UpsertItem(ctx, ws.DB, &models.Artifact{
			ID: id, Title: "stable " + id, Status: models.StatusActive, ArtifactType: "task",
			CustomFields: map[string]any{"size": "M"},
		}))
	}

	const iterations = 100
	writeErrs := make(chan error, iterations)
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < iterations; i++ {
			// Rewrite a churn row plus toggle a stable row's status so a writer is
			// consistently contending for the WAL while reads run.
			if err := db.UpsertItem(ctx, ws.DB, &models.Artifact{
				ID: "973.900-T", Title: "churn", Status: models.StatusActive, ArtifactType: "task",
			}); err != nil {
				writeErrs <- err
				return
			}
			status := models.StatusActive
			if i%2 == 0 {
				status = models.StatusBlocked
			}
			if err := db.UpsertItem(ctx, ws.DB, &models.Artifact{
				ID: "973.001-T", Title: "stable 973.001-T", Status: status, ArtifactType: "task",
				CustomFields: map[string]any{"size": "M"},
			}); err != nil {
				writeErrs <- err
				return
			}
		}
	}()

	for i := 0; i < iterations; i++ {
		resolved, err := db.GetItemsByIDs(ctx, ws.DB, stableIDs)
		require.NoError(t, err, "batch read must not error under concurrent writes")
		require.Len(t, resolved, len(stableIDs), "all stable IDs must resolve on every read")
	}

	<-done
	close(writeErrs)
	for err := range writeErrs {
		require.NoError(t, err, "concurrent writer must not error")
	}
}

// TestGetItemsByIDsReadsProceedUnderOpenWriteTx is a deterministic regression
// guard against reintroducing an explicit BEGIN IMMEDIATE read transaction in
// GetItemsByIDs. The DB DSN sets _txlock=immediate, so any explicit BeginTx —
// even a read-only one — acquires the write lock at BEGIN and would serialize
// behind an open writer up to the 30s busy_timeout. WAL mode lets a deferred
// (implicit) pooled read proceed against the last committed snapshot while a
// writer transaction is still open. Holding a write transaction open and giving
// the batch read a deadline far shorter than busy_timeout converts a serializing
// regression into a hard failure: the deferred implementation returns promptly;
// an immediate-lock implementation blocks past the deadline and errors.
func TestGetItemsByIDsReadsProceedUnderOpenWriteTx(t *testing.T) {
	ctx := context.Background()
	ws, _ := newSizeEstimationHarnessWorkspace(t)

	ids := []string{"982.001-T", "982.002-T", "982.003-T"}
	for _, id := range ids {
		require.NoError(t, db.UpsertItem(ctx, ws.DB, &models.Artifact{
			ID: id, Title: "row " + id, Status: models.StatusActive, ArtifactType: "task",
			CustomFields: map[string]any{"size": "M"},
		}))
	}

	// Open and hold a write transaction. BEGIN IMMEDIATE (via _txlock=immediate)
	// acquires the write lock at BEGIN; the explicit UPDATE guarantees the lock
	// is held for the duration of the concurrent read below.
	writeTx, err := ws.DB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = writeTx.Rollback() }()
	_, err = writeTx.ExecContext(ctx, `UPDATE items SET title = title WHERE id = ?`, ids[0])
	require.NoError(t, err)

	// The batch read must complete well within a deadline far shorter than the
	// 30s busy_timeout while the writer lock is still held.
	readCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	start := time.Now()
	resolved, err := db.GetItemsByIDs(readCtx, ws.DB, ids)
	require.NoError(t, err, "deferred batch read must proceed while a write transaction is open (WAL)")
	require.Len(t, resolved, len(ids), "all IDs must resolve from the last committed snapshot")
	require.Less(t, time.Since(start), 3*time.Second, "read must not block on the held write lock")

	require.NoError(t, writeTx.Rollback())
}
