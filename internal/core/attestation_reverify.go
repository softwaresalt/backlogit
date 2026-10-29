package core

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/softwaresalt/backlogit/internal/canonical"
	"github.com/softwaresalt/backlogit/internal/config"
)

// ErrAttestationEnvelopeMissing reports a reconcile attestation binding that
// carries no durable signed envelope to re-verify.
var ErrAttestationEnvelopeMissing = errors.New("attestation envelope not persisted")

// ErrAttestationRootUnavailable reports that the external root of trust (the
// pin set) needed to re-verify a persisted attestation is not available.
var ErrAttestationRootUnavailable = errors.New("attestation external root of trust unavailable")

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

// Reverify re-verifies the durable signed envelope persisted in a reconcile
// event binding, offline, against the external root of trust. It fails closed:
// a missing envelope, a missing pin set, an envelope that does not match the
// recorded claim, an untrusted key or a failed signature or evidence check
// all return an error. The recorded At bounds key and statement validity.
func (r *AttestationReverifier) Reverify(ctx context.Context, in AttestationReverifyInput) (AttestationVerification, error) {
	var zero AttestationVerification
	fail := func(err error) (AttestationVerification, error) {
		return zero, fmt.Errorf("reverify attestation for %s: %w", in.ShipmentID, err)
	}
	att := in.Attestation
	if att == nil || strings.TrimSpace(att.Envelope) == "" {
		return fail(ErrAttestationEnvelopeMissing)
	}
	if r == nil || r.Pins == nil {
		return fail(ErrAttestationRootUnavailable)
	}
	parsed, err := ParseAttestation([]byte(att.Envelope))
	if err != nil {
		return fail(err)
	}
	if canonical.HashBytes(parsed.PayloadBytes) != att.StatementDigest {
		return fail(&AttestationVerifyError{Field: "statement_digest", Err: ErrAttestationEvidenceMismatch})
	}
	key, err := ResolveVerificationKeyAuthoritative(ctx, r.Anchors, KeySelector{ID: att.KeyID, Purpose: AttestationSignerRole}, in.At, r.Pins, r.Authority)
	if err != nil {
		return fail(err)
	}
	public, err := LoadVerificationKey(r.WorkspaceRoot, key.Anchor)
	if err != nil {
		return fail(err)
	}
	if err := VerifyVerificationKeyPin(public, key.Anchor, r.Pins); err != nil {
		return fail(err)
	}
	verified, err := VerifyAttestation(AttestationVerifyInput{
		Parsed: parsed, Key: key, PublicKey: public, At: in.At,
		Expected: AttestationExpectedEvidence{
			ShipmentID: in.ShipmentID, MergeSHA: in.Evidence.MergeSHA,
			ManifestDigest: in.Evidence.ManifestDigest, ClosureContentHash: in.Evidence.ClosureContentHash,
		},
	})
	if err != nil {
		return fail(err)
	}
	if verified.KeyID != att.KeyID || verified.SignerIdentity != att.SignerIdentity || verified.SigAlgo != att.SigAlgo {
		return fail(&AttestationVerifyError{Field: "claim", Err: ErrAttestationEvidenceMismatch})
	}
	return verified, nil
}

// newDoctorAttestationReverifier builds the doctor re-verification root from
// the workspace trust anchors and the externally provisioned pin set. A missing
// or failing pin provider yields a reverifier without pins, which fails closed.
func newDoctorAttestationReverifier(ctx context.Context, ws *Workspace, opts *DoctorOptions) *AttestationReverifier {
	rv := &AttestationReverifier{WorkspaceRoot: ws.RootPath}
	if ws.Config != nil {
		rv.Anchors = ws.Config.TrustAnchors
	}
	if opts.AttestationPins != nil {
		if pins, err := opts.AttestationPins.ExternalPinSet(ctx); err == nil {
			rv.Pins = pins
		}
	}
	return rv
}
