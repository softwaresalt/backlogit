package core_test

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/config"
	"github.com/softwaresalt/backlogit/internal/core"
)

var attVerifyNow = time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)

type attFixture struct {
	pub      ed25519.PublicKey
	priv     ed25519.PrivateKey
	anchors  []config.TrustAnchor
	expected core.AttestationExpectedEvidence
}

func newAttFixture(t *testing.T) attFixture {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(nil)
	require.NoError(t, err)
	return attFixture{
		pub:  pub,
		priv: priv,
		anchors: []config.TrustAnchor{{
			ID: "att", Role: core.AttestationSignerRole, Algo: "ed25519", Fingerprint: "sha256:a1",
			Status: config.TrustAnchorStatusActive,
		}},
		expected: core.AttestationExpectedEvidence{
			ShipmentID: "150-S", MergeSHA: "deadbeef", ManifestDigest: "m1", ClosureContentHash: "c1",
		},
	}
}

func (f attFixture) statement() core.AttestationStatement {
	return core.AttestationStatement{
		Type:          "https://in-toto.io/Statement/v1",
		Subject:       []core.AttestationSubject{{Name: f.expected.ShipmentID, Digest: map[string]string{"sha256": "abc"}}},
		PredicateType: "https://backlogit.dev/delivery-evidence/v1",
		Predicate: core.AttestationPredicate{
			ShipmentID: f.expected.ShipmentID, MergeSHA: f.expected.MergeSHA,
			ManifestDigest: f.expected.ManifestDigest, ClosureContentHash: f.expected.ClosureContentHash,
			SignerIdentity: "ci@example",
			IssuedAt:       attVerifyNow.Add(-time.Hour).Format(time.RFC3339),
			ExpiresAt:      attVerifyNow.Add(24 * time.Hour).Format(time.RFC3339),
		},
	}
}

func attPAE(payloadType string, payload []byte) []byte {
	return []byte(fmt.Sprintf("DSSEv1 %d %s %d %s", len(payloadType), payloadType, len(payload), payload))
}

// sign builds a signed envelope for st using priv and the declared algo.
func (f attFixture) sign(t *testing.T, priv ed25519.PrivateKey, keyID, algo string, st core.AttestationStatement) core.ParsedAttestation {
	t.Helper()
	body, err := json.Marshal(st)
	require.NoError(t, err)
	const payloadType = "application/vnd.in-toto+json"
	sig := ed25519.Sign(priv, attPAE(payloadType, body))
	env, err := json.Marshal(core.AttestationEnvelope{
		PayloadType: payloadType,
		Payload:     base64.StdEncoding.EncodeToString(body),
		Signatures:  []core.AttestationSignature{{KeyID: keyID, Sig: base64.StdEncoding.EncodeToString(sig), Algo: algo}},
	})
	require.NoError(t, err)
	parsed, err := core.ParseAttestation(env)
	require.NoError(t, err)
	return parsed
}

func (f attFixture) input(t *testing.T, parsed core.ParsedAttestation) core.AttestationVerifyInput {
	t.Helper()
	key, err := core.ResolveVerificationKey(f.anchors, core.KeySelector{ID: "att", Purpose: core.AttestationSignerRole}, attVerifyNow, allPinned(f.anchors))
	require.NoError(t, err)
	return core.AttestationVerifyInput{
		Parsed:    parsed,
		Key:       key,
		PublicKey: &core.VerificationKey{Algo: "ed25519", Public: f.pub, Fingerprint: "sha256:a1"},
		Expected:  f.expected,
		At:        attVerifyNow,
	}
}

func TestAttestationVerify(t *testing.T) {
	t.Run("valid signature over the canonical statement verifies", func(t *testing.T) {
		f := newAttFixture(t)
		got, err := core.VerifyAttestation(f.input(t, f.sign(t, f.priv, "att", "ed25519", f.statement())))
		require.NoError(t, err)
		require.Equal(t, "att", got.KeyID)
		require.Equal(t, "ci@example", got.SignerIdentity)
		require.Equal(t, "ed25519", got.SigAlgo)
		require.Len(t, got.StatementDigest, 64)
	})

	t.Run("signature mismatch fails closed", func(t *testing.T) {
		f := newAttFixture(t)
		_, otherPriv, err := ed25519.GenerateKey(nil)
		require.NoError(t, err)
		wrongKey := f.sign(t, otherPriv, "att", "ed25519", f.statement())
		_, err = core.VerifyAttestation(f.input(t, wrongKey))
		require.ErrorIs(t, err, core.ErrAttestationSignatureInvalid)
		var ve *core.AttestationVerifyError
		require.ErrorAs(t, err, &ve)

		tampered := f.sign(t, f.priv, "att", "ed25519", f.statement())
		tampered.PayloadBytes = append([]byte(nil), tampered.PayloadBytes...)
		tampered.PayloadBytes[len(tampered.PayloadBytes)-3] ^= 0x01
		_, err = core.VerifyAttestation(f.input(t, tampered))
		require.ErrorIs(t, err, core.ErrAttestationSignatureInvalid)

		unknownKeyID := f.sign(t, f.priv, "other", "ed25519", f.statement())
		_, err = core.VerifyAttestation(f.input(t, unknownKeyID))
		require.ErrorIs(t, err, core.ErrAttestationSignatureInvalid)
	})

	t.Run("evidence field mismatch fails closed", func(t *testing.T) {
		for _, field := range []string{"merge_sha", "manifest_digest", "closure_content_hash"} {
			t.Run(field, func(t *testing.T) {
				f := newAttFixture(t)
				st := f.statement()
				switch field {
				case "merge_sha":
					st.Predicate.MergeSHA = "other"
				case "manifest_digest":
					st.Predicate.ManifestDigest = "other"
				default:
					st.Predicate.ClosureContentHash = "other"
				}
				_, err := core.VerifyAttestation(f.input(t, f.sign(t, f.priv, "att", "ed25519", st)))
				require.ErrorIs(t, err, core.ErrAttestationEvidenceMismatch)
				var ve *core.AttestationVerifyError
				require.ErrorAs(t, err, &ve)
				require.Equal(t, field, ve.Field)
			})
		}
	})
}
