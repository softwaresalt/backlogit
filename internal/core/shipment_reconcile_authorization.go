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
	if err := VerifyAuthorizationTokenAudited(TokenVerifyInput{
		Token: auth.Token, Key: auth.Key, PublicKey: auth.PublicKey, ShipmentID: shipmentID, At: auth.At,
	}, auth.Auditor); err != nil {
		return "", fmt.Errorf("%w: %w", ErrAuthorizationRejected, err)
	}
	if err := VerifyTokenRequestIdentity(auth.Token, requestIdentityDigest); err != nil {
		if auth.Auditor != nil {
			if aerr := auth.Auditor.RecordTokenRejection(TokenRejectionEvent{
				ShipmentID: shipmentID, KeyID: auth.Token.Header.KeyID,
				Field: "binding.request_identity_digest", Reason: err.Error(),
			}); aerr != nil {
				err = fmt.Errorf("%w: %w: %w", ErrTokenRejectionAuditFailed, aerr, err)
			}
		}
		return "", fmt.Errorf("%w: %w", ErrAuthorizationRejected, err)
	}
	if dryRun {
		return AuthorizationOutcomeVerified, nil
	}
	if auth.Nonces == nil {
		return "", fmt.Errorf("%w: %w", ErrAuthorizationRejected, ErrNonceProviderUnavailable)
	}
	if err := auth.Nonces.Consume(ctx, auth.Token.Claims.Nonce, auth.Token.Claims.NotAfter); err != nil {
		return "", fmt.Errorf("%w: consume nonce: %w", ErrAuthorizationRejected, err)
	}
	return AuthorizationOutcomeConsumed, nil
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
