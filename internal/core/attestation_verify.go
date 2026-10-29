package core

import (
	"context"
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

// EventAttestationRejected is the event type persisted when attestation
// verification fails closed (169.013-T).
const EventAttestationRejected = "attestation_rejected"

// VerifyAttestationAudited runs VerifyAttestation and persists a rejection
// audit event on the shipment's log when verification fails closed. Not yet
// implemented (169.013-T harness stub): no audit is persisted.
func VerifyAttestationAudited(ctx context.Context, ws *Workspace, in AttestationVerifyInput) (AttestationVerification, error) {
	return VerifyAttestation(in)
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
	if err := attestationBindingError(statement, in.Expected); err != nil {
		return fail("binding", err)
	}
	if err := attestationExpiryError(in, statement.Predicate); err != nil {
		return fail("expiry", err)
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

// attestationBindingError checks that the statement is bound to the target
// shipment: the predicate shipment id and a subject name must both equal it.
func attestationBindingError(st AttestationStatement, want AttestationExpectedEvidence) error {
	if st.Predicate.ShipmentID != want.ShipmentID {
		return fmt.Errorf("%w: predicate shipment_id %q, want %q", ErrAttestationBindingMismatch, st.Predicate.ShipmentID, want.ShipmentID)
	}
	for _, subject := range st.Subject {
		if subject.Name == want.ShipmentID {
			return nil
		}
	}
	return fmt.Errorf("%w: no subject names shipment %q", ErrAttestationBindingMismatch, want.ShipmentID)
}

// attestationExpiryError rejects a signing key outside its validity window and
// a statement that is expired, or carries no parsable expiry (fail closed).
func attestationExpiryError(in AttestationVerifyInput, p AttestationPredicate) error {
	at := in.At
	if at.IsZero() {
		at = time.Now().UTC()
	}
	if err := checkAnchorWindow(&in.Key.Anchor, at); err != nil {
		return fmt.Errorf("%w: signing key: %w", ErrAttestationExpired, err)
	}
	expires, err := time.Parse(time.RFC3339, p.ExpiresAt)
	if err != nil {
		return fmt.Errorf("%w: statement expires_at %q: %w", ErrAttestationExpired, p.ExpiresAt, err)
	}
	if at.After(expires) {
		return fmt.Errorf("%w: statement expired at %s", ErrAttestationExpired, p.ExpiresAt)
	}
	return nil
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
