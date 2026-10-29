package core

import (
	"context"
	"crypto/ed25519"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/softwaresalt/backlogit/internal/config"
)

// Sentinel errors for the trust-anchor mutation core (168.005-T).
var (
	// ErrTrustAnchorMutationInvalid reports a malformed or inapplicable mutation request.
	ErrTrustAnchorMutationInvalid = errors.New("trust anchor mutation is invalid")
	// ErrApprovalRequired reports a rotate/revoke presented without operator-approval evidence.
	ErrApprovalRequired = errors.New("operator approval evidence required")
	// ErrApprovalInvalid reports approval evidence that failed verification:
	// forged signature, wrong action or anchor, expired, or missing nonce.
	ErrApprovalInvalid = errors.New("operator approval evidence invalid")
)

// TrustAnchorAction is a trust-anchor mutation kind.
type TrustAnchorAction string

// Trust-anchor mutation kinds.
const (
	TrustAnchorActionAdd    TrustAnchorAction = "add"
	TrustAnchorActionRotate TrustAnchorAction = "rotate"
	TrustAnchorActionRevoke TrustAnchorAction = "revoke"
)

// TrustAnchorMutationError is the typed error returned by the mutation core.
type TrustAnchorMutationError struct {
	Action   TrustAnchorAction
	AnchorID string
	Err      error
}

// Error implements error.
func (e *TrustAnchorMutationError) Error() string {
	return fmt.Sprintf("trust anchor %s %q: %v", e.Action, e.AnchorID, e.Err)
}

// Unwrap returns the underlying error.
func (e *TrustAnchorMutationError) Unwrap() error { return e.Err }

// ApprovalEvidence is verifiable operator-approval evidence: a signed token
// binding one action on one anchor id to an expiry and a nonce. A bare
// caller-supplied boolean is never accepted as approval.
type ApprovalEvidence struct {
	Action    TrustAnchorAction
	AnchorID  string
	Approver  string
	Expiry    time.Time
	Nonce     string
	Signature []byte
	// Replacement is the replacement anchor a rotate approval binds. It is
	// required for rotate and covered by the signature.
	Replacement *config.TrustAnchor
}

// MaxApprovalLifetime bounds how far beyond Now an approval expiry may lie.
const MaxApprovalLifetime = 24 * time.Hour

// SigningPayload returns the canonical bytes covered by Signature. For rotate
// it includes the replacement anchor (id, fingerprint, role, algo, not_before,
// not_after) so the approval binds the key being introduced.
func (e ApprovalEvidence) SigningPayload() []byte {
	repl := "-"
	if e.Replacement != nil {
		repl = replacementBinding(*e.Replacement)
	}
	return []byte("backlogit-trust-anchor-approval-v2\n" + string(e.Action) + "\n" + e.AnchorID + "\n" +
		e.Approver + "\n" + e.Expiry.UTC().Format(time.RFC3339Nano) + "\n" + e.Nonce + "\n" + repl)
}

func replacementBinding(a config.TrustAnchor) string {
	ts := func(t *time.Time) string {
		if t == nil {
			return "none"
		}
		return t.UTC().Format(time.RFC3339Nano)
	}
	return strings.Join([]string{a.ID, a.Fingerprint, a.Role, a.Algo, ts(a.NotBefore), ts(a.NotAfter)}, "|")
}

// ApproverVerifier verifies the signature of approval evidence against an
// operator-held approver key. It is injected so the core never trusts a
// caller-supplied flag.
type ApproverVerifier interface {
	VerifyApproval(ev ApprovalEvidence) error
}

// Ed25519ApproverVerifier verifies approval evidence signed with Ed25519.
type Ed25519ApproverVerifier struct {
	key ed25519.PublicKey
}

// NewEd25519ApproverVerifier returns a verifier for the given approver public key.
func NewEd25519ApproverVerifier(key ed25519.PublicKey) *Ed25519ApproverVerifier {
	return &Ed25519ApproverVerifier{key: key}
}

// VerifyApproval implements ApproverVerifier.
func (v *Ed25519ApproverVerifier) VerifyApproval(ev ApprovalEvidence) error {
	if v == nil || len(v.key) != ed25519.PublicKeySize || len(ev.Signature) != ed25519.SignatureSize ||
		!ed25519.Verify(v.key, ev.SigningPayload(), ev.Signature) {
		return ErrApprovalInvalid
	}
	return nil
}

// WriteOutcome is the base result of a mutation: which ordered steps took
// effect. Write-outcome classification (not-applied vs indeterminate, markers)
// is layered on this type by 168.017-T.
type WriteOutcome struct {
	ConfigWritten  bool
	StatusWritten  bool
	AuditPersisted bool
	// RevocationRecorded is true when the external revocation authority
	// recorded the tombstone (168.009-T).
	RevocationRecorded bool
	// MarkerOutstanding is true when the indeterminate marker was left in
	// place (168.017-T).
	MarkerOutstanding bool
}

// TrustAnchorStatusChange is one status mutation.
type TrustAnchorStatusChange struct {
	ID     string
	Status config.TrustAnchorStatus
}

// TrustAnchorState is the injected persistence for anchor config and status.
type TrustAnchorState interface {
	LoadAnchors(ctx context.Context) ([]config.TrustAnchor, error)
	WriteConfig(ctx context.Context, anchors []config.TrustAnchor) error
	WriteStatus(ctx context.Context, change TrustAnchorStatusChange) error
}

// Audit outcomes recorded by the mutation core.
const (
	TrustAnchorAuditApplied = "applied"
	TrustAnchorAuditDenied  = "denied"
)

// TrustAnchorAuditEvent is the durable audit record of a mutation attempt.
type TrustAnchorAuditEvent struct {
	Action      TrustAnchorAction
	AnchorID    string
	NewAnchorID string
	Fingerprint string
	Outcome     string
	Reason      string
	Approver    string
	At          time.Time
}

// TrustAnchorAuditor durably persists audit events.
type TrustAnchorAuditor interface {
	RecordAudit(ctx context.Context, ev TrustAnchorAuditEvent) error
}

// TrustAnchorMutation is one add/rotate/revoke request.
type TrustAnchorMutation struct {
	Action TrustAnchorAction
	// AnchorID is the anchor to rotate or revoke; ignored for add.
	AnchorID string
	// Anchor is the new anchor declaration for add and rotate.
	Anchor config.TrustAnchor
	// Approval is required for rotate and revoke.
	Approval *ApprovalEvidence
}

// TrustAnchorMutator applies trust-anchor mutations through one shared path.
type TrustAnchorMutator struct {
	State TrustAnchorState
	Pins  ExternalPinSetProvider
	Audit TrustAnchorAuditor
	// Marker persists the indeterminate marker around the write steps
	// (168.017-T). NewFileTrustAnchorMutator always wires it; a nil Marker
	// skips only the marker steps (write-outcome classification still applies).
	Marker TrustAnchorMarkerStore
	// Revocation is the injected non-filesystem external revocation authority
	// (168.009-T). Revoke succeeds only when it records the tombstone; with a
	// nil Revocation a revoke fails closed with ErrRevocationAuthorityRequired.
	Revocation RevocationAuthority
	Approver   ApproverVerifier
	Now        func() time.Time
}

// Apply runs: validate -> enforce external-pin policy -> write marker ->
// mutate config -> mutate status -> persist audit event -> clear marker. It is
// deterministic and ordered but not cross-file atomic; failures are classified
// as ErrWriteNotApplied or ErrWriteIndeterminate (see 168.017-T), and a denied
// mutation whose denial audit fails yields ErrAuditNotPersisted.
func (m *TrustAnchorMutator) Apply(ctx context.Context, req TrustAnchorMutation) (WriteOutcome, error) {
	var out WriteOutcome
	fail := func(id string, err error) (WriteOutcome, error) {
		return out, &TrustAnchorMutationError{Action: req.Action, AnchorID: id, Err: err}
	}
	if m == nil || m.State == nil || m.Audit == nil || m.Now == nil {
		return fail(req.AnchorID, fmt.Errorf("%w: mutator is not fully configured", ErrTrustAnchorMutationInvalid))
	}
	// Serialize Apply calls on the same storage root; the O_EXCL marker file
	// is the cross-process guard.
	unlock := lockTrustAnchorStorage(m)
	defer unlock()

	// 1. validate (request shape, then verifiable approval).
	current, err := m.State.LoadAnchors(ctx)
	if err != nil {
		return fail(req.AnchorID, fmt.Errorf("load trust anchors: %w", err))
	}
	plan, err := planTrustAnchorMutation(current, req)
	if err != nil {
		return fail(plan.targetID, err)
	}
	approver := ""
	if req.Action != TrustAnchorActionAdd {
		approver, err = m.verifyApproval(req)
		if err != nil {
			return fail(plan.targetID, err)
		}
	}

	plan.approver = approver

	// 2. enforce external-pin policy; a denial is audited before returning.
	if plan.newAnchor != nil {
		if pinErr := VerifyTrustAnchorPins(ctx, []config.TrustAnchor{*plan.newAnchor}, m.Pins); pinErr != nil {
			mutErr := &TrustAnchorMutationError{Action: req.Action, AnchorID: plan.targetID, Err: pinErr}
			ev := m.auditEvent(req, plan, TrustAnchorAuditDenied, pinErr.Error(), approver)
			if auditErr := m.Audit.RecordAudit(ctx, ev); auditErr != nil {
				// The denial stands; the audit failure is surfaced, never ignored.
				return out, errors.Join(mutErr, fmt.Errorf("%w: %w", ErrAuditNotPersisted, auditErr))
			}
			out.AuditPersisted = true
			return out, mutErr
		}
	}

	// 2b. a revoke needs the external authority; workspace status is advisory
	// and a status-only flip must never report success (168.009-T).
	if (req.Action == TrustAnchorActionRevoke || req.Action == TrustAnchorActionRotate) && m.Revocation == nil {
		reason := fmt.Errorf("%w: fingerprint %s must be revoked externally (de-pin/tombstone it in the external trust authority) and the operation confirmed; no workspace state was changed",
			ErrRevocationAuthorityRequired, plan.revokeFingerprint)
		mutErr := &TrustAnchorMutationError{Action: req.Action, AnchorID: plan.targetID, Err: reason}
		if auditErr := m.Audit.RecordAudit(ctx, m.auditEvent(req, plan, TrustAnchorAuditDenied, reason.Error(), approver)); auditErr != nil {
			return out, errors.Join(mutErr, fmt.Errorf("%w: %w", ErrAuditNotPersisted, auditErr))
		}
		out.AuditPersisted = true
		return out, mutErr
	}

	// Write protocol (168.017-T), each step durable:
	// (1) marker, (2) config+status, (3) audit, (4) clear marker.
	// Failures before step 2 are ErrWriteNotApplied; failures at or after
	// step 2 are ErrWriteIndeterminate with the marker left in place.
	if m.Marker != nil {
		marker := TrustAnchorMarker{Action: req.Action, AnchorID: plan.targetID, At: m.Now()}
		if plan.newAnchor != nil {
			marker.Fingerprint = plan.newAnchor.Fingerprint
			if req.Action == TrustAnchorActionRotate {
				marker.NewAnchorID = plan.newAnchor.ID
			}
		}
		if err := m.Marker.WriteMarker(ctx, marker); err != nil {
			if errors.Is(err, ErrTrustAnchorMarkerOutstanding) {
				// Refused before any state change; the other marker is untouched.
				return fail(plan.targetID, err)
			}
			return fail(plan.targetID, fmt.Errorf("%w: write marker: %w", ErrWriteNotApplied, err))
		}
	}

	if err := m.applyTrustAnchorState(ctx, plan, &out); err != nil {
		if !out.ConfigWritten && !out.StatusWritten && !out.RevocationRecorded && !errors.Is(err, ErrWriteIndeterminate) {
			// Nothing took effect: retract the marker so it is absent after.
			return m.notApplied(ctx, &out, req, plan.targetID, err)
		}
		out.MarkerOutstanding = m.Marker != nil
		return fail(plan.targetID, indeterminate(err))
	}

	if err := m.Audit.RecordAudit(ctx, m.auditEvent(req, plan, TrustAnchorAuditApplied, "", approver)); err != nil {
		out.MarkerOutstanding = m.Marker != nil
		return fail(plan.targetID, indeterminate(fmt.Errorf("persist audit: %w", err)))
	}
	out.AuditPersisted = true

	if m.Marker != nil {
		if err := m.Marker.ClearMarker(ctx); err != nil {
			out.MarkerOutstanding = true
			return fail(plan.targetID, indeterminate(fmt.Errorf("clear marker: %w", err)))
		}
	}
	return out, nil
}

// trustAnchorLocks holds one mutex per storage key (process-level serialization).
var trustAnchorLocks sync.Map

// lockTrustAnchorStorage locks the mutator's storage root (or the mutator
// itself when the marker store exposes no key) and returns the unlock func.
func lockTrustAnchorStorage(m *TrustAnchorMutator) func() {
	key := fmt.Sprintf("%p", m)
	if k, ok := m.Marker.(interface{ LockKey() string }); ok {
		key = k.LockKey()
	}
	mu, _ := trustAnchorLocks.LoadOrStore(key, &sync.Mutex{})
	l := mu.(*sync.Mutex) //nolint:forcetypeassert // only *sync.Mutex is ever stored
	l.Lock()
	return l.Unlock
}

// applyTrustAnchorState performs step 2: the config write then the status
// writes, recording which took effect in out.
func (m *TrustAnchorMutator) applyTrustAnchorState(ctx context.Context, plan trustAnchorPlan, out *WriteOutcome) error {
	if plan.action == TrustAnchorActionRevoke || plan.action == TrustAnchorActionRotate {
		// The authoritative revocation is recorded first; the workspace status
		// write below is only a contained advisory projection of it.
		rec := RevocationRecord{AnchorID: plan.targetID, Fingerprint: plan.revokeFingerprint, Approver: plan.approver, At: m.Now()}
		if err := m.Revocation.RecordRevocation(ctx, rec); err != nil {
			return fmt.Errorf("record external revocation: %w", err)
		}
		out.RevocationRecorded = true
	}
	if plan.config != nil {
		if err := m.State.WriteConfig(ctx, plan.config); err != nil {
			return fmt.Errorf("write config: %w", err)
		}
		out.ConfigWritten = true
	}
	for _, change := range plan.status {
		if err := m.State.WriteStatus(ctx, change); err != nil {
			return fmt.Errorf("write status: %w", err)
		}
		out.StatusWritten = true
	}
	return nil
}

// notApplied retracts the marker after a failure that left state unchanged and
// classifies the outcome as ErrWriteNotApplied. If the marker cannot be
// cleared it stays outstanding and is reported on the outcome and error.
func (m *TrustAnchorMutator) notApplied(ctx context.Context, out *WriteOutcome, req TrustAnchorMutation, id string, cause error) (WriteOutcome, error) {
	err := fmt.Errorf("%w: %w", ErrWriteNotApplied, cause)
	if m.Marker != nil {
		if clearErr := m.Marker.ClearMarker(ctx); clearErr != nil {
			out.MarkerOutstanding = true
			err = errors.Join(err, fmt.Errorf("clear marker: %w", clearErr))
		}
	}
	return *out, &TrustAnchorMutationError{Action: req.Action, AnchorID: id, Err: err}
}

// indeterminate classifies err as ErrWriteIndeterminate.
func indeterminate(err error) error {
	if errors.Is(err, ErrWriteIndeterminate) {
		return err
	}
	return fmt.Errorf("%w: %w", ErrWriteIndeterminate, err)
}

type trustAnchorPlan struct {
	action            TrustAnchorAction
	revokeFingerprint string
	approver          string
	targetID          string
	newAnchor         *config.TrustAnchor
	config            []config.TrustAnchor
	status            []TrustAnchorStatusChange
}

func (m *TrustAnchorMutator) auditEvent(req TrustAnchorMutation, plan trustAnchorPlan, outcome, reason, approver string) TrustAnchorAuditEvent {
	ev := TrustAnchorAuditEvent{Action: req.Action, AnchorID: plan.targetID, Outcome: outcome, Reason: reason,
		Approver: approver, At: m.Now()}
	if plan.newAnchor != nil {
		ev.Fingerprint = plan.newAnchor.Fingerprint
		if req.Action == TrustAnchorActionRotate {
			ev.NewAnchorID = plan.newAnchor.ID
		}
	}
	return ev
}

// planTrustAnchorMutation validates req against current and computes the
// config and status changes for the shared mutation path.
func planTrustAnchorMutation(current []config.TrustAnchor, req TrustAnchorMutation) (trustAnchorPlan, error) {
	invalid := func(format string, args ...any) error {
		return fmt.Errorf("%w: %s", ErrTrustAnchorMutationInvalid, fmt.Sprintf(format, args...))
	}
	find := func(id string) *config.TrustAnchor {
		for i := range current {
			if current[i].ID == id {
				return &current[i]
			}
		}
		return nil
	}
	plan := trustAnchorPlan{action: req.Action, targetID: req.AnchorID}
	introduces := false
	switch req.Action {
	case TrustAnchorActionAdd:
		plan.targetID = req.Anchor.ID
		introduces = true
	case TrustAnchorActionRotate:
		introduces = true
		fallthrough
	case TrustAnchorActionRevoke:
		target := find(req.AnchorID)
		if target == nil {
			return plan, invalid("anchor %q does not exist", req.AnchorID)
		}
		if target.Status == config.TrustAnchorStatusRevoked {
			return plan, invalid("anchor %q is already revoked", req.AnchorID)
		}
		plan.revokeFingerprint = target.Fingerprint
	default:
		return plan, invalid("unknown action %q", req.Action)
	}

	if introduces {
		na := req.Anchor
		if na.ID == "" || na.Fingerprint == "" || na.Role == "" {
			return plan, invalid("new anchor requires id, role and fingerprint")
		}
		if find(na.ID) != nil {
			return plan, invalid("anchor id %q already exists", na.ID)
		}
		na.Status = config.TrustAnchorStatusActive
		next := append(append(make([]config.TrustAnchor, 0, len(current)+1), current...), na)
		if err := config.ValidateTrustAnchors(next); err != nil {
			return plan, fmt.Errorf("%w: %w", ErrTrustAnchorMutationInvalid, err)
		}
		plan.newAnchor = &na
		plan.config = next
	}
	if req.Action != TrustAnchorActionAdd {
		plan.status = append(plan.status, TrustAnchorStatusChange{ID: req.AnchorID, Status: config.TrustAnchorStatusRevoked})
	}
	if plan.newAnchor != nil {
		plan.status = append(plan.status, TrustAnchorStatusChange{ID: plan.newAnchor.ID, Status: config.TrustAnchorStatusActive})
	}
	return plan, nil
}

// verifyApproval validates the operator-approval evidence itself and returns
// the verified approver identity.
func (m *TrustAnchorMutator) verifyApproval(req TrustAnchorMutation) (string, error) {
	ev := req.Approval
	if ev == nil {
		return "", ErrApprovalRequired
	}
	switch {
	case m.Approver == nil:
		return "", fmt.Errorf("%w: no approver verifier configured", ErrApprovalInvalid)
	case ev.Action != req.Action || ev.AnchorID != req.AnchorID:
		return "", fmt.Errorf("%w: evidence is bound to a different action or anchor", ErrApprovalInvalid)
	case ev.Nonce == "" || ev.Approver == "":
		return "", fmt.Errorf("%w: evidence lacks nonce or approver", ErrApprovalInvalid)
	case !ev.Expiry.After(m.Now()):
		return "", fmt.Errorf("%w: evidence expired", ErrApprovalInvalid)
	case ev.Expiry.After(m.Now().Add(MaxApprovalLifetime)):
		return "", fmt.Errorf("%w: evidence expiry exceeds the %s maximum lifetime", ErrApprovalInvalid, MaxApprovalLifetime)
	case req.Action == TrustAnchorActionRotate &&
		(ev.Replacement == nil || replacementBinding(*ev.Replacement) != replacementBinding(req.Anchor)):
		return "", fmt.Errorf("%w: rotate evidence is not bound to the replacement anchor", ErrApprovalInvalid)
	}
	if err := m.Approver.VerifyApproval(*ev); err != nil {
		if errors.Is(err, ErrApprovalInvalid) {
			return "", err
		}
		return "", fmt.Errorf("%w: %w", ErrApprovalInvalid, err)
	}
	return ev.Approver, nil
}
