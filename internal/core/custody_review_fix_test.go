package core

import (
	"context"
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/config"
	bldb "github.com/softwaresalt/backlogit/internal/db"
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

// custodyReviewLedgerRecord returns the NonceLedger record for nonce as stored on disk.
func custodyReviewLedgerRecord(t *testing.T, root, nonce string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(custodyReviewLedgerPath(root))
	require.NoError(t, err)
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		var record map[string]any
		require.NoError(t, json.Unmarshal([]byte(line), &record))
		if record["nonce"] == nonce {
			return record
		}
	}
	require.Failf(t, "nonce is not in the ledger", "%s", nonce)
	return nil
}

// custodyReviewSetBranchField rewrites the shipment's top-level branch custom field,
// simulating a blocked-envelope recovery that overwrote it.
func custodyReviewSetBranchField(t *testing.T, ws *Workspace, shipmentID, branch string) {
	t.Helper()
	art := custExecShipment(t, ws, shipmentID)
	art.CustomFields["branch"] = branch
	path, err := FindArtifactPath(context.Background(), ws, shipmentID)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, []byte(models.SerializeFrontmatter(art.ToFrontmatterMap(), art.Description)), 0o644))
}

// TestReview_WindowsPathAliases covers lexical rejection of Windows path aliases, so the
// result is the same on every OS: trailing dot or space, 8.3 short names, alternate data
// streams, reserved device names, and pathspec magic.
func TestReview_WindowsPathAliases(t *testing.T) {
	ws := &Workspace{RootPath: t.TempDir()}
	for _, path := range []string{
		".git./config", "GIT~1/HEAD", "a.md:stream", ".git ", "docs/a.md.", "docs/a.md ",
		"docs/NUL.md", "CON/x.md", ":(glob)docs/a.md",
	} {
		t.Run(strings.ReplaceAll(path, "/", "_"), func(t *testing.T) {
			assert.ErrorIs(t, ValidateCustodyEntryPath(ws, path), ErrCustodyPathRejected)
		})
	}
}

// TestReview_AncestorNoFollow covers a symlink at a non-final ancestor component.
func TestReview_AncestorNoFollow(t *testing.T) {
	ws := u3Workspace(t)
	u3Symlink(t, t.TempDir(), filepath.Join(ws.RootPath, "docs", "linked"))
	assert.ErrorIs(t, ValidateCustodyEntryPath(ws, "docs/linked/deep/x.md"), ErrCustodyPathRejected)
}

// TestReview_BranchFieldFollowsScopeBranch covers R3-4: the top-level branch field must
// agree with the envelope's scope_branch, which is authoritative.
func TestReview_BranchFieldFollowsScopeBranch(t *testing.T) {
	ctx := context.Background()
	repo := custodyReviewStageAccepted(t)
	custodyReviewSetBranchField(t, repo.ws, repo.shipmentID, "feat/overwritten-by-recover")

	_, err := VerifyCustodyState(ctx, repo.ws, CustodyVerifyOptions{ShipmentID: repo.shipmentID, Branch: custExecFeatureBranch})
	assert.ErrorIs(t, err, ErrCustodyWrongBranch, "verify must fail closed when branch disagrees with scope_branch")
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

	t.Run("preexisting_adoption_action_requires_profile_a", func(t *testing.T) {
		repo := custExecNewRepo(t)
		custExecWrite(t, repo.root, "docs/review/x.md", "content\n")
		preview, err := PreviewCustodyCandidates(ctx, repo.ws, custExecPreviewOpts(repo,
			custodyReviewUntrackedCandidate("docs/review/x.md", "stage", "stage_artifact")))
		require.NoError(t, err)
		receipt := custExecReceipt("rcpt-adopt-b-1", "stage_to_ship_handoff", "agent_workflow_handoff", "stage", "ship",
			"preexisting_adoption_commit", preview.CandidateSetDigest, custodyReviewUntrackedEntry("docs/review/x.md", "content\n", "stage", "stage_artifact"))
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

	t.Run("live_accept_records_consumed_at_in_ledger_and_adoption_event", func(t *testing.T) {
		fx := custodyReviewOfferAdoption(t)
		fx.repo.ws.Config.TrustAnchors = []config.TrustAnchor{fx.anchor}
		live := WithCustodyNonceConsumer(ctx, custodyReviewConsumer(t, fx.repo.root, &custodyReviewProvider{}))
		_, err := AcceptCustodyHandoff(live, fx.repo.ws, fx.acceptOpts(fx.token(t, "review-consumed-nonce")))
		require.NoError(t, err)

		record := custodyReviewLedgerRecord(t, fx.repo.root, "review-consumed-nonce")
		assert.NotEmpty(t, record["consumed_at"], "a consumed nonce must record consumed_at in the NonceLedger")

		deltas := custodyReviewEventDeltas(t, fx.repo.root, fx.repo.shipmentID, EventCustodyAdoptionRecorded)
		require.Len(t, deltas, 1)
		assert.NotEmpty(t, deltas[0]["consumed_at"], "the adoption event must carry consumed_at")
	})

	t.Run("legacy_ledger_entry_without_consumed_at_is_unverified", func(t *testing.T) {
		fx := custodyReviewOfferAdoption(t)
		fx.repo.ws.Config.TrustAnchors = []config.TrustAnchor{fx.anchor}
		env := custExecEnvelope(t, custExecShipment(t, fx.repo.ws, fx.repo.shipmentID))
		proof, err := VerifyCustodyApprovalToken(fx.token(t, "review-legacy-nonce"), env, time.Now().UTC(), fx.anchor)
		require.NoError(t, err)
		env.ActiveReceiptID = "rcpt-adopt-1"
		env.Receipts[0].Status = "accepted"
		env.Receipts[0].AcceptedAtUTC = time.Now().UTC()
		env.Receipts[0].ExpiresAtUTC = time.Now().UTC().Add(time.Hour)
		env.Receipts[0].ApprovalProof = &proof
		custExecWriteCustody(t, fx.repo.ws, env)

		legacy, err := json.Marshal(map[string]any{"nonce": proof.Nonce, "not_after": proof.NotAfterUTC, "request_identity_digest": proof.RequestIdentityDigest})
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(custodyReviewLedgerPath(fx.repo.root), append(legacy, '\n'), 0o600))

		verify, err := VerifyCustodyState(ctx, fx.repo.ws, CustodyVerifyOptions{ShipmentID: fx.repo.shipmentID, Branch: custExecFeatureBranch})
		require.NoError(t, err)
		assert.Equal(t, "UNVERIFIED_OPERATOR_PROVENANCE", custExecVerdict(verify, custExecCovered),
			"a ledger entry without consumed_at must not prove redemption when no item-log redemption exists")
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

// reviewFixWorkspace opens a workspace with default config and no git repository.
func reviewFixWorkspace(t *testing.T) *Workspace {
	t.Helper()
	root := t.TempDir()
	backlogitDir := filepath.Join(root, ".backlogit")
	require.NoError(t, os.MkdirAll(backlogitDir, 0o755))
	require.NoError(t, config.WriteDefaults(backlogitDir))
	ws, err := NewWorkspace(context.Background(), root)
	require.NoError(t, err)
	t.Cleanup(func() { _ = ws.Close() })
	return ws
}

// reviewFixSetCustody rewrites the canonical Markdown of id with value as its workflow_custody field.
func reviewFixSetCustody(t *testing.T, ws *Workspace, id string, value map[string]any) {
	t.Helper()
	ctx := context.Background()
	art, err := findArtifact(ctx, ws, id)
	require.NoError(t, err)
	if art.CustomFields == nil {
		art.CustomFields = map[string]any{}
	}
	art.CustomFields["workflow_custody"] = value
	path, err := FindArtifactPath(ctx, ws, id)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, []byte(models.SerializeFrontmatter(art.ToFrontmatterMap(), art.Description)), 0o644))
}

// reviewFixCustodyMarker returns the marker stored in the workflow_custody value of art.
func reviewFixCustodyMarker(t *testing.T, art *models.Artifact) string {
	t.Helper()
	envelope, ok := art.CustomFields["workflow_custody"].(map[string]any)
	require.True(t, ok, "workflow_custody must be a map")
	marker, _ := envelope["marker"].(string)
	return marker
}

// reviewFixMarkdownCount counts the Markdown artifact files under the workspace storage root.
func reviewFixMarkdownCount(t *testing.T, ws *Workspace) int {
	t.Helper()
	count := 0
	require.NoError(t, filepath.WalkDir(WorkspaceStorageRoot(ws.RootPath), func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && filepath.Ext(path) == ".md" {
			count++
		}
		return nil
	}))
	return count
}

// TestReview_AdoptReadsCanonicalEnvelope covers R3-1: adopt must rewrite the canonical
// Markdown envelope and never a stale SQLite projection of it.
func TestReview_AdoptReadsCanonicalEnvelope(t *testing.T) {
	ctx := context.Background()
	ws := reviewFixWorkspace(t)
	origin, err := CreateArtifact(ctx, ws, "Origin feature", "feature")
	require.NoError(t, err)
	destination, err := CreateArtifact(ctx, ws, "Destination feature", "feature")
	require.NoError(t, err)
	task, err := CreateArtifact(ctx, ws, "Task to adopt", "task", WithParent(origin.ID))
	require.NoError(t, err)
	reviewFixSetCustody(t, ws, task.ID, map[string]any{"marker": "canonical"})

	row, err := bldb.GetItem(ctx, ws.DB, task.ID)
	require.NoError(t, err)
	if row.CustomFields == nil {
		row.CustomFields = map[string]any{}
	}
	row.CustomFields["workflow_custody"] = map[string]any{"marker": "db-stale"}
	require.NoError(t, bldb.UpsertItem(ctx, ws.DB, row))

	result, err := AdoptItem(ctx, ws, task.ID, destination.ID)
	require.NoError(t, err)
	require.NotEmpty(t, result.NewID, "adopt must assign a new hierarchical ID")

	adopted, err := findArtifact(ctx, ws, result.NewID)
	require.NoError(t, err)
	assert.Equal(t, "canonical", reviewFixCustodyMarker(t, adopted),
		"adopt must keep the Markdown envelope rather than write the DB projection")
}

// TestReview_CreateRefusesWorkflowCustody covers R3-5: a create must not carry a
// workflow_custody key, so no caller can mint a pre-accepted agent receipt.
func TestReview_CreateRefusesWorkflowCustody(t *testing.T) {
	ctx := context.Background()
	ws := reviewFixWorkspace(t)
	before := reviewFixMarkdownCount(t, ws)

	cases := map[string]any{
		"populated": map[string]any{"marker": "forged"},
		"nil":       nil,
	}
	for name, value := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := CreateArtifact(ctx, ws, "Forged receipt", "feature",
				WithFields(map[string]any{"workflow_custody": value}))
			assert.ErrorIs(t, err, ErrCustodyUngovernedMutation)
		})
	}
	assert.Equal(t, before, reviewFixMarkdownCount(t, ws), "a refused create must write no file")
}

// TestReview_CustodyWritePreservesDBOnlyLinks covers F-14: a custody rewrite must not drop
// a link that exists only in the SQLite index.
func TestReview_CustodyWritePreservesDBOnlyLinks(t *testing.T) {
	ctx := context.Background()
	repo := custExecNewRepo(t)
	target, err := CreateArtifact(ctx, repo.ws, "Link target", "feature")
	require.NoError(t, err)
	link := models.ArtifactLink{TargetID: target.ID, LinkType: "related_to"}
	require.NoError(t, bldb.AddLink(ctx, repo.ws.DB, repo.shipmentID, target.ID, link.LinkType))

	before := custExecShipment(t, repo.ws, repo.shipmentID)
	require.NotContains(t, before.Links, link, "precondition: the link must exist only in SQLite")

	custExecWrite(t, repo.root, custExecCovered, custExecRevised)
	preview, err := PreviewCustodyCandidates(ctx, repo.ws, custExecPreviewOpts(repo, custExecStageCandidate()))
	require.NoError(t, err)
	_, err = OfferCustodyHandoff(ctx, repo.ws, CustodyOfferOptions{
		ShipmentID: repo.shipmentID,
		Spec:       custExecStageReceipt(preview.CandidateSetDigest),
	})
	require.NoError(t, err)

	after := custExecShipment(t, repo.ws, repo.shipmentID)
	assert.Contains(t, after.Links, link, "custody offer must keep DB-only links in the Markdown")
}

// TestReview_ProfileBOnlyForStageHandoff covers 211-S F12: Profile B is permitted only for
// stage_to_ship_handoff, so a ship_task_custody offer over a docs path requires Profile A.
func TestReview_ProfileBOnlyForStageHandoff(t *testing.T) {
	ctx := context.Background()
	repo := custExecNewRepo(t)
	custExecWrite(t, repo.root, "docs/review/x.md", "content\n")
	preview, err := PreviewCustodyCandidates(ctx, repo.ws, custExecPreviewOpts(repo,
		custodyReviewUntrackedCandidate("docs/review/x.md", "stage", "stage_artifact")))
	require.NoError(t, err)
	receipt := custExecReceipt("rcpt-ship-task-1", "ship_task_custody", "agent_workflow_handoff", "stage", "ship",
		"ship_task_commit", preview.CandidateSetDigest, custodyReviewUntrackedEntry("docs/review/x.md", "content\n", "stage", "stage_artifact"))
	_, err = OfferCustodyHandoff(ctx, repo.ws, CustodyOfferOptions{ShipmentID: repo.shipmentID, Spec: receipt})
	assert.ErrorIs(t, err, ErrOperatorApprovalRequired, "ship_task_custody must resolve to Profile A even inside docs/")
}

// TestReview_ExpiredAdoptionIsNotCarriedOver covers 211-S F13: an accepted Profile A receipt
// whose ExpiresAtUTC has passed does not count as adoption for a later Profile B handoff.
func TestReview_ExpiredAdoptionIsNotCarriedOver(t *testing.T) {
	ctx := context.Background()
	fx := custodyReviewOfferAdoption(t)
	fx.repo.ws.Config.TrustAnchors = []config.TrustAnchor{fx.anchor}
	live := WithCustodyNonceConsumer(ctx, custodyReviewConsumer(t, fx.repo.root, &custodyReviewProvider{}))
	_, err := AcceptCustodyHandoff(live, fx.repo.ws, fx.acceptOpts(fx.token(t, "carryover-nonce-1")))
	require.NoError(t, err)

	env := custExecEnvelope(t, custExecShipment(t, fx.repo.ws, fx.repo.shipmentID))
	idx := custodyReceiptIndex(env.Receipts, "rcpt-adopt-1")
	require.GreaterOrEqual(t, idx, 0)
	entry := env.Receipts[idx].Entries[0]
	trust := custodyTrustFor(fx.repo.ws, nil, nil, time.Now().UTC())
	require.True(t, custodyAdoptedFunc(fx.repo.ws, env, trust)(&entry), "control: an unexpired accepted adoption carries over")

	env.Receipts[idx].ExpiresAtUTC = time.Now().UTC().Add(-time.Minute)
	assert.False(t, custodyAdoptedFunc(fx.repo.ws, env, trust)(&entry), "an expired adoption must not carry over")
}

// TestReview_UnknownOriginOutOfScopeIsProfileA covers 211-S F13: a path outside .backlogit/
// and docs/ with no carried-over adoption resolves to Profile A, never Profile B.
func TestReview_UnknownOriginOutOfScopeIsProfileA(t *testing.T) {
	entries := []CustodyEntry{custodyReviewUntrackedEntry("internal/core/x.go", "content\n", "stage", "stage_artifact")}
	none := func(*CustodyEntry) bool { return false }
	assert.Equal(t, custodyProfileOperator,
		custodyRequiredProfile("stage_to_ship_handoff", "stage", "ship", "stage_artifact_commit", entries, none))
	assert.Equal(t, custodyProfileOperator,
		custodyRequiredProfile("stage_to_ship_handoff", "stage", "ship", "stage_artifact_commit", entries, nil))
}

// TestReview_DefaultBranchLiteralsRefused covers 211-S F17: develop and trunk are refused as
// custody scopes even when origin/HEAD does not name them.
func TestReview_DefaultBranchLiteralsRefused(t *testing.T) {
	ctx := context.Background()
	preview := func(repo *custExecRepo, branch string) error {
		_, err := PreviewCustodyCandidates(ctx, repo.ws, CustodyPreviewOptions{
			ShipmentID: repo.shipmentID, Branch: branch, Candidates: []CustodyCandidateSpec{custExecStageCandidate()},
		})
		return err
	}

	t.Run("develop_refused_without_origin_head", func(t *testing.T) {
		repo := custExecNewRepo(t)
		custExecWrite(t, repo.root, custExecCovered, custExecRevised)
		runGit(t, repo.root, "checkout", "-b", "develop")
		assert.ErrorIs(t, preview(repo, "develop"), ErrCustodyDefaultBranchForbidden)
	})

	t.Run("trunk_refused_when_origin_head_is_trunk", func(t *testing.T) {
		repo := custExecNewRepo(t)
		custExecWrite(t, repo.root, custExecCovered, custExecRevised)
		runGit(t, repo.root, "update-ref", "refs/remotes/origin/trunk", "HEAD")
		runGit(t, repo.root, "symbolic-ref", "refs/remotes/origin/HEAD", "refs/remotes/origin/trunk")
		runGit(t, repo.root, "checkout", "-b", "trunk")
		assert.ErrorIs(t, preview(repo, "trunk"), ErrCustodyDefaultBranchForbidden)
	})
}

// TestReview_AdoptFailsClosedWithoutCanonicalMarkdown covers 211-S F18: adopt reads the
// canonical Markdown preimage only and never falls back to the SQLite projection.
func TestReview_AdoptFailsClosedWithoutCanonicalMarkdown(t *testing.T) {
	ctx := context.Background()
	ws := setupShipmentWorkspace(t)
	feat1, err := CreateArtifact(ctx, ws, "Adopt origin feature", "feature")
	require.NoError(t, err)
	feat2, err := CreateArtifact(ctx, ws, "Adopt destination feature", "feature")
	require.NoError(t, err)
	task, err := CreateArtifact(ctx, ws, "Adopt task", "task", WithParent(feat1.ID))
	require.NoError(t, err)
	path, err := FindArtifactPath(ctx, ws, task.ID)
	require.NoError(t, err)
	require.NoError(t, os.Remove(path))
	_, err = bldb.GetItem(ctx, ws.DB, task.ID)
	require.NoError(t, err, "precondition: the SQLite projection still holds the item")

	_, err = AdoptItem(ctx, ws, task.ID, feat2.ID)
	assert.ErrorIs(t, err, blerrors.ErrNotFound, "adopt must fail when the canonical Markdown is missing")
	_, statErr := os.Stat(path)
	assert.True(t, os.IsNotExist(statErr), "a failed adopt must not recreate the canonical file")
}

// TestReview_NonceLedgerAndItemLogRefuseLinks covers 211-S F6: redemption reads never follow
// a symlink or reparse point at the nonce ledger or the shipment item log.
func TestReview_NonceLedgerAndItemLogRefuseLinks(t *testing.T) {
	now := time.Now().UTC()
	proof := CustodyApprovalProof{
		Nonce: "link-nonce-1", RequestIdentityDigest: "digest-link-1",
		NotBeforeUTC: now.Add(-time.Minute), NotAfterUTC: now.Add(5 * time.Minute),
	}

	t.Run("symlinked_nonce_ledger_fails_closed", func(t *testing.T) {
		repo := custExecNewRepo(t)
		line, err := json.Marshal(nonceLedgerRecord{
			Nonce: proof.Nonce, NotAfter: proof.NotAfterUTC, RequestIdentityDigest: proof.RequestIdentityDigest, ConsumedAt: &now,
		})
		require.NoError(t, err)
		target := filepath.Join(t.TempDir(), "nonce-ledger.json")
		require.NoError(t, os.WriteFile(target, append(line, '\n'), 0o600))
		if err := os.Symlink(target, custodyNonceLedgerPath(repo.root)); err != nil {
			t.Skipf("symlink creation is not permitted on this host: %v", err)
		}
		assert.False(t, custodyRedeemed(repo.ws, repo.shipmentID, "rcpt-link-1", proof),
			"a symlinked nonce ledger must not be followed")
	})

	t.Run("symlinked_item_log_fails_closed", func(t *testing.T) {
		repo := custExecNewRepo(t)
		line, err := json.Marshal(events.Event{EventType: EventCustodyHandoffAccepted, Delta: map[string]any{
			"receipt_id": "rcpt-link-1", "nonce": proof.Nonce,
			"request_identity_digest": proof.RequestIdentityDigest, "consumed_at": now,
		}})
		require.NoError(t, err)
		target := filepath.Join(t.TempDir(), "item.jsonl")
		require.NoError(t, os.WriteFile(target, append(line, '\n'), 0o600))
		logPath := events.LogPathForItem(WorkspaceLogsRoot(repo.root), repo.shipmentID)
		require.NoError(t, os.Remove(logPath), "replace the shipment item log created by CreateShipment")
		if err := os.Symlink(target, logPath); err != nil {
			t.Skipf("symlink creation is not permitted on this host: %v", err)
		}
		assert.False(t, custodyRedeemed(repo.ws, repo.shipmentID, "rcpt-link-1", proof),
			"a symlinked shipment item log must not be followed")
	})
}

// TestReview_RedemptionMustFallWithinTokenWindow covers 211-S F2: a redemption whose
// consumed_at falls outside the stored proof's [not_before, not_after] window fails closed.
func TestReview_RedemptionMustFallWithinTokenWindow(t *testing.T) {
	ctx := context.Background()
	fx := custodyReviewOfferAdoption(t)
	fx.repo.ws.Config.TrustAnchors = []config.TrustAnchor{fx.anchor}
	live := WithCustodyNonceConsumer(ctx, custodyReviewConsumer(t, fx.repo.root, &custodyReviewProvider{}))
	_, err := AcceptCustodyHandoff(live, fx.repo.ws, fx.acceptOpts(fx.token(t, "window-nonce-1")))
	require.NoError(t, err)

	env := custExecEnvelope(t, custExecShipment(t, fx.repo.ws, fx.repo.shipmentID))
	idx := custodyReceiptIndex(env.Receipts, "rcpt-adopt-1")
	require.GreaterOrEqual(t, idx, 0)
	receipt := env.Receipts[idx]
	require.NotNil(t, receipt.ApprovalProof)
	trust := custodyTrustFor(fx.repo.ws, nil, nil, time.Now().UTC())
	require.NoError(t, custodyVerifyOperatorAdoption(fx.repo.ws, env, receipt, trust),
		"control: a redemption inside the window verifies")

	late := receipt.ApprovalProof.NotAfterUTC.Add(time.Hour)
	line, err := json.Marshal(nonceLedgerRecord{
		Nonce: "window-nonce-1", NotAfter: late,
		RequestIdentityDigest: receipt.ApprovalProof.RequestIdentityDigest, ConsumedAt: &late,
	})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(custodyReviewLedgerPath(fx.repo.root), append(line, '\n'), 0o600))
	err = custodyVerifyOperatorAdoption(fx.repo.ws, env, receipt, trust)
	assert.ErrorIs(t, err, ErrAuthorizationRejected, "a redemption consumed after not_after must fail closed")
}
