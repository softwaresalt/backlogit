package core

import "errors"

// ErrTokenRequestIdentityMismatch reports a token whose request-identity
// digest differs from the reconcile-computed digest, or an empty digest.
var ErrTokenRequestIdentityMismatch = errors.New("authorization token request-identity digest mismatch")

// ReconcileRequestIdentityDigest returns the 167.008-T request-identity digest
// for req. It delegates to the single authoritative producer; the token path
// never re-encodes the digest (170.011-T contract).
func ReconcileRequestIdentityDigest(req ShipmentShippedReconcileRequest) (string, error) {
	return shipmentReconcileRequestIdentityDigest(req)
}

// VerifyTokenRequestIdentity fails closed unless the token binds exactly to
// the reconcile-computed digest. The comparison is exact byte equality and an
// empty digest on either side never binds.
func VerifyTokenRequestIdentity(token AuthorizationToken, computed string) error {
	bound := token.Binding.RequestIdentityDigest
	if bound == "" || computed == "" || bound != computed {
		return &TokenVerifyError{Field: "binding.request_identity_digest", Err: ErrTokenRequestIdentityMismatch}
	}
	return nil
}
