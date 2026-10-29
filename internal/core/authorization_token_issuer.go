package core

import (
	"errors"
	"fmt"
	"time"

	"github.com/softwaresalt/backlogit/internal/config"
)

// ErrTokenUntrustedIssuer reports a token whose signing key id does not
// resolve to an anchor trusted by the external pin set (a self-minted token).
var ErrTokenUntrustedIssuer = errors.New("authorization token issuer is not trusted")

// ResolveTokenIssuerKey resolves the token-issuer anchor named by keyID at
// time at against the external pin set. Any resolution failure (unknown id,
// unpinned, revoked, outside the anchor window, absent pin set, wrong role)
// fails closed with a TokenVerifyError wrapping ErrTokenUntrustedIssuer and
// the resolver cause.
func ResolveTokenIssuerKey(anchors []config.TrustAnchor, keyID string, at time.Time, pins *ExternalPinSet) (ResolvedKey, error) {
	key, err := ResolveVerificationKey(anchors, KeySelector{ID: keyID, Purpose: TokenIssuerRole}, at, pins)
	if err != nil {
		return ResolvedKey{}, &TokenVerifyError{
			Field: "header.key_id",
			Err:   fmt.Errorf("%w: %w", ErrTokenUntrustedIssuer, err),
		}
	}
	return key, nil
}
