package core

import (
	"errors"
	"fmt"
)

// TokenIssuerRole is the trust-anchor role a token signing key must hold.
const TokenIssuerRole = "token-issuer"

// Sentinel errors for role/algorithm binding and rejection audit (170.016-T).
var (
	// ErrTokenRoleMismatch reports a resolved anchor whose role is not the
	// token-issuer purpose (cross-role key reuse).
	ErrTokenRoleMismatch = errors.New("authorization token key role is not token-issuer")
	// ErrTokenAlgoMismatch reports a header algorithm that differs from the
	// anchor pinned algorithm, including "none" and an empty algorithm.
	ErrTokenAlgoMismatch = errors.New("authorization token header algorithm differs from pinned algorithm")
	// ErrTokenRejectionAuditFailed reports that persisting the rejection audit
	// failed. It is never ignored.
	ErrTokenRejectionAuditFailed = errors.New("authorization token rejection audit write failed")
)

// TokenRejectionEvent is the audit record of a fail-closed token rejection.
type TokenRejectionEvent struct {
	// ShipmentID is the shipment the verification targeted.
	ShipmentID string
	// KeyID is the key id named by the token header.
	KeyID string
	// Field names the failing field or check.
	Field string
	// Reason is the rejection error text.
	Reason string
}

// TokenRejectionAuditor persists token rejection audit events.
type TokenRejectionAuditor interface {
	// RecordTokenRejection persists ev or returns an error.
	RecordTokenRejection(ev TokenRejectionEvent) error
}

func verifyTokenRoleAndAlgo(in TokenVerifyInput) error {
	if in.Key.Anchor.Role != TokenIssuerRole {
		return &TokenVerifyError{Field: "key.role", Err: ErrTokenRoleMismatch}
	}
	if in.Token.Header.Algo == "" || in.Token.Header.Algo != in.Key.Algo {
		return &TokenVerifyError{Field: "header.alg", Err: ErrTokenAlgoMismatch}
	}
	return nil
}

// VerifyAuthorizationTokenAudited runs VerifyAuthorizationToken and persists a
// rejection audit event for any fail-closed rejection. When the audit cannot
// be written (or no auditor is supplied) the returned error wraps
// ErrTokenRejectionAuditFailed, the audit error and the original rejection.
func VerifyAuthorizationTokenAudited(in TokenVerifyInput, auditor TokenRejectionAuditor) error {
	verr := VerifyAuthorizationToken(in)
	if verr == nil {
		return nil
	}
	ev := TokenRejectionEvent{
		ShipmentID: in.ShipmentID,
		KeyID:      in.Token.Header.KeyID,
		Reason:     verr.Error(),
	}
	var typed *TokenVerifyError
	if errors.As(verr, &typed) {
		ev.Field = typed.Field
	}
	if auditor == nil {
		return fmt.Errorf("%w: no auditor: %w", ErrTokenRejectionAuditFailed, verr)
	}
	if aerr := auditor.RecordTokenRejection(ev); aerr != nil {
		return fmt.Errorf("%w: %w: %w", ErrTokenRejectionAuditFailed, aerr, verr)
	}
	return verr
}
