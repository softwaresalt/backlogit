package core

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/softwaresalt/backlogit/internal/canonical"
)

// AttestationSignerRole is the trust-anchor role bound to the attestation
// purpose.
const AttestationSignerRole = "attestation-signer"

// Sentinel errors for attestation verification (169.002-T).
var (
	// ErrAttestationSignatureInvalid reports a missing, malformed or
	// non-verifying signature over the canonical statement.
	ErrAttestationSignatureInvalid = errors.New("attestation signature invalid")
	// ErrAttestationEvidenceMismatch reports statement evidence fields that
	// differ from the reconcile-computed evidence.
	ErrAttestationEvidenceMismatch = errors.New("attestation evidence does not match reconcile evidence")
	// ErrAttestationBindingMismatch reports a statement whose shipment
	// binding does not match the reconcile request.
	ErrAttestationBindingMismatch = errors.New("attestation binding does not match reconcile request")
	// ErrAttestationExpired reports an expired or not-yet-valid signing key
	// or an expired (or expiry-less) statement.
	ErrAttestationExpired = errors.New("attestation or signing key expired")
)

// AttestationVerifyError is the typed fail-closed error returned by
// VerifyAttestation.
type AttestationVerifyError struct {
	// Field names the failing field or check (may be empty).
	Field string
	// Err is the sentinel category.
	Err error
}

// Error implements error.
func (e *AttestationVerifyError) Error() string {
	if e.Field == "" {
		return "verify attestation: " + e.Err.Error()
	}
	return "verify attestation " + e.Field + ": " + e.Err.Error()
}

// Unwrap returns the sentinel category.
func (e *AttestationVerifyError) Unwrap() error { return e.Err }

// AttestationExpectedEvidence is the reconcile-computed evidence the
// attestation statement must equal.
type AttestationExpectedEvidence struct {
	ShipmentID         string
	MergeSHA           string
	ManifestDigest     string
	ClosureContentHash string
}

// AttestationVerifyInput is the input of VerifyAttestation.
type AttestationVerifyInput struct {
	// Parsed is the parsed attestation.
	Parsed ParsedAttestation
	// Key is the anchor resolved through ResolveVerificationKey.
	Key ResolvedKey
	// PublicKey is the loaded public key material of the resolved anchor.
	PublicKey *VerificationKey
	// Expected is the reconcile-computed evidence.
	Expected AttestationExpectedEvidence
	// At is the verification time; zero means now.
	At time.Time
}

// AttestationVerification describes a successfully verified attestation; it
// carries the values bound into the reconcile event.
type AttestationVerification struct {
	KeyID           string
	SignerIdentity  string
	SigAlgo         string
	StatementDigest string
}

// VerifyAttestation verifies an attestation fail-closed: the envelope
// signature over the DSSE pre-authentication encoding of the payload must
// verify under the resolved anchor's public key, and the statement evidence
// fields must equal the reconcile-computed evidence. Role, pinned-algorithm,
// binding and expiry enforcement are layered on by later tasks.
func VerifyAttestation(in AttestationVerifyInput) (AttestationVerification, error) {
	var zero AttestationVerification
	fail := func(field string, err error) (AttestationVerification, error) {
		return zero, &AttestationVerifyError{Field: field, Err: err}
	}
	if err := verifyAttestationSignature(in); err != nil {
		return fail("signature", err)
	}
	var statement AttestationStatement
	if err := json.Unmarshal(in.Parsed.PayloadBytes, &statement); err != nil {
		return fail("statement", fmt.Errorf("%w: %w", ErrAttestationSignatureInvalid, err))
	}
	if field := attestationEvidenceMismatch(statement.Predicate, in.Expected); field != "" {
		return fail(field, ErrAttestationEvidenceMismatch)
	}
	return AttestationVerification{
		KeyID:           in.Key.Anchor.ID,
		SignerIdentity:  statement.Predicate.SignerIdentity,
		SigAlgo:         in.Key.Algo,
		StatementDigest: canonical.HashBytes(in.Parsed.PayloadBytes),
	}, nil
}

// attestationEvidenceMismatch returns the first predicate field that differs
// from the expected evidence, or "" when all fields match.
func attestationEvidenceMismatch(p AttestationPredicate, want AttestationExpectedEvidence) string {
	switch {
	case p.ShipmentID != want.ShipmentID:
		return "shipment_id"
	case p.MergeSHA != want.MergeSHA:
		return "merge_sha"
	case p.ManifestDigest != want.ManifestDigest:
		return "manifest_digest"
	case p.ClosureContentHash != want.ClosureContentHash:
		return "closure_content_hash"
	}
	return ""
}

// attestationPAE returns the DSSE v1 pre-authentication encoding the signature
// covers.
func attestationPAE(payloadType string, payload []byte) []byte {
	return fmt.Appendf(nil, "DSSEv1 %d %s %d %s", len(payloadType), payloadType, len(payload), payload)
}

// verifyAttestationSignature verifies a signature entry whose keyid names the
// resolved anchor over the PAE of the payload.
func verifyAttestationSignature(in AttestationVerifyInput) error {
	if in.PublicKey == nil || in.PublicKey.Public == nil {
		return fmt.Errorf("%w: no public key material", ErrAttestationSignatureInvalid)
	}
	pae := attestationPAE(in.Parsed.Envelope.PayloadType, in.Parsed.PayloadBytes)
	for _, sig := range in.Parsed.Envelope.Signatures {
		if sig.KeyID != in.Key.Anchor.ID {
			continue
		}
		raw, err := base64.StdEncoding.DecodeString(sig.Sig)
		if err != nil {
			continue
		}
		if verifyAttestationSignatureBytes(in.PublicKey, pae, raw) {
			return nil
		}
	}
	return fmt.Errorf("%w: no signature by key %q verifies", ErrAttestationSignatureInvalid, in.Key.Anchor.ID)
}

func verifyAttestationSignatureBytes(key *VerificationKey, message, sig []byte) bool {
	switch pub := key.Public.(type) {
	case ed25519.PublicKey:
		return len(pub) == ed25519.PublicKeySize && ed25519.Verify(pub, message, sig)
	case *ecdsa.PublicKey:
		digest := sha256.Sum256(message)
		return ecdsa.VerifyASN1(pub, digest[:], sig)
	}
	return false
}
