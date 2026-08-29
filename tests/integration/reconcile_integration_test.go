// Package integration_test contains end-to-end integration tests for backlogit features.
// 152.009-T: Reconcile and stash provenance integration tests.
// P-002 exemption: RED evidence comes from 152.002-T and 152.006-T.
// These tests exercise the complete data path in a real workspace, including
// workspace state verification via file and DB round-trips.
package integration_test

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/config"
	"github.com/softwaresalt/backlogit/internal/core"
	"github.com/softwaresalt/backlogit/internal/models"
)

// setupReconcileIntegrationWorkspace creates a temporary workspace initialized
// via config.WriteDefaults and core.NewWorkspace for reconcile and stash
// provenance integration tests. The workspace is fully wired (hooks, header-def,
// templates) and is cleaned up via t.Cleanup when the test exits.
func setupReconcileIntegrationWorkspace(t *testing.T) *core.Workspace {
	t.Helper()
	tmpDir := t.TempDir()
	backlogDir := filepath.Join(tmpDir, ".backlogit")
	require.NoError(t, os.MkdirAll(backlogDir, 0o755))
	require.NoError(t, config.WriteDefaults(backlogDir))
	// config.WriteDefaults creates queue/ but not archive/; create it so
	// ArchiveItem and CorrectStashProvenance can operate without error.
	require.NoError(t, os.MkdirAll(filepath.Join(backlogDir, "archive"), 0o755))
	ws, err := core.NewWorkspace(context.Background(), tmpDir)
	require.NoError(t, err)
	t.Cleanup(func() { _ = ws.Close() })
	return ws
}

// writeStashProvenanceArchiveEntry appends one stash archive entry to
// .backlogit/archive/stash.jsonl, creating the file when absent. The entry
// carries the given stashID as its id and harvestedArtifactID as the
// harvested_artifact_id so CorrectStashProvenance can locate and verify it.
func writeStashProvenanceArchiveEntry(t *testing.T, ws *core.Workspace, stashID, harvestedArtifactID string) {
	t.Helper()
	archiveDir := filepath.Join(ws.StorageRoot, "archive")
	require.NoError(t, os.MkdirAll(archiveDir, 0o755))

	entry := map[string]any{
		"id":                    stashID,
		"priority":              "medium",
		"kind":                  "feature",
		"text":                  "Test stash entry " + stashID,
		"archived_at":           "2026-01-01T00:00:00Z",
		"reason":                "harvested",
		"harvested_artifact_id": harvestedArtifactID,
	}
	data, err := json.Marshal(entry)
	require.NoError(t, err)

	stashArchivePath := filepath.Join(archiveDir, "stash.jsonl")
	f, err := os.OpenFile(stashArchivePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	require.NoError(t, err)
	defer func() { _ = f.Close() }()
	_, err = f.Write(append(data, '\n'))
	require.NoError(t, err)
}

// TestReconcileIntegration_EndToEnd verifies the full reconcile lifecycle in a
// real workspace:
//  1. An artifact is created, transitioned to active, and archived.
//  2. The archived file carries archived_status: active.
//  3. ReconcileArchivedLifecycle transitions it to archived_status: done.
//  4. The archive file reflects the corrected status and reconciliation metadata.
//  5. A second call returns ReconciliationNoOp (idempotency).
func TestReconcileIntegration_EndToEnd(t *testing.T) {
	// Arrange: full workspace with a feature artifact created, activated,
	// and archived so archived_status is "active".
	ws := setupReconcileIntegrationWorkspace(t)
	ctx := context.Background()

	artifact, err := core.CreateArtifact(ctx, ws, "Test feature for reconciliation", "feature")
	require.NoError(t, err)

	_, err = core.UpdateArtifact(ctx, ws, artifact.ID, map[string]any{"status": "active"})
	require.NoError(t, err)

	record, err := core.ArchiveItem(ctx, ws.DB, ws, artifact.ID)
	require.NoError(t, err)
	require.NotEmpty(t, record.ArchivePath, "ArchiveItem must return a non-empty ArchivePath")

	// Confirm precondition: the archived file carries archived_status: active.
	raw, err := os.ReadFile(record.ArchivePath)
	require.NoError(t, err)
	fm, _, err := models.ParseFrontmatter(string(raw))
	require.NoError(t, err)
	archivedStatus, _ := fm["archived_status"].(string)
	assert.Equal(t, "active", archivedStatus,
		"archived item must carry archived_status: active before reconciliation")

	// Act: reconcile the archived item to "done".
	req := core.ReconciliationRequest{
		ItemIDs:      []string{artifact.ID},
		TargetStatus: "done",
		Reason:       "lifecycle reconcile for integration test",
		Actor:        "integration-test",
	}
	result, err := core.ReconcileArchivedLifecycle(ctx, ws.DB, ws, req)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Assert: overall outcome is completed with a single item result.
	assert.Equal(t, core.ReconciliationCompleted, result.Outcome)
	require.Len(t, result.Items, 1)
	assert.Equal(t, artifact.ID, result.Items[0].ID)
	assert.Equal(t, core.ReconciliationCompleted, result.Items[0].Outcome)
	assert.Empty(t, result.Items[0].Error)

	// Assert: archive file reflects the reconciled state — archived_status and
	// reconciliation metadata must be durably persisted in frontmatter.
	rawAfter, err := os.ReadFile(record.ArchivePath)
	require.NoError(t, err)
	fmAfter, _, err := models.ParseFrontmatter(string(rawAfter))
	require.NoError(t, err)

	archivedStatusAfter, _ := fmAfter["archived_status"].(string)
	assert.Equal(t, "done", archivedStatusAfter,
		"archived_status must be corrected to done after reconciliation")

	cf, ok := fmAfter["custom_fields"].(map[string]any)
	require.True(t, ok, "custom_fields must be present in archive frontmatter after reconciliation")
	assert.Contains(t, cf, "reconciled_at",
		"custom_fields must contain reconciled_at timestamp")
	assert.Equal(t, "integration-test", cf["reconciliation_actor"],
		"reconciliation_actor must match the actor supplied in the request")

	// Assert idempotency: a second call with the same target returns NoOp because
	// archived_status is already "done".
	result2, err := core.ReconcileArchivedLifecycle(ctx, ws.DB, ws, req)
	require.NoError(t, err)
	require.NotNil(t, result2)
	assert.Equal(t, core.ReconciliationNoOp, result2.Outcome,
		"reconciling an already-done item must return ReconciliationNoOp")
}

// TestReconcileIntegration_ErrWriteIndeterminate documents the ErrWriteIndeterminate
// contract for the reconciliation path. Without a write-seam to inject durable-write
// failures, this test verifies that the normal reconciliation path completes
// successfully and does not return ReconciliationIndeterminate. Injecting
// ErrWriteIndeterminate requires a test seam that is not exposed at the integration
// boundary; the unit-level characterization tests in archive_reconcile_test.go cover
// that branch.
func TestReconcileIntegration_ErrWriteIndeterminate(t *testing.T) {
	// Arrange: a workspace with an archived "active" artifact.
	ws := setupReconcileIntegrationWorkspace(t)
	ctx := context.Background()

	artifact, err := core.CreateArtifact(ctx, ws, "Indeterminate contract feature", "feature")
	require.NoError(t, err)
	_, err = core.UpdateArtifact(ctx, ws, artifact.ID, map[string]any{"status": "active"})
	require.NoError(t, err)
	_, err = core.ArchiveItem(ctx, ws.DB, ws, artifact.ID)
	require.NoError(t, err)

	req := core.ReconciliationRequest{
		ItemIDs:      []string{artifact.ID},
		TargetStatus: "done",
		Reason:       "ErrWriteIndeterminate contract verification",
		Actor:        "integration-test",
	}

	// Act: reconcile without any injected failure.
	result, err := core.ReconcileArchivedLifecycle(ctx, ws.DB, ws, req)

	// Assert: the happy path must succeed and return ReconciliationCompleted,
	// never ReconciliationIndeterminate, proving the seam is only triggered by
	// an injected durable-write fault.
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, core.ReconciliationCompleted, result.Outcome,
		"without write-seam injection the reconciliation must complete successfully")
	assert.NotEqual(t, core.ReconciliationIndeterminate, result.Outcome,
		"ReconciliationIndeterminate must not appear on the happy path")
}

// TestStashProvenance_EndToEnd verifies the complete stash provenance correction
// workflow in a real workspace:
//  1. A stash archive entry is seeded with a historical harvested_artifact_id.
//  2. A canonical delivery artifact is created with source_stash_id matching the stash.
//  3. CorrectStashProvenance records a correction and returns StashProvenanceCorrected.
//  4. provenance_corrections.jsonl is verified to contain the correct fields.
//  5. A repeat call with the same parameters returns StashProvenanceNoOp (idempotency).
//  6. A call with a different canonical artifact for the same stash returns an error.
func TestStashProvenance_EndToEnd(t *testing.T) {
	// Arrange: full workspace with a stash archive entry and two candidate
	// canonical delivery artifacts.
	ws := setupReconcileIntegrationWorkspace(t)
	ctx := context.Background()

	const stashID = "TEST-STASH-001"
	const historicalID = "historical-001-F"

	// Seed the stash archive with a harvested entry whose corrected canonical
	// delivery differs from the historically auto-linked artifact.
	writeStashProvenanceArchiveEntry(t, ws, stashID, historicalID)

	// Create the canonical delivery artifact with source_stash_id in custom_fields.
	canonical1, err := core.CreateArtifact(ctx, ws, "Canonical delivery feature", "feature",
		core.WithFields(map[string]any{"source_stash_id": stashID}),
	)
	require.NoError(t, err)

	// Create a second artifact (also linked to the same stash) for the conflict test.
	canonical2, err := core.CreateArtifact(ctx, ws, "Alternate canonical delivery", "feature",
		core.WithFields(map[string]any{"source_stash_id": stashID}),
	)
	require.NoError(t, err)

	// Act: correct the stash provenance to point at canonical1.
	req1 := core.StashProvenanceCorrectionRequest{
		StashID:                     stashID,
		CanonicalDeliveryArtifactID: canonical1.ID,
		Reason:                      "provenance correction for integration test",
		Actor:                       "integration-test",
	}
	result1, err := core.CorrectStashProvenance(ctx, ws, req1)
	require.NoError(t, err)
	require.NotNil(t, result1)

	// Assert: first correction succeeds with all expected fields populated.
	assert.Equal(t, core.StashProvenanceCorrected, result1.Outcome)
	assert.Equal(t, stashID, result1.StashID)
	assert.Equal(t, historicalID, result1.HistoricalArtifactID)
	assert.Equal(t, canonical1.ID, result1.CanonicalDeliveryArtifactID)
	assert.NotEmpty(t, result1.Message)

	// Assert: provenance_corrections.jsonl contains the durable correction record.
	correctionsPath := filepath.Join(ws.StorageRoot, "archive", "provenance_corrections.jsonl")
	require.FileExists(t, correctionsPath,
		"provenance_corrections.jsonl must be created after a successful correction")

	corrFile, err := os.Open(correctionsPath)
	require.NoError(t, err)
	defer func() { _ = corrFile.Close() }()

	scanner := bufio.NewScanner(corrFile)
	found := false
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var correction core.ProvenanceCorrection
		require.NoError(t, json.Unmarshal(line, &correction),
			"each line in provenance_corrections.jsonl must be valid JSON")
		if correction.StashID == stashID {
			found = true
			assert.Equal(t, canonical1.ID, correction.CanonicalDeliveryArtifactID,
				"correction must record the canonical delivery artifact ID")
			assert.Equal(t, historicalID, correction.HistoricalArtifactID,
				"correction must preserve the original harvested artifact ID")
			assert.Equal(t, "stash_provenance_corrected", correction.EventType,
				"correction must carry the stash_provenance_corrected event type")
			assert.NotEmpty(t, correction.CorrectedAt,
				"corrected_at must be a non-empty RFC3339 timestamp")
			assert.Equal(t, "integration-test", correction.Actor,
				"actor must match the request actor")
		}
	}
	require.NoError(t, scanner.Err())
	assert.True(t, found,
		"provenance_corrections.jsonl must contain a correction record for stash %s", stashID)

	// Assert idempotency: a second call with the same parameters returns NoOp
	// without appending a duplicate record.
	result2, err := core.CorrectStashProvenance(ctx, ws, req1)
	require.NoError(t, err)
	require.NotNil(t, result2)
	assert.Equal(t, core.StashProvenanceNoOp, result2.Outcome,
		"correcting the same stash entry to the same canonical must return StashProvenanceNoOp")

	// Assert conflict: correcting the same stash entry to a different canonical
	// artifact is rejected because a conflicting correction already exists.
	req2 := core.StashProvenanceCorrectionRequest{
		StashID:                     stashID,
		CanonicalDeliveryArtifactID: canonical2.ID,
		Reason:                      "conflicting correction attempt",
		Actor:                       "integration-test",
	}
	_, err = core.CorrectStashProvenance(ctx, ws, req2)
	require.Error(t, err,
		"correcting the same stash entry to a different canonical must be rejected as a conflict")
}
