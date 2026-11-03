package core

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/config"
)

// round2Receipt returns a Profile B stage-to-ship receipt at revision with the given status.
func round2Receipt(id string, revision int, status string) CustodyReceipt {
	receipt := custExecStageReceipt("digest-round2")
	receipt.ReceiptID = id
	receipt.Revision = revision
	receipt.Status = status
	receipt.OfferedAtUTC = time.Now().UTC()
	receipt.ExpiresAtUTC = time.Now().UTC().Add(time.Hour)
	if status == custodyReceiptAccepted {
		receipt.AcceptedAtUTC = time.Now().UTC()
	}
	return receipt
}

// round2ApprovalToken signs a proof bound to receipt within env. mutate changes the proof
// before signing, so each case alters only the claims it names.
func round2ApprovalToken(t *testing.T, env WorkflowCustodyEnvelope, receipt CustodyReceipt, mutate func(*CustodyApprovalProof)) (string, config.TrustAnchor) {
	t.Helper()
	priv, anchor := u3OperatorFixture(t)
	digest, err := custodyReceiptIdentityDigest(CustodyRequestIdentityDomain, env, receipt)
	require.NoError(t, err)
	now := u3Clock()
	proof := CustodyApprovalProof{
		KeyID: anchor.ID, Algorithm: "ed25519", Principal: "operator:local", AuthorizedRole: "operator",
		WorkflowID: env.WorkflowID, ShipmentID: env.ShipmentID, ScopeBranch: env.ScopeBranch,
		BaseCommitSHA: env.BaseCommitSHA, CandidateSetDigest: receipt.CandidateSetDigest,
		AllowedAction: receipt.AllowedAction, TargetRevision: receipt.Revision,
		NotBeforeUTC: now.Add(-time.Minute), NotAfterUTC: now.Add(5 * time.Minute),
		Nonce: "round2-nonce-0001", RequestIdentityDigest: digest,
		ReceiptID: receipt.ReceiptID, AcceptingRole: receipt.AcceptingRole,
	}
	mutate(&proof)
	token, err := MintCustodyApprovalTokenWithSigner(proof, priv)
	require.NoError(t, err)
	return token, anchor
}

// TestRound2_PreviewResolvedEntriesMatchOfferedEntries covers 211-S round 2 item 1: the preview
// carries one resolved entry per candidate, with the path and digests an offer would store.
func TestRound2_PreviewResolvedEntriesMatchOfferedEntries(t *testing.T) {
	ctx := context.Background()
	repo := custExecNewRepo(t)
	custExecWrite(t, repo.root, custExecCovered, custExecRevised)
	const added = "docs/plans/round2.md"
	custExecWrite(t, repo.root, added, "round two\n")
	specs := []CustodyCandidateSpec{custExecStageCandidate(), custodyReviewUntrackedCandidate(added, "stage", "stage_artifact")}

	preview, err := PreviewCustodyCandidates(ctx, repo.ws, custExecPreviewOpts(repo, specs...))
	require.NoError(t, err)

	raw, err := json.Marshal(preview)
	require.NoError(t, err)
	var wire struct {
		Resolved []CustodyEntry `json:"resolved_entries"`
	}
	require.NoError(t, json.Unmarshal(raw, &wire))
	require.Len(t, wire.Resolved, len(specs), "resolved_entries must carry one entry per candidate")
	for i, spec := range specs {
		assert.Equal(t, custodyReviewLiveEntry(t, repo, spec), wire.Resolved[i],
			"resolved entry %d must carry the digests the offer would store", i)
	}
	assert.Len(t, preview.Candidates, len(specs), "Candidates must stay the candidate specs")
}

// TestRound2_ValidateCustodyEnvelopeAcceptedCoherence covers 211-S round 2 item 2: an accepted
// receipt must match the custody revision, and only one receipt may be accepted.
func TestRound2_ValidateCustodyEnvelopeAcceptedCoherence(t *testing.T) {
	envelope := func(custodyRevision int, active string, receipts ...CustodyReceipt) WorkflowCustodyEnvelope {
		return WorkflowCustodyEnvelope{
			SchemaVersion: WorkflowCustodySchemaVersion, WorkflowID: "wf-round2", ShipmentID: "211-S",
			ScopeBranch: custExecFeatureBranch, BaseCommitSHA: "0123456789abcdef", CustodyRevision: custodyRevision,
			ActiveReceiptID: active, Receipts: receipts,
		}
	}

	t.Run("accepted_receipt_above_custody_revision_is_refused", func(t *testing.T) {
		err := ValidateCustodyEnvelope(envelope(1, "rcpt-a", round2Receipt("rcpt-a", 2, custodyReceiptAccepted)))
		assert.ErrorIs(t, err, ErrCustodyStaleRevision)
	})
	t.Run("active_accepted_receipt_below_custody_revision_is_refused", func(t *testing.T) {
		err := ValidateCustodyEnvelope(envelope(2, "rcpt-a", round2Receipt("rcpt-a", 1, custodyReceiptAccepted)))
		assert.ErrorIs(t, err, ErrCustodyStaleRevision)
	})
	t.Run("multiple_accepted_receipts_are_refused", func(t *testing.T) {
		err := ValidateCustodyEnvelope(envelope(2, "rcpt-b",
			round2Receipt("rcpt-a", 3, custodyReceiptAccepted), round2Receipt("rcpt-b", 2, custodyReceiptAccepted)))
		assert.ErrorIs(t, err, ErrCustodyStaleRevision)
	})
	t.Run("single_accepted_receipt_with_superseded_history_is_valid", func(t *testing.T) {
		require.NoError(t, ValidateCustodyEnvelope(envelope(2, "rcpt-b",
			round2Receipt("rcpt-a", 1, custodyReceiptSuperseded), round2Receipt("rcpt-b", 2, custodyReceiptAccepted))))
	})
}

// TestRound2_VerifyCustodyStateReportsActiveReceipt covers 211-S round 2 item 3: the verify
// result names the receipt in force.
func TestRound2_VerifyCustodyStateReportsActiveReceipt(t *testing.T) {
	repo := custodyReviewStageAccepted(t)
	verify, err := VerifyCustodyState(context.Background(), repo.ws, CustodyVerifyOptions{ShipmentID: repo.shipmentID, Branch: custExecFeatureBranch})
	require.NoError(t, err)
	assert.Equal(t, "rcpt-stage-ship-1", verify.ActiveReceiptID)
}

// TestRound2_ApprovalProofBindsSelectedReceiptClaims covers 211-S round 2 item 4: each claim the
// proof sets must match the selected receipt, and unset optional claims never refuse a proof.
func TestRound2_ApprovalProofBindsSelectedReceiptClaims(t *testing.T) {
	single := u3Envelope()

	t.Run("mismatched_claim_is_refused", func(t *testing.T) {
		cases := []struct {
			name   string
			mutate func(*CustodyApprovalProof)
		}{
			{"allowed_action", func(p *CustodyApprovalProof) { p.AllowedAction = "preexisting_adoption_commit" }},
			{"target_revision", func(p *CustodyApprovalProof) { p.TargetRevision = 2 }},
			{"accepting_role", func(p *CustodyApprovalProof) { p.AcceptingRole = "operator" }},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				token, anchor := round2ApprovalToken(t, single, single.Receipts[0], tc.mutate)
				_, err := VerifyCustodyApprovalToken(token, single, u3Clock(), anchor)
				assert.ErrorIs(t, err, ErrAuthorizationRejected)
			})
		}
	})

	t.Run("candidate_set_digest_of_another_receipt_is_refused", func(t *testing.T) {
		env := u3Envelope()
		other := env.Receipts[0]
		other.ReceiptID, other.Revision, other.CandidateSetDigest = "rcpt-0002", 2, "cand-2"
		env.Receipts = append(env.Receipts, other)
		token, anchor := round2ApprovalToken(t, env, env.Receipts[0], func(p *CustodyApprovalProof) {
			p.CandidateSetDigest = "cand-2"
		})
		_, err := VerifyCustodyApprovalToken(token, env, u3Clock(), anchor)
		assert.ErrorIs(t, err, ErrAuthorizationRejected)
	})

	t.Run("matching_claims_verify", func(t *testing.T) {
		token, anchor := round2ApprovalToken(t, single, single.Receipts[0], func(*CustodyApprovalProof) {})
		_, err := VerifyCustodyApprovalToken(token, single, u3Clock(), anchor)
		require.NoError(t, err)
	})

	t.Run("unset_optional_claims_still_verify", func(t *testing.T) {
		token, anchor := round2ApprovalToken(t, single, single.Receipts[0], func(p *CustodyApprovalProof) {
			p.AllowedAction, p.TargetRevision, p.ReceiptID, p.AcceptingRole = "", 0, "", ""
		})
		_, err := VerifyCustodyApprovalToken(token, single, u3Clock(), anchor)
		require.NoError(t, err)
	})
}
