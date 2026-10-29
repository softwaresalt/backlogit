package core

import (
	"crypto/ed25519"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/canonical"
	"github.com/softwaresalt/backlogit/internal/config"
)

const attestationTestPayloadType = "application/vnd.in-toto+json"

// attestationTestSigner is an ed25519 attestation-signer anchor with the
// matching private key, an inline PEM public_key_ref and an external pin set.
type attestationTestSigner struct {
	id     string
	priv   ed25519.PrivateKey
	anchor config.TrustAnchor
	pins   *ExternalPinSet
}

func newAttestationTestSigner(t *testing.T, id string) attestationTestSigner {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(nil)
	require.NoError(t, err)
	der, err := x509.MarshalPKIXPublicKey(pub)
	require.NoError(t, err)
	fingerprint := "sha256:" + canonical.HashBytes(der)
	pemText := "-----BEGIN PUBLIC KEY-----\n" + base64.StdEncoding.EncodeToString(der) + "\n-----END PUBLIC KEY-----\n"
	anchor := config.TrustAnchor{
		ID: id, Role: AttestationSignerRole, Algo: config.TrustAnchorAlgoEd25519,
		PublicKeyRef: pemText, Fingerprint: fingerprint, Status: config.TrustAnchorStatusActive,
	}
	return attestationTestSigner{
		id: id, priv: priv, anchor: anchor,
		pins: NewBoundExternalPinSet(PinBinding{Fingerprint: fingerprint, Role: AttestationSignerRole}),
	}
}

func attestationTestStatement(exp AttestationExpectedEvidence, at time.Time) AttestationStatement {
	return AttestationStatement{
		Type:          "https://in-toto.io/Statement/v1",
		Subject:       []AttestationSubject{{Name: exp.ShipmentID, Digest: map[string]string{"sha256": "abc"}}},
		PredicateType: "https://backlogit.dev/delivery-evidence/v1",
		Predicate: AttestationPredicate{
			ShipmentID: exp.ShipmentID, MergeSHA: exp.MergeSHA, ManifestDigest: exp.ManifestDigest,
			ClosureContentHash: exp.ClosureContentHash, SignerIdentity: "ci@example",
			IssuedAt:  at.Add(-time.Hour).UTC().Format(time.RFC3339),
			ExpiresAt: at.Add(24 * time.Hour).UTC().Format(time.RFC3339),
		},
	}
}

// signEnvelope returns the raw signed DSSE envelope bytes for st under keyID.
func (s attestationTestSigner) signEnvelope(t *testing.T, keyID string, st AttestationStatement) []byte {
	t.Helper()
	body, err := json.Marshal(st)
	require.NoError(t, err)
	sig := ed25519.Sign(s.priv, attestationPAE(attestationTestPayloadType, body))
	env, err := json.Marshal(AttestationEnvelope{
		PayloadType: attestationTestPayloadType,
		Payload:     base64.StdEncoding.EncodeToString(body),
		Signatures:  []AttestationSignature{{KeyID: keyID, Sig: base64.StdEncoding.EncodeToString(sig), Algo: config.TrustAnchorAlgoEd25519}},
	})
	require.NoError(t, err)
	return env
}

// claimFor derives the event attestation claim for a raw envelope.
func attestationTestClaim(t *testing.T, envelope []byte, keyID string) *ShipmentReconciledShippedAttestation {
	t.Helper()
	parsed, err := ParseAttestation(envelope)
	require.NoError(t, err)
	return &ShipmentReconciledShippedAttestation{
		KeyID: keyID, SignerIdentity: parsed.Statement.Predicate.SignerIdentity,
		SigAlgo: config.TrustAnchorAlgoEd25519, StatementDigest: canonical.HashBytes(parsed.PayloadBytes),
	}
}
