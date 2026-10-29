package core

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"time"
)

// Sentinel errors for authorization-token verification (170.002-T).
var (
	// ErrTokenSignatureInvalid reports a missing, malformed or non-verifying
	// signature, or a token whose key id does not name the resolved anchor.
	ErrTokenSignatureInvalid = errors.New("authorization token signature invalid")
	// ErrTokenWrongShipment reports a token bound to a different shipment.
	ErrTokenWrongShipment = errors.New("authorization token names a different shipment")
	// ErrTokenNotYetValid reports a token whose not_before is in the future.
	ErrTokenNotYetValid = errors.New("authorization token not yet valid")
	// ErrTokenExpired reports a token whose not_after is in the past.
	ErrTokenExpired = errors.New("authorization token expired")
	// ErrTokenInvalidWindow reports a token with a missing or inverted
	// validity window.
	ErrTokenInvalidWindow = errors.New("authorization token validity window invalid")
)

// TokenVerifyError is the typed fail-closed error returned by
// VerifyAuthorizationToken.
type TokenVerifyError struct {
	// Field names the failing field or check.
	Field string
	// Err is the sentinel category.
	Err error
}

// Error implements error.
func (e *TokenVerifyError) Error() string {
	return "verify authorization token " + e.Field + ": " + e.Err.Error()
}

// Unwrap returns the sentinel category.
func (e *TokenVerifyError) Unwrap() error { return e.Err }

// TokenVerifyInput is the input of VerifyAuthorizationToken.
type TokenVerifyInput struct {
	// Token is the parsed token.
	Token AuthorizationToken
	// Key is the anchor resolved through ResolveVerificationKey.
	Key ResolvedKey
	// PublicKey is the loaded public key material of the resolved anchor.
	PublicKey *VerificationKey
	// ShipmentID is the shipment the token must be bound to.
	ShipmentID string
	// At is the verification time; zero means now.
	At time.Time
}

// SigningPayload returns the exact bytes the operator signs: the JSON
// encoding of the header, claims and binding objects in declaration order.
// The signature member is excluded.
func (t AuthorizationToken) SigningPayload() []byte {
	payload, err := json.Marshal(struct {
		Header  AuthorizationTokenHeader  `json:"header"`
		Claims  AuthorizationTokenClaims  `json:"claims"`
		Binding AuthorizationTokenBinding `json:"binding"`
	}{t.Header, t.Claims, t.Binding})
	if err != nil {
		// Marshaling plain strings and times cannot fail for valid years; an
		// unmarshalable token yields an empty payload that never verifies.
		return nil
	}
	return payload
}

// VerifyAuthorizationToken fails closed unless the token signature verifies
// under the resolved key, the token is bound to in.ShipmentID, and in.At lies
// inside the inclusive [not_before, not_after] window. The request-identity
// digest binding is verified separately (170.013-T).
func VerifyAuthorizationToken(in TokenVerifyInput) error {
	if err := verifyTokenRoleAndAlgo(in); err != nil {
		return err
	}
	if err := verifyTokenSignature(in); err != nil {
		return &TokenVerifyError{Field: "signature", Err: err}
	}
	if in.Token.Binding.ShipmentID != in.ShipmentID {
		return &TokenVerifyError{Field: "binding.shipment_id", Err: ErrTokenWrongShipment}
	}
	at := in.At
	if at.IsZero() {
		at = time.Now()
	}
	claims := in.Token.Claims
	switch {
	case claims.NotBefore.IsZero() || claims.NotAfter.IsZero() || !claims.NotAfter.After(claims.NotBefore):
		return &TokenVerifyError{Field: "claims", Err: ErrTokenInvalidWindow}
	case at.Before(claims.NotBefore):
		return &TokenVerifyError{Field: "claims.not_before", Err: ErrTokenNotYetValid}
	case at.After(claims.NotAfter):
		return &TokenVerifyError{Field: "claims.not_after", Err: ErrTokenExpired}
	}
	return nil
}

func verifyTokenSignature(in TokenVerifyInput) error {
	if in.PublicKey == nil || in.PublicKey.Public == nil {
		return ErrTokenSignatureInvalid
	}
	if in.Token.Header.KeyID != in.Key.Anchor.ID {
		return ErrTokenSignatureInvalid
	}
	raw, err := base64.StdEncoding.DecodeString(in.Token.Signature)
	if err != nil || len(raw) == 0 {
		return ErrTokenSignatureInvalid
	}
	payload := in.Token.SigningPayload()
	if payload == nil || !verifyAttestationSignatureBytes(in.PublicKey, payload, raw) {
		return ErrTokenSignatureInvalid
	}
	return nil
}
