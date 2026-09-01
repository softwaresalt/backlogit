package core_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/core"
	"github.com/softwaresalt/backlogit/internal/db"
	blerrors "github.com/softwaresalt/backlogit/internal/errors"
	"github.com/softwaresalt/backlogit/internal/models"
)

// setupReconcileWorkspace creates an isolated workspace for archive lifecycle
// reconciliation tests. It seeds both feature and task artifact types.
func setupReconcileWorkspace(t *testing.T) *core.Workspace {
	t.Helper()
	tmpDir := t.TempDir()
	backlogDir := filepath.Join(tmpDir, ".backlogit")
	require.NoError(t, os.MkdirAll(filepath.Join(backlogDir, "queue"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(backlogDir, "archive"), 0o755))

	dbPath := filepath.Join(backlogDir, "backlogit.db")
	database, err := db.Open(dbPath)
	require.NoError(t, err)
	require.NoError(t, db.EnsureSchema(database))
	t.Cleanup(func() { database.Close() })

	configData := []byte(
		"artifact_types:\n" +
			"  feature:\n" +
			"    prefix: F\n" +
			"    suffix: \"-F\"\n" +
			"    name_format: \"{NNN}{suffix}\"\n" +
			"  task:\n" +
			"    prefix: T\n" +
			"    suffix: \"-T\"\n" +
			"    name_format: \"{NNN}{suffix}\"\n" +
			"max_slug_length: 60\n",
	)
	require.NoError(t, os.WriteFile(filepath.Join(backlogDir, "config.yaml"), configData, 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(backlogDir, "header-def.yaml"), []byte("defaults: {}\ntypes: {}\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(backlogDir, "registry.yaml"), []byte("routes: {}\n"), 0o644))

	return &core.Workspace{
		RootPath:    tmpDir,
		StorageRoot: backlogDir,
		DB:          database,
	}
}

// writeArchivedItem seeds an archived markdown file in the archive directory and
// upserts a corresponding DB record with status "archived".
func writeArchivedItem(t *testing.T, ws *core.Workspace, id, artifactType, archivedStatus string) {
	t.Helper()
	archiveDir := filepath.Join(ws.StorageRoot, "archive")
	require.NoError(t, os.MkdirAll(archiveDir, 0o755))

	content := fmt.Sprintf(
		"---\nid: %s\ntitle: Test %s\nstatus: archived\nartifact_type: %s\narchived_from: .backlogit/queue/%s.md\narchived_status: %s\n---\nTest body\n",
		id, id, artifactType, id, archivedStatus,
	)
	require.NoError(t, os.WriteFile(filepath.Join(archiveDir, id+".md"), []byte(content), 0o644))
	require.NoError(t, db.UpsertItem(context.Background(), ws.DB, &models.Artifact{
		ID:           id,
		Title:        "Test " + id,
		Status:       models.StatusArchived,
		ArtifactType: artifactType,
	}))
}

// findItemError returns the Error string from the first ReconciliationItemResult
// whose ID matches. Returns empty string when no match is found.
func findItemError(items []core.ReconciliationItemResult, id string) string {
	for _, item := range items {
		if item.ID == id {
			return item.Error
		}
	}
	return ""
}

// TestReconcileArchivedLifecycle_HappyPath verifies that an archived item whose
// archived_status is "active" is transitioned to "done", re-archived, and
// returns ReconciliationCompleted with durable metadata in the archive file.
func TestReconcileArchivedLifecycle_HappyPath(t *testing.T) {
	ws := setupReconcileWorkspace(t)
	ctx := context.Background()
	writeArchivedItem(t, ws, "001-F", "feature", "active")

	req := core.ReconciliationRequest{
		ItemIDs:      []string{"001-F"},
		TargetStatus: "done",
		Reason:       "lifecycle reconcile to done",
		Actor:        "test-actor",
	}

	result, err := core.ReconcileArchivedLifecycle(ctx, ws.DB, ws, req)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, core.ReconciliationCompleted, result.Outcome)
	require.Len(t, result.Items, 1)
	assert.Equal(t, "001-F", result.Items[0].ID)
	assert.Equal(t, core.ReconciliationCompleted, result.Items[0].Outcome)
	assert.Empty(t, result.Items[0].Error)

	archivePath := filepath.Join(ws.StorageRoot, "archive", "001-F.md")
	raw, readErr := os.ReadFile(archivePath)
	require.NoError(t, readErr)
	fm, _, parseErr := models.ParseFrontmatter(string(raw))
	require.NoError(t, parseErr)
	assert.Equal(t, "done", fm["archived_status"], "archived_status must be updated to done")
	require.Contains(t, fm, "custom_fields", "custom_fields must be present after reconciliation")
}

// TestReconcileArchivedLifecycle_AlreadyDone_NoOp verifies that reconciling an
// archived item already at the target archived_status returns ReconciliationNoOp
// without modifying the archive file.
func TestReconcileArchivedLifecycle_AlreadyDone_NoOp(t *testing.T) {
	ws := setupReconcileWorkspace(t)
	ctx := context.Background()
	writeArchivedItem(t, ws, "001-F", "feature", "done")

	req := core.ReconciliationRequest{
		ItemIDs:      []string{"001-F"},
		TargetStatus: "done",
		Reason:       "already at target status",
		Actor:        "test-actor",
	}

	result, err := core.ReconcileArchivedLifecycle(ctx, ws.DB, ws, req)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, core.ReconciliationNoOp, result.Outcome)
	require.Len(t, result.Items, 1)
	assert.Equal(t, core.ReconciliationNoOp, result.Items[0].Outcome)
}

// TestReconcileArchivedLifecycle_NotArchived_Error verifies that attempting to
// reconcile a non-archived (active/queued) item returns an error.
func TestReconcileArchivedLifecycle_NotArchived_Error(t *testing.T) {
	ws := setupReconcileWorkspace(t)
	ctx := context.Background()

	queueDir := filepath.Join(ws.StorageRoot, "queue")
	content := "---\nid: 002-F\ntitle: Active feature\nstatus: active\nartifact_type: feature\n---\nBody\n"
	require.NoError(t, os.WriteFile(filepath.Join(queueDir, "002-F.md"), []byte(content), 0o644))
	require.NoError(t, db.UpsertItem(ctx, ws.DB, &models.Artifact{
		ID: "002-F", Title: "Active feature", Status: models.StatusActive, ArtifactType: "feature",
	}))

	req := core.ReconciliationRequest{
		ItemIDs:      []string{"002-F"},
		TargetStatus: "done",
		Reason:       "not archived test",
		Actor:        "test-actor",
	}

	_, err := core.ReconcileArchivedLifecycle(ctx, ws.DB, ws, req)
	require.Error(t, err)
}

// TestReconcileArchivedLifecycle_NotFound_Error verifies that reconciling a
// non-existent item returns ErrNotFound.
func TestReconcileArchivedLifecycle_NotFound_Error(t *testing.T) {
	ws := setupReconcileWorkspace(t)
	ctx := context.Background()

	req := core.ReconciliationRequest{
		ItemIDs:      []string{"nonexistent-F"},
		TargetStatus: "done",
		Reason:       "not found test",
		Actor:        "test-actor",
	}

	_, err := core.ReconcileArchivedLifecycle(ctx, ws.DB, ws, req)
	require.Error(t, err)
	assert.ErrorIs(t, err, blerrors.ErrNotFound)
}

// TestReconcileArchivedLifecycle_EmptyReason_Error verifies that an empty Reason
// field is rejected with ErrValidation before any filesystem access occurs.
func TestReconcileArchivedLifecycle_EmptyReason_Error(t *testing.T) {
	ws := setupReconcileWorkspace(t)
	ctx := context.Background()
	writeArchivedItem(t, ws, "001-F", "feature", "active")

	req := core.ReconciliationRequest{
		ItemIDs:      []string{"001-F"},
		TargetStatus: "done",
		Reason:       "",
		Actor:        "test-actor",
	}

	_, err := core.ReconcileArchivedLifecycle(ctx, ws.DB, ws, req)
	require.Error(t, err)
	assert.ErrorIs(t, err, blerrors.ErrValidation)
}

// TestReconcileArchivedLifecycle_EmptyActor_Error verifies that an empty Actor
// field is rejected with ErrValidation before any filesystem access occurs.
func TestReconcileArchivedLifecycle_EmptyActor_Error(t *testing.T) {
	ws := setupReconcileWorkspace(t)
	ctx := context.Background()
	writeArchivedItem(t, ws, "001-F", "feature", "active")

	req := core.ReconciliationRequest{
		ItemIDs:      []string{"001-F"},
		TargetStatus: "done",
		Reason:       "valid reason",
		Actor:        "",
	}

	_, err := core.ReconcileArchivedLifecycle(ctx, ws.DB, ws, req)
	require.Error(t, err)
	assert.ErrorIs(t, err, blerrors.ErrValidation)
}

// TestReconcileArchivedLifecycle_IdempotencyKey_Repeat verifies that a second
// call carrying the same IdempotencyKey returns ReconciliationNoOp without
// re-applying any status change.
func TestReconcileArchivedLifecycle_IdempotencyKey_Repeat(t *testing.T) {
	ws := setupReconcileWorkspace(t)
	ctx := context.Background()
	writeArchivedItem(t, ws, "001-F", "feature", "active")

	req := core.ReconciliationRequest{
		ItemIDs:        []string{"001-F"},
		TargetStatus:   "done",
		Reason:         "idempotency test",
		Actor:          "test-actor",
		IdempotencyKey: "idem-key-001",
	}

	result1, err1 := core.ReconcileArchivedLifecycle(ctx, ws.DB, ws, req)
	require.NoError(t, err1)
	assert.Equal(t, core.ReconciliationCompleted, result1.Outcome)

	result2, err2 := core.ReconcileArchivedLifecycle(ctx, ws.DB, ws, req)
	require.NoError(t, err2)
	assert.Equal(t, core.ReconciliationNoOp, result2.Outcome)
}

// TestReconcileArchivedLifecycle_MultipleItems verifies that all eligible items
// in the request are reconciled and each is returned as ReconciliationCompleted.
func TestReconcileArchivedLifecycle_MultipleItems(t *testing.T) {
	ws := setupReconcileWorkspace(t)
	ctx := context.Background()
	writeArchivedItem(t, ws, "001-F", "feature", "active")
	writeArchivedItem(t, ws, "001-T", "task", "active")

	req := core.ReconciliationRequest{
		ItemIDs:      []string{"001-F", "001-T"},
		TargetStatus: "done",
		Reason:       "multiple items test",
		Actor:        "test-actor",
	}

	result, err := core.ReconcileArchivedLifecycle(ctx, ws.DB, ws, req)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, core.ReconciliationCompleted, result.Outcome)
	require.Len(t, result.Items, 2)
	for _, item := range result.Items {
		assert.Equal(t, core.ReconciliationCompleted, item.Outcome, "item %s must be completed", item.ID)
		assert.Empty(t, item.Error)
	}
}

// TestReconcileArchivedLifecycle_PartialFailure verifies that when one item
// succeeds and one does not exist, the overall outcome is ReconciliationPartial
// and the failing item carries a non-empty Error field.
func TestReconcileArchivedLifecycle_PartialFailure(t *testing.T) {
	ws := setupReconcileWorkspace(t)
	ctx := context.Background()
	writeArchivedItem(t, ws, "001-F", "feature", "active")

	req := core.ReconciliationRequest{
		ItemIDs:      []string{"001-F", "nonexistent-F"},
		TargetStatus: "done",
		Reason:       "partial failure test",
		Actor:        "test-actor",
	}

	result, err := core.ReconcileArchivedLifecycle(ctx, ws.DB, ws, req)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, core.ReconciliationPartial, result.Outcome)
	require.Len(t, result.Items, 2)

	outcomes := make(map[string]core.ReconciliationOutcome, len(result.Items))
	for _, item := range result.Items {
		outcomes[item.ID] = item.Outcome
	}
	assert.Equal(t, core.ReconciliationCompleted, outcomes["001-F"])
	assert.NotEqual(t, core.ReconciliationCompleted, outcomes["nonexistent-F"])
	assert.NotEmpty(t, findItemError(result.Items, "nonexistent-F"), "failed item must carry an error message")
}

// TestReconcileArchivedLifecycle_PathTraversal_Rejected verifies that an item ID
// containing path traversal sequences is rejected before any filesystem access.
func TestReconcileArchivedLifecycle_PathTraversal_Rejected(t *testing.T) {
	ws := setupReconcileWorkspace(t)
	ctx := context.Background()

	req := core.ReconciliationRequest{
		ItemIDs:      []string{"../etc/passwd"},
		TargetStatus: "done",
		Reason:       "path traversal test",
		Actor:        "test-actor",
	}

	_, err := core.ReconcileArchivedLifecycle(ctx, ws.DB, ws, req)
	require.Error(t, err)
}

// TestReconcileArchivedLifecycle_NoCascadeOnReArchive verifies that reconciling
// a childless archived item completes without error, confirming WithCascade(false)
// behavior does not interfere with a clean re-archive.
func TestReconcileArchivedLifecycle_NoCascadeOnReArchive(t *testing.T) {
	ws := setupReconcileWorkspace(t)
	ctx := context.Background()
	writeArchivedItem(t, ws, "001-F", "feature", "active")

	req := core.ReconciliationRequest{
		ItemIDs:      []string{"001-F"},
		TargetStatus: "done",
		Reason:       "no cascade test",
		Actor:        "test-actor",
	}

	result, err := core.ReconcileArchivedLifecycle(ctx, ws.DB, ws, req)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, core.ReconciliationCompleted, result.Outcome)
}

// TestReconcileArchivedLifecycle_InvalidTransitionPath_Error verifies that a
// non-terminal TargetStatus (e.g. "queued") is rejected with ErrValidation.
func TestReconcileArchivedLifecycle_InvalidTransitionPath_Error(t *testing.T) {
	ws := setupReconcileWorkspace(t)
	ctx := context.Background()
	writeArchivedItem(t, ws, "001-F", "feature", "active")

	req := core.ReconciliationRequest{
		ItemIDs:      []string{"001-F"},
		TargetStatus: "queued", // not a terminal status — invalid for reconciliation
		Reason:       "invalid transition test",
		Actor:        "test-actor",
	}

	_, err := core.ReconcileArchivedLifecycle(ctx, ws.DB, ws, req)
	require.Error(t, err)
	assert.ErrorIs(t, err, blerrors.ErrValidation)
}

// TestReconcileArchivedLifecycle_DurableCustomFieldsMetadata verifies that after
// successful reconciliation, the re-archived file's custom_fields contains
// reconciled_at, reconciliation_reason, and reconciliation_actor.
func TestReconcileArchivedLifecycle_DurableCustomFieldsMetadata(t *testing.T) {
	ws := setupReconcileWorkspace(t)
	ctx := context.Background()
	writeArchivedItem(t, ws, "001-F", "feature", "active")

	req := core.ReconciliationRequest{
		ItemIDs:      []string{"001-F"},
		TargetStatus: "done",
		Reason:       "metadata durability test",
		Actor:        "metadata-actor",
	}

	result, err := core.ReconcileArchivedLifecycle(ctx, ws.DB, ws, req)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, core.ReconciliationCompleted, result.Outcome)

	archivePath := filepath.Join(ws.StorageRoot, "archive", "001-F.md")
	raw, readErr := os.ReadFile(archivePath)
	require.NoError(t, readErr)
	fm, _, parseErr := models.ParseFrontmatter(string(raw))
	require.NoError(t, parseErr)

	require.Contains(t, fm, "custom_fields", "custom_fields must be present in archive frontmatter")
	cf, ok := fm["custom_fields"].(map[string]any)
	require.True(t, ok, "custom_fields must unmarshal as map[string]any")
	assert.Contains(t, cf, "reconciled_at", "custom_fields must contain reconciled_at")
	assert.Contains(t, cf, "reconciliation_reason", "custom_fields must contain reconciliation_reason")
	assert.Contains(t, cf, "reconciliation_actor", "custom_fields must contain reconciliation_actor")
	assert.Equal(t, "metadata durability test", cf["reconciliation_reason"])
	assert.Equal(t, "metadata-actor", cf["reconciliation_actor"])
}

// TestReconcileArchivedLifecycle_UnarchiveIndeterminate is a characterization test
// documenting the ErrWriteIndeterminate contract: when any step returns
// ErrWriteIndeterminate, the item is recorded as indeterminate and no rollback
// occurs. Without a write-seam injection, we verify the normal success path and
// that the outcome is ReconciliationCompleted (not indeterminate).
func TestReconcileArchivedLifecycle_UnarchiveIndeterminate(t *testing.T) {
	ws := setupReconcileWorkspace(t)
	ctx := context.Background()
	writeArchivedItem(t, ws, "001-F", "feature", "active")

	req := core.ReconciliationRequest{
		ItemIDs:      []string{"001-F"},
		TargetStatus: "done",
		Reason:       "indeterminate characterization",
		Actor:        "test-actor",
	}

	// Without write-seam injection, the normal success path applies.
	// ErrWriteIndeterminate injection would require a test seam not yet exposed.
	result, err := core.ReconcileArchivedLifecycle(ctx, ws.DB, ws, req)
	require.NoError(t, err)
	require.NotNil(t, result)
	// Verify the normal (non-indeterminate) path returns completed.
	assert.Equal(t, core.ReconciliationCompleted, result.Outcome)
}
