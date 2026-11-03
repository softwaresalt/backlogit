package core

import (
	"context"
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/config"
	blerrors "github.com/softwaresalt/backlogit/internal/errors"
	"github.com/softwaresalt/backlogit/internal/events"
	"github.com/softwaresalt/backlogit/internal/models"
)

// custodyReviewOversizeBytes is one byte above the 64 MiB read and output cap.
const custodyReviewOversizeBytes = 64<<20 + 1

// custodyReviewProvider is an in-memory NonceConsumeProvider that records successful consumes.
type custodyReviewProvider struct {
	mu       sync.Mutex
	consumed map[string]struct{}
}

func (p *custodyReviewProvider) CompareAndConsumeNonce(_ context.Context, nonce string, _ time.Time) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.consumed == nil {
		p.consumed = map[string]struct{}{}
	}
	if _, seen := p.consumed[nonce]; seen {
		return ErrNonceReplayed
	}
	p.consumed[nonce] = struct{}{}
	return nil
}

func (p *custodyReviewProvider) consumedCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.consumed)
}

type custodyReviewAuditor struct{}

func (custodyReviewAuditor) RecordNonceConsume(NonceConsumeEvent) error { return nil }

// custodyReviewConsumer builds a live consumer over provider and the canonical workspace nonce ledger.
func custodyReviewConsumer(t *testing.T, root string, provider NonceConsumeProvider) *NonceConsumer {
	t.Helper()
	ledger, err := OpenNonceLedger(custodyReviewLedgerPath(root))
	require.NoError(t, err)
	consumer, err := NewNonceConsumer(provider, ledger, custodyReviewAuditor{})
	require.NoError(t, err)
	return consumer
}

func custodyReviewLedgerPath(root string) string {
	return filepath.Join(WorkspaceStorageRoot(root), "nonce-ledger.json")
}

// custodyReviewAdoption is a repository with an operator-edited covered file and an
// offered Profile A adoption receipt.
type custodyReviewAdoption struct {
	repo      *custExecRepo
	priv      ed25519.PrivateKey
	anchor    config.TrustAnchor
	candidate string
}

func custodyReviewOfferAdoption(t *testing.T) custodyReviewAdoption {
	t.Helper()
	ctx := context.Background()
	repo := custExecNewRepo(t)
	custExecWrite(t, repo.root, custExecCovered, custExecRevised)
	priv, anchor := u3OperatorFixture(t)
	preview, err := PreviewCustodyCandidates(ctx, repo.ws, custExecPreviewOpts(repo, custExecOperatorCandidate()))
	require.NoError(t, err)
	_, err = OfferCustodyHandoff(ctx, repo.ws, CustodyOfferOptions{
		ShipmentID: repo.shipmentID,
		Spec:       custExecAdoptionReceipt(preview.CandidateSetDigest),
	})
	require.NoError(t, err)
	return custodyReviewAdoption{repo: repo, priv: priv, anchor: anchor, candidate: preview.CandidateSetDigest}
}

// token mints an operator approval token over the shipment's current envelope with the given nonce.
func (a custodyReviewAdoption) token(t *testing.T, nonce string) string {
	t.Helper()
	env := custExecEnvelope(t, custExecShipment(t, a.repo.ws, a.repo.shipmentID))
	digest, err := CustodyRequestIdentityDigest(env)
	require.NoError(t, err)
	proof := custExecProof(a.repo, env, a.candidate, digest, a.anchor.ID)
	proof.Nonce = nonce
	token, err := MintCustodyApprovalTokenWithSigner(proof, a.priv)
	require.NoError(t, err)
	return token
}

func (a custodyReviewAdoption) acceptOpts(token string) CustodyAcceptOptions {
	return CustodyAcceptOptions{ShipmentID: a.repo.shipmentID, ReceiptID: "rcpt-adopt-1", By: "operator", AuthToken: token}
}

// custodyReviewStageAccepted offers and accepts the Profile B stage handoff for a docs file.
func custodyReviewStageAccepted(t *testing.T) *custExecRepo {
	t.Helper()
	ctx := context.Background()
	repo := custExecNewRepo(t)
	custExecWrite(t, repo.root, custExecCovered, custExecRevised)
	preview, err := PreviewCustodyCandidates(ctx, repo.ws, custExecPreviewOpts(repo, custExecStageCandidate()))
	require.NoError(t, err)
	_, err = OfferCustodyHandoff(ctx, repo.ws, CustodyOfferOptions{
		ShipmentID: repo.shipmentID,
		Spec:       custExecStageReceipt(preview.CandidateSetDigest),
	})
	require.NoError(t, err)
	_, err = AcceptCustodyHandoff(ctx, repo.ws, CustodyAcceptOptions{ShipmentID: repo.shipmentID, ReceiptID: "rcpt-stage-ship-1", By: "ship"})
	require.NoError(t, err)
	return repo
}

// custodyReviewUntrackedEntry describes a worktree-only file with no base or index blob.
func custodyReviewUntrackedEntry(path, content, contributor, source string) CustodyEntry {
	worktree := custExecDigest(content)
	return CustodyEntry{
		Path: path, ChangeKind: "added", StagedState: "unstaged_only",
		WorktreeSHA256: worktree, PatchDigestSHA256: custExecPatchDigest("", "", worktree),
		Contributor: contributor, SourceKind: source,
	}
}

func custodyReviewUntrackedCandidate(path, contributor, source string) CustodyCandidateSpec {
	return CustodyCandidateSpec{Path: path, ChangeKind: "added", StagedState: "unstaged_only", Contributor: contributor, SourceKind: source}
}

// custodyReviewEnvelope builds a workflow-custody/v1 envelope on the feature branch with the given receipts.
func custodyReviewEnvelope(repo *custExecRepo, receipts ...CustodyReceipt) WorkflowCustodyEnvelope {
	return WorkflowCustodyEnvelope{
		SchemaVersion: WorkflowCustodySchemaVersion, WorkflowID: "review-wf", ShipmentID: repo.shipmentID,
		ScopeBranch: custExecFeatureBranch, BaseCommitSHA: repo.base, Receipts: receipts,
	}
}

// custodyReviewWriteEnvelope stores env as the custody envelope of shipmentID, bypassing the governed writer.
func custodyReviewWriteEnvelope(t *testing.T, ws *Workspace, shipmentID string, env WorkflowCustodyEnvelope) {
	t.Helper()
	art := custExecShipment(t, ws, shipmentID)
	custExecSetCustody(t, art, env)
	path, err := FindArtifactPath(context.Background(), ws, shipmentID)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, []byte(models.SerializeFrontmatter(art.ToFrontmatterMap(), art.Description)), 0o644))
}

// custodyReviewEventDeltas returns the delta of every event of eventType in the shipment item log.
func custodyReviewEventDeltas(t *testing.T, root, shipmentID, eventType string) []map[string]any {
	t.Helper()
	data, err := os.ReadFile(events.LogPathForItem(WorkspaceLogsRoot(root), shipmentID))
	require.NoError(t, err)
	var deltas []map[string]any
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		var ev events.Event
		require.NoError(t, json.Unmarshal([]byte(line), &ev))
		if ev.EventType == eventType {
			deltas = append(deltas, ev.Delta)
		}
	}
	return deltas
}

// TestReview_TrustAnchorProvenance covers D1: caller-injected trust anchors and external
// pins are authoritative, and workspace-config anchors must satisfy the strict rule.
func TestReview_TrustAnchorProvenance(t *testing.T) {
	ctx := context.Background()

	t.Run("options_anchors_are_authoritative_over_workspace_config", func(t *testing.T) {
		fx := custodyReviewOfferAdoption(t)
		token := fx.token(t, "review-nonce-d1-1")
		fx.repo.ws.Config.TrustAnchors = []config.TrustAnchor{revokedAnchor(fx.anchor)}
		opts := fx.acceptOpts(token)
		opts.DryRun = true
		opts.TrustAnchors = []config.TrustAnchor{fx.anchor}
		_, err := AcceptCustodyHandoff(ctx, fx.repo.ws, opts)
		require.NoError(t, err, "caller-supplied anchors must verify the token even when workspace config holds a revoked anchor with the same id")
	})

	t.Run("options_anchors_replace_workspace_anchors", func(t *testing.T) {
		fx := custodyReviewOfferAdoption(t)
		token := fx.token(t, "review-nonce-d1-2")
		fx.repo.ws.Config.TrustAnchors = []config.TrustAnchor{fx.anchor}
		other := fx.anchor
		other.ID = "op-2"
		opts := fx.acceptOpts(token)
		opts.DryRun = true
		opts.TrustAnchors = []config.TrustAnchor{other}
		_, err := AcceptCustodyHandoff(ctx, fx.repo.ws, opts)
		assert.ErrorIs(t, err, ErrApprovalChannelUnavailable, "supplied anchors must replace, not extend, workspace anchors")
	})

	t.Run("external_pins_bind_the_supplied_anchor", func(t *testing.T) {
		fx := custodyReviewOfferAdoption(t)
		token := fx.token(t, "review-nonce-d1-3")
		anchor := fx.anchor
		anchor.Fingerprint = "fp-op-1"
		opts := fx.acceptOpts(token)
		opts.DryRun = true
		opts.TrustAnchors = []config.TrustAnchor{anchor}
		opts.ExternalPins = NewBoundExternalPinSet(PinBinding{Fingerprint: "fp-op-1", Role: "operator"})
		_, err := AcceptCustodyHandoff(ctx, fx.repo.ws, opts)
		require.NoError(t, err, "an externally pinned, bound anchor must verify the token")

		opts.ExternalPins = NewExternalPinSet("fp-other")
		_, err = AcceptCustodyHandoff(ctx, fx.repo.ws, opts)
		require.Error(t, err, "an anchor whose fingerprint is not externally pinned must fail closed")
	})

	t.Run("workspace_only_anchor_must_be_in_window_with_operator_role", func(t *testing.T) {
		fx := custodyReviewOfferAdoption(t)
		token := fx.token(t, "review-nonce-d1-4")
		past := time.Now().UTC().Add(-time.Hour)
		expired := fx.anchor
		expired.NotAfter = &past
		fx.repo.ws.Config.TrustAnchors = []config.TrustAnchor{expired}
		opts := fx.acceptOpts(token)
		opts.DryRun = true
		_, err := AcceptCustodyHandoff(ctx, fx.repo.ws, opts)
		require.Error(t, err, "an expired workspace anchor must fail closed")

		stage := fx.anchor
		stage.Role = "stage"
		fx.repo.ws.Config.TrustAnchors = []config.TrustAnchor{stage}
		_, err = AcceptCustodyHandoff(ctx, fx.repo.ws, opts)
		require.Error(t, err, "a non-operator workspace anchor must fail closed")
	})
}

// TestReview_ProfileEligibility covers D2: deterministic Profile A / Profile B eligibility.
func TestReview_ProfileEligibility(t *testing.T) {
	ctx := context.Background()

	t.Run("preview_requires_operator_profile_outside_docs_and_backlogit", func(t *testing.T) {
		repo := custExecNewRepo(t)
		custExecWrite(t, repo.root, "internal/core/x.go", "package core\n")
		preview, err := PreviewCustodyCandidates(ctx, repo.ws, custExecPreviewOpts(repo,
			custodyReviewUntrackedCandidate("internal/core/x.go", "stage", "stage_artifact")))
		require.NoError(t, err)
		assert.Equal(t, "operator_verified", preview.RequiredAuthorityProfile)
	})

	for _, path := range []string{"internal/core/x.go", ".autoharness/x.yaml", ".gsd/carry.yaml"} {
		t.Run("stage_handoff_refused_for_"+strings.ReplaceAll(path, "/", "_"), func(t *testing.T) {
			repo := custExecNewRepo(t)
			custExecWrite(t, repo.root, path, "content\n")
			preview, err := PreviewCustodyCandidates(ctx, repo.ws, custExecPreviewOpts(repo,
				custodyReviewUntrackedCandidate(path, "stage", "stage_artifact")))
			require.NoError(t, err)
			receipt := custExecReceipt("rcpt-outside-1", "stage_to_ship_handoff", "agent_workflow_handoff", "stage", "ship",
				"stage_artifact_commit", preview.CandidateSetDigest, custodyReviewUntrackedEntry(path, "content\n", "stage", "stage_artifact"))
			_, err = OfferCustodyHandoff(ctx, repo.ws, CustodyOfferOptions{ShipmentID: repo.shipmentID, Spec: receipt})
			assert.ErrorIs(t, err, ErrOperatorApprovalRequired, "profile B must not cover %s", path)
		})
	}

	t.Run("docs_handoff_stays_profile_b", func(t *testing.T) {
		repo := custExecNewRepo(t)
		custExecWrite(t, repo.root, "docs/review/x.md", "content\n")
		preview, err := PreviewCustodyCandidates(ctx, repo.ws, custExecPreviewOpts(repo,
			custodyReviewUntrackedCandidate("docs/review/x.md", "stage", "stage_artifact")))
		require.NoError(t, err)
		assert.Equal(t, "agent_workflow_handoff", preview.RequiredAuthorityProfile)
		receipt := custExecReceipt("rcpt-docs-1", "stage_to_ship_handoff", "agent_workflow_handoff", "stage", "ship",
			"stage_artifact_commit", preview.CandidateSetDigest, custodyReviewUntrackedEntry("docs/review/x.md", "content\n", "stage", "stage_artifact"))
		_, err = OfferCustodyHandoff(ctx, repo.ws, CustodyOfferOptions{ShipmentID: repo.shipmentID, Spec: receipt})
		require.NoError(t, err)
	})

	t.Run("operator_producer_and_accepting_role_requires_profile_a", func(t *testing.T) {
		repo := custExecNewRepo(t)
		custExecWrite(t, repo.root, "docs/review/x.md", "content\n")
		preview, err := PreviewCustodyCandidates(ctx, repo.ws, custExecPreviewOpts(repo,
			custodyReviewUntrackedCandidate("docs/review/x.md", "stage", "stage_artifact")))
		require.NoError(t, err)
		receipt := custExecReceipt("rcpt-op-role-1", "ship_task_custody", "agent_workflow_handoff", "operator", "operator",
			"ship_task_commit", preview.CandidateSetDigest, custodyReviewUntrackedEntry("docs/review/x.md", "content\n", "stage", "stage_artifact"))
		_, err = OfferCustodyHandoff(ctx, repo.ws, CustodyOfferOptions{ShipmentID: repo.shipmentID, Spec: receipt})
		assert.ErrorIs(t, err, ErrOperatorApprovalRequired)
	})

	t.Run("accept_refuses_profile_b_outside_scope", func(t *testing.T) {
		repo := custExecNewRepo(t)
		path := "internal/core/x.go"
		custExecWrite(t, repo.root, path, "content\n")
		preview, err := PreviewCustodyCandidates(ctx, repo.ws, custExecPreviewOpts(repo,
			custodyReviewUntrackedCandidate(path, "stage", "stage_artifact")))
		require.NoError(t, err)
		receipt := custExecReceipt("rcpt-outside-accept", "stage_to_ship_handoff", "agent_workflow_handoff", "stage", "ship",
			"stage_artifact_commit", preview.CandidateSetDigest, custodyReviewUntrackedEntry(path, "content\n", "stage", "stage_artifact"))
		receipt.ExpiresAtUTC = time.Now().UTC().Add(time.Hour)
		custodyReviewWriteEnvelope(t, repo.ws, repo.shipmentID, custodyReviewEnvelope(repo, receipt))
		_, err = AcceptCustodyHandoff(ctx, repo.ws, CustodyAcceptOptions{ShipmentID: repo.shipmentID, ReceiptID: "rcpt-outside-accept", By: "ship"})
		assert.ErrorIs(t, err, ErrOperatorApprovalRequired, "accept must apply the same eligibility as offer")
	})
}

// TestReview_RedemptionBinding covers D3: an operator receipt verifies only with a
// matching redemption in the NonceLedger or the shipment item log.
func TestReview_RedemptionBinding(t *testing.T) {
	ctx := context.Background()

	t.Run("status_flip_without_redemption_is_unverified", func(t *testing.T) {
		fx := custodyReviewOfferAdoption(t)
		fx.repo.ws.Config.TrustAnchors = []config.TrustAnchor{fx.anchor}
		env := custExecEnvelope(t, custExecShipment(t, fx.repo.ws, fx.repo.shipmentID))
		proof, err := VerifyCustodyApprovalToken(fx.token(t, "review-flip-nonce"), env, time.Now().UTC(), fx.anchor)
		require.NoError(t, err)
		env.ActiveReceiptID = "rcpt-adopt-1"
		env.Receipts[0].Status = "accepted"
		env.Receipts[0].AcceptedAtUTC = time.Now().UTC()
		env.Receipts[0].ExpiresAtUTC = time.Now().UTC().Add(time.Hour)
		env.Receipts[0].ApprovalProof = &proof
		custExecWriteCustody(t, fx.repo.ws, env)

		verify, err := VerifyCustodyState(ctx, fx.repo.ws, CustodyVerifyOptions{ShipmentID: fx.repo.shipmentID, Branch: custExecFeatureBranch})
		require.NoError(t, err)
		assert.Equal(t, "UNVERIFIED_OPERATOR_PROVENANCE", custExecVerdict(verify, custExecCovered),
			"flipping status to accepted without a matching redemption must not verify")
	})

	t.Run("live_accept_records_redemption_in_ledger_and_log", func(t *testing.T) {
		fx := custodyReviewOfferAdoption(t)
		fx.repo.ws.Config.TrustAnchors = []config.TrustAnchor{fx.anchor}
		provider := &custodyReviewProvider{}
		live := WithCustodyNonceConsumer(ctx, custodyReviewConsumer(t, fx.repo.root, provider))
		_, err := AcceptCustodyHandoff(live, fx.repo.ws, fx.acceptOpts(fx.token(t, "review-live-nonce")))
		require.NoError(t, err)

		env := custExecEnvelope(t, custExecShipment(t, fx.repo.ws, fx.repo.shipmentID))
		digest, err := CustodyRequestIdentityDigest(env)
		require.NoError(t, err)
		ledger, err := OpenNonceLedger(custodyReviewLedgerPath(fx.repo.root))
		require.NoError(t, err)
		bound, found, err := ledger.BoundDigest("review-live-nonce")
		require.NoError(t, err)
		require.True(t, found, "a live operator accept must record its nonce in the NonceLedger")
		assert.Equal(t, digest, bound)

		deltas := custodyReviewEventDeltas(t, fx.repo.root, fx.repo.shipmentID, EventCustodyHandoffAccepted)
		require.Len(t, deltas, 1)
		delta := deltas[0]
		assert.Equal(t, "rcpt-adopt-1", delta["receipt_id"])
		assert.Equal(t, "operator_verified", delta["authority_profile"])
		assert.Equal(t, "operator", delta["accepting_role"])
		assert.Equal(t, fx.anchor.ID, delta["key_id"])
		assert.Equal(t, "review-live-nonce", delta["nonce"])
		assert.Equal(t, digest, delta["request_identity_digest"])
		assert.EqualValues(t, 1, delta["revision"])
		assert.NotEmpty(t, delta["accepted_at_utc"])

		verify, err := VerifyCustodyState(ctx, fx.repo.ws, CustodyVerifyOptions{ShipmentID: fx.repo.shipmentID, Branch: custExecFeatureBranch})
		require.NoError(t, err)
		assert.Equal(t, "ADOPTED_OPERATOR_VERIFIED", custExecVerdict(verify, custExecCovered))
	})

	t.Run("missing_ledger_and_log_leaves_operator_unverified", func(t *testing.T) {
		fx := custodyReviewOfferAdoption(t)
		fx.repo.ws.Config.TrustAnchors = []config.TrustAnchor{fx.anchor}
		live := WithCustodyNonceConsumer(ctx, custodyReviewConsumer(t, fx.repo.root, &custodyReviewProvider{}))
		_, err := AcceptCustodyHandoff(live, fx.repo.ws, fx.acceptOpts(fx.token(t, "review-missing-nonce")))
		require.NoError(t, err)
		require.NoError(t, os.Remove(custodyReviewLedgerPath(fx.repo.root)))
		require.NoError(t, os.Remove(events.LogPathForItem(WorkspaceLogsRoot(fx.repo.root), fx.repo.shipmentID)))

		verify, err := VerifyCustodyState(ctx, fx.repo.ws, CustodyVerifyOptions{ShipmentID: fx.repo.shipmentID, Branch: custExecFeatureBranch})
		require.NoError(t, err)
		assert.Equal(t, "UNVERIFIED_OPERATOR_PROVENANCE", custExecVerdict(verify, custExecCovered),
			"a missing ledger and log must count as no redemption, so the receipt fails closed")
	})

	t.Run("log_redemption_survives_ledger_loss", func(t *testing.T) {
		fx := custodyReviewOfferAdoption(t)
		fx.repo.ws.Config.TrustAnchors = []config.TrustAnchor{fx.anchor}
		live := WithCustodyNonceConsumer(ctx, custodyReviewConsumer(t, fx.repo.root, &custodyReviewProvider{}))
		_, err := AcceptCustodyHandoff(live, fx.repo.ws, fx.acceptOpts(fx.token(t, "review-log-nonce")))
		require.NoError(t, err)
		require.NoError(t, os.Remove(custodyReviewLedgerPath(fx.repo.root)))

		verify, err := VerifyCustodyState(ctx, fx.repo.ws, CustodyVerifyOptions{ShipmentID: fx.repo.shipmentID, Branch: custExecFeatureBranch})
		require.NoError(t, err)
		assert.Equal(t, "ADOPTED_OPERATOR_VERIFIED", custExecVerdict(verify, custExecCovered))
	})
}

// TestReview_ReceiptExpiry covers mechanical finding 1: receipt expiry is enforced.
func TestReview_ReceiptExpiry(t *testing.T) {
	ctx := context.Background()

	t.Run("verify_reports_expired_accepted_receipt", func(t *testing.T) {
		repo := custodyReviewStageAccepted(t)
		env := custExecEnvelope(t, custExecShipment(t, repo.ws, repo.shipmentID))
		env.Receipts[0].ExpiresAtUTC = time.Now().UTC().Add(-time.Hour)
		custExecWriteCustody(t, repo.ws, env)

		verify, err := VerifyCustodyState(ctx, repo.ws, CustodyVerifyOptions{ShipmentID: repo.shipmentID, Branch: custExecFeatureBranch})
		require.NoError(t, err)
		assert.Equal(t, "RECEIPT_EXPIRED", custExecVerdict(verify, custExecCovered))
	})

	t.Run("verify_fails_closed_without_accepted_expiry", func(t *testing.T) {
		repo := custodyReviewStageAccepted(t)
		env := custExecEnvelope(t, custExecShipment(t, repo.ws, repo.shipmentID))
		env.Receipts[0].ExpiresAtUTC = time.Time{}
		custExecWriteCustody(t, repo.ws, env)

		verify, err := VerifyCustodyState(ctx, repo.ws, CustodyVerifyOptions{ShipmentID: repo.shipmentID, Branch: custExecFeatureBranch})
		require.NoError(t, err)
		assert.Equal(t, "RECEIPT_EXPIRED", custExecVerdict(verify, custExecCovered))
	})

	t.Run("accept_refuses_offer_past_expiry", func(t *testing.T) {
		repo := custExecNewRepo(t)
		custExecWrite(t, repo.root, custExecCovered, custExecRevised)
		preview, err := PreviewCustodyCandidates(ctx, repo.ws, custExecPreviewOpts(repo, custExecStageCandidate()))
		require.NoError(t, err)
		_, err = OfferCustodyHandoff(ctx, repo.ws, CustodyOfferOptions{
			ShipmentID: repo.shipmentID,
			Spec:       custExecStageReceipt(preview.CandidateSetDigest),
		})
		require.NoError(t, err)
		env := custExecEnvelope(t, custExecShipment(t, repo.ws, repo.shipmentID))
		env.Receipts[0].ExpiresAtUTC = time.Now().UTC().Add(-time.Minute)
		custExecWriteCustody(t, repo.ws, env)

		_, err = AcceptCustodyHandoff(ctx, repo.ws, CustodyAcceptOptions{ShipmentID: repo.shipmentID, ReceiptID: "rcpt-stage-ship-1", By: "ship"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "expired")
	})
}

// TestReview_DefaultBranchOriginPrefix covers mechanical finding 2: origin/ is stripped
// from the resolved default branch before the default-branch refusal.
func TestReview_DefaultBranchOriginPrefix(t *testing.T) {
	ctx := context.Background()
	repo := custExecNewRepo(t)
	custExecWrite(t, repo.root, custExecCovered, custExecRevised)
	preview, err := PreviewCustodyCandidates(ctx, repo.ws, custExecPreviewOpts(repo, custExecStageCandidate()))
	require.NoError(t, err)

	runGit(t, repo.root, "update-ref", "refs/remotes/origin/develop", "HEAD")
	runGit(t, repo.root, "symbolic-ref", "refs/remotes/origin/HEAD", "refs/remotes/origin/develop")
	runGit(t, repo.root, "checkout", "-b", "develop")

	_, err = PreviewCustodyCandidates(ctx, repo.ws, CustodyPreviewOptions{
		ShipmentID: repo.shipmentID, Branch: "develop", Candidates: []CustodyCandidateSpec{custExecStageCandidate()},
	})
	assert.ErrorIs(t, err, ErrCustodyDefaultBranchForbidden, "a default branch reported as origin/develop must refuse develop")

	_, err = OfferCustodyHandoff(ctx, repo.ws, CustodyOfferOptions{
		ShipmentID: repo.shipmentID,
		Spec:       custExecStageReceipt(preview.CandidateSetDigest),
	})
	assert.ErrorIs(t, err, ErrCustodyDefaultBranchForbidden, "offer on the default branch must fail closed")
}

// TestReview_NonceConsumedOnlyAfterValidation covers mechanical finding 3: a nonce is
// consumed only after every check that can fail has passed.
func TestReview_NonceConsumedOnlyAfterValidation(t *testing.T) {
	ctx := context.Background()
	fx := custodyReviewOfferAdoption(t)
	fx.repo.ws.Config.TrustAnchors = []config.TrustAnchor{fx.anchor}

	env := custExecEnvelope(t, custExecShipment(t, fx.repo.ws, fx.repo.shipmentID))
	corrupt := env.Receipts[0]
	corrupt.ReceiptID = "rcpt-corrupt-0"
	corrupt.Revision = 0
	corrupt.Status = "superseded"
	env.Receipts = append([]CustodyReceipt{corrupt}, env.Receipts...)
	custExecWriteCustody(t, fx.repo.ws, env)

	token := fx.token(t, "review-validate-nonce")
	provider := &custodyReviewProvider{}
	live := WithCustodyNonceConsumer(ctx, custodyReviewConsumer(t, fx.repo.root, provider))
	_, err := AcceptCustodyHandoff(live, fx.repo.ws, fx.acceptOpts(token))
	require.Error(t, err, "a post-state envelope that fails validation must refuse the accept")
	assert.Zero(t, provider.consumedCount(), "the operator nonce must not be consumed before validation completes")
}

// TestReview_IdempotencyKey covers mechanical finding 4: a retried accept is compared
// against the stored request digest.
func TestReview_IdempotencyKey(t *testing.T) {
	ctx := context.Background()

	t.Run("retry_with_same_key_and_token_returns_original_result", func(t *testing.T) {
		fx := custodyReviewOfferAdoption(t)
		fx.repo.ws.Config.TrustAnchors = []config.TrustAnchor{fx.anchor}
		provider := &custodyReviewProvider{}
		live := WithCustodyNonceConsumer(ctx, custodyReviewConsumer(t, fx.repo.root, provider))
		token := fx.token(t, "review-idem-1")
		opts := fx.acceptOpts(token)
		opts.IdempotencyKey = "idem-1"
		_, err := AcceptCustodyHandoff(live, fx.repo.ws, opts)
		require.NoError(t, err)

		retry, err := AcceptCustodyHandoff(live, fx.repo.ws, opts)
		require.NoError(t, err, "a retried accept with the same key and request must succeed")
		assert.Equal(t, "accepted", retry.Status)
		assert.Equal(t, 1, provider.consumedCount(), "a retry must not consume the nonce again")
	})

	t.Run("same_key_with_different_request_fails_closed", func(t *testing.T) {
		fx := custodyReviewOfferAdoption(t)
		fx.repo.ws.Config.TrustAnchors = []config.TrustAnchor{fx.anchor}
		live := WithCustodyNonceConsumer(ctx, custodyReviewConsumer(t, fx.repo.root, &custodyReviewProvider{}))
		opts := fx.acceptOpts(fx.token(t, "review-idem-2"))
		opts.IdempotencyKey = "idem-2"
		_, err := AcceptCustodyHandoff(live, fx.repo.ws, opts)
		require.NoError(t, err)

		opts.AuthToken = fx.token(t, "review-idem-3")
		_, err = AcceptCustodyHandoff(live, fx.repo.ws, opts)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "idempotency", "reusing a key with a different request must be a conflict")
	})
}

// TestReview_CompensatedJournalIsTerminal covers mechanical finding 5: a failed journal
// removal after rollback leaves a terminal compensated record, not a replayable intent.
func TestReview_CompensatedJournalIsTerminal(t *testing.T) {
	ctx := context.Background()
	repo := custExecNewRepo(t)
	custExecWrite(t, repo.root, custExecCovered, custExecRevised)
	preview, err := PreviewCustodyCandidates(ctx, repo.ws, custExecPreviewOpts(repo, custExecStageCandidate()))
	require.NoError(t, err)
	pre := custExecShipment(t, repo.ws, repo.shipmentID)
	_, err = OfferCustodyHandoff(ctx, repo.ws, CustodyOfferOptions{
		ShipmentID: repo.shipmentID,
		Spec:       custExecStageReceipt(preview.CandidateSetDigest),
	})
	require.NoError(t, err)

	journal := custExecJournal(repo.shipmentID, custExecCorrelationA, "intent", "custody_offer", "offered", pre)
	journalPath, err := writeShipmentLifecycleJournalForWorkspace(repo.ws, "shipment-operation-"+custExecCorrelationA+".json", journal)
	require.NoError(t, err)

	repo.ws.removeShipmentOperationJournal = func(string) error { return errors.New("injected journal removal failure") }
	require.NoError(t, recoverPendingShipmentOperations(ctx, repo.ws))

	raw, err := os.ReadFile(journalPath)
	require.NoError(t, err)
	var stored shipmentLifecycleJournal
	require.NoError(t, json.Unmarshal(raw, &stored))
	assert.Equal(t, "compensated", stored.Phase, "a rolled-back intent whose removal failed must become terminal")
	_, present := custExecShipment(t, repo.ws, repo.shipmentID).CustomFields["workflow_custody"]
	assert.False(t, present, "rollback must restore the preimage")

	repo.ws.removeShipmentOperationJournal = nil
	require.NoError(t, recoverPendingShipmentOperations(ctx, repo.ws))
	_, present = custExecShipment(t, repo.ws, repo.shipmentID).CustomFields["workflow_custody"]
	assert.False(t, present, "a later recovery barrier must not replay the stale intent")
}

// TestReview_RecoveryLockOrderGuard covers mechanical finding 6: recovery reconciles
// custody only under the shipment lifecycle lock.
func TestReview_RecoveryLockOrderGuard(t *testing.T) {
	repo := custExecNewRepo(t)
	journal := custExecJournal(repo.shipmentID, custExecCorrelationB, "intent", "custody_offer", "offered",
		custExecShipment(t, repo.ws, repo.shipmentID))
	_, err := reconcileCustodyIntent(context.Background(), repo.ws, "unused.json", journal)
	require.ErrorIs(t, err, blerrors.ErrValidation, "reconciliation must require the shipment lifecycle lock first")
}

// TestReview_LockReleasedOnPanic covers mechanical finding 7: a panicking custody callback
// still releases the shipment item-log lock.
func TestReview_LockReleasedOnPanic(t *testing.T) {
	ctx := context.Background()
	repo := custExecNewRepo(t)
	func() {
		defer func() { _ = recover() }()
		_ = custodyWithLocks(ctx, repo.ws, repo.shipmentID, func(context.Context) error {
			panic("injected callback panic")
		})
	}()

	acquired := make(chan struct{})
	go func() {
		_, unlock, err := events.LockItemLogCrossProcess(ctx, WorkspaceLocksRoot(repo.root), WorkspaceLogsRoot(repo.root), repo.shipmentID)
		if err == nil {
			unlock()
		}
		close(acquired)
	}()
	select {
	case <-acquired:
	case <-time.After(10 * time.Second):
		t.Fatal("the shipment item-log lock is still held after a panicking custody callback")
	}
}

// TestReview_ShipmentIDBinding covers mechanical finding 8: the envelope must name the
// shipment it is stored on, on accept and on verify.
func TestReview_ShipmentIDBinding(t *testing.T) {
	ctx := context.Background()
	repo := custExecNewRepo(t)
	custExecWrite(t, repo.root, custExecCovered, custExecRevised)
	preview, err := PreviewCustodyCandidates(ctx, repo.ws, custExecPreviewOpts(repo, custExecStageCandidate()))
	require.NoError(t, err)
	_, err = OfferCustodyHandoff(ctx, repo.ws, CustodyOfferOptions{
		ShipmentID: repo.shipmentID,
		Spec:       custExecStageReceipt(preview.CandidateSetDigest),
	})
	require.NoError(t, err)

	env := custExecEnvelope(t, custExecShipment(t, repo.ws, repo.shipmentID))
	env.ShipmentID = "999-S"
	custodyReviewWriteEnvelope(t, repo.ws, repo.shipmentID, env)

	_, err = AcceptCustodyHandoff(ctx, repo.ws, CustodyAcceptOptions{ShipmentID: repo.shipmentID, ReceiptID: "rcpt-stage-ship-1", By: "ship"})
	assert.Error(t, err, "accept must refuse an envelope that names another shipment")

	_, err = VerifyCustodyState(ctx, repo.ws, CustodyVerifyOptions{ShipmentID: repo.shipmentID, Branch: custExecFeatureBranch})
	assert.Error(t, err, "verify must refuse an envelope that names another shipment")
}

// TestReview_BranchMustBeCheckedOut covers mechanical finding 9: preview and verify use
// the checked-out branch and refuse a requested branch that is not checked out.
func TestReview_BranchMustBeCheckedOut(t *testing.T) {
	ctx := context.Background()

	t.Run("preview_refuses_requested_branch_not_checked_out", func(t *testing.T) {
		repo := custExecNewRepo(t)
		custExecWrite(t, repo.root, custExecCovered, custExecRevised)
		runGit(t, repo.root, "checkout", "-b", "review-other")
		_, err := PreviewCustodyCandidates(ctx, repo.ws, custExecPreviewOpts(repo, custExecStageCandidate()))
		assert.ErrorIs(t, err, ErrCustodyWrongBranch)
	})

	t.Run("verify_reports_wrong_branch_when_requested_branch_not_checked_out", func(t *testing.T) {
		repo := custodyReviewStageAccepted(t)
		runGit(t, repo.root, "checkout", "-b", "review-other")
		verify, err := VerifyCustodyState(ctx, repo.ws, CustodyVerifyOptions{ShipmentID: repo.shipmentID, Branch: custExecFeatureBranch})
		require.NoError(t, err)
		assert.Equal(t, "WRONG_BRANCH", custExecVerdict(verify, custExecCovered))
	})
}

// TestReview_BoundedReads covers mechanical finding 10: worktree reads and git output
// fail closed above the 64 MiB cap.
func TestReview_BoundedReads(t *testing.T) {
	t.Run("worktree_read_fails_closed_above_cap", func(t *testing.T) {
		repo := custExecNewRepo(t)
		path := filepath.Join(repo.root, "docs", "review", "oversize.md")
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, nil, 0o644))
		require.NoError(t, os.Truncate(path, custodyReviewOversizeBytes))

		_, err := readCustodyWorktreeFileNoFollow(repo.ws, "docs/review/oversize.md")
		require.Error(t, err)
	})

	t.Run("git_output_fails_closed_above_cap", func(t *testing.T) {
		repo := custExecNewRepo(t)
		path := filepath.Join(repo.root, "oversize.bin")
		require.NoError(t, os.WriteFile(path, nil, 0o644))
		require.NoError(t, os.Truncate(path, custodyReviewOversizeBytes))
		oid := strings.TrimSpace(runGit(t, repo.root, "hash-object", "-w", "oversize.bin"))

		_, err := custodyBlobDigest(context.Background(), repo.root, oid)
		require.Error(t, err)
	})
}
