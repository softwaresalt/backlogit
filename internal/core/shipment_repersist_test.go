package core

// Regression tests for archive/shipment re-persist field-drop bugs.
//
// D04D63D0 (129.001-T): ship_shipment aborts when a feature links an
// already-archived deliberation because attachCommitToItems tries to re-persist
// the deliberation via the DB fast-path, which returns empty provenance, causing
// the write-boundary guard to refuse.
//
// 7A965F8A (129.002-T): attachCommitToItems re-persists every stamped candidate
// via the DB fast-path, which carries neither item_links nor archive provenance,
// so modeled links (e.g. spike_ref) are silently dropped from frontmatter.
//
// Fix seam: internal/core/shipment_lifecycle.go attachCommitToItems.

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	bldb "github.com/softwaresalt/backlogit/internal/db"
	"github.com/softwaresalt/backlogit/internal/models"
)

// ---------------------------------------------------------------------------
// 129.001-T: Skip already-archived linked deliberation in shipment archive flow
// ---------------------------------------------------------------------------

// TestShipShipment_SkipsAlreadyArchivedLinkedDeliberation is the RED harness
// for stash D04D63D0. It builds a shipment whose feature links an
// already-archived deliberation, ships the shipment, and asserts three things:
//
//	(a) ShipShipment completes without the provenance-guard error.
//	(b) The shipment manifest is archived.
//	(c) The already-archived deliberation's pre-existing commit SHA and archive
//	    provenance (archived_from / archived_status) are UNCHANGED after ship —
//	    proving the deliberation was SKIPPED, not re-stamped or re-persisted.
//
// Assertion (c) is mandatory: without it the test can false-green under the
// Unit-2 reload-from-Markdown fix, which would let a (semantically wrong)
// commit-stamp succeed by restoring provenance, while still re-persisting
// the deliberation. Only the skip assertion proves skip semantics.
func TestShipShipment_SkipsAlreadyArchivedLinkedDeliberation(t *testing.T) {
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()

	// --- Arrange ---

	// 1. Create and archive a deliberation, capturing its pre-ship provenance.
	deliberation, err := CreateArtifact(ctx, ws, "Already archived deliberation", "deliberation")
	require.NoError(t, err)
	require.NoError(t, bldb.UpsertItem(ctx, ws.DB, deliberation))

	_, err = ArchiveItem(ctx, ws.DB, ws, deliberation.ID, WithCommitSHA("original-commit-abc123"))
	require.NoError(t, err)

	// Reload the archived deliberation from Markdown to capture pre-ship provenance.
	preShipDelibPath, err := FindArtifactPath(ctx, ws, deliberation.ID)
	require.NoError(t, err)
	preShipRaw, err := os.ReadFile(preShipDelibPath)
	require.NoError(t, err)
	preShipFM, _, err := models.ParseFrontmatter(string(preShipRaw))
	require.NoError(t, err)
	require.Equal(t, "archived", preShipFM["status"], "deliberation must be archived before ship")
	preShipCommit, _ := preShipFM["commit"].(string)
	preShipArchivedFrom, _ := preShipFM["archived_from"].(string)
	preShipArchivedStatus, _ := preShipFM["archived_status"].(string)
	require.NotEmpty(t, preShipArchivedFrom, "deliberation must have archived_from before ship")
	require.NotEmpty(t, preShipArchivedStatus, "deliberation must have archived_status before ship")

	// 2. Create a feature that references the deliberation ID in its description
	//    so linkedDeliberationIDs picks it up.
	feature, err := CreateArtifact(ctx, ws, "Feature with archived deliberation link", "feature",
		WithDescription("Origin: "+deliberation.ID))
	require.NoError(t, err)
	require.NoError(t, bldb.UpsertItem(ctx, ws.DB, feature))

	// 3. Create a task under the feature and build a shipment.
	task, err := CreateArtifact(ctx, ws, "Feature task", "task", WithParent(feature.ID))
	require.NoError(t, err)
	require.NoError(t, bldb.UpsertItem(ctx, ws.DB, task))

	// Feature-inclusive manifest: under the membership contract (133-F) a
	// children-only manifest would skip the feature's linkedDeliberationIDs
	// collection entirely, leaving the already-archived-deliberation skip path
	// (this test's actual focus) unexercised. Listing the feature keeps that
	// path reachable.
	shipment, err := CreateShipment(ctx, ws, "Shipment with archived deliberation link", []string{feature.ID, task.ID})
	require.NoError(t, err)
	_, err = ClaimShipment(ctx, ws, shipment.ID)
	require.NoError(t, err)

	commit := &CommitMetadata{
		SHA:     "ship-commit-deadbeef",
		Message: "merge: feature with archived deliberation link",
		Author:  "tester@example.com",
	}

	// --- Act ---
	// (a) Ship must complete without the provenance-guard error.
	result, err := ShipShipment(ctx, ws, shipment.ID, commit)

	// --- Assert ---

	// (a) No error — the provenance-guard abort must not occur.
	require.NoError(t, err, "ShipShipment must complete when feature links an already-archived deliberation")
	require.NotNil(t, result)

	// (b) Shipment manifest is archived.
	assert.Contains(t, result.ArchivedIDs, shipment.ID,
		"shipment manifest must be archived after ship")

	// (c) The already-archived deliberation's commit SHA and archive provenance
	// are UNCHANGED — proving it was skipped, not re-stamped or re-persisted.
	assertArchivedDeliberationUnchanged(t, ctx, ws, deliberation.ID,
		preShipCommit, preShipArchivedFrom, preShipArchivedStatus)
}

// assertArchivedDeliberationUnchanged re-reads an archived deliberation from
// disk and asserts its commit SHA and archive provenance match the expected
// snapshot (proving it was skipped, not re-persisted, during ship).
func assertArchivedDeliberationUnchanged(t *testing.T, ctx context.Context, ws *Workspace, id, wantCommit, wantFrom, wantStatus string) {
	t.Helper()
	path, err := FindArtifactPath(ctx, ws, id)
	require.NoError(t, err)
	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	fm, _, err := models.ParseFrontmatter(string(raw))
	require.NoError(t, err)

	gotCommit, _ := fm["commit"].(string)
	gotFrom, _ := fm["archived_from"].(string)
	gotStatus, _ := fm["archived_status"].(string)

	assert.Equal(t, wantCommit, gotCommit, "%s: commit SHA must be unchanged after ship", id)
	assert.Equal(t, wantFrom, gotFrom, "%s: archived_from must be unchanged after ship", id)
	assert.Equal(t, wantStatus, gotStatus, "%s: archived_status must be unchanged after ship", id)
}

// ---------------------------------------------------------------------------
// 129.002-T: Reload from Markdown before re-persist so item_links survive
// ---------------------------------------------------------------------------

// TestAttachCommitToItems_PreservesItemLinks_AfterRepersist is the RED harness
// for stash 7A965F8A. It exercises attachCommitToItems (the re-persist seam in
// ShipShipment) on a stamped NON-ARCHIVED candidate carrying a spike_ref link,
// and asserts the frontmatter links block survives re-persist.
//
// This is the reachable seam: attachCommitToItems re-persists non-archived
// members (Unit 1 skips already-archived candidates). The test table covers
// both the populated and empty/nil-links cases to guard against the documented
// omitempty false-green (docs/compound/2026-07-21-omitempty-defeats-arrays-...).
func TestAttachCommitToItems_PreservesItemLinks_AfterRepersist(t *testing.T) {
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()

	// Create a spike target that the feature will link to.
	spike, err := CreateArtifact(ctx, ws, "Spike artifact", "feature")
	require.NoError(t, err)
	require.NoError(t, bldb.UpsertItem(ctx, ws.DB, spike))

	// Create a feature carrying a spike_ref link in its frontmatter.
	feature, err := CreateArtifact(ctx, ws, "Feature with spike_ref link", "feature")
	require.NoError(t, err)

	// Write the spike_ref link directly to the feature's Markdown file, as the
	// CLI/MCP AddLink path would, so the DB fast-path (GetItem) does NOT carry
	// it — this reproduces the divergence the fix must close.
	featurePath, err := FindArtifactPath(ctx, ws, feature.ID)
	require.NoError(t, err)
	raw, err := os.ReadFile(featurePath)
	require.NoError(t, err)
	fm, body, err := models.ParseFrontmatter(string(raw))
	require.NoError(t, err)
	fm["links"] = []map[string]any{{"target_id": spike.ID, "link_type": "spike_ref"}}
	updated := models.SerializeFrontmatter(fm, body)
	require.NoError(t, os.WriteFile(featurePath, []byte(updated), 0o644))

	// UpsertItem with the original struct (without links) so the DB index has
	// the feature WITHOUT links. The Markdown file now has links; the DB does not.
	// This is the exact divergence that causes the drop.
	require.NoError(t, bldb.UpsertItem(ctx, ws.DB, feature))

	// Create a task with no links (the nil/empty-links case).
	task, err := CreateArtifact(ctx, ws, "Task without links", "task", WithParent(feature.ID))
	require.NoError(t, err)
	require.NoError(t, bldb.UpsertItem(ctx, ws.DB, task))

	commit := &CommitMetadata{SHA: "repersist-commit-cafebabe"}

	tests := []struct {
		name         string
		id           string
		wantLinks    []models.ArtifactLink
		wantNilLinks bool
	}{
		{
			name:      "feature with spike_ref link survives re-persist",
			id:        feature.ID,
			wantLinks: []models.ArtifactLink{{TargetID: spike.ID, LinkType: "spike_ref"}},
		},
		{
			name:         "task with no links retains nil/empty links after re-persist",
			id:           task.ID,
			wantNilLinks: true,
		},
	}

	// --- Act ---
	err = attachCommitToItems(ctx, ws, []string{feature.ID, task.ID}, commit)
	require.NoError(t, err, "attachCommitToItems must not fail")

	// --- Assert ---
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assertCommitStampedWithLinks(t, ctx, ws, tc.id, commit.SHA, tc.wantLinks, tc.wantNilLinks)
		})
	}
}

// assertCommitStampedWithLinks reads the artifact from disk and asserts the
// commit SHA was stamped and the links block matches expectations.
func assertCommitStampedWithLinks(t *testing.T, ctx context.Context, ws *Workspace, id, wantCommit string, wantLinks []models.ArtifactLink, wantNilLinks bool) {
	t.Helper()
	path, err := FindArtifactPath(ctx, ws, id)
	require.NoError(t, err)
	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	fm, body, err := models.ParseFrontmatter(string(raw))
	require.NoError(t, err)

	// Commit must be stamped.
	assert.Equal(t, wantCommit, fm["commit"], "%s: commit must be stamped after re-persist", id)

	// Links integrity: use ArtifactFromFrontmatter so the same codec that
	// reads production artifacts is exercised here.
	a, err := models.ArtifactFromFrontmatter(fm, body)
	require.NoError(t, err)

	if wantNilLinks {
		assert.Empty(t, a.Links, "%s: links must be nil/empty for no-links artifact (omitempty)", id)
		_, hasLinksKey := fm["links"]
		assert.False(t, hasLinksKey, "%s: links key must be absent in frontmatter (omitempty)", id)
	} else {
		require.NotEmpty(t, a.Links, "%s: links must be non-empty after re-persist", id)
		assert.Equal(t, wantLinks, a.Links,
			"%s: spike_ref link must survive attachCommitToItems re-persist", id)
	}
}
