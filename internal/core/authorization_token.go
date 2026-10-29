package core

import "time"

// AuthorizationToken is the operator pre-commit authorization token (170-F).
// It is a signed binding of {shipment_id, request_identity_digest, not_before,
// not_after, nonce} laid out per the 170.011-T request-identity-digest
// contract (docs/design-docs/2026-09-16-866fdc8c-request-identity-digest-contract.md).
// This file declares the shape only; parsing and verification live elsewhere.
type AuthorizationToken struct {
	// Header names the signing algorithm and key id.
	Header AuthorizationTokenHeader `json:"header"`
	// Claims carries the validity window and the per-invocation nonce.
	Claims AuthorizationTokenClaims `json:"claims"`
	// Binding names the shipment and request the token authorizes.
	Binding AuthorizationTokenBinding `json:"binding"`
	// Signature is the operator signature over the header, claims and binding.
	Signature string `json:"signature"`
}

// AuthorizationTokenHeader is the token header object.
type AuthorizationTokenHeader struct {
	// Algo is the declared signature algorithm.
	Algo string `json:"alg"`
	// KeyID is the id of the trust anchor that signed the token.
	KeyID string `json:"key_id"`
}

// AuthorizationTokenClaims is the token claims object.
type AuthorizationTokenClaims struct {
	// NotBefore is the issued-at instant; the token is not valid earlier.
	NotBefore time.Time `json:"not_before"`
	// NotAfter is the expiry instant; the token is not valid later.
	NotAfter time.Time `json:"not_after"`
	// Nonce is the per-invocation unique value.
	Nonce string `json:"nonce"`
}

// AuthorizationTokenBinding is the token binding object.
type AuthorizationTokenBinding struct {
	// ShipmentID is the target shipment the token authorizes.
	ShipmentID string `json:"shipment_id"`
	// RequestIdentityDigest is the 167.008-T producer digest of the request.
	RequestIdentityDigest string `json:"request_identity_digest"`
}
