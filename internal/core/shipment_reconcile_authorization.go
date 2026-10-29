package core

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// Sentinel errors for the authenticated-approval gate (170.004-T).
var (
	// ErrAuthorizationRequired reports a live or dry-run reconcile under an
	// authenticated enforcement mode that supplied no authorization token
	// material. The self-suppliable --confirm/TTY guard never substitutes.
	ErrAuthorizationRequired = errors.New("authenticated authorization token required (fail closed)")
	// ErrAuthorizationRejected categorizes every fail-closed rejection of the
	// supplied authorization token inside the locked critical section.
	ErrAuthorizationRejected = errors.New("authorization token rejected (fail closed)")
	// ErrApprovalWrongShipment is the distinct typed category for a token
	// bound to another shipment (170.014-T).
	ErrApprovalWrongShipment = errors.New("approval rejected: token bound to a different shipment")
	// ErrApprovalWrongRequestIdentity is the distinct typed category for a
	// token whose request-identity digest does not match (170.014-T).
	ErrApprovalWrongRequestIdentity = errors.New("approval rejected: token bound to a different request identity")
	// ErrApprovalReplay is the distinct typed category for a token whose
	// nonce was already consumed (170.014-T).
	ErrApprovalReplay = errors.New("approval rejected: token replayed")
)

// Authorization outcome values reported on ShipmentShippedReconcileResult.
const (
	// AuthorizationOutcomeNotRequired is the v1 confirmation-only path.
	AuthorizationOutcomeNotRequired = "not_required"
	// AuthorizationOutcomeVerified means the token verified; on a dry run the
	// nonce was not consumed.
	AuthorizationOutcomeVerified = "verified"
	// AuthorizationOutcomeConsumed means the token verified and its nonce was
	// atomically consumed.
	AuthorizationOutcomeConsumed = "consumed"
)

// ReconcileAuthorization is the verification material for one authenticated
// reconcile invocation. Key resolution (against the external pin set) and
// public-key loading happen before the call; verification, request-identity
// binding and nonce consumption happen inside the locked critical section.
type ReconcileAuthorization struct {
	// Token is the parsed authorization token.
	Token AuthorizationToken
	// Key is the token-issuer anchor resolved through the external pin set.
	Key ResolvedKey
	// PublicKey is the loaded public key of the resolved anchor.
	PublicKey *VerificationKey
	// Nonces is the atomic compare-and-consume owner (170.008-T).
	Nonces *NonceConsumer
	// Auditor persists token rejection audit events.
	Auditor TokenRejectionAuditor
	// At is the verification time; zero means now.
	At time.Time
}

// reconcileApproverFor returns the approver identity the prepared event
// records for req, or nil for the confirmation-only (v1) path. The identity is
// only ever persisted by a run whose token later passes the locked-section
// gate; a rejected run writes nothing.
func reconcileApproverFor(req ShipmentShippedReconcileRequest) *ShipmentReconciledShippedApprover {
	if req.AuthEnforcement != AuthEnforcementAuthenticated || req.Authorization == nil {
		return nil
	}
	tok := req.Authorization.Token
	return &ShipmentReconciledShippedApprover{
		KeyID: tok.Header.KeyID,
		Algo:  req.Authorization.Key.Algo,
		Nonce: tok.Claims.Nonce,
	}
}

// enforceReconcileAuthorization is the authenticated-approval gate. It runs
// inside the locked critical section (Phase D, after the final
// re-classification and before any write). Under authenticated enforcement it
// verifies the token (signature, role, algorithm, shipment, validity window),
// binds it to the reconcile-computed request-identity digest, and on a live
// run consumes the nonce through the atomic NonceConsumer. A dry run verifies
// but never consumes. Confirmation-only (v1) enforcement returns
// AuthorizationOutcomeNotRequired. Every failure is fail-closed.
func enforceReconcileAuthorization(ctx context.Context, req ShipmentShippedReconcileRequest, shipmentID, requestIdentityDigest string, dryRun bool) (string, error) {
	switch req.AuthEnforcement {
	case "", AuthEnforcementConfirmation:
		return AuthorizationOutcomeNotRequired, nil
	case AuthEnforcementAuthenticated:
	default:
		return "", fmt.Errorf("%w: unknown enforcement mode %q", ErrAuthPolicyDenied, string(req.AuthEnforcement))
	}
	auth := req.Authorization
	if auth == nil {
		return "", ErrAuthorizationRequired
	}
	return AuthorizeApproval(ctx, *auth, shipmentID, requestIdentityDigest, dryRun)
}

// AuthorizeApproval verifies auth against shipmentID and the
// reconcile-computed requestIdentityDigest and, unless dryRun, atomically
// consumes the nonce. It returns AuthorizationOutcomeVerified (dry run) or
// AuthorizationOutcomeConsumed, or a fail-closed error wrapping
// ErrAuthorizationRejected.
func AuthorizeApproval(ctx context.Context, auth ReconcileAuthorization, shipmentID, requestIdentityDigest string, dryRun bool) (string, error) {
	if err := verifyApprovalToken(auth, shipmentID, requestIdentityDigest, dryRun); err != nil {
		return "", err
	}
	if dryRun {
		return AuthorizationOutcomeVerified, nil
	}
	if auth.Nonces == nil {
		return "", fmt.Errorf("%w: %w", ErrAuthorizationRejected, ErrNonceProviderUnavailable)
	}
	if err := auth.Nonces.Consume(ctx, auth.Token.Claims.Nonce, auth.Token.Claims.NotAfter); err != nil {
		return "", rejectApproval(fmt.Errorf("consume nonce: %w", err))
	}
	return AuthorizationOutcomeConsumed, nil
}

// verifyApprovalToken verifies the token and its request-identity binding
// without consuming the nonce. Rejection audit records are written only on a
// live run; a dry run verifies but never writes.
func verifyApprovalToken(auth ReconcileAuthorization, shipmentID, requestIdentityDigest string, dryRun bool) error {
	in := TokenVerifyInput{
		Token: auth.Token, Key: auth.Key, PublicKey: auth.PublicKey, ShipmentID: shipmentID, At: auth.At,
	}
	var verr error
	if dryRun {
		verr = VerifyAuthorizationToken(in)
	} else {
		verr = VerifyAuthorizationTokenAudited(in, auth.Auditor)
	}
	if verr != nil {
		return rejectApproval(verr)
	}
	if err := VerifyTokenRequestIdentity(auth.Token, requestIdentityDigest); err != nil {
		if !dryRun && auth.Auditor != nil {
			if aerr := auth.Auditor.RecordTokenRejection(TokenRejectionEvent{
				ShipmentID: shipmentID, KeyID: auth.Token.Header.KeyID,
				Field: "binding.request_identity_digest", Reason: err.Error(),
			}); aerr != nil {
				err = fmt.Errorf("%w: %w: %w", ErrTokenRejectionAuditFailed, aerr, err)
			}
		}
		return rejectApproval(err)
	}
	return nil
}

// authorizeReconcileResume is the authenticated-approval gate for the
// resume-append branch, where the prepared event is already persisted. Under
// authenticated enforcement the token must verify, and the prepared event must
// record the same approver (key id and nonce) as the token: the gate that
// wrote the prepared event already consumed that nonce, so a replay of the
// same nonce for the same prepared approver is not re-rejected. Any other
// approver, or none, fails closed before any nonce is consumed.
func authorizeReconcileResume(ctx context.Context, req ShipmentShippedReconcileRequest, shipmentID, requestIdentityDigest string, prepared *ShipmentReconciledShippedApprover) (string, error) {
	switch req.AuthEnforcement {
	case "", AuthEnforcementConfirmation:
		return AuthorizationOutcomeNotRequired, nil
	case AuthEnforcementAuthenticated:
	default:
		return "", fmt.Errorf("%w: unknown enforcement mode %q", ErrAuthPolicyDenied, string(req.AuthEnforcement))
	}
	auth := req.Authorization
	if auth == nil {
		return "", ErrAuthorizationRequired
	}
	if err := verifyApprovalToken(*auth, shipmentID, requestIdentityDigest, req.DryRun); err != nil {
		return "", err
	}
	if prepared == nil || prepared.KeyID != auth.Token.Header.KeyID || prepared.Nonce != auth.Token.Claims.Nonce {
		return "", fmt.Errorf("%w: prepared event approver does not match the supplied token", ErrAuthorizationRejected)
	}
	if req.DryRun {
		return AuthorizationOutcomeVerified, nil
	}
	if auth.Nonces == nil {
		return "", fmt.Errorf("%w: %w", ErrAuthorizationRejected, ErrNonceProviderUnavailable)
	}
	if err := auth.Nonces.Consume(ctx, auth.Token.Claims.Nonce, auth.Token.Claims.NotAfter); err != nil && !errors.Is(err, ErrNonceReplayed) {
		return "", rejectApproval(fmt.Errorf("consume nonce: %w", err))
	}
	return AuthorizationOutcomeConsumed, nil
}

// rejectApproval wraps err under ErrAuthorizationRejected, classifying the
// binding/replay categories into a distinct typed ApprovalRejectionError.
func rejectApproval(err error) error {
	var category error
	switch {
	case errors.Is(err, ErrTokenWrongShipment):
		category = ErrApprovalWrongShipment
	case errors.Is(err, ErrTokenRequestIdentityMismatch):
		category = ErrApprovalWrongRequestIdentity
	case errors.Is(err, ErrNonceReplayed):
		category = ErrApprovalReplay
	default:
		return fmt.Errorf("%w: %w", ErrAuthorizationRejected, err)
	}
	return fmt.Errorf("%w: %w", ErrAuthorizationRejected, &ApprovalRejectionError{Category: category, Err: err})
}

// ApprovalRejectionError is the typed fail-closed rejection returned by
// AuthorizeApproval for the binding/replay categories (170.014-T).
type ApprovalRejectionError struct {
	// Category is one of ErrApprovalWrongShipment,
	// ErrApprovalWrongRequestIdentity or ErrApprovalReplay.
	Category error
	// Err is the underlying verification or consume error.
	Err error
}

// Error implements error.
func (e *ApprovalRejectionError) Error() string {
	return e.Category.Error() + ": " + e.Err.Error()
}

// Unwrap exposes the rejection category, the generic rejection sentinel and
// the underlying cause.
func (e *ApprovalRejectionError) Unwrap() []error {
	return []error{e.Category, e.Err}
}

// ShipmentReconciledShippedApprover is the "approver" sub-object of an
// EventShipmentReconciledShipped delta. It records the authenticated approver
// identity that backed an authenticated reconcile, and is omitted entirely
// from confirmation-only (v1) events.
type ShipmentReconciledShippedApprover struct {
	// KeyID is the id of the trust anchor that signed the token.
	KeyID string `json:"key_id"`
	// Algo is the pinned signature algorithm of that anchor.
	Algo string `json:"algo"`
	// Nonce is the single-use nonce the token carried.
	Nonce string `json:"nonce"`
}
