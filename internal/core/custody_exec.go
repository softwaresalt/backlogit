package core

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"

	"github.com/softwaresalt/backlogit/internal/atomicfile"
	"github.com/softwaresalt/backlogit/internal/canonical"
	"github.com/softwaresalt/backlogit/internal/config"
	bldb "github.com/softwaresalt/backlogit/internal/db"
	blerrors "github.com/softwaresalt/backlogit/internal/errors"
	"github.com/softwaresalt/backlogit/internal/events"
	"github.com/softwaresalt/backlogit/internal/models"
)

const (
	custodyArtifactKey           = "workflow_custody"
	custodyJournalSchemaVersion  = "shipment-operation/v1"
	custodyReceiptOffered        = "offered"
	custodyReceiptAccepted       = "accepted"
	custodyReceiptSuperseded     = "superseded"
	custodyProfileAgent          = "agent_workflow_handoff"
	custodyPatchDomain           = "workflow-custody/patch/v1"
	custodyCandidateSetDomain    = "backlogit/workflow-custody/candidate-set/v1"
	custodyOperatorContributor   = "operator"
	custodySourceOperatorConfig  = "operator_config"
	custodySourceOperatorCodeDoc = "operator_code_or_doc"
)

// ErrCustodyUngovernedMutation reports a generic artifact write that would change
// custom_fields.workflow_custody outside the governed custody operations.
var ErrCustodyUngovernedMutation = errors.New("custody: workflow_custody may change only through governed custody operations")

type custodyNonceConsumerContextKey struct{}

// WithCustodyNonceConsumer returns a context carrying the nonce consumer that live
// operator_verified acceptance consumes approval nonces through. Host supervisors
// build the consumer over their external NonceConsumeProvider. Without one, live
// operator acceptance fails closed with ErrNonceProviderUnavailable.
func WithCustodyNonceConsumer(ctx context.Context, consumer *NonceConsumer) context.Context {
	return context.WithValue(ctx, custodyNonceConsumerContextKey{}, consumer)
}

func custodyNonceConsumerFrom(ctx context.Context) *NonceConsumer {
	consumer, _ := ctx.Value(custodyNonceConsumerContextKey{}).(*NonceConsumer)
	return consumer
}

// custodyEntryState carries the pinned digests of one covered path.
type custodyEntryState struct {
	base     string
	index    string
	worktree string
}

// custodyEvent is one JSONL event appended to the shipment item log.
type custodyEvent struct {
	eventType string
	delta     map[string]any
}

// custodyCommitPlan describes one governed custody write: the preimage shipment,
// the post-state shipment, and the events appended after the rewrite.
type custodyCommitPlan struct {
	shipmentID string
	operation  string
	target     string
	pre        *models.Artifact
	next       *models.Artifact
	events     []custodyEvent
}

// PreviewCustodyCandidates reports custody candidates for ws without writing any state.
// It classifies each candidate's pinned digests against the base commit, the Git index,
// and the worktree, and it requires the candidate set to fit one authority profile.
func PreviewCustodyCandidates(ctx context.Context, ws *Workspace, opts CustodyPreviewOptions) (CustodyPreviewResult, error) {
	if ws == nil {
		return CustodyPreviewResult{}, fmt.Errorf("preview custody: workspace is required: %w", blerrors.ErrValidation)
	}
	branch, err := custodyBranch(ctx, ws, opts.Branch)
	if err != nil {
		return CustodyPreviewResult{}, err
	}
	if err := custodyRefuseDefaultBranch(ws.RootPath, branch); err != nil {
		return CustodyPreviewResult{}, err
	}
	if _, err := custodyShipment(ctx, ws, opts.ShipmentID); err != nil {
		return CustodyPreviewResult{}, err
	}
	if len(opts.Candidates) == 0 {
		return CustodyPreviewResult{}, fmt.Errorf("preview custody %s: at least one candidate is required: %w", opts.ShipmentID, blerrors.ErrValidation)
	}
	commit, err := custodyHeadCommit(ctx, ws.RootPath)
	if err != nil {
		return CustodyPreviewResult{}, err
	}
	entries := make([]CustodyEntry, 0, len(opts.Candidates))
	for _, candidate := range opts.Candidates {
		entry, err := custodyEntryFromCandidate(ctx, ws, commit, candidate)
		if err != nil {
			return CustodyPreviewResult{}, err
		}
		entries = append(entries, entry)
	}
	specs := custodyCandidatesOf(entries)
	profile := custodyRequiredProfile(specs)
	for i := range entries {
		if err := validateCustodyEntry(&entries[i], profile); err != nil {
			return CustodyPreviewResult{}, fmt.Errorf("preview custody candidate %q: %w", entries[i].Path, err)
		}
	}
	if err := ValidateCustodyRenameCasing(ws, WorkflowCustodyEnvelope{Receipts: []CustodyReceipt{{Entries: entries}}}); err != nil {
		return CustodyPreviewResult{}, err
	}
	digest, err := custodyCandidateSetDigest(specs)
	if err != nil {
		return CustodyPreviewResult{}, err
	}
	return CustodyPreviewResult{
		ShipmentID:               opts.ShipmentID,
		Branch:                   branch,
		CandidateSetDigest:       digest,
		RequiredAuthorityProfile: profile,
		Candidates:               specs,
		Warnings:                 []string{},
	}, nil
}

// OfferCustodyHandoff records a custody handoff offer for a shipment. A live offer runs
// under the global, membership, item-log, and artifact locks and commits through the
// shipment-operation/v1 journal. A dry run validates the offer without locks or writes.
func OfferCustodyHandoff(ctx context.Context, ws *Workspace, opts CustodyOfferOptions) (CustodyMutationResult, error) {
	if ws == nil {
		return CustodyMutationResult{}, fmt.Errorf("offer custody: workspace is required: %w", blerrors.ErrValidation)
	}
	var result CustodyMutationResult
	mutate := func(lockedCtx context.Context) error {
		var err error
		result, err = custodyOffer(lockedCtx, ws, opts)
		return err
	}
	if opts.DryRun {
		if err := mutate(ctx); err != nil {
			return CustodyMutationResult{}, err
		}
		return result, nil
	}
	if err := custodyWithLocks(ctx, ws, opts.ShipmentID, mutate); err != nil {
		return CustodyMutationResult{}, err
	}
	return result, nil
}

// AcceptCustodyHandoff records acceptance of a previously offered custody handoff.
// Profile B (agent_workflow_handoff) needs only the accepting role. Profile A
// (operator_verified) needs a CustodyApprovalToken from a configured operator trust
// anchor, and a live accept consumes its nonce through the injected NonceConsumer.
func AcceptCustodyHandoff(ctx context.Context, ws *Workspace, opts CustodyAcceptOptions) (CustodyMutationResult, error) {
	if ws == nil {
		return CustodyMutationResult{}, fmt.Errorf("accept custody: workspace is required: %w", blerrors.ErrValidation)
	}
	var result CustodyMutationResult
	mutate := func(lockedCtx context.Context) error {
		var err error
		result, err = custodyAccept(lockedCtx, ws, opts)
		return err
	}
	if opts.DryRun {
		if err := mutate(ctx); err != nil {
			return CustodyMutationResult{}, err
		}
		return result, nil
	}
	if err := custodyWithLocks(ctx, ws, opts.ShipmentID, mutate); err != nil {
		return CustodyMutationResult{}, err
	}
	return result, nil
}

// VerifyCustodyState checks recorded custody receipts against the current workspace.
// It is read-only: it reads the shipment, Git objects, and the worktree, and writes nothing.
func VerifyCustodyState(ctx context.Context, ws *Workspace, opts CustodyVerifyOptions) (CustodyVerifyResult, error) {
	if ws == nil {
		return CustodyVerifyResult{}, fmt.Errorf("verify custody: workspace is required: %w", blerrors.ErrValidation)
	}
	branch, err := custodyBranch(ctx, ws, opts.Branch)
	if err != nil {
		return CustodyVerifyResult{}, err
	}
	art, err := custodyShipment(ctx, ws, opts.ShipmentID)
	if err != nil {
		return CustodyVerifyResult{}, err
	}
	result := CustodyVerifyResult{ShipmentID: opts.ShipmentID, Branch: branch, Verdicts: []CustodyPathVerdict{}, Warnings: []string{}}
	env, present, err := custodyEnvelopeOf(art)
	if err != nil {
		return CustodyVerifyResult{}, err
	}
	if !present {
		result.Warnings = append(result.Warnings, "no workflow custody envelope is recorded")
		return result, nil
	}
	result.ActiveReceiptID = env.ActiveReceiptID
	anchors := custodyAnchors(ws)
	now := time.Now().UTC()
	for i := range env.Receipts {
		receipt := env.Receipts[i]
		if receipt.Status != custodyReceiptOffered && receipt.Status != custodyReceiptAccepted {
			continue
		}
		for _, entry := range receipt.Entries {
			result.Verdicts = append(result.Verdicts, custodyVerdict(ctx, ws, env, receipt, entry, branch, anchors, now))
		}
	}
	return result, nil
}

// reconcileCustodyIntent recovers one pending custody journal. The recovery barrier
// already holds the lifecycle, membership, and artifact locks, so this acquires only
// the item-log lock C. An intent journal rolls back to its preimage. A committed
// journal is never rolled back: recovery only removes the journal.
func reconcileCustodyIntent(ctx context.Context, ws *Workspace, journalPath string, journal shipmentLifecycleJournal) (*models.Artifact, error) {
	if ws == nil || journal.Preimage.Shipment == nil {
		return nil, fmt.Errorf("custody journal %s is incomplete: %w", journalPath, blerrors.ErrValidation)
	}
	if _, held := ctx.Value(shipmentLifecycleGlobalLockContextKey{}).(struct{}); !held {
		return nil, fmt.Errorf("custody journal %s requires the shipment lifecycle lock: %w", journalPath, blerrors.ErrValidation)
	}
	itemCtx, unlockItemLog, err := events.LockItemLogCrossProcess(ctx, WorkspaceLocksRoot(ws.RootPath), WorkspaceLogsRoot(ws.RootPath), journal.ShipmentID)
	if err != nil {
		return nil, fmt.Errorf("lock custody item log for recovery: %w", err)
	}
	defer unlockItemLog()
	switch journal.Phase {
	case "committed":
		removeShipmentOperationJournal(itemCtx, ws, journalPath)
		return nil, nil
	case "intent":
		return nil, custodyRollback(itemCtx, ws, journal.CorrelationID, journalPath, journal.Preimage.Shipment)
	default:
		return nil, fmt.Errorf("custody journal %s has unsupported phase %q: %w", journalPath, journal.Phase, blerrors.ErrValidation)
	}
}

// isCustodyShipmentOperation reports whether operation is a custody journal operation.
func isCustodyShipmentOperation(operation string) bool {
	return operation == shipmentOperationCustodyOffer || operation == shipmentOperationCustodyAccept
}

// guardWorkflowCustodyMutation refuses a governed artifact rewrite that changes
// custom_fields.workflow_custody unless the write is a custody operation.
func guardWorkflowCustodyMutation(previous, next *models.Artifact, allowed bool) error {
	if allowed || previous == nil || next == nil {
		return nil
	}
	if custodyValuesEqual(previous.CustomFields[custodyArtifactKey], next.CustomFields[custodyArtifactKey]) {
		return nil
	}
	return fmt.Errorf("refusing ungoverned %s mutation on %s: %w", custodyArtifactKey, next.ID, ErrCustodyUngovernedMutation)
}

// guardWorkflowCustodyUpdate refuses a generic update whose custom_fields change
// custom_fields.workflow_custody relative to the current artifact.
func guardWorkflowCustodyUpdate(artifact *models.Artifact, updates map[string]any) error {
	fields, ok := updates["custom_fields"].(map[string]any)
	if !ok {
		return nil
	}
	next, present := fields[custodyArtifactKey]
	if !present || custodyValuesEqual(artifact.CustomFields[custodyArtifactKey], next) {
		return nil
	}
	return fmt.Errorf("refusing ungoverned %s mutation on %s: %w", custodyArtifactKey, artifact.ID, ErrCustodyUngovernedMutation)
}

func custodyValuesEqual(a, b any) bool {
	left, errLeft := json.Marshal(a)
	right, errRight := json.Marshal(b)
	return errLeft == nil && errRight == nil && bytes.Equal(left, right)
}

func custodyOffer(ctx context.Context, ws *Workspace, opts CustodyOfferOptions) (CustodyMutationResult, error) {
	branch, commit, err := custodyMutationContext(ctx, ws)
	if err != nil {
		return CustodyMutationResult{}, err
	}
	pre, err := custodyShipment(ctx, ws, opts.ShipmentID)
	if err != nil {
		return CustodyMutationResult{}, err
	}
	next, err := custodyShipment(ctx, ws, opts.ShipmentID)
	if err != nil {
		return CustodyMutationResult{}, err
	}
	env, err := custodyEnvelopeForOffer(pre, opts.ShipmentID, branch, commit)
	if err != nil {
		return CustodyMutationResult{}, err
	}
	spec := opts.Spec
	if err := custodyValidateOfferSpec(env, spec); err != nil {
		return CustodyMutationResult{}, err
	}
	if err := custodyVerifyEntriesLive(ctx, ws, env.BaseCommitSHA, spec.Entries, spec.AuthorityProfile); err != nil {
		return CustodyMutationResult{}, err
	}
	spec.OfferedAtUTC = time.Now().UTC()
	env.Receipts = append(env.Receipts, spec)
	if err := ValidateCustodyEnvelope(env); err != nil {
		return CustodyMutationResult{}, fmt.Errorf("offer custody receipt %s: %w", spec.ReceiptID, err)
	}
	if err := ValidateCustodyRenameCasing(ws, env); err != nil {
		return CustodyMutationResult{}, fmt.Errorf("offer custody receipt %s: %w", spec.ReceiptID, err)
	}
	result := custodyResult(opts.ShipmentID, spec, custodyReceiptOffered, env.CustodyRevision, opts.DryRun)
	if opts.DryRun {
		return result, nil
	}
	if err := custodySetEnvelope(next, env, branch); err != nil {
		return CustodyMutationResult{}, err
	}
	err = custodyCommitTransaction(ctx, ws, custodyCommitPlan{
		shipmentID: opts.ShipmentID,
		operation:  shipmentOperationCustodyOffer,
		target:     custodyReceiptOffered,
		pre:        pre,
		next:       next,
		events: []custodyEvent{{eventType: EventCustodyHandoffOffered, delta: map[string]any{
			"receipt_id":        spec.ReceiptID,
			"kind":              spec.Kind,
			"authority_profile": spec.AuthorityProfile,
			"custody_revision":  spec.Revision,
		}}},
	})
	if err != nil {
		return CustodyMutationResult{}, err
	}
	return result, nil
}

func custodyAccept(ctx context.Context, ws *Workspace, opts CustodyAcceptOptions) (CustodyMutationResult, error) {
	branch, _, err := custodyMutationContext(ctx, ws)
	if err != nil {
		return CustodyMutationResult{}, err
	}
	pre, err := custodyShipment(ctx, ws, opts.ShipmentID)
	if err != nil {
		return CustodyMutationResult{}, err
	}
	next, err := custodyShipment(ctx, ws, opts.ShipmentID)
	if err != nil {
		return CustodyMutationResult{}, err
	}
	env, present, err := custodyEnvelopeOf(pre)
	if err != nil {
		return CustodyMutationResult{}, err
	}
	if !present {
		return CustodyMutationResult{}, fmt.Errorf("accept custody %s: no custody offer is recorded: %w", opts.ShipmentID, blerrors.ErrValidation)
	}
	if env.ScopeBranch != branch {
		return CustodyMutationResult{}, fmt.Errorf("%w: envelope scope %q, checked out %q", ErrCustodyWrongBranch, env.ScopeBranch, branch)
	}
	idx := custodyReceiptIndex(env.Receipts, opts.ReceiptID)
	if idx < 0 {
		return CustodyMutationResult{}, fmt.Errorf("accept custody %s: receipt %q not found: %w", opts.ShipmentID, opts.ReceiptID, blerrors.ErrValidation)
	}
	receipt := env.Receipts[idx]
	if receipt.Status != custodyReceiptOffered {
		return CustodyMutationResult{}, fmt.Errorf("accept custody receipt %s: status is %q, not offered: %w", receipt.ReceiptID, receipt.Status, blerrors.ErrValidation)
	}
	if opts.By != receipt.AcceptingRole {
		return CustodyMutationResult{}, fmt.Errorf("accept custody receipt %s: principal %q is not accepting role %q: %w", receipt.ReceiptID, opts.By, receipt.AcceptingRole, blerrors.ErrValidation)
	}
	if err := custodyVerifyEntriesLive(ctx, ws, env.BaseCommitSHA, receipt.Entries, receipt.AuthorityProfile); err != nil {
		return CustodyMutationResult{}, err
	}
	var proof *CustodyApprovalProof
	switch receipt.AuthorityProfile {
	case custodyProfileAgent:
		if custodyRequiredProfile(custodyCandidatesOf(receipt.Entries)) == custodyProfileOperator {
			return CustodyMutationResult{}, fmt.Errorf("accept custody receipt %s: %w: entries include operator-contributed paths", receipt.ReceiptID, ErrOperatorApprovalRequired)
		}
		if receipt.ProducerRole == receipt.AcceptingRole {
			return CustodyMutationResult{}, fmt.Errorf("accept custody receipt %s: producer and accepting role must differ: %w", receipt.ReceiptID, blerrors.ErrValidation)
		}
	case custodyProfileOperator:
		proof, err = custodyAuthorizeOperator(ctx, ws, env, receipt, opts)
		if err != nil {
			return CustodyMutationResult{}, err
		}
	default:
		return CustodyMutationResult{}, fmt.Errorf("accept custody receipt %s: unknown authority profile %q: %w", receipt.ReceiptID, receipt.AuthorityProfile, blerrors.ErrValidation)
	}

	now := time.Now().UTC()
	receipt.Status = custodyReceiptAccepted
	receipt.AcceptedAtUTC = now
	receipt.ExpiresAtUTC = DefaultCustodyReceiptExpiresAt(now)
	receipt.IdempotencyKey = opts.IdempotencyKey
	receipt.ApprovalProof = proof
	if prev := custodyReceiptIndex(env.Receipts, env.ActiveReceiptID); prev >= 0 && env.Receipts[prev].Status == custodyReceiptAccepted {
		env.Receipts[prev].Status = custodyReceiptSuperseded
		receipt.SupersedesReceiptID = env.Receipts[prev].ReceiptID
	}
	env.Receipts[idx] = receipt
	env.ActiveReceiptID = receipt.ReceiptID
	receiptDigest, err := custodyReceiptDigest(receipt)
	if err != nil {
		return CustodyMutationResult{}, err
	}
	env.ActiveReceiptDigest = receiptDigest
	env.CustodyRevision = receipt.Revision
	if err := ValidateCustodyEnvelope(env); err != nil {
		return CustodyMutationResult{}, fmt.Errorf("accept custody receipt %s: %w", receipt.ReceiptID, err)
	}
	if err := ValidateCustodyRenameCasing(ws, env); err != nil {
		return CustodyMutationResult{}, fmt.Errorf("accept custody receipt %s: %w", receipt.ReceiptID, err)
	}
	result := custodyResult(opts.ShipmentID, receipt, custodyReceiptAccepted, env.CustodyRevision, opts.DryRun)
	if opts.DryRun {
		return result, nil
	}
	if err := custodySetEnvelope(next, env, branch); err != nil {
		return CustodyMutationResult{}, err
	}
	acceptEvents := []custodyEvent{{eventType: EventCustodyHandoffAccepted, delta: map[string]any{
		"receipt_id":        receipt.ReceiptID,
		"authority_profile": receipt.AuthorityProfile,
		"accepted_by":       opts.By,
		"custody_revision":  receipt.Revision,
	}}}
	if proof != nil {
		acceptEvents = append(acceptEvents, custodyEvent{eventType: EventCustodyAdoptionRecorded, delta: map[string]any{
			"receipt_id": receipt.ReceiptID,
			"nonce":      proof.Nonce,
		}})
	}
	if err := custodyCommitTransaction(ctx, ws, custodyCommitPlan{
		shipmentID: opts.ShipmentID,
		operation:  shipmentOperationCustodyAccept,
		target:     custodyReceiptAccepted,
		pre:        pre,
		next:       next,
		events:     acceptEvents,
	}); err != nil {
		return CustodyMutationResult{}, err
	}
	return result, nil
}

// custodyAuthorizeOperator verifies a Profile A approval token against the configured
// operator trust anchors and binds it to the receipt. A live accept consumes the token
// nonce through the injected NonceConsumer. A dry run verifies without consuming.
func custodyAuthorizeOperator(ctx context.Context, ws *Workspace, env WorkflowCustodyEnvelope, receipt CustodyReceipt, opts CustodyAcceptOptions) (*CustodyApprovalProof, error) {
	if opts.AuthToken == "" {
		return nil, fmt.Errorf("%w: operator_verified receipt %s requires a custody approval token", ErrAuthorizationRequired, receipt.ReceiptID)
	}
	anchors := custodyAnchors(ws)
	if len(anchors) == 0 {
		return nil, fmt.Errorf("%w: no operator trust anchor is configured", ErrApprovalChannelUnavailable)
	}
	proof, err := VerifyCustodyApprovalToken(opts.AuthToken, env, time.Now().UTC(), anchors...)
	if err != nil {
		return nil, err
	}
	if proof.CandidateSetDigest != receipt.CandidateSetDigest || proof.AllowedAction != receipt.AllowedAction || proof.TargetRevision != receipt.Revision {
		return nil, fmt.Errorf("%w: approval token does not bind receipt %s", ErrAuthorizationRejected, receipt.ReceiptID)
	}
	if opts.DryRun {
		return &proof, nil
	}
	consumer := custodyNonceConsumerFrom(ctx)
	if consumer == nil {
		return nil, fmt.Errorf("%w: %w", ErrAuthorizationRejected, ErrNonceProviderUnavailable)
	}
	if err := consumer.ConsumeBound(ctx, proof.Nonce, proof.NotAfterUTC, proof.RequestIdentityDigest); err != nil {
		return nil, rejectApproval(fmt.Errorf("consume nonce: %w", err))
	}
	return &proof, nil
}

// custodyVerifyOperatorProvenance re-verifies a stored operator approval proof
// against the configured trust anchors, so a proof written out of band is not trusted.
func custodyVerifyOperatorProvenance(env WorkflowCustodyEnvelope, receipt CustodyReceipt, anchors []config.TrustAnchor, now time.Time) error {
	if receipt.ApprovalProof == nil {
		return fmt.Errorf("%w: accepted operator receipt has no approval proof", ErrOperatorApprovalRequired)
	}
	proof := *receipt.ApprovalProof
	digest, err := CustodyRequestIdentityDigest(env)
	if err != nil {
		return err
	}
	if digest != proof.RequestIdentityDigest {
		return fmt.Errorf("%w: proof does not bind the current request identity", ErrAuthorizationRejected)
	}
	signature, err := base64.RawURLEncoding.DecodeString(proof.Signature)
	if err != nil || len(signature) != ed25519.SignatureSize {
		return fmt.Errorf("%w: proof signature is malformed", ErrAuthorizationRejected)
	}
	pub, err := resolveCustodyOperatorAnchor(proof.KeyID, now, anchors)
	if err != nil {
		return err
	}
	proof.Signature = ""
	payload, err := json.Marshal(proof)
	if err != nil {
		return fmt.Errorf("encode approval proof: %w", err)
	}
	if !ed25519.Verify(pub, custodyTokenSigningInput(payload), signature) {
		return fmt.Errorf("%w: proof signature does not verify", ErrAuthorizationRejected)
	}
	return nil
}

// custodyVerdict classifies one covered path of an offered or accepted receipt.
func custodyVerdict(ctx context.Context, ws *Workspace, env WorkflowCustodyEnvelope, receipt CustodyReceipt, entry CustodyEntry, branch string, anchors []config.TrustAnchor, now time.Time) CustodyPathVerdict {
	verdict := CustodyPathVerdict{Path: entry.Path, OldPath: entry.OldPath, ReceiptID: receipt.ReceiptID}
	switch {
	case env.ScopeBranch != branch:
		verdict.Verdict = "WRONG_BRANCH"
		verdict.Reason = fmt.Sprintf("receipt scope_branch %q does not match %q", env.ScopeBranch, branch)
	case receipt.Status == custodyReceiptOffered:
		verdict.Verdict = "UNCOVERED"
		verdict.Reason = "offered receipt is not accepted"
	default:
		verdict = custodyAcceptedVerdict(ctx, ws, env, receipt, entry, anchors, now, verdict)
	}
	return verdict
}

func custodyAcceptedVerdict(ctx context.Context, ws *Workspace, env WorkflowCustodyEnvelope, receipt CustodyReceipt, entry CustodyEntry, anchors []config.TrustAnchor, now time.Time, verdict CustodyPathVerdict) CustodyPathVerdict {
	live, err := custodyLiveDigests(ctx, ws, env.BaseCommitSHA, entry.Path)
	if err != nil {
		verdict.Verdict = "UNCOVERED"
		verdict.Reason = err.Error()
		return verdict
	}
	oldBase := ""
	if entry.OldPath != "" {
		if oldBase, err = custodyCommittedDigest(ctx, ws.RootPath, env.BaseCommitSHA, entry.OldPath); err != nil {
			verdict.Verdict = "UNCOVERED"
			verdict.Reason = err.Error()
			return verdict
		}
	}
	if live.base != entry.BaseBlobSHA256 || live.index != entry.IndexSHA256 || live.worktree != entry.WorktreeSHA256 || oldBase != entry.OldBaseBlobSHA256 {
		verdict.Verdict = "STALE_DIGEST"
		verdict.Reason = "covered content changed since it was offered"
		return verdict
	}
	if receipt.AuthorityProfile == custodyProfileOperator {
		if err := custodyVerifyOperatorProvenance(env, receipt, anchors, now); err != nil {
			verdict.Verdict = "UNVERIFIED_OPERATOR_PROVENANCE"
			verdict.Reason = err.Error()
			return verdict
		}
		verdict.Verdict = "ADOPTED_OPERATOR_VERIFIED"
		return verdict
	}
	verdict.Verdict = "HANDOFF_AGENT_VERIFIED"
	return verdict
}

// custodyWithLocks runs fn under the canonical lock order global -> A -> C -> B,
// where A is shipment membership, C is the shipment item log, and B is the artifact.
func custodyWithLocks(ctx context.Context, ws *Workspace, shipmentID string, fn func(context.Context) error) error {
	lockedCtx, unlockGlobal, err := lockShipmentLifecycleGlobal(ctx, ws)
	if err != nil {
		return fmt.Errorf("lock shipment lifecycle for custody: %w", err)
	}
	unlockMembership, err := lockShipmentMembership(lockedCtx, ws, shipmentID)
	if err != nil {
		return errors.Join(fmt.Errorf("lock shipment membership for custody: %w", err), unlockGlobal())
	}
	itemCtx, unlockItemLog, err := events.LockItemLogCrossProcess(lockedCtx, WorkspaceLocksRoot(ws.RootPath), WorkspaceLogsRoot(ws.RootPath), shipmentID)
	if err != nil {
		return errors.Join(fmt.Errorf("lock shipment item log for custody: %w", err), unlockMembership(), unlockGlobal())
	}
	artifactCtx, unlockArtifacts, err := lockArtifactMutations(itemCtx, ws, []string{shipmentID})
	if err != nil {
		unlockItemLog()
		return errors.Join(fmt.Errorf("lock shipment artifact for custody: %w", err), unlockMembership(), unlockGlobal())
	}
	fnErr := fn(artifactCtx)
	artifactErr := unlockArtifacts()
	unlockItemLog()
	return errors.Join(fnErr, artifactErr, unlockMembership(), unlockGlobal())
}

// custodyCommitTransaction writes the intent journal, rewrites the shipment, appends the
// events, and then records the committed phase. A failure before the committed record
// rolls back to the preimage. An indeterminate committed record is not rolled back.
func custodyCommitTransaction(ctx context.Context, ws *Workspace, plan custodyCommitPlan) error {
	correlationID, err := custodyRandomID()
	if err != nil {
		return err
	}
	journal := shipmentLifecycleJournal{
		SchemaVersion:  custodyJournalSchemaVersion,
		CorrelationID:  correlationID,
		Phase:          shipmentOperationPhaseIntent,
		Operation:      plan.operation,
		RecoveryPolicy: shipmentOperationRecoveryPolicyRollback,
		ShipmentID:     plan.shipmentID,
		Target:         plan.target,
		Preimage:       shipmentLifecyclePreimage{Shipment: plan.pre},
	}
	name := "shipment-operation-" + correlationID + ".json"
	journalPath, err := writeShipmentLifecycleJournalForWorkspace(ws, name, journal)
	if err != nil {
		return fmt.Errorf("write custody intent journal: %w", err)
	}
	opCtx := withShipmentOperation(ctx, correlationID)
	if err := custodyApply(opCtx, ws, plan); err != nil {
		return errors.Join(err, custodyRollback(opCtx, ws, correlationID, journalPath, plan.pre))
	}
	journal.Phase = shipmentOperationPhaseCommitted
	if _, err := writeShipmentLifecycleJournalForWorkspace(ws, name, journal); err != nil {
		commitErr := fmt.Errorf("record committed custody journal %s: %w", correlationID, err)
		if errors.Is(err, blerrors.ErrWriteIndeterminate) {
			return commitErr
		}
		return errors.Join(commitErr, custodyRollback(opCtx, ws, correlationID, journalPath, plan.pre))
	}
	removeShipmentOperationJournal(ctx, ws, journalPath)
	return nil
}

func custodyApply(ctx context.Context, ws *Workspace, plan custodyCommitPlan) error {
	if err := custodyWriteArtifact(ctx, ws, plan.next); err != nil {
		return err
	}
	for _, ev := range plan.events {
		if err := appendItemEventErr(ctx, ws, plan.shipmentID, ev.eventType, ev.delta); err != nil {
			return fmt.Errorf("append custody event %s: %w", ev.eventType, err)
		}
	}
	return nil
}

// custodyRollback restores the shipment preimage, removes this operation's events from
// the item log, and removes the journal. The journal remains if any restore step fails,
// so the next recovery barrier retries the rollback.
func custodyRollback(ctx context.Context, ws *Workspace, correlationID, journalPath string, pre *models.Artifact) error {
	if err := custodyWriteArtifact(ctx, ws, pre); err != nil {
		return fmt.Errorf("restore custody preimage %s: %w", pre.ID, err)
	}
	if err := custodyDropOperationEvents(ws, pre.ID, correlationID); err != nil {
		return fmt.Errorf("restore custody event log %s: %w", pre.ID, err)
	}
	removeShipmentOperationJournal(ctx, ws, journalPath)
	return nil
}

// custodyDropOperationEvents removes the item-log lines tagged with correlationID and
// rewrites the log atomically. A log with no tagged lines is left untouched.
func custodyDropOperationEvents(ws *Workspace, shipmentID, correlationID string) error {
	logPath := events.LogPathForItem(WorkspaceLogsRoot(ws.RootPath), shipmentID)
	data, err := os.ReadFile(logPath)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	var kept []byte
	dropped := false
	for _, line := range bytes.SplitAfter(data, []byte("\n")) {
		trimmed := bytes.TrimSpace(line)
		if len(trimmed) == 0 {
			kept = append(kept, line...)
			continue
		}
		var record struct {
			Delta map[string]any `json:"delta"`
		}
		if json.Unmarshal(trimmed, &record) == nil {
			if op, _ := record.Delta[shipmentOperationDeltaKey].(string); op == correlationID {
				dropped = true
				continue
			}
		}
		kept = append(kept, line...)
	}
	if !dropped {
		return nil
	}
	return atomicfile.WriteFileAtomicWithOptions(logPath, kept, atomicfile.Options{DurableWrites: true})
}

// custodyWriteArtifact rewrites the shipment through the governed writer, which is the
// only path permitted to change custom_fields.workflow_custody, and mirrors it to SQLite.
func custodyWriteArtifact(ctx context.Context, ws *Workspace, art *models.Artifact) error {
	path, err := FindArtifactPath(ctx, ws, art.ID)
	if err != nil {
		return fmt.Errorf("locate custody shipment %s: %w", art.ID, err)
	}
	envelope := artifactWriteEnvelope{operation: "custody", allowCustodyMutation: true}
	if err := writeArtifactFileGoverned(art, path, path, true, envelope, nil); err != nil {
		return fmt.Errorf("write custody shipment %s: %w", art.ID, err)
	}
	if ws.DB != nil {
		if err := bldb.UpsertItem(ctx, ws.DB, art); err != nil {
			return fmt.Errorf("index custody shipment %s: %w", art.ID, err)
		}
	}
	return nil
}

func custodyBranch(ctx context.Context, ws *Workspace, requested string) (string, error) {
	if requested != "" {
		return requested, nil
	}
	return custodyCurrentBranch(ctx, ws.RootPath)
}

// custodyMutationContext returns the checked-out branch and HEAD for a mutation and
// refuses the default branch, because custody is scoped to a non-default work branch.
func custodyMutationContext(ctx context.Context, ws *Workspace) (string, string, error) {
	if ws == nil {
		return "", "", fmt.Errorf("custody mutation: workspace is required: %w", blerrors.ErrValidation)
	}
	branch, err := custodyCurrentBranch(ctx, ws.RootPath)
	if err != nil {
		return "", "", err
	}
	if err := custodyRefuseDefaultBranch(ws.RootPath, branch); err != nil {
		return "", "", err
	}
	commit, err := custodyHeadCommit(ctx, ws.RootPath)
	if err != nil {
		return "", "", err
	}
	return branch, commit, nil
}

func custodyRefuseDefaultBranch(root, branch string) error {
	defaultBranch, err := config.ResolveDefaultBranch(root)
	if err != nil {
		return fmt.Errorf("resolve default branch: %w", err)
	}
	if strings.EqualFold(branch, defaultBranch) || strings.EqualFold(branch, "main") || strings.EqualFold(branch, "master") {
		return fmt.Errorf("%w: %q", ErrCustodyDefaultBranchForbidden, branch)
	}
	return nil
}

func custodyCurrentBranch(ctx context.Context, root string) (string, error) {
	out, err := custodyGit(ctx, root, "symbolic-ref", "--quiet", "--short", "HEAD")
	if err != nil {
		return "", fmt.Errorf("resolve custody branch (detached HEAD is refused): %w: %w", blerrors.ErrValidation, err)
	}
	return strings.TrimSpace(string(out)), nil
}

func custodyHeadCommit(ctx context.Context, root string) (string, error) {
	out, err := custodyGit(ctx, root, "rev-parse", "--verify", "HEAD^{commit}")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// custodyGit runs git against root with --no-replace-objects, so replacement refs
// cannot change what custody inspects, and --no-optional-locks, so inspection never
// refreshes the index.
func custodyGit(ctx context.Context, root string, args ...string) ([]byte, error) {
	full := append([]string{"-C", root, "--no-replace-objects", "--no-optional-locks"}, args...)
	cmd := exec.CommandContext(ctx, "git", full...)
	cmd.Env = config.ChildProcessEnv()
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("git %s: %w: %s", args[0], err, strings.TrimSpace(stderr.String()))
	}
	return stdout.Bytes(), nil
}

// custodyDigest is the pinned content digest: SHA-256 hex over LF-normalized bytes.
func custodyDigest(data []byte) string {
	return canonical.HashBytes(bytes.ReplaceAll(data, []byte("\r\n"), []byte("\n")))
}

// custodyPatchDigest pins the adopted patch as a digest over the base, index, and worktree digests.
func custodyPatchDigest(base, index, worktree string) string {
	return canonical.HashBytes([]byte(custodyPatchDomain + "\n" + base + "\n" + index + "\n" + worktree))
}

func custodyCommittedDigest(ctx context.Context, root, commit, path string) (string, error) {
	out, err := custodyGit(ctx, root, "ls-tree", "-z", commit, "--", path)
	if err != nil {
		return "", err
	}
	oid, ok := custodyBlobOID(out, path, 2, false)
	if !ok {
		return "", nil
	}
	return custodyBlobDigest(ctx, root, oid)
}

func custodyIndexDigest(ctx context.Context, root, path string) (string, error) {
	out, err := custodyGit(ctx, root, "ls-files", "-s", "-z", "--", path)
	if err != nil {
		return "", err
	}
	oid, ok := custodyBlobOID(out, path, 1, true)
	if !ok {
		return "", nil
	}
	return custodyBlobDigest(ctx, root, oid)
}

func custodyBlobDigest(ctx context.Context, root, oid string) (string, error) {
	data, err := custodyGit(ctx, root, "cat-file", "blob", oid)
	if err != nil {
		return "", err
	}
	return custodyDigest(data), nil
}

// custodyBlobOID returns the object id of the entry named path in a NUL-separated
// ls-tree or ls-files listing. When stageZero is set, only stage 0 entries match.
func custodyBlobOID(listing []byte, path string, oidField int, stageZero bool) (string, bool) {
	for _, record := range bytes.Split(listing, []byte{0}) {
		meta, name, ok := strings.Cut(string(record), "\t")
		if !ok || name != path {
			continue
		}
		fields := strings.Fields(meta)
		if len(fields) <= oidField {
			continue
		}
		if stageZero && (len(fields) < 3 || fields[2] != "0") {
			continue
		}
		return fields[oidField], true
	}
	return "", false
}

// custodyLiveDigests reads the base, index, and worktree digests of path. The worktree
// read goes through the handle-pinned no-follow reader.
func custodyLiveDigests(ctx context.Context, ws *Workspace, commit, path string) (custodyEntryState, error) {
	base, err := custodyCommittedDigest(ctx, ws.RootPath, commit, path)
	if err != nil {
		return custodyEntryState{}, err
	}
	index, err := custodyIndexDigest(ctx, ws.RootPath, path)
	if err != nil {
		return custodyEntryState{}, err
	}
	worktree, err := custodyWorktreeDigest(ws, path)
	if err != nil {
		return custodyEntryState{}, err
	}
	return custodyEntryState{base: base, index: index, worktree: worktree}, nil
}

func custodyWorktreeDigest(ws *Workspace, path string) (string, error) {
	data, err := readCustodyWorktreeFileNoFollow(ws, path)
	if errors.Is(err, fs.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return custodyDigest(data), nil
}

// custodyEntryFromCandidate builds a covered entry with its pinned digests from the
// live repository state.
func custodyEntryFromCandidate(ctx context.Context, ws *Workspace, commit string, c CustodyCandidateSpec) (CustodyEntry, error) {
	live, err := custodyLiveDigests(ctx, ws, commit, c.Path)
	if err != nil {
		return CustodyEntry{}, fmt.Errorf("read custody candidate %q: %w", c.Path, err)
	}
	entry := CustodyEntry{
		Path:           c.Path,
		OldPath:        c.OldPath,
		ChangeKind:     c.ChangeKind,
		StagedState:    c.StagedState,
		Contributor:    c.Contributor,
		SourceKind:     c.SourceKind,
		RenameGroupID:  c.RenameGroupID,
		BaseBlobSHA256: live.base,
		IndexSHA256:    live.index,
		WorktreeSHA256: live.worktree,
	}
	if c.ChangeKind == "renamed" && c.OldPath != "" {
		entry.CaseOnlyRename = c.OldPath != c.Path && strings.EqualFold(c.OldPath, c.Path)
		oldBase, err := custodyCommittedDigest(ctx, ws.RootPath, commit, c.OldPath)
		if err != nil {
			return CustodyEntry{}, fmt.Errorf("read custody rename source %q: %w", c.OldPath, err)
		}
		entry.OldBaseBlobSHA256 = oldBase
	}
	entry.PatchDigestSHA256 = custodyPatchDigest(entry.BaseBlobSHA256, entry.IndexSHA256, entry.WorktreeSHA256)
	return entry, nil
}

// custodyVerifyEntriesLive checks each entry against the current repository state.
// A mismatch in any pinned digest is a stale digest, and the patch digest must match
// its pre- and post-image digests.
func custodyVerifyEntriesLive(ctx context.Context, ws *Workspace, commit string, entries []CustodyEntry, profile string) error {
	for i := range entries {
		entry := entries[i]
		if err := validateCustodyEntry(&entry, profile); err != nil {
			return err
		}
		live, err := custodyEntryFromCandidate(ctx, ws, commit, custodyCandidateOf(entry))
		if err != nil {
			return err
		}
		if live.BaseBlobSHA256 != entry.BaseBlobSHA256 || live.IndexSHA256 != entry.IndexSHA256 ||
			live.WorktreeSHA256 != entry.WorktreeSHA256 || live.OldBaseBlobSHA256 != entry.OldBaseBlobSHA256 {
			return fmt.Errorf("%w: %s", ErrCustodyStaleDigest, entry.Path)
		}
		if entry.PatchDigestSHA256 != custodyPatchDigest(entry.BaseBlobSHA256, entry.IndexSHA256, entry.WorktreeSHA256) {
			return fmt.Errorf("custody patch digest for %s does not match its pinned digests: %w", entry.Path, blerrors.ErrValidation)
		}
	}
	return nil
}

func custodyCandidateOf(e CustodyEntry) CustodyCandidateSpec {
	return CustodyCandidateSpec{
		Path:          e.Path,
		OldPath:       e.OldPath,
		ChangeKind:    e.ChangeKind,
		StagedState:   e.StagedState,
		Contributor:   e.Contributor,
		SourceKind:    e.SourceKind,
		RenameGroupID: e.RenameGroupID,
	}
}

func custodyCandidatesOf(entries []CustodyEntry) []CustodyCandidateSpec {
	specs := make([]CustodyCandidateSpec, 0, len(entries))
	for i := range entries {
		specs = append(specs, custodyCandidateOf(entries[i]))
	}
	return specs
}

// custodyCandidateSetDigest is the domain-separated digest over the candidate specs,
// independent of their order.
func custodyCandidateSetDigest(specs []CustodyCandidateSpec) (string, error) {
	sorted := append([]CustodyCandidateSpec(nil), specs...)
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].Path != sorted[j].Path {
			return sorted[i].Path < sorted[j].Path
		}
		return sorted[i].OldPath < sorted[j].OldPath
	})
	payload, err := json.Marshal(sorted)
	if err != nil {
		return "", fmt.Errorf("encode candidate set: %w", err)
	}
	return canonical.HashBytes(append([]byte(custodyCandidateSetDomain+"\n"), payload...)), nil
}

// custodyRequiredProfile returns Profile A when any candidate is operator-contributed
// and Profile B otherwise.
func custodyRequiredProfile(specs []CustodyCandidateSpec) string {
	for _, s := range specs {
		if s.Contributor == custodyOperatorContributor || s.Contributor == custodyContributorMixed ||
			s.SourceKind == custodySourceOperatorConfig || s.SourceKind == custodySourceOperatorCodeDoc {
			return custodyProfileOperator
		}
	}
	return custodyProfileAgent
}

func custodyShipment(ctx context.Context, ws *Workspace, id string) (*models.Artifact, error) {
	if id == "" {
		return nil, fmt.Errorf("custody shipment id is required: %w", blerrors.ErrValidation)
	}
	art, err := findArtifact(ctx, ws, id)
	if err != nil {
		return nil, fmt.Errorf("load custody shipment %s: %w", id, err)
	}
	if art.ArtifactType != "shipment" {
		return nil, fmt.Errorf("custody target %s is not a shipment: %w", id, blerrors.ErrValidation)
	}
	return art, nil
}

func custodyEnvelopeOf(art *models.Artifact) (WorkflowCustodyEnvelope, bool, error) {
	raw, ok := art.CustomFields[custodyArtifactKey]
	if !ok || raw == nil {
		return WorkflowCustodyEnvelope{}, false, nil
	}
	data, err := json.Marshal(raw)
	if err != nil {
		return WorkflowCustodyEnvelope{}, false, fmt.Errorf("encode custody envelope: %w", err)
	}
	env, err := DecodeCustodyEnvelopeJSON(data)
	if err != nil {
		return WorkflowCustodyEnvelope{}, false, err
	}
	return env, true, nil
}

// custodyEnvelopeForOffer returns the shipment's envelope, or a new one bound to branch
// and commit when the shipment has no custody record yet.
func custodyEnvelopeForOffer(art *models.Artifact, shipmentID, branch, commit string) (WorkflowCustodyEnvelope, error) {
	env, present, err := custodyEnvelopeOf(art)
	if err != nil {
		return WorkflowCustodyEnvelope{}, err
	}
	if present {
		if env.ShipmentID != shipmentID {
			return WorkflowCustodyEnvelope{}, fmt.Errorf("custody envelope names shipment %s, not %s: %w", env.ShipmentID, shipmentID, blerrors.ErrValidation)
		}
		if env.ScopeBranch != branch {
			return WorkflowCustodyEnvelope{}, fmt.Errorf("%w: envelope scope %q, checked out %q", ErrCustodyWrongBranch, env.ScopeBranch, branch)
		}
		return env, nil
	}
	workflowID, _ := art.CustomFields["workflow_id"].(string)
	if workflowID == "" {
		if workflowID, err = custodyRandomID(); err != nil {
			return WorkflowCustodyEnvelope{}, err
		}
	}
	return WorkflowCustodyEnvelope{
		SchemaVersion: WorkflowCustodySchemaVersion,
		WorkflowID:    workflowID,
		ShipmentID:    shipmentID,
		ScopeBranch:   branch,
		BaseCommitSHA: commit,
		Receipts:      []CustodyReceipt{},
	}, nil
}

// custodyValidateOfferSpec checks the offered receipt against the envelope and the
// authority rules before any digest is compared against the live repository.
func custodyValidateOfferSpec(env WorkflowCustodyEnvelope, spec CustodyReceipt) error {
	if spec.ReceiptID == "" || spec.Status != custodyReceiptOffered {
		return fmt.Errorf("offer custody: receipt_id is required and status must be offered: %w", blerrors.ErrValidation)
	}
	if spec.Revision != env.CustodyRevision+1 {
		return fmt.Errorf("offer custody receipt %s: revision must be %d: %w", spec.ReceiptID, env.CustodyRevision+1, blerrors.ErrValidation)
	}
	if custodyHasReceipt(env.Receipts, spec.ReceiptID) {
		return fmt.Errorf("offer custody receipt %s: receipt already exists: %w", spec.ReceiptID, blerrors.ErrValidation)
	}
	specs := custodyCandidatesOf(spec.Entries)
	if spec.AuthorityProfile == custodyProfileAgent && custodyRequiredProfile(specs) == custodyProfileOperator {
		return fmt.Errorf("offer custody receipt %s: %w: entries include operator-contributed paths", spec.ReceiptID, ErrOperatorApprovalRequired)
	}
	digest, err := custodyCandidateSetDigest(specs)
	if err != nil {
		return err
	}
	if digest != spec.CandidateSetDigest {
		return fmt.Errorf("offer custody receipt %s: candidate_set_digest does not match its entries: %w", spec.ReceiptID, blerrors.ErrValidation)
	}
	return nil
}

func custodySetEnvelope(art *models.Artifact, env WorkflowCustodyEnvelope, branch string) error {
	raw, err := json.Marshal(env)
	if err != nil {
		return fmt.Errorf("encode custody envelope: %w", err)
	}
	var value map[string]any
	if err := json.Unmarshal(raw, &value); err != nil {
		return fmt.Errorf("encode custody envelope: %w", err)
	}
	if art.CustomFields == nil {
		art.CustomFields = map[string]any{}
	}
	art.CustomFields[custodyArtifactKey] = value
	art.CustomFields["workflow_id"] = env.WorkflowID
	art.CustomFields["branch"] = branch
	return nil
}

func custodyResult(shipmentID string, receipt CustodyReceipt, status string, revision int, dryRun bool) CustodyMutationResult {
	return CustodyMutationResult{
		ShipmentID:       shipmentID,
		ReceiptID:        receipt.ReceiptID,
		Status:           status,
		AuthorityProfile: receipt.AuthorityProfile,
		CustodyRevision:  revision,
		DryRun:           dryRun,
		Verdicts:         []CustodyPathVerdict{},
		Warnings:         []string{},
	}
}

func custodyReceiptIndex(receipts []CustodyReceipt, id string) int {
	for i := range receipts {
		if receipts[i].ReceiptID == id {
			return i
		}
	}
	return -1
}

func custodyReceiptDigest(receipt CustodyReceipt) (string, error) {
	raw, err := json.Marshal(receipt)
	if err != nil {
		return "", fmt.Errorf("encode custody receipt: %w", err)
	}
	return canonical.HashBytes(raw), nil
}

func custodyAnchors(ws *Workspace) []config.TrustAnchor {
	if ws == nil || ws.Config == nil {
		return nil
	}
	return ws.Config.TrustAnchors
}

func custodyRandomID() (string, error) {
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", fmt.Errorf("generate custody id: %w", err)
	}
	return hex.EncodeToString(buf[:]), nil
}
