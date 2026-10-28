package core

import (
	"errors"

	"github.com/softwaresalt/backlogit/internal/config"
)

// ErrVerificationKeyFingerprintMismatch reports a loaded key whose computed
// fingerprint is not the externally pinned fingerprint for its anchor.
var ErrVerificationKeyFingerprintMismatch = errors.New("verification key fingerprint does not match pinned fingerprint")

// VerifyVerificationKeyPin binds a loaded key to its external pin: the
// fingerprint computed from the key material must equal the anchor's declared
// fingerprint AND be present in the externally provisioned pin set. It fails
// closed: a nil pin set returns ErrTrustPinSetAbsent; a nil key, a computed
// fingerprint differing from the anchor's, or an unpinned computed fingerprint
// returns *VerificationKeyError wrapping ErrVerificationKeyFingerprintMismatch.
func VerifyVerificationKeyPin(key *VerificationKey, anchor config.TrustAnchor, pins *ExternalPinSet) error {
	if pins == nil {
		return ErrTrustPinSetAbsent
	}
	if key == nil || key.Fingerprint == "" || key.Fingerprint != anchor.Fingerprint || !pins.Contains(key.Fingerprint) {
		return &VerificationKeyError{AnchorID: anchor.ID, Err: ErrVerificationKeyFingerprintMismatch}
	}
	return nil
}
