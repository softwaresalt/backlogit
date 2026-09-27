package core

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	blerrors "github.com/softwaresalt/backlogit/internal/errors"
	"github.com/softwaresalt/backlogit/internal/models"
)

func TestU20C6_RefusesPendingReturnBlockedJournalForTargetAggregate(t *testing.T) {
	ctx := context.Background()

	prepareTarget := func(t *testing.T) (
		*Workspace,
		*models.Artifact,
		*models.Artifact,
		*models.Artifact,
		string,
	) {
		t.Helper()

		ws := setupShipmentWorkspace(t)
		shipment, err := CreateShipment(ctx, ws, "U20C6 target shipment", nil)
		require.NoError(t, err)
		feature, err := CreateArtifact(ctx, ws, "U20C6 target feature", "feature")
		require.NoError(t, err)
		member, err := CreateArtifact(ctx, ws, "U20C6 target member", "task", WithParent(feature.ID))
		require.NoError(t, err)
		require.NoError(t, AddItemToShipment(ctx, ws, shipment.ID, member.ID))

		originalShipment := cloneArtifact(loadURCanonicalArtifact(t, ws, shipment.ID))
		originalMember := cloneArtifact(loadURCanonicalArtifact(t, ws, member.ID))
		p021ForceOutOfBandBlockedShipment(t, ws, urBlockedFixture{
			shipment: shipment,
			members:  []*models.Artifact{member},
		})
		snapshotRef := p021WriteBlockedSnapshot(t, ws, shipment.ID)
		return ws, originalShipment, originalMember, feature, snapshotRef
	}

	readArtifactBytes := func(t *testing.T, ws *Workspace, artifactID string) []byte {
		t.Helper()

		path, err := FindArtifactPath(ctx, ws, artifactID)
		require.NoError(t, err)
		data, err := os.ReadFile(path)
		require.NoError(t, err)
		return data
	}

	snapshotJournalBytes := func(t *testing.T, ws *Workspace) map[string][]byte {
		t.Helper()

		opsRoot := shipmentOpsRoot(ws.RootPath)
		entries, err := os.ReadDir(opsRoot)
		require.NoError(t, err)
		snapshot := make(map[string][]byte, len(entries))
		for _, entry := range entries {
			require.False(t, entry.IsDir(), "unexpected directory in shipment operations root")
			data, readErr := os.ReadFile(filepath.Join(opsRoot, entry.Name()))
			require.NoError(t, readErr)
			snapshot[entry.Name()] = data
		}
		return snapshot
	}

	assertUnchanged := func(
		t *testing.T,
		ws *Workspace,
		shipmentID string,
		memberID string,
		shipmentBefore []byte,
		memberBefore []byte,
		journalsBefore map[string][]byte,
	) {
		t.Helper()

		assert.Equal(t, shipmentBefore, readArtifactBytes(t, ws, shipmentID),
			"target shipment bytes must remain unchanged")
		assert.Equal(t, memberBefore, readArtifactBytes(t, ws, memberID),
			"target member bytes must remain unchanged")
		journalsAfter := snapshotJournalBytes(t, ws)
		assert.Equal(t, len(journalsBefore), len(journalsAfter),
			"shipment operation journal count must remain unchanged")
		assert.Equal(t, journalsBefore, journalsAfter,
			"shipment operation journal names and bytes must remain unchanged")
	}

	tests := []struct {
		name        string
		shape       string
		phase       string
		matchTarget bool
	}{
		{name: "v2_intent", shape: "v2", phase: "intent"},
		{name: "v2_committed_unremoved", shape: "v2", phase: "committed"},
		{name: "legacy_schema_less", shape: "legacy"},
		{name: "malformed_member_filename", shape: "malformed_member"},
		{name: "malformed_target_only_filename", shape: "malformed_target", matchTarget: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ws, originalShipment, originalMember, feature, snapshotRef := prepareTarget(t)
			nonMember, err := CreateArtifact(ctx, ws, "U20C6 non-member", "task", WithParent(feature.ID))
			require.NoError(t, err)

			journalPath := returnBlockedJournalPath(
				ws.RootPath,
				originalShipment.ID,
				originalMember.ID,
			)
			switch tt.shape {
			case "v2":
				reason := "U20C6 pending return-blocked"
				targetShipment := cloneArtifact(originalShipment)
				if targetShipment.CustomFields == nil {
					targetShipment.CustomFields = map[string]any{}
				}
				targetShipment.CustomFields["items"] = removeString(
					NormalizeShipmentItems(originalShipment),
					originalMember.ID,
				)
				targetShipment.UpdatedAt = models.NowUTC()

				targetMember := cloneArtifact(originalMember)
				if targetMember.CustomFields == nil {
					targetMember.CustomFields = map[string]any{}
				}
				targetMember.Status = models.StatusBlocked
				targetMember.CustomFields["blocked_reason"] = reason
				targetMember.UpdatedAt = models.NowUTC()

				journal, journalErr := newReturnBlockedJournal(
					originalShipment,
					originalMember,
					targetShipment,
					targetMember,
					reason,
				)
				require.NoError(t, journalErr)
				journal.Phase = tt.phase
				require.NoError(t, writeReturnBlockedJournalRecord(ws, journal))
			case "legacy":
				require.NoError(t, writeReturnBlockedJournal(ws, originalShipment, originalMember))
			case "malformed_member":
				journalPath = returnBlockedJournalPath(
					ws.RootPath,
					originalShipment.ID,
					originalMember.ID,
				)
				kind, captures, nameErr := validateShipmentOperationJournalName(filepath.Base(journalPath))
				require.NoError(t, nameErr)
				require.Equal(t, returnBlockedJournalKind, kind)
				require.Equal(t, []string{originalShipment.ID, originalMember.ID}, captures)
				require.NoError(t, os.MkdirAll(filepath.Dir(journalPath), 0o755))
				require.NoError(t, os.WriteFile(journalPath, []byte(`{"schema_version":`), 0o600))
			case "malformed_target":
				journalPath = returnBlockedJournalPath(
					ws.RootPath,
					originalShipment.ID,
					nonMember.ID,
				)
				kind, captures, nameErr := validateShipmentOperationJournalName(filepath.Base(journalPath))
				require.NoError(t, nameErr)
				require.Equal(t, returnBlockedJournalKind, kind)
				require.Equal(t, []string{originalShipment.ID, nonMember.ID}, captures)
				require.NoError(t, os.MkdirAll(filepath.Dir(journalPath), 0o755))
				require.NoError(t, os.WriteFile(journalPath, []byte(`{"schema_version":`), 0o600))
			default:
				t.Fatalf("unknown return-blocked journal shape %q", tt.shape)
			}

			if tt.shape == "v2" || tt.shape == "legacy" {
				records, inspectErr := loadShipmentOperationJournals(ws)
				require.NoError(t, inspectErr)
				require.Len(t, records, 1)
				require.Equal(t, returnBlockedJournalKind, records[0].kind)
				require.Equal(t, originalShipment.ID, records[0].returnBlocked.Shipment.ID)
				require.Equal(t, originalMember.ID, records[0].returnBlocked.Item.ID)
				if tt.shape == "v2" {
					require.Equal(t, returnBlockedJournalSchemaVersion, records[0].returnBlocked.SchemaVersion)
					require.Equal(t, tt.phase, records[0].returnBlocked.Phase)
				} else {
					require.Empty(t, records[0].returnBlocked.SchemaVersion)
				}
			}

			journalBase := filepath.Base(journalPath)
			matchedID := originalMember.ID
			if tt.matchTarget {
				matchedID = originalShipment.ID
			}
			shipmentBefore := readArtifactBytes(t, ws, originalShipment.ID)
			memberBefore := readArtifactBytes(t, ws, originalMember.ID)
			journalsBefore := snapshotJournalBytes(t, ws)

			_, normalizeErr := NormalizeBlockedShipmentForRecovery(
				ctx,
				ws,
				originalShipment.ID,
				snapshotRef,
				"U20C6 return-blocked guard test",
			)

			assert.ErrorIs(t, normalizeErr, blerrors.ErrShipmentConflict)
			message := ""
			if normalizeErr != nil {
				message = normalizeErr.Error()
			}
			assert.Contains(t, message, journalBase)
			assert.Contains(t, message, "kind=return-blocked matched="+matchedID)
			assertUnchanged(
				t,
				ws,
				originalShipment.ID,
				originalMember.ID,
				shipmentBefore,
				memberBefore,
				journalsBefore,
			)
		})
	}
}

func TestU20C6_RefusesPendingLifecycleIntentReferencingTargetMembers(t *testing.T) {
	ctx := context.Background()

	prepareTarget := func(t *testing.T) (
		*Workspace,
		*models.Artifact,
		*models.Artifact,
		string,
	) {
		t.Helper()

		ws := setupShipmentWorkspace(t)
		shipment, err := CreateShipment(ctx, ws, "U20C6 lifecycle target shipment", nil)
		require.NoError(t, err)
		feature, err := CreateArtifact(ctx, ws, "U20C6 lifecycle target feature", "feature")
		require.NoError(t, err)
		member, err := CreateArtifact(
			ctx,
			ws,
			"U20C6 lifecycle target member",
			"task",
			WithParent(feature.ID),
		)
		require.NoError(t, err)
		require.NoError(t, AddItemToShipment(ctx, ws, shipment.ID, member.ID))

		p021ForceOutOfBandBlockedShipment(t, ws, urBlockedFixture{
			shipment: shipment,
			members:  []*models.Artifact{member},
		})
		snapshotRef := p021WriteBlockedSnapshot(t, ws, shipment.ID)
		return ws,
			cloneArtifact(loadURCanonicalArtifact(t, ws, shipment.ID)),
			cloneArtifact(loadURCanonicalArtifact(t, ws, member.ID)),
			snapshotRef
	}

	readArtifactBytes := func(t *testing.T, ws *Workspace, artifactID string) []byte {
		t.Helper()

		path, err := FindArtifactPath(ctx, ws, artifactID)
		require.NoError(t, err)
		data, err := os.ReadFile(path)
		require.NoError(t, err)
		return data
	}

	snapshotJournalBytes := func(t *testing.T, ws *Workspace) map[string][]byte {
		t.Helper()

		opsRoot := shipmentOpsRoot(ws.RootPath)
		entries, err := os.ReadDir(opsRoot)
		require.NoError(t, err)
		snapshot := make(map[string][]byte, len(entries))
		for _, entry := range entries {
			require.False(t, entry.IsDir(), "unexpected directory in shipment operations root")
			data, readErr := os.ReadFile(filepath.Join(opsRoot, entry.Name()))
			require.NoError(t, readErr)
			snapshot[entry.Name()] = data
		}
		return snapshot
	}

	assertUnchanged := func(
		t *testing.T,
		ws *Workspace,
		shipmentID string,
		memberID string,
		shipmentBefore []byte,
		memberBefore []byte,
		journalsBefore map[string][]byte,
	) {
		t.Helper()

		assert.Equal(t, shipmentBefore, readArtifactBytes(t, ws, shipmentID),
			"target shipment bytes must remain unchanged")
		assert.Equal(t, memberBefore, readArtifactBytes(t, ws, memberID),
			"target member bytes must remain unchanged")
		journalsAfter := snapshotJournalBytes(t, ws)
		assert.Equal(t, len(journalsBefore), len(journalsAfter),
			"shipment operation journal count must remain unchanged")
		assert.Equal(t, journalsBefore, journalsAfter,
			"shipment operation journal names and bytes must remain unchanged")
	}

	createOtherShipmentIntent := func(
		t *testing.T,
		ws *Workspace,
		targetMemberID string,
		includeTargetMember bool,
	) (string, shipmentLifecycleJournal) {
		t.Helper()

		feature, err := CreateArtifact(ctx, ws, "U20C6 other feature", "feature")
		require.NoError(t, err)
		member, err := CreateArtifact(ctx, ws, "U20C6 other member", "task", WithParent(feature.ID))
		require.NoError(t, err)
		otherShipment, err := CreateShipment(
			ctx,
			ws,
			"U20C6 other shipment",
			[]string{member.ID},
		)
		require.NoError(t, err)

		journal := p021LifecycleJournal(
			t,
			ws,
			"normalize",
			"roll_forward",
			otherShipment.ID,
			"snapshots/u20c6-other-shipment.json",
		)
		if includeTargetMember {
			preimageShipment := cloneArtifact(journal.Preimage.Shipment)
			memberIDs := append(NormalizeShipmentItems(preimageShipment), targetMemberID)
			if preimageShipment.CustomFields == nil {
				preimageShipment.CustomFields = map[string]any{}
			}
			preimageShipment.CustomFields["items"] = memberIDs
			journal.Preimage.Shipment = preimageShipment
			journal.Preimage.Members = append(
				journal.Preimage.Members,
				cloneArtifact(loadURCanonicalArtifact(t, ws, targetMemberID)),
			)
		}
		journalPath := p021WriteLifecycleJournal(t, ws, journal)
		records, inspectErr := loadShipmentOperationJournals(ws)
		require.NoError(t, inspectErr)
		require.Len(t, records, 1)
		require.Equal(t, shipmentLifecycleJournalKind, records[0].kind)
		require.Equal(t, journal.ShipmentID, records[0].lifecycle.ShipmentID)
		require.Equal(t, "intent", records[0].lifecycle.Phase)
		return journalPath, journal
	}

	t.Run("intent_preimage_member_matches_target_aggregate", func(t *testing.T) {
		ws, targetShipment, targetMember, snapshotRef := prepareTarget(t)
		journalPath, journal := createOtherShipmentIntent(t, ws, targetMember.ID, true)

		require.NotEqual(t, targetShipment.ID, journal.ShipmentID)
		require.NotEqual(t, targetShipment.ID, journal.Preimage.Shipment.ID)
		require.Contains(t, NormalizeShipmentItems(journal.Preimage.Shipment), targetMember.ID)
		preimageMemberIDs := make([]string, 0, len(journal.Preimage.Members))
		for _, member := range journal.Preimage.Members {
			preimageMemberIDs = append(preimageMemberIDs, member.ID)
		}
		require.Contains(t, preimageMemberIDs, targetMember.ID)

		shipmentBefore := readArtifactBytes(t, ws, targetShipment.ID)
		memberBefore := readArtifactBytes(t, ws, targetMember.ID)
		journalsBefore := snapshotJournalBytes(t, ws)
		_, normalizeErr := NormalizeBlockedShipmentForRecovery(
			ctx,
			ws,
			targetShipment.ID,
			snapshotRef,
			"U20C6 lifecycle guard test",
		)

		assert.ErrorIs(t, normalizeErr, blerrors.ErrShipmentConflict)
		message := ""
		if normalizeErr != nil {
			message = normalizeErr.Error()
		}
		assert.Contains(t, message, filepath.Base(journalPath))
		assert.Contains(t, message, "kind=lifecycle matched="+targetMember.ID)
		assertUnchanged(
			t,
			ws,
			targetShipment.ID,
			targetMember.ID,
			shipmentBefore,
			memberBefore,
			journalsBefore,
		)
	})

	t.Run("disjoint_intent_does_not_block_normalize", func(t *testing.T) {
		ws, targetShipment, targetMember, snapshotRef := prepareTarget(t)
		_, journal := createOtherShipmentIntent(t, ws, targetMember.ID, false)

		require.NotEqual(t, targetShipment.ID, journal.ShipmentID)
		require.NotEqual(t, targetShipment.ID, journal.Preimage.Shipment.ID)
		require.NotEqual(t, targetMember.ID, journal.ShipmentID)
		require.NotEqual(t, targetMember.ID, journal.Preimage.Shipment.ID)
		for _, member := range journal.Preimage.Members {
			require.NotEqual(t, targetMember.ID, member.ID)
		}

		normalized, err := NormalizeBlockedShipmentForRecovery(
			ctx,
			ws,
			targetShipment.ID,
			snapshotRef,
			"U20C6 disjoint lifecycle control",
		)
		assert.NoError(t, err, "disjoint lifecycle intent must not block normalization")
		if assert.NotNil(t, normalized) {
			assert.Equal(t, models.StatusBlocked, normalized.Status)
		}
	})
}
