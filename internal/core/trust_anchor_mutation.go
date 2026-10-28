package core

import (
	"context"
	"crypto/ed25519"
	"errors"
	"fmt"
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
}

// SigningPayload returns the canonical bytes covered by Signature.
func (e ApprovalEvidence) SigningPayload() []byte {
	return []byte("backlogit-trust-anchor-approval-v1\n" + string(e.Action) + "\n" + e.AnchorID + "\n" +
		e.Approver + "\n" + e.Expiry.UTC().Format(time.RFC3339Nano) + "\n" + e.Nonce)
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
	State    TrustAnchorState
	Pins     ExternalPinSetProvider
	Audit    TrustAnchorAuditor
	Approver ApproverVerifier
	Now      func() time.Time
}

// Apply runs: validate -> enforce external-pin policy -> mutate config ->
// mutate status -> persist audit event. It is deterministic and ordered but
// not cross-file atomic.
func (m *TrustAnchorMutator) Apply(ctx context.Context, req TrustAnchorMutation) (WriteOutcome, error) {
	var out WriteOutcome
	fail := func(id string, err error) (WriteOutcome, error) {
		return out, &TrustAnchorMutationError{Action: req.Action, AnchorID: id, Err: err}
	}
	if m == nil || m.State == nil || m.Audit == nil || m.Now == nil {
		return fail(req.AnchorID, fmt.Errorf("%w: mutator is not fully configured", ErrTrustAnchorMutationInvalid))
	}

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

	// 2. enforce external-pin policy; a denial is audited before returning.
	if plan.newAnchor != nil {
		if pinErr := VerifyTrustAnchorPins(ctx, []config.TrustAnchor{*plan.newAnchor}, m.Pins); pinErr != nil {
			mutErr := &TrustAnchorMutationError{Action: req.Action, AnchorID: plan.targetID, Err: pinErr}
			ev := m.auditEvent(req, plan, TrustAnchorAuditDenied, pinErr.Error(), approver)
			if auditErr := m.Audit.RecordAudit(ctx, ev); auditErr != nil {
				return out, errors.Join(mutErr, fmt.Errorf("persist denial audit: %w", auditErr))
			}
			out.AuditPersisted = true
			return out, mutErr
		}
	}

	// 3. mutate config.
	if plan.config != nil {
		if err := m.State.WriteConfig(ctx, plan.config); err != nil {
			return fail(plan.targetID, fmt.Errorf("write config: %w", err))
		}
		out.ConfigWritten = true
	}

	// 4. mutate status.
	for _, change := range plan.status {
		if err := m.State.WriteStatus(ctx, change); err != nil {
			return fail(plan.targetID, fmt.Errorf("write status: %w", err))
		}
		out.StatusWritten = true
	}

	// 5. persist the audit event as the final step.
	if err := m.Audit.RecordAudit(ctx, m.auditEvent(req, plan, TrustAnchorAuditApplied, "", approver)); err != nil {
		return fail(plan.targetID, fmt.Errorf("persist audit: %w", err))
	}
	out.AuditPersisted = true
	return out, nil
}

type trustAnchorPlan struct {
	targetID  string
	newAnchor *config.TrustAnchor
	config    []config.TrustAnchor
	status    []TrustAnchorStatusChange
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
	plan := trustAnchorPlan{targetID: req.AnchorID}
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
	}
	if err := m.Approver.VerifyApproval(*ev); err != nil {
		if errors.Is(err, ErrApprovalInvalid) {
			return "", err
		}
		return "", fmt.Errorf("%w: %w", ErrApprovalInvalid, err)
	}
	return ev.Approver, nil
}
