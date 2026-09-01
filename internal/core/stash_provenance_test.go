package core_test

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/core"
	"github.com/softwaresalt/backlogit/internal/db"
	blerrors "github.com/softwaresalt/backlogit/internal/errors"
	"github.com/softwaresalt/backlogit/internal/models"
)

// setupProvenanceWorkspace creates an isolated workspace for stash provenance
// correction tests. It seeds both feature and task artifact types.
func setupProvenanceWorkspace(t *testing.T) *core.Workspace {
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

// writeArchivedStashEntry appends one stash archive entry to
// .backlogit/archive/stash.jsonl, creating the file when absent.
func writeArchivedStashEntry(t *testing.T, ws *core.Workspace, stashID, harvestedArtifactID string) {
	t.Helper()
	archiveDir := filepath.Join(ws.StorageRoot, "archive")
	require.NoError(t, os.MkdirAll(archiveDir, 0o755))

	archivedAt := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC).Format(time.RFC3339)
	line := fmt.Sprintf(
		`{"id":%q,"priority":"medium","kind":"feature","text":"Test stash %s","archived_at":%q,"reason":"harvested","harvested_artifact_id":%q}`,
		stashID, stashID, archivedAt, harvestedArtifactID,
	)

	stashArchivePath := filepath.Join(archiveDir, "stash.jsonl")
	f, err := os.OpenFile(stashArchivePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	require.NoError(t, err)
	defer func() { _ = f.Close() }()
	_, writeErr := fmt.Fprintln(f, line)
	require.NoError(t, writeErr)
}

// writeArtifactWithSourceStash writes an artifact markdown file to the queue
// directory and upserts a matching DB record whose custom_fields contain
// source_stash_id pointing to sourceStashID.
func writeArtifactWithSourceStash(t *testing.T, ws *core.Workspace, artifactID, sourceStashID string) {
	t.Helper()
	queueDir := filepath.Join(ws.StorageRoot, "queue")
	require.NoError(t, os.MkdirAll(queueDir, 0o755))

	content := fmt.Sprintf(
		"---\nid: %s\ntitle: Test artifact %s\nstatus: active\nartifact_type: feature\ncustom_fields:\n  source_stash_id: \"%s\"\n---\nTest body\n",
		artifactID, artifactID, sourceStashID,
	)
	require.NoError(t, os.WriteFile(filepath.Join(queueDir, artifactID+".md"), []byte(content), 0o644))
	require.NoError(t, db.UpsertItem(context.Background(), ws.DB, &models.Artifact{
		ID:           artifactID,
		Title:        "Test artifact " + artifactID,
		Status:       models.StatusActive,
		ArtifactType: "feature",
		CustomFields: map[string]any{"source_stash_id": sourceStashID},
	}))
}

// TestCorrectStashProvenance_HappyPath verifies that a provenance correction is
// recorded when the stash entry exists, the canonical artifact exists, and its
// source_stash_id matches the stash entry.
func TestCorrectStashProvenance_HappyPath(t *testing.T) {
	ws := setupProvenanceWorkspace(t)
	ctx := context.Background()
	writeArchivedStashEntry(t, ws, "AAAA1111", "001-F")
	writeArtifactWithSourceStash(t, ws, "002-F", "AAAA1111")

	req := core.StashProvenanceCorrectionRequest{
		StashID:                     "AAAA1111",
		CanonicalDeliveryArtifactID: "002-F",
		Reason:                      "canonical delivery correction",
		Actor:                       "test-actor",
	}

	result, err := core.CorrectStashProvenance(ctx, ws, req)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, core.StashProvenanceCorrected, result.Outcome)
	assert.Equal(t, "AAAA1111", result.StashID)
	assert.Equal(t, "001-F", result.HistoricalArtifactID)
	assert.Equal(t, "002-F", result.CanonicalDeliveryArtifactID)
	assert.NotEmpty(t, result.Message)
}

// TestCorrectStashProvenance_AlreadyCorrected_NoOp verifies that a second call
// with identical parameters returns StashProvenanceNoOp without writing a
// duplicate correction record.
func TestCorrectStashProvenance_AlreadyCorrected_NoOp(t *testing.T) {
	ws := setupProvenanceWorkspace(t)
	ctx := context.Background()
	writeArchivedStashEntry(t, ws, "AAAA1111", "001-F")
	writeArtifactWithSourceStash(t, ws, "002-F", "AAAA1111")

	req := core.StashProvenanceCorrectionRequest{
		StashID:                     "AAAA1111",
		CanonicalDeliveryArtifactID: "002-F",
		Reason:                      "canonical delivery correction",
		Actor:                       "test-actor",
	}

	result1, err1 := core.CorrectStashProvenance(ctx, ws, req)
	require.NoError(t, err1)
	assert.Equal(t, core.StashProvenanceCorrected, result1.Outcome)

	result2, err2 := core.CorrectStashProvenance(ctx, ws, req)
	require.NoError(t, err2)
	assert.Equal(t, core.StashProvenanceNoOp, result2.Outcome)
}

// TestCorrectStashProvenance_ConflictingCorrection_Error verifies that attempting
// to correct the same stash entry to a different canonical artifact than a
// previously recorded correction is rejected.
func TestCorrectStashProvenance_ConflictingCorrection_Error(t *testing.T) {
	ws := setupProvenanceWorkspace(t)
	ctx := context.Background()
	writeArchivedStashEntry(t, ws, "AAAA1111", "001-F")
	writeArtifactWithSourceStash(t, ws, "002-F", "AAAA1111")
	writeArtifactWithSourceStash(t, ws, "003-F", "AAAA1111")

	req1 := core.StashProvenanceCorrectionRequest{
		StashID:                     "AAAA1111",
		CanonicalDeliveryArtifactID: "002-F",
		Reason:                      "first correction",
		Actor:                       "test-actor",
	}

	result1, err1 := core.CorrectStashProvenance(ctx, ws, req1)
	require.NoError(t, err1)
	assert.Equal(t, core.StashProvenanceCorrected, result1.Outcome)

	// Attempt to correct the same stash entry to a different canonical artifact.
	req2 := core.StashProvenanceCorrectionRequest{
		StashID:                     "AAAA1111",
		CanonicalDeliveryArtifactID: "003-F",
		Reason:                      "conflicting correction",
		Actor:                       "test-actor",
	}

	_, err2 := core.CorrectStashProvenance(ctx, ws, req2)
	require.Error(t, err2, "conflicting correction must be rejected")
}

// TestCorrectStashProvenance_StashNotFound_Error verifies that correcting a
// non-existent stash entry returns ErrNotFound.
func TestCorrectStashProvenance_StashNotFound_Error(t *testing.T) {
	ws := setupProvenanceWorkspace(t)
	ctx := context.Background()

	req := core.StashProvenanceCorrectionRequest{
		StashID:                     "DOESNOTEXIST",
		CanonicalDeliveryArtifactID: "001-F",
		Reason:                      "stash not found test",
		Actor:                       "test-actor",
	}

	_, err := core.CorrectStashProvenance(ctx, ws, req)
	require.Error(t, err)
	assert.ErrorIs(t, err, blerrors.ErrNotFound)
}

// TestCorrectStashProvenance_ArtifactNotFound_Error verifies that specifying a
// canonical delivery artifact that does not exist in the workspace returns
// ErrNotFound.
func TestCorrectStashProvenance_ArtifactNotFound_Error(t *testing.T) {
	ws := setupProvenanceWorkspace(t)
	ctx := context.Background()
	writeArchivedStashEntry(t, ws, "AAAA1111", "001-F")
	// 999-F is never created in queue or DB.

	req := core.StashProvenanceCorrectionRequest{
		StashID:                     "AAAA1111",
		CanonicalDeliveryArtifactID: "999-F",
		Reason:                      "artifact not found test",
		Actor:                       "test-actor",
	}

	_, err := core.CorrectStashProvenance(ctx, ws, req)
	require.Error(t, err)
	assert.ErrorIs(t, err, blerrors.ErrNotFound)
}

// TestCorrectStashProvenance_SourceStashMismatch_Error verifies that when the
// canonical artifact's source_stash_id points to a different stash entry, the
// correction is rejected.
func TestCorrectStashProvenance_SourceStashMismatch_Error(t *testing.T) {
	ws := setupProvenanceWorkspace(t)
	ctx := context.Background()
	writeArchivedStashEntry(t, ws, "AAAA1111", "001-F")
	// 002-F was harvested from a different stash entry.
	writeArtifactWithSourceStash(t, ws, "002-F", "BBBB2222")

	req := core.StashProvenanceCorrectionRequest{
		StashID:                     "AAAA1111",
		CanonicalDeliveryArtifactID: "002-F",
		Reason:                      "source stash mismatch test",
		Actor:                       "test-actor",
	}

	_, err := core.CorrectStashProvenance(ctx, ws, req)
	require.Error(t, err, "mismatched source_stash_id must be rejected")
}

// TestCorrectStashProvenance_EmptyReason_Error verifies that an empty Reason
// field is rejected with ErrValidation.
func TestCorrectStashProvenance_EmptyReason_Error(t *testing.T) {
	ws := setupProvenanceWorkspace(t)
	ctx := context.Background()
	writeArchivedStashEntry(t, ws, "AAAA1111", "001-F")
	writeArtifactWithSourceStash(t, ws, "002-F", "AAAA1111")

	req := core.StashProvenanceCorrectionRequest{
		StashID:                     "AAAA1111",
		CanonicalDeliveryArtifactID: "002-F",
		Reason:                      "",
		Actor:                       "test-actor",
	}

	_, err := core.CorrectStashProvenance(ctx, ws, req)
	require.Error(t, err)
	assert.ErrorIs(t, err, blerrors.ErrValidation)
}

// TestCorrectStashProvenance_EmptyActor_Error verifies that an empty Actor field
// is rejected with ErrValidation.
func TestCorrectStashProvenance_EmptyActor_Error(t *testing.T) {
	ws := setupProvenanceWorkspace(t)
	ctx := context.Background()
	writeArchivedStashEntry(t, ws, "AAAA1111", "001-F")
	writeArtifactWithSourceStash(t, ws, "002-F", "AAAA1111")

	req := core.StashProvenanceCorrectionRequest{
		StashID:                     "AAAA1111",
		CanonicalDeliveryArtifactID: "002-F",
		Reason:                      "valid reason",
		Actor:                       "",
	}

	_, err := core.CorrectStashProvenance(ctx, ws, req)
	require.Error(t, err)
	assert.ErrorIs(t, err, blerrors.ErrValidation)
}

// TestCorrectStashProvenance_EventDurability verifies that after a successful
// correction, a durable ProvenanceCorrection record is appended to
// provenance_corrections.jsonl with all expected fields populated.
func TestCorrectStashProvenance_EventDurability(t *testing.T) {
	ws := setupProvenanceWorkspace(t)
	ctx := context.Background()
	writeArchivedStashEntry(t, ws, "AAAA1111", "001-F")
	writeArtifactWithSourceStash(t, ws, "002-F", "AAAA1111")

	req := core.StashProvenanceCorrectionRequest{
		StashID:                     "AAAA1111",
		CanonicalDeliveryArtifactID: "002-F",
		Reason:                      "durability test",
		Actor:                       "durability-actor",
	}

	result, err := core.CorrectStashProvenance(ctx, ws, req)
	require.NoError(t, err)
	assert.Equal(t, core.StashProvenanceCorrected, result.Outcome)

	provenancePath := filepath.Join(ws.StorageRoot, "archive", "provenance_corrections.jsonl")
	require.FileExists(t, provenancePath, "provenance_corrections.jsonl must exist after correction")

	f, openErr := os.Open(provenancePath)
	require.NoError(t, openErr)
	defer func() { _ = f.Close() }()

	var corrections []core.ProvenanceCorrection
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var c core.ProvenanceCorrection
		require.NoError(t, json.Unmarshal(line, &c), "each JSONL line must be valid ProvenanceCorrection JSON")
		corrections = append(corrections, c)
	}
	require.NoError(t, scanner.Err())

	require.Len(t, corrections, 1, "exactly one correction record must be written")
	assert.Equal(t, "AAAA1111", corrections[0].StashID)
	assert.Equal(t, "001-F", corrections[0].HistoricalArtifactID)
	assert.Equal(t, "002-F", corrections[0].CanonicalDeliveryArtifactID)
	assert.Equal(t, "durability test", corrections[0].Reason)
	assert.Equal(t, "durability-actor", corrections[0].Actor)
	assert.NotEmpty(t, corrections[0].CorrectedAt, "corrected_at must be a non-empty timestamp")
}
