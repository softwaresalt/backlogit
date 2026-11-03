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
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
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
	branch, err := custodyPreviewBranch(ctx, ws, opts.Branch)
	if err != nil {
		return CustodyPreviewResult{}, err
	}
	art, err := custodyShipment(ctx, ws, opts.ShipmentID)
	if err != nil {
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
	env, _, err := custodyEnvelopeOf(art)
	if err != nil {
		return CustodyPreviewResult{}, err
	}
	adopted := custodyAdoptedFunc(ws, env, custodyTrustFor(ws, nil, nil, time.Now().UTC()))
	profile := custodyRequiredProfileFor(custodyStorageRootName(ws), "", "", "", "", entries, adopted)
	if err := validateCustodyEntries(entries, profile); err != nil {
		return CustodyPreviewResult{}, fmt.Errorf("preview custody %s: %w", opts.ShipmentID, err)
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
// Profile B (agent_workflow_handoff) needs the accepting role and eligible paths. Profile A
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
	branch, err := custodyCurrentBranch(ctx, ws.RootPath)
	if err != nil {
		return CustodyVerifyResult{}, err
	}
	wrongBranch := opts.Branch != "" && opts.Branch != branch
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
	if env.ShipmentID != opts.ShipmentID {
		return CustodyVerifyResult{}, fmt.Errorf("verify custody %s: envelope names shipment %s: %w", opts.ShipmentID, env.ShipmentID, blerrors.ErrValidation)
	}
	if err := custodyCheckBranchField(art, env); err != nil {
		return CustodyVerifyResult{}, err
	}
	if err := ValidateCustodyEnvelope(env); err != nil {
		return CustodyVerifyResult{}, fmt.Errorf("verify custody %s: %w", opts.ShipmentID, err)
	}

	now := time.Now().UTC()
	trust := custodyTrustFor(ws, opts.TrustAnchors, opts.ExternalPins, now)
	adopted := custodyAdoptedFunc(ws, env, trust)
	for i := range env.Receipts {
		receipt := env.Receipts[i]
		if receipt.Status != custodyReceiptOffered && receipt.Status != custodyReceiptAccepted {
			continue
		}
		eligible := receipt.AuthorityProfile != custodyProfileAgent ||
			custodyRequiredProfileFor(custodyStorageRootName(ws), receipt.Kind, receipt.ProducerRole, receipt.AcceptingRole, receipt.AllowedAction, receipt.Entries, adopted) != custodyProfileOperator
		for _, entry := range receipt.Entries {
			if wrongBranch {
				result.Verdicts = append(result.Verdicts, CustodyPathVerdict{
					Path: entry.Path, OldPath: entry.OldPath, ReceiptID: receipt.ReceiptID, Verdict: "WRONG_BRANCH",
					Reason: fmt.Sprintf("requested branch %q is not the checked-out branch %q", opts.Branch, branch),
				})
				continue
			}
			result.Verdicts = append(result.Verdicts, custodyVerdict(ctx, ws, env, receipt, entry, branch, trust, eligible, now))
		}
	}
	return result, nil
}

// reconcileCustodyIntent recovers one pending custody journal. The recovery barrier
// already holds the shipment lifecycle lock (global), membership (A), and artifact (B)
// locks, so this acquires only the item-log lock (C). That makes recovery order B before C,
// the reverse of custodyWithLocks (C before B). This is safe because every custody writer
// takes the global lock first, so the global lock serialises recovery against them. An
// intent journal rolls back to its preimage. A committed journal is never rolled back:
// recovery only removes the journal.
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
		return nil, custodyRollback(itemCtx, ws, journal, journalPath)
	default:
		return nil, fmt.Errorf("custody journal %s has unsupported phase %q: %w", journalPath, journal.Phase, blerrors.ErrValidation)
	}
}

// isCustodyOperation reports whether operation is a custody journal operation.
func isCustodyOperation(operation string) bool {
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
	// Accept owns these fields, so an offer never stores caller-supplied values for them.
	spec.AcceptedAtUTC = time.Time{}
	spec.SupersedesReceiptID = ""
	spec.IdempotencyKey = ""
	spec.IdempotencyRequestDigest = ""
	spec.ApprovalProof = nil
	adopted := custodyAdoptedFunc(ws, env, custodyTrustFor(ws, nil, nil, time.Now().UTC()))
	if err := custodyValidateOfferSpec(env, spec, custodyStorageRootName(ws), adopted); err != nil {
		return CustodyMutationResult{}, err
	}
	if err := custodyVerifyEntriesLive(ctx, ws, env.BaseCommitSHA, spec.Entries, spec.AuthorityProfile); err != nil {
		return CustodyMutationResult{}, err
	}
	spec.OfferedAtUTC = time.Now().UTC()
	spec.ExpiresAtUTC = custodyOfferExpiresAt(spec.OfferedAtUTC)
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
	if err := custodySetEnvelope(next, env); err != nil {
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
	if env.ShipmentID != opts.ShipmentID {
		return CustodyMutationResult{}, fmt.Errorf("accept custody %s: envelope names shipment %s: %w", opts.ShipmentID, env.ShipmentID, blerrors.ErrValidation)
	}
	if env.ScopeBranch != branch {
		return CustodyMutationResult{}, fmt.Errorf("%w: envelope scope %q, checked out %q", ErrCustodyWrongBranch, env.ScopeBranch, branch)
	}
	idx := custodyReceiptIndex(env.Receipts, opts.ReceiptID)
	if idx < 0 {
		return CustodyMutationResult{}, fmt.Errorf("accept custody %s: receipt %q not found: %w", opts.ShipmentID, opts.ReceiptID, blerrors.ErrValidation)
	}
	receipt := env.Receipts[idx]
	requestDigest, err := custodyAcceptRequestDigest(opts)
	if err != nil {
		return CustodyMutationResult{}, err
	}
	if receipt.Status == custodyReceiptAccepted && opts.IdempotencyKey != "" && receipt.IdempotencyKey == opts.IdempotencyKey {
		if receipt.IdempotencyRequestDigest != requestDigest {
			return CustodyMutationResult{}, fmt.Errorf("accept custody receipt %s: %w", receipt.ReceiptID, ErrCustodyIdempotencyConflict)
		}
		return custodyResult(opts.ShipmentID, receipt, custodyReceiptAccepted, env.CustodyRevision, opts.DryRun), nil
	}
	if receipt.Status != custodyReceiptOffered {
		return CustodyMutationResult{}, fmt.Errorf("accept custody receipt %s: status is %q, not offered: %w", receipt.ReceiptID, receipt.Status, blerrors.ErrValidation)
	}
	if receipt.Revision != env.CustodyRevision+1 {
		return CustodyMutationResult{}, fmt.Errorf("accept custody receipt %s: revision %d is not the next custody revision %d: %w", receipt.ReceiptID, receipt.Revision, env.CustodyRevision+1, ErrCustodyStaleRevision)
	}
	if opts.By != receipt.AcceptingRole {
		return CustodyMutationResult{}, fmt.Errorf("accept custody receipt %s: principal %q is not accepting role %q: %w", receipt.ReceiptID, opts.By, receipt.AcceptingRole, blerrors.ErrValidation)
	}
	now := time.Now().UTC()
	if custodyReceiptExpired(receipt, now) {
		return CustodyMutationResult{}, fmt.Errorf("accept custody receipt %s: offer expired or has no expiry: %w", receipt.ReceiptID, ErrCustodyReceiptExpired)
	}
	if err := custodyVerifyEntriesLive(ctx, ws, env.BaseCommitSHA, receipt.Entries, receipt.AuthorityProfile); err != nil {
		return CustodyMutationResult{}, err
	}
	trust := custodyTrustFor(ws, opts.TrustAnchors, opts.ExternalPins, now)
	adopted := custodyAdoptedFunc(ws, env, trust)
	var proof *CustodyApprovalProof
	switch receipt.AuthorityProfile {
	case custodyProfileAgent:
		if custodyRequiredProfileFor(custodyStorageRootName(ws), receipt.Kind, receipt.ProducerRole, receipt.AcceptingRole, receipt.AllowedAction, receipt.Entries, adopted) == custodyProfileOperator {
			return CustodyMutationResult{}, fmt.Errorf("accept custody receipt %s: %w: entries require operator_verified", receipt.ReceiptID, ErrOperatorApprovalRequired)
		}
		if receipt.ProducerRole == receipt.AcceptingRole {
			return CustodyMutationResult{}, fmt.Errorf("accept custody receipt %s: producer and accepting role must differ: %w", receipt.ReceiptID, blerrors.ErrValidation)
		}
	case custodyProfileOperator:
		proof, err = custodyCheckOperatorApproval(ctx, env, receipt, opts, trust)
		if err != nil {
			return CustodyMutationResult{}, err
		}
	default:
		return CustodyMutationResult{}, fmt.Errorf("accept custody receipt %s: unknown authority profile %q: %w", receipt.ReceiptID, receipt.AuthorityProfile, blerrors.ErrValidation)
	}

	receipt.Status = custodyReceiptAccepted
	receipt.AcceptedAtUTC = now
	receipt.ExpiresAtUTC = DefaultCustodyReceiptExpiresAt(now)
	receipt.IdempotencyKey = opts.IdempotencyKey
	if opts.IdempotencyKey != "" {
		receipt.IdempotencyRequestDigest = requestDigest
	}
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
	if err := custodySetEnvelope(next, env); err != nil {
		return CustodyMutationResult{}, err
	}
	acceptDelta := map[string]any{
		"receipt_id":        receipt.ReceiptID,
		"revision":          receipt.Revision,
		"authority_profile": receipt.AuthorityProfile,
		"accepting_role":    receipt.AcceptingRole,
		"accepted_by":       opts.By,
		"accepted_at_utc":   now,
		"custody_revision":  receipt.Revision,
	}
	var adoptionDelta map[string]any
	acceptEvents := []custodyEvent{{eventType: EventCustodyHandoffAccepted, delta: acceptDelta}}
	if proof != nil {
		acceptDelta["key_id"] = proof.KeyID
		acceptDelta["nonce"] = proof.Nonce
		acceptDelta["request_identity_digest"] = proof.RequestIdentityDigest
		adoptionDelta = map[string]any{
			"receipt_id":              receipt.ReceiptID,
			"nonce":                   proof.Nonce,
			"request_identity_digest": proof.RequestIdentityDigest,
		}
		acceptEvents = append(acceptEvents, custodyEvent{eventType: EventCustodyAdoptionRecorded, delta: adoptionDelta})
	}
	// Every check that can fail has run. The nonce is consumed last, so only the commit
	// transaction can fail after consumption.
	if proof != nil {
		consumer := custodyNonceConsumerFrom(ctx)
		if consumer == nil {
			return CustodyMutationResult{}, fmt.Errorf("%w: %w", ErrAuthorizationRejected, ErrNonceProviderUnavailable)
		}
		if err := consumer.ConsumeBound(ctx, proof.Nonce, proof.NotAfterUTC, proof.RequestIdentityDigest); err != nil {
			return CustodyMutationResult{}, rejectApproval(fmt.Errorf("consume nonce: %w", err))
		}
		consumedAt, err := custodyConsumedAt(ws, proof.Nonce)
		if err != nil {
			return CustodyMutationResult{}, fmt.Errorf("accept custody receipt %s: operator approval nonce %q was consumed; the approval token must be re-minted: %w", receipt.ReceiptID, proof.Nonce, err)
		}
		adoptionDelta["consumed_at"] = consumedAt
	}
	if err := custodyCommitTransaction(ctx, ws, custodyCommitPlan{
		shipmentID: opts.ShipmentID,
		operation:  shipmentOperationCustodyAccept,
		target:     custodyReceiptAccepted,
		pre:        pre,
		next:       next,
		events:     acceptEvents,
	}); err != nil {
		if proof != nil {
			return CustodyMutationResult{}, fmt.Errorf("accept custody receipt %s: operator approval nonce %q was consumed; the approval token must be re-minted: %w", receipt.ReceiptID, proof.Nonce, err)
		}
		return CustodyMutationResult{}, err
	}
	return result, nil
}

// custodyCheckOperatorApproval verifies a Profile A approval token against the trust root
// and binds it to the receipt. It consumes nothing: a live accept consumes the nonce only
// after every other check has passed.
func custodyCheckOperatorApproval(ctx context.Context, env WorkflowCustodyEnvelope, receipt CustodyReceipt, opts CustodyAcceptOptions, trust custodyTrust) (*CustodyApprovalProof, error) {
	if opts.AuthToken == "" {
		return nil, fmt.Errorf("%w: operator_verified receipt %s requires a custody approval token", ErrAuthorizationRequired, receipt.ReceiptID)
	}
	if trust.pins == nil && len(trust.anchors) == 0 {
		return nil, fmt.Errorf("%w: no operator trust anchor is configured", ErrApprovalChannelUnavailable)
	}
	proof, err := trust.verifyToken(opts.AuthToken, env)
	if err != nil {
		return nil, err
	}
	if proof.CandidateSetDigest != receipt.CandidateSetDigest || proof.AllowedAction != receipt.AllowedAction || proof.TargetRevision != receipt.Revision {
		return nil, fmt.Errorf("%w: approval token does not bind receipt %s", ErrAuthorizationRejected, receipt.ReceiptID)
	}
	if proof.ShipmentID != opts.ShipmentID {
		return nil, fmt.Errorf("%w: %w", ErrAuthorizationRejected, ErrApprovalWrongShipment)
	}
	if proof.ReceiptID != "" && proof.ReceiptID != receipt.ReceiptID {
		return nil, fmt.Errorf("%w: approval token names receipt %s, not %s", ErrAuthorizationRejected, proof.ReceiptID, receipt.ReceiptID)
	}
	if proof.AcceptingRole != "" && proof.AcceptingRole != receipt.AcceptingRole {
		return nil, fmt.Errorf("%w: approval token names accepting role %q, not %q", ErrAuthorizationRejected, proof.AcceptingRole, receipt.AcceptingRole)
	}
	if !opts.DryRun && custodyNonceConsumerFrom(ctx) == nil {
		return nil, fmt.Errorf("%w: %w", ErrAuthorizationRejected, ErrNonceProviderUnavailable)
	}
	return &proof, nil
}

// custodyVerifyOperatorAdoption verifies an accepted operator receipt. Its approval proof
// must bind this shipment, receipt, and accepting role, authenticate against the trust
// root, match the current request identity, and have a matching redemption.
func custodyVerifyOperatorAdoption(ws *Workspace, env WorkflowCustodyEnvelope, receipt CustodyReceipt, trust custodyTrust) error {
	if receipt.ApprovalProof == nil {
		return fmt.Errorf("%w: accepted operator receipt has no approval proof", ErrOperatorApprovalRequired)
	}
	proof := *receipt.ApprovalProof
	if proof.ShipmentID != env.ShipmentID {
		return fmt.Errorf("%w: %w", ErrAuthorizationRejected, ErrApprovalWrongShipment)
	}
	if proof.ReceiptID != "" && proof.ReceiptID != receipt.ReceiptID {
		return fmt.Errorf("%w: proof names receipt %s, not %s", ErrAuthorizationRejected, proof.ReceiptID, receipt.ReceiptID)
	}
	if proof.AcceptingRole != "" && proof.AcceptingRole != receipt.AcceptingRole {
		return fmt.Errorf("%w: proof names accepting role %q, not %q", ErrAuthorizationRejected, proof.AcceptingRole, receipt.AcceptingRole)
	}
	digest, err := custodyReceiptIdentityDigest(CustodyRequestIdentityDomain, env, receipt)
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
	pub, err := trust.keyResolver()(proof.KeyID)
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
	if !custodyRedeemed(ws, env.ShipmentID, receipt.ReceiptID, proof) {
		return fmt.Errorf("%w: no redemption of nonce for request identity within the approval token window in the NonceLedger or shipment item log", ErrAuthorizationRejected)
	}
	return nil
}

// custodyRedeemed reports whether proof's nonce was consumed against its request identity at an
// instant inside proof's [not_before, not_after] window. The NonceLedger projection is checked
// first, then the shipment item log. A ledger record without consumed_at does not prove
// redemption, and a ledger record consumed outside the window fails closed without consulting
// the log. A missing or unreadable source counts as no redemption.
func custodyRedeemed(ws *Workspace, shipmentID, receiptID string, proof CustodyApprovalProof) bool {
	if ledger, err := OpenNonceLedger(custodyNonceLedgerPath(ws.RootPath)); err == nil {
		digest, consumedAt, found, err := ledger.BoundConsumption(proof.Nonce)
		if err == nil && found && digest == proof.RequestIdentityDigest && !consumedAt.IsZero() {
			return custodyWithinTokenWindow(consumedAt, proof)
		}
	}
	consumedAt, ok := custodyLogRedemption(ws.RootPath, shipmentID, receiptID, proof)
	return ok && custodyWithinTokenWindow(consumedAt, proof)
}

// custodyWithinTokenWindow reports whether consumedAt lies inside proof's validity window.
func custodyWithinTokenWindow(consumedAt time.Time, proof CustodyApprovalProof) bool {
	return !consumedAt.Before(proof.NotBeforeUTC) && !consumedAt.After(proof.NotAfterUTC)
}

// custodyConsumedAt returns the consume time the NonceLedger recorded for nonce, which the
// accept path carries into its adoption event. It fails closed when that time is missing.
func custodyConsumedAt(ws *Workspace, nonce string) (time.Time, error) {
	ledger, err := OpenNonceLedger(custodyNonceLedgerPath(ws.RootPath))
	if err != nil {
		return time.Time{}, err
	}
	_, consumedAt, found, err := ledger.BoundConsumption(nonce)
	if err != nil {
		return time.Time{}, err
	}
	if !found || consumedAt.IsZero() {
		return time.Time{}, fmt.Errorf("nonce ledger has no consumed_at for the consumed nonce: %w", ErrNonceLedgerCorrupt)
	}
	return consumedAt, nil
}

// custodyLogRedemption returns the consume time recorded on the shipment item log's accept or
// adoption event for receiptID with proof's nonce and request identity digest. Only an event that
// carries a parseable consumed_at proves redemption.
func custodyLogRedemption(root, shipmentID, receiptID string, proof CustodyApprovalProof) (time.Time, bool) {
	f, err := openCustodyNoFollow(events.LogPathForItem(WorkspaceLogsRoot(root), shipmentID))
	if err != nil {
		return time.Time{}, false
	}
	defer func() { _ = f.Close() }()
	data, err := custodyReadBounded(f, custodyMaxReadBytes)
	if err != nil {
		return time.Time{}, false
	}
	for _, line := range bytes.Split(data, []byte("\n")) {
		var ev events.Event
		if json.Unmarshal(line, &ev) != nil {
			continue
		}
		if ev.EventType != EventCustodyHandoffAccepted && ev.EventType != EventCustodyAdoptionRecorded {
			continue
		}
		if ev.Delta["receipt_id"] != receiptID || ev.Delta["nonce"] != proof.Nonce ||
			ev.Delta["request_identity_digest"] != proof.RequestIdentityDigest {
			continue
		}
		raw, _ := ev.Delta["consumed_at"].(string)
		consumedAt, err := time.Parse(time.RFC3339Nano, raw)
		if err != nil {
			continue
		}
		return consumedAt, true
	}
	return time.Time{}, false
}

func custodyNonceLedgerPath(root string) string {
	return filepath.Join(WorkspaceStorageRoot(root), "nonce-ledger.json")
}

// custodyAcceptRequestDigest pins an accept request, so an idempotent retry replays the
// original result only for an identical request. The token is digested, never stored.
func custodyAcceptRequestDigest(opts CustodyAcceptOptions) (string, error) {
	payload, err := json.Marshal(struct {
		ShipmentID  string `json:"shipment_id"`
		ReceiptID   string `json:"receipt_id"`
		By          string `json:"by"`
		TokenDigest string `json:"auth_token_digest"`
	}{opts.ShipmentID, opts.ReceiptID, opts.By, canonical.HashBytes([]byte(opts.AuthToken))})
	if err != nil {
		return "", fmt.Errorf("encode accept request: %w", err)
	}
	return canonical.HashBytes(payload), nil
}

// custodyReceiptExpired reports whether receipt is past its expiry at now. A receipt with
// no expiry is treated as expired, so it fails closed.
func custodyReceiptExpired(receipt CustodyReceipt, now time.Time) bool {
	return receipt.ExpiresAtUTC.IsZero() || !now.Before(receipt.ExpiresAtUTC)
}

func custodyOfferExpiresAt(offeredAt time.Time) time.Time {
	return offeredAt.Add(DefaultCustodyOfferExpiry)
}

// custodyVerdict classifies one covered path of an offered or accepted receipt. eligible
// reports whether an agent receipt's entries satisfy the deterministic profile rules.
func custodyVerdict(ctx context.Context, ws *Workspace, env WorkflowCustodyEnvelope, receipt CustodyReceipt, entry CustodyEntry, branch string, trust custodyTrust, eligible bool, now time.Time) CustodyPathVerdict {
	verdict := CustodyPathVerdict{Path: entry.Path, OldPath: entry.OldPath, ReceiptID: receipt.ReceiptID}
	switch {
	case env.ScopeBranch != branch:
		verdict.Verdict = "WRONG_BRANCH"
		verdict.Reason = fmt.Sprintf("receipt scope_branch %q does not match %q", env.ScopeBranch, branch)
	case receipt.Status == custodyReceiptOffered:
		verdict.Verdict = "UNCOVERED"
		verdict.Reason = "offered receipt is not accepted"
	default:
		verdict = custodyAcceptedVerdict(ctx, ws, env, receipt, entry, trust, eligible, now, verdict)
	}
	return verdict
}

func custodyAcceptedVerdict(ctx context.Context, ws *Workspace, env WorkflowCustodyEnvelope, receipt CustodyReceipt, entry CustodyEntry, trust custodyTrust, eligible bool, now time.Time, verdict CustodyPathVerdict) CustodyPathVerdict {
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
		if err := custodyVerifyOperatorAdoption(ws, env, receipt, trust); err != nil {
			verdict.Verdict = "UNVERIFIED_OPERATOR_PROVENANCE"
			verdict.Reason = err.Error()
			return verdict
		}
	} else if !eligible {
		verdict.Verdict = "PROFILE_INELIGIBLE"
		verdict.Reason = "entries require operator_verified; " + ErrOperatorApprovalRequired.Error()
		return verdict
	}
	if custodyReceiptExpired(receipt, now) {
		verdict.Verdict = "RECEIPT_EXPIRED"
		verdict.Reason = "accepted receipt is past expires_at_utc or has no expiry"
		return verdict
	}
	if receipt.AuthorityProfile == custodyProfileOperator {
		verdict.Verdict = "ADOPTED_OPERATOR_VERIFIED"
		return verdict
	}
	verdict.Verdict = "HANDOFF_AGENT_VERIFIED"
	return verdict
}

// custodyWithLocks runs fn under the canonical lock order global -> A -> C -> B,
// where A is shipment membership, C is the shipment item log, and B is the artifact.
// Each lock is released by defer, so a panicking fn still releases every lock.
func custodyWithLocks(ctx context.Context, ws *Workspace, shipmentID string, fn func(context.Context) error) (err error) {
	lockedCtx, unlockGlobal, err := lockShipmentLifecycleGlobal(ctx, ws)
	if err != nil {
		return fmt.Errorf("lock shipment lifecycle for custody: %w", err)
	}
	defer func() { err = errors.Join(err, unlockGlobal()) }()
	unlockMembership, err := lockShipmentMembership(lockedCtx, ws, shipmentID)
	if err != nil {
		return fmt.Errorf("lock shipment membership for custody: %w", err)
	}
	defer func() { err = errors.Join(err, unlockMembership()) }()
	itemCtx, unlockItemLog, err := events.LockItemLogCrossProcess(lockedCtx, WorkspaceLocksRoot(ws.RootPath), WorkspaceLogsRoot(ws.RootPath), shipmentID)
	if err != nil {
		return fmt.Errorf("lock shipment item log for custody: %w", err)
	}
	defer unlockItemLog()
	artifactCtx, unlockArtifacts, err := lockArtifactMutations(itemCtx, ws, []string{shipmentID})
	if err != nil {
		return fmt.Errorf("lock shipment artifact for custody: %w", err)
	}
	defer func() { err = errors.Join(err, unlockArtifacts()) }()
	return fn(artifactCtx)
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
		return errors.Join(err, custodyRollback(opCtx, ws, journal, journalPath))
	}
	journal.Phase = shipmentOperationPhaseCommitted
	if _, err := writeShipmentLifecycleJournalForWorkspace(ws, name, journal); err != nil {
		commitErr := fmt.Errorf("record committed custody journal %s: %w", correlationID, err)
		if errors.Is(err, blerrors.ErrWriteIndeterminate) {
			return commitErr
		}
		journal.Phase = shipmentOperationPhaseIntent
		return errors.Join(commitErr, custodyRollback(opCtx, ws, journal, journalPath))
	}
	removeShipmentOperationJournal(ctx, ws, journalPath)
	return nil
}

func custodyApply(ctx context.Context, ws *Workspace, plan custodyCommitPlan) error {
	if err := custodyWriteArtifact(ctx, ws, plan.next, true); err != nil {
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
// the item log, and finishes the journal. A failed restore leaves the intent journal in
// place, so the next recovery barrier retries the rollback.
func custodyRollback(ctx context.Context, ws *Workspace, journal shipmentLifecycleJournal, journalPath string) error {
	pre := journal.Preimage.Shipment
	if err := custodyWriteArtifact(ctx, ws, pre, false); err != nil {
		return fmt.Errorf("restore custody preimage %s: %w", pre.ID, err)
	}
	if err := custodyDropOperationEvents(ws, pre.ID, journal.CorrelationID); err != nil {
		return fmt.Errorf("restore custody event log %s: %w", pre.ID, err)
	}
	return custodyFinishJournal(ws, journal, journalPath)
}

// custodyFinishJournal removes a finished custody journal. When removal fails, the journal
// is rewritten to the terminal compensated phase, so a later recovery barrier cannot
// replay the stale intent. The removal failure is returned in either case, so recovery
// fails loudly rather than reporting a clean rollback.
func custodyFinishJournal(ws *Workspace, journal shipmentLifecycleJournal, journalPath string) error {
	removeErr := custodyRemoveJournal(ws, journalPath)
	if removeErr == nil {
		return nil
	}
	journal.Phase = "compensated"
	if _, err := writeShipmentLifecycleJournalForWorkspace(ws, filepath.Base(journalPath), journal); err != nil {
		return errors.Join(fmt.Errorf("remove custody journal %s: %w", journalPath, removeErr),
			fmt.Errorf("record compensated custody journal: %w", err))
	}
	return fmt.Errorf("remove custody journal %s after compensation: %w", journalPath, removeErr)
}

// custodyRemoveJournal removes the journal through the workspace seam when one is set.
func custodyRemoveJournal(ws *Workspace, journalPath string) error {
	remove := os.Remove
	if ws.removeShipmentOperationJournal != nil {
		remove = ws.removeShipmentOperationJournal
	}
	if err := remove(journalPath); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}

// custodyReadFileNoFollow reads path without following a symlink or reparse point at its final
// component. A missing file reports fs.ErrNotExist.
func custodyReadFileNoFollow(path string) ([]byte, error) {
	f, err := openCustodyNoFollow(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	return io.ReadAll(f)
}

// custodyDropOperationEvents removes the item-log lines tagged with correlationID and
// rewrites the log atomically. A log with no tagged lines is left untouched.
func custodyDropOperationEvents(ws *Workspace, shipmentID, correlationID string) error {
	logPath := events.LogPathForItem(WorkspaceLogsRoot(ws.RootPath), shipmentID)
	data, err := custodyReadFileNoFollow(logPath)
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
// mergeLinks carries SQLite-only links into the Markdown first. A rollback passes false, so
// the restored file matches the journaled preimage byte for byte.
func custodyWriteArtifact(ctx context.Context, ws *Workspace, art *models.Artifact, mergeLinks bool) error {
	path, err := FindArtifactPath(ctx, ws, art.ID)
	if err != nil {
		return fmt.Errorf("locate custody shipment %s: %w", art.ID, err)
	}
	if mergeLinks && ws.DB != nil {
		if err := mergeDBOnlyLinksIntoArtifact(ctx, ws, art); err != nil {
			return fmt.Errorf("preserve database-only links on custody shipment %s: %w", art.ID, err)
		}
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

// custodyPreviewBranch returns the checked-out branch. A requested branch is refused when it
// is the default branch, and it must equal the checked-out branch, so a preview never reads
// a branch other than the one it is evaluated against.
func custodyPreviewBranch(ctx context.Context, ws *Workspace, requested string) (string, error) {
	checked, err := custodyCurrentBranch(ctx, ws.RootPath)
	if err != nil {
		return "", err
	}
	if requested != "" {
		if err := custodyRefuseDefaultBranch(ctx, ws.RootPath, requested); err != nil {
			return "", err
		}
		if requested != checked {
			return "", fmt.Errorf("WRONG_BRANCH: requested branch %q is not the checked-out branch %q: %w", requested, checked, ErrCustodyWrongBranch)
		}
	}
	if err := custodyRefuseDefaultBranch(ctx, ws.RootPath, checked); err != nil {
		return "", err
	}
	return checked, nil
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
	if err := custodyRefuseDefaultBranch(ctx, ws.RootPath, branch); err != nil {
		return "", "", err
	}
	commit, err := custodyHeadCommit(ctx, ws.RootPath)
	if err != nil {
		return "", "", err
	}
	return branch, commit, nil
}

// custodyRefuseDefaultBranch refuses the default branch and the literal main, master, develop,
// and trunk, whatever origin/HEAD names. The default branch is origin/HEAD with its origin/
// prefix removed, so origin/develop refuses develop. When origin/HEAD does not resolve, the
// default is main.
func custodyRefuseDefaultBranch(ctx context.Context, root, branch string) error {
	defaultBranch := custodyDefaultBranch(ctx, root)
	for _, forbidden := range []string{defaultBranch, "main", "master", "develop", "trunk"} {
		if strings.EqualFold(branch, forbidden) {
			return fmt.Errorf("%w: %q", ErrCustodyDefaultBranchForbidden, branch)
		}
	}
	return nil
}

func custodyDefaultBranch(ctx context.Context, root string) string {
	out, err := custodyGit(ctx, root, "symbolic-ref", "--short", "refs/remotes/origin/HEAD")
	if err != nil {
		return "main"
	}
	name := strings.TrimPrefix(strings.TrimSpace(string(out)), "origin/")
	if name == "" {
		return "main"
	}
	return name
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
// refreshes the index. Stdout and stderr are each capped at custodyMaxReadBytes.
func custodyGit(ctx context.Context, root string, args ...string) ([]byte, error) {
	full := append([]string{"-C", root, "--no-replace-objects", "--no-optional-locks"}, args...)
	cmd := exec.CommandContext(ctx, "git", full...)
	cmd.Env = config.ChildProcessEnv()
	var stdout, stderr custodyCappedBuffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("git %s: %w: %s", args[0], err, strings.TrimSpace(stderr.String()))
	}
	return stdout.Bytes(), nil
}

// custodyCappedBuffer collects process output up to custodyMaxReadBytes and fails any
// write that would exceed it, so an oversized git stream cannot exhaust memory.
type custodyCappedBuffer struct {
	buf bytes.Buffer
}

func (b *custodyCappedBuffer) Write(p []byte) (int, error) {
	if b.buf.Len()+len(p) > custodyMaxReadBytes {
		return 0, fmt.Errorf("%w: git output exceeds %d bytes", ErrCustodyOutputTooLarge, custodyMaxReadBytes)
	}
	return b.buf.Write(p)
}

func (b *custodyCappedBuffer) Bytes() []byte { return b.buf.Bytes() }

func (b *custodyCappedBuffer) String() string { return b.buf.String() }

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
// independent of their order. Candidates are ordered by their full canonical encoding, so
// specs that tie on path still hash in one fixed order.
func custodyCandidateSetDigest(specs []CustodyCandidateSpec) (string, error) {
	type keyedSpec struct {
		key  []byte
		spec CustodyCandidateSpec
	}
	keyed := make([]keyedSpec, 0, len(specs))
	for _, spec := range specs {
		key, err := json.Marshal(spec)
		if err != nil {
			return "", fmt.Errorf("encode candidate set: %w", err)
		}
		keyed = append(keyed, keyedSpec{key: key, spec: spec})
	}
	sort.Slice(keyed, func(i, j int) bool { return bytes.Compare(keyed[i].key, keyed[j].key) < 0 })
	sorted := make([]CustodyCandidateSpec, 0, len(keyed))
	for _, k := range keyed {
		sorted = append(sorted, k.spec)
	}
	payload, err := json.Marshal(sorted)
	if err != nil {
		return "", fmt.Errorf("encode candidate set: %w", err)
	}
	return canonical.HashBytes(append([]byte(custodyCandidateSetDomain+"\n"), payload...)), nil
}

// custodyRequiredProfile is custodyRequiredProfileFor with no workspace storage root, so
// <storage_root>/queue and <storage_root>/archive paths are never Profile B.
func custodyRequiredProfile(kind, producerRole, acceptingRole, allowedAction string, entries []CustodyEntry, adopted func(*CustodyEntry) bool) string {
	return custodyRequiredProfileFor("", kind, producerRole, acceptingRole, allowedAction, entries, adopted)
}

// custodyRequiredProfileFor returns the authority profile the covered entries require.
// Profile A is mandatory for the operator triggers and for every kind except
// stage_to_ship_handoff. Profile B is permitted only for stage_to_ship_handoff, and only when
// every entry path, and any old path, is on the handoff allow-list for storageRoot, the base
// name of the workspace storage root. The caller-declared contributor and source labels alone
// never grant Profile B. An empty kind is a preview with no receipt yet, so only the entry rules
// apply. Baseline corroboration of modified docs/ files is not checked here (211-S Decision 4b
// is pending), so the adopted callback is not consulted.
func custodyRequiredProfileFor(storageRoot, kind, producerRole, acceptingRole, allowedAction string, entries []CustodyEntry, _ func(*CustodyEntry) bool) string {
	if kind == "preexisting_adoption" || allowedAction == "preexisting_adoption_commit" ||
		(producerRole == "operator" && acceptingRole == "operator") {
		return custodyProfileOperator
	}
	if kind != "" && kind != "stage_to_ship_handoff" {
		return custodyProfileOperator
	}
	for i := range entries {
		e := &entries[i]
		if e.Contributor == custodyOperatorContributor || e.Contributor == custodyContributorMixed ||
			e.SourceKind == custodySourceOperatorConfig || e.SourceKind == custodySourceOperatorCodeDoc ||
			e.StagedState == custodyStagedDivergesOK {
			return custodyProfileOperator
		}
	}
	for i := range entries {
		e := &entries[i]
		if !custodyPathInHandoffScope(storageRoot, e.Path) {
			return custodyProfileOperator
		}
		if e.OldPath != "" && !custodyPathInHandoffScope(storageRoot, e.OldPath) {
			return custodyProfileOperator
		}
	}
	return custodyProfileAgent
}

// custodyHandoffDocDirs are the docs/ directories a Stage-to-Ship handoff may cover.
var custodyHandoffDocDirs = []string{"docs/decisions/", "docs/plans/", "docs/exec-plans/", "docs/reviews/", "docs/memory/"}

// custodyPathInHandoffScope reports whether p is a valid repository-relative path on the
// Profile B allow-list: <storageRoot>/queue/** and <storageRoot>/archive/**, or the docs/
// directories in custodyHandoffDocDirs. An empty storageRoot admits no storage paths. Matching
// is case-sensitive.
func custodyPathInHandoffScope(storageRoot, p string) bool {
	if _, err := validateCustodyRelPath(p); err != nil {
		return false
	}
	if storageRoot != "" && (strings.HasPrefix(p, storageRoot+"/queue/") || strings.HasPrefix(p, storageRoot+"/archive/")) {
		return true
	}
	for _, dir := range custodyHandoffDocDirs {
		if strings.HasPrefix(p, dir) {
			return true
		}
	}
	return false
}

// custodyStorageRootName returns the base name of the workspace storage root, which scopes the
// Profile B queue and archive paths.
func custodyStorageRootName(ws *Workspace) string {
	return filepath.Base(workspaceStorageRoot(ws))
}

// custodyAdoptedFunc reports whether an entry was adopted at its exact baseline digest by an
// earlier, fully verified, unexpired Profile A receipt in env. A path with no baseline blob is
// never treated as adopted, because it has no baseline digest to match.
func custodyAdoptedFunc(ws *Workspace, env WorkflowCustodyEnvelope, trust custodyTrust) func(*CustodyEntry) bool {
	return func(entry *CustodyEntry) bool {
		if entry.BaseBlobSHA256 == "" {
			return false
		}
		now := time.Now().UTC()
		for i := range env.Receipts {
			prior := env.Receipts[i]
			if prior.AuthorityProfile != custodyProfileOperator || prior.Status != custodyReceiptAccepted ||
				custodyReceiptExpired(prior, now) {
				continue
			}
			if custodyVerifyOperatorAdoption(ws, env, prior, trust) != nil {
				continue
			}
			for j := range prior.Entries {
				if prior.Entries[j].Path == entry.Path && prior.Entries[j].BaseBlobSHA256 == entry.BaseBlobSHA256 {
					return true
				}
			}
		}
		return false
	}
}

// custodyEnvelopeNames reports whether art carries a workflow-custody envelope bound to
// shipmentID. An envelope that does not decode is not a custody record, so it does not match.
func custodyEnvelopeNames(art *models.Artifact, shipmentID string) bool {
	env, present, err := custodyEnvelopeOf(art)
	return err == nil && present && env.ShipmentID == shipmentID
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
func custodyValidateOfferSpec(env WorkflowCustodyEnvelope, spec CustodyReceipt, storageRoot string, adopted func(*CustodyEntry) bool) error {
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
	if spec.AuthorityProfile == custodyProfileAgent &&
		custodyRequiredProfileFor(storageRoot, spec.Kind, spec.ProducerRole, spec.AcceptingRole, spec.AllowedAction, spec.Entries, adopted) == custodyProfileOperator {
		return fmt.Errorf("offer custody receipt %s: %w: entries require operator_verified", spec.ReceiptID, ErrOperatorApprovalRequired)
	}
	for i := range env.Receipts {
		if spec.Revision <= env.Receipts[i].Revision {
			return fmt.Errorf("offer custody receipt %s: revision %d is not greater than receipt %s at revision %d: %w",
				spec.ReceiptID, spec.Revision, env.Receipts[i].ReceiptID, env.Receipts[i].Revision, ErrCustodyStaleRevision)
		}
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

func custodySetEnvelope(art *models.Artifact, env WorkflowCustodyEnvelope) error {
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
	art.CustomFields["branch"] = env.ScopeBranch
	return nil
}

// custodyCheckBranchField fails closed when the shipment's top-level branch field disagrees
// with the envelope's scope_branch, which is authoritative. A missing field is not a
// disagreement. A blocked-envelope recovery can overwrite the field, so verify reports it.
func custodyCheckBranchField(art *models.Artifact, env WorkflowCustodyEnvelope) error {
	raw, present := art.CustomFields["branch"]
	if !present || raw == env.ScopeBranch {
		return nil
	}
	return fmt.Errorf("verify custody %s: branch field %v disagrees with scope_branch %q: %w", env.ShipmentID, raw, env.ScopeBranch, ErrCustodyWrongBranch)
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

// custodyTrust is the operator trust root for one custody call. Caller-supplied anchors
// and external pins are authoritative and replace workspace config. Otherwise only the
// workspace anchors that pass custodyWorkspaceOperatorAnchors are trusted.
type custodyTrust struct {
	anchors []config.TrustAnchor
	pins    *ExternalPinSet
	now     time.Time
}

func custodyTrustFor(ws *Workspace, anchors []config.TrustAnchor, pins *ExternalPinSet, now time.Time) custodyTrust {
	if len(anchors) > 0 || pins != nil {
		return custodyTrust{anchors: anchors, pins: pins, now: now}
	}
	return custodyTrust{anchors: custodyWorkspaceOperatorAnchors(custodyAnchors(ws), now), now: now}
}

// keyResolver returns the operator key resolver for this trust root. When external pins are
// set, the anchor must be pinned, bound, and in window, as ResolveVerificationKey enforces.
func (t custodyTrust) keyResolver() func(keyID string) (ed25519.PublicKey, error) {
	if t.pins != nil {
		return func(keyID string) (ed25519.PublicKey, error) {
			resolved, err := ResolveVerificationKey(t.anchors, KeySelector{ID: keyID, Purpose: "operator"}, t.now, t.pins)
			if err != nil {
				return nil, fmt.Errorf("%w: %w", ErrAuthorizationRejected, err)
			}
			if resolved.Algo != config.TrustAnchorAlgoEd25519 {
				return nil, fmt.Errorf("%w: trust anchor %q algo is not ed25519", ErrAuthorizationRejected, keyID)
			}
			return custodyAnchorPublicKey(resolved.Anchor)
		}
	}
	return func(keyID string) (ed25519.PublicKey, error) {
		return resolveCustodyOperatorAnchor(keyID, t.now, t.anchors)
	}
}

// verifyToken authenticates and claim-checks an approval token under this trust root. With
// external pins, the anchor named by the token key_id is resolved through the pin set first,
// and only that resolved anchor is passed to VerifyCustodyApprovalToken.
func (t custodyTrust) verifyToken(token string, env WorkflowCustodyEnvelope) (CustodyApprovalProof, error) {
	if t.pins == nil {
		return VerifyCustodyApprovalToken(token, env, t.now, t.anchors...)
	}
	payload, _, err := splitCustodyApprovalToken(token)
	if err != nil {
		return CustodyApprovalProof{}, err
	}
	claimed, err := decodeCustodyApprovalPayload(payload)
	if err != nil {
		return CustodyApprovalProof{}, err
	}
	resolved, err := ResolveVerificationKey(t.anchors, KeySelector{ID: claimed.KeyID, Purpose: "operator"}, t.now, t.pins)
	if err != nil {
		return CustodyApprovalProof{}, fmt.Errorf("%w: %w", ErrAuthorizationRejected, err)
	}
	return VerifyCustodyApprovalToken(token, env, t.now, resolved.Anchor)
}

func custodyRandomID() (string, error) {
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", fmt.Errorf("generate custody id: %w", err)
	}
	return hex.EncodeToString(buf[:]), nil
}
