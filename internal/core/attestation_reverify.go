package core

import (
	"context"
	"errors"
	"time"

	"github.com/softwaresalt/backlogit/internal/config"
)

// ErrAttestationEnvelopeMissing reports a reconcile attestation binding that
// carries no durable signed envelope to re-verify.
var ErrAttestationEnvelopeMissing = errors.New("attestation envelope not persisted")

// AttestationReverifier re-verifies a persisted attestation offline against
// the external root of trust (169.007-T). A nil reverifier fails closed.
type AttestationReverifier struct {
	// WorkspaceRoot anchors file-based verification key references.
	WorkspaceRoot string
	// Anchors are the declared trust anchors.
	Anchors []config.TrustAnchor
	// Pins is the externally provisioned pin set; nil fails closed.
	Pins *ExternalPinSet
	// Authority optionally supplies external revocation tombstones.
	Authority RevocationAuthority
}

// AttestationReverifyInput is the recorded event data a re-verification covers.
type AttestationReverifyInput struct {
	ShipmentID  string
	At          time.Time
	Attestation *ShipmentReconciledShippedAttestation
	Evidence    ShipmentReconciledShippedEvidence
}

// Reverify is not implemented yet (169.007-T harness stub).
func (r *AttestationReverifier) Reverify(ctx context.Context, in AttestationReverifyInput) (AttestationVerification, error) {
	return AttestationVerification{}, errors.New("attestation reverify not implemented")
}
