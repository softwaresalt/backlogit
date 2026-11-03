package core

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/softwaresalt/backlogit/internal/config"
	bldb "github.com/softwaresalt/backlogit/internal/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// duckReceiptStatus returns the status of the receipt named id in env, or "" when absent.
func duckReceiptStatus(env WorkflowCustodyEnvelope, id string) string {
	if idx := custodyReceiptIndex(env.Receipts, id); idx >= 0 {
		return env.Receipts[idx].Status
	}
	return ""
}

// duckStageOffer returns a stage-to-ship receipt at revision rev named id with a live expiry.
func duckStageOffer(id string, rev int, digest string) CustodyReceipt {
	receipt := custExecReceipt(id, "stage_to_ship_handoff", "agent_workflow_handoff", "stage", "ship",
		"stage_artifact_commit", digest, custExecStageEntry())
	receipt.Revision = rev
	receipt.OfferedAtUTC = time.Now().UTC()
	receipt.ExpiresAtUTC = time.Now().UTC().Add(time.Hour)
	return receipt
}

// duckAcceptedReceipt returns an accepted stage-to-ship receipt for ValidateCustodyEnvelope cases.
func duckAcceptedReceipt(id string, rev int, status string) CustodyReceipt {
	receipt := duckStageOffer(id, rev, "digest-duck")
	receipt.Status = status
	receipt.AcceptedAtUTC = time.Now().UTC()
	return receipt
}

func TestRubberDuck_F1_OfferRefusesDuplicateRevision(t *testing.T) {
	ctx := context.Background()
	repo := custExecNewRepo(t)
	custExecWrite(t, repo.root, custExecCovered, custExecRevised)
	preview, err := PreviewCustodyCandidates(ctx, repo.ws, custExecPreviewOpts(repo, custExecStageCandidate()))
	require.NoError(t, err)

	_, err = OfferCustodyHandoff(ctx, repo.ws, CustodyOfferOptions{
		ShipmentID: repo.shipmentID,
		Spec:       duckStageOffer("rcpt-stage-ship-1", 1, preview.CandidateSetDigest),
	})
	require.NoError(t, err)

	_, err = OfferCustodyHandoff(ctx, repo.ws, CustodyOfferOptions{
		ShipmentID: repo.shipmentID,
		Spec:       duckStageOffer("rcpt-stage-ship-2", 1, preview.CandidateSetDigest),
	})
	require.Error(t, err, "a second offer must not reuse the pending offer's revision")
	assert.Contains(t, err.Error(), "revision")
	env := custExecEnvelope(t, custExecShipment(t, repo.ws, repo.shipmentID))
	assert.Len(t, env.Receipts, 1, "a refused offer must not be recorded")
}

func TestRubberDuck_F1_AcceptRefusesRevisionRegression(t *testing.T) {
	ctx := context.Background()
	repo := custExecNewRepo(t)
	custExecWrite(t, repo.root, custExecCovered, custExecRevised)
	preview, err := PreviewCustodyCandidates(ctx, repo.ws, custExecPreviewOpts(repo, custExecStageCandidate()))
	require.NoError(t, err)

	for rev := 1; rev <= 2; rev++ {
		id := fmt.Sprintf("rcpt-stage-ship-%d", rev)
		_, err = OfferCustodyHandoff(ctx, repo.ws, CustodyOfferOptions{
			ShipmentID: repo.shipmentID,
			Spec:       duckStageOffer(id, rev, preview.CandidateSetDigest),
		})
		require.NoError(t, err)
		_, err = AcceptCustodyHandoff(ctx, repo.ws, CustodyAcceptOptions{ShipmentID: repo.shipmentID, ReceiptID: id, By: "ship"})
		require.NoError(t, err)
	}

	// An older offer at revision 1 is injected after revision 2 is accepted and active.
	env := custExecEnvelope(t, custExecShipment(t, repo.ws, repo.shipmentID))
	require.Equal(t, 2, env.CustodyRevision)
	env.Receipts = append(env.Receipts, duckStageOffer("rcpt-stale-0", 1, preview.CandidateSetDigest))
	custodyReviewWriteEnvelope(t, repo.ws, repo.shipmentID, env)

	_, err = AcceptCustodyHandoff(ctx, repo.ws, CustodyAcceptOptions{ShipmentID: repo.shipmentID, ReceiptID: "rcpt-stale-0", By: "ship"})
	require.Error(t, err, "an older offer must not supersede a newer accepted receipt")

	after := custExecEnvelope(t, custExecShipment(t, repo.ws, repo.shipmentID))
	assert.Equal(t, 2, after.CustodyRevision, "custody revision must not regress")
	assert.Equal(t, "rcpt-stage-ship-2", after.ActiveReceiptID)
	assert.Equal(t, custodyReceiptAccepted, duckReceiptStatus(after, "rcpt-stage-ship-2"))
	assert.Equal(t, custodyReceiptOffered, duckReceiptStatus(after, "rcpt-stale-0"))
}

func TestRubberDuck_F1_ValidateCustodyEnvelopeRevisionRules(t *testing.T) {
	base := func(receipts ...CustodyReceipt) WorkflowCustodyEnvelope {
		return WorkflowCustodyEnvelope{
			SchemaVersion: WorkflowCustodySchemaVersion, WorkflowID: "wf-duck", ShipmentID: "001-S",
			ScopeBranch: "feat/duck", BaseCommitSHA: "0123456789abcdef", CustodyRevision: 2,
			ActiveReceiptID: "rcpt-active", Receipts: receipts,
		}
	}
	active := duckAcceptedReceipt("rcpt-active", 2, custodyReceiptAccepted)
	superseded := duckAcceptedReceipt("rcpt-old", 1, custodyReceiptSuperseded)

	t.Run("control_superseded_lower_revision_and_active_current", func(t *testing.T) {
		require.NoError(t, ValidateCustodyEnvelope(base(superseded, active)))
	})
	t.Run("duplicate_accepted_revision_is_refused", func(t *testing.T) {
		dup := duckAcceptedReceipt("rcpt-dup", 2, custodyReceiptAccepted)
		assert.Error(t, ValidateCustodyEnvelope(base(active, dup)))
	})
	t.Run("accepted_revision_at_or_below_custody_revision_that_is_not_current_is_refused", func(t *testing.T) {
		stale := duckAcceptedReceipt("rcpt-stale", 1, custodyReceiptAccepted)
		assert.Error(t, ValidateCustodyEnvelope(base(active, stale)))
	})
}

func TestRubberDuck_F3_RollbackRestoresPreimageWithoutDBOnlyLinks(t *testing.T) {
	ctx := context.Background()
	repo := custExecNewRepo(t)
	target, err := CreateArtifact(ctx, repo.ws, "Duck link target", "feature")
	require.NoError(t, err)
	require.NoError(t, bldb.AddLink(ctx, repo.ws.DB, repo.shipmentID, target.ID, "related_to"))

	pre := custExecShipment(t, repo.ws, repo.shipmentID)
	path, err := FindArtifactPath(ctx, repo.ws, repo.shipmentID)
	require.NoError(t, err)
	before, err := os.ReadFile(path)
	require.NoError(t, err)
	require.False(t, strings.Contains(string(before), target.ID), "precondition: the link exists only in SQLite")

	correlationID, err := custodyRandomID()
	require.NoError(t, err)
	journal := custExecJournal(repo.shipmentID, correlationID, shipmentOperationPhaseIntent, shipmentOperationCustodyOffer, custodyReceiptOffered, pre)
	journalPath, err := writeShipmentLifecycleJournalForWorkspace(repo.ws, "shipment-operation-"+correlationID+".json", journal)
	require.NoError(t, err)
	require.NoError(t, custodyWithLocks(ctx, repo.ws, repo.shipmentID, func(lockedCtx context.Context) error {
		return custodyRollback(lockedCtx, repo.ws, journal, journalPath)
	}))

	after, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.NotContains(t, string(after), target.ID, "rollback must not add a link that was absent from the journaled preimage")
}

func TestRubberDuck_F6_RevokedWorkspaceAnchorReportsRevoked(t *testing.T) {
	adoption := custodyReviewOfferAdoption(t)
	token := adoption.token(t, "duck-f6-nonce-1")
	adoption.anchor.Status = config.TrustAnchorStatusRevoked
	adoption.repo.ws.Config.TrustAnchors = []config.TrustAnchor{adoption.anchor}

	_, err := AcceptCustodyHandoff(context.Background(), adoption.repo.ws, adoption.acceptOpts(token))
	assert.ErrorIs(t, err, ErrTrustAnchorRevoked, "a revoked workspace anchor must report revocation, not an unavailable channel")
}

func TestRubberDuck_F6_WorkspaceAnchorWindowIsInclusive(t *testing.T) {
	_, anchor := u3OperatorFixture(t)
	now := time.Now().UTC()

	t.Run("not_after_equal_to_now_is_in_window", func(t *testing.T) {
		atBoundary := anchor
		atBoundary.NotAfter = &now
		anchors := custodyWorkspaceOperatorAnchors([]config.TrustAnchor{atBoundary}, now)
		require.Len(t, anchors, 1, "a workspace anchor at not_after == now must be kept")
		_, err := resolveCustodyOperatorAnchor(atBoundary.ID, now, anchors)
		require.NoError(t, err)
	})

	t.Run("expired_workspace_anchor_reports_expired", func(t *testing.T) {
		past := now.Add(-time.Minute)
		expired := anchor
		expired.NotAfter = &past
		anchors := custodyWorkspaceOperatorAnchors([]config.TrustAnchor{expired}, now)
		_, err := resolveCustodyOperatorAnchor(expired.ID, now, anchors)
		assert.ErrorIs(t, err, ErrTrustAnchorExpired)
	})
}
