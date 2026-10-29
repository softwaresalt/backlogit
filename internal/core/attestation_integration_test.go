package core

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/config"
)

var errCaptureExpected = errors.New("captured expected evidence")

// attestationIntegrationVerifier mirrors the CLI verifier hook: parse, resolve
// the anchor through the external pin set, load the key and verify.
func attestationIntegrationVerifier(t *testing.T, s attestationTestSigner, envelope []byte, at time.Time) func(AttestationExpectedEvidence) error {
	t.Helper()
	return func(expected AttestationExpectedEvidence) error {
		parsed, err := ParseAttestation(envelope)
		if err != nil {
			return err
		}
		anchors := []config.TrustAnchor{s.anchor}
		key, err := ResolveVerificationKey(anchors, KeySelector{ID: s.id, Purpose: AttestationSignerRole}, at, s.pins)
		if err != nil {
			return err
		}
		public, err := LoadVerificationKey("", key.Anchor)
		if err != nil {
			return err
		}
		_, err = VerifyAttestation(AttestationVerifyInput{Parsed: parsed, Key: key, PublicKey: public, Expected: expected, At: at})
		return err
	}
}

// TestAttestationIntegration drives signed attestations through the real
// reconcile evidence preparation over the 048-S-shaped fixture (169.005-T).
func TestAttestationIntegration(t *testing.T) {
	fixture := setupShipmentReconcileEvidenceFixture(t)
	fixture.ws.Config.Reconcile = &config.ReconcileConfig{}
	at := time.Now().UTC()
	signer := newAttestationTestSigner(t, "ci-signer")

	req := fixture.request("048-S", fixture.merge048)
	var expected AttestationExpectedEvidence
	req.AttestationVerifier = func(e AttestationExpectedEvidence) error { expected = e; return errCaptureExpected }
	_, err := prepareShipmentReconcileEvidence(context.Background(), fixture.ws, req, fixedShipmentReconcileEvidenceInput())
	require.ErrorIs(t, err, errCaptureExpected)
	require.Equal(t, "048-S", expected.ShipmentID)
	require.NotEmpty(t, expected.ClosureContentHash)

	prepare := func(t *testing.T, envelope []byte) error {
		t.Helper()
		r := fixture.request("048-S", fixture.merge048)
		r.Attestation = attestationTestClaim(t, envelope, signer.id)
		r.Attestation.Envelope = string(envelope)
		r.AttestationVerifier = attestationIntegrationVerifier(t, signer, envelope, at)
		_, perr := prepareShipmentReconcileEvidence(context.Background(), fixture.ws, r, fixedShipmentReconcileEvidenceInput())
		return perr
	}

	t.Run("signed happy path over the 048-S fixture passes", func(t *testing.T) {
		envelope := signer.signEnvelope(t, signer.id, attestationTestStatement(expected, at))
		require.NoError(t, prepare(t, envelope))
	})

	t.Run("tampered statement is rejected", func(t *testing.T) {
		envelope := signer.signEnvelope(t, signer.id, attestationTestStatement(expected, at))
		var env AttestationEnvelope
		require.NoError(t, json.Unmarshal(envelope, &env))
		forged := attestationTestStatement(expected, at)
		forged.Predicate.MergeSHA = "0000000000000000000000000000000000000000"
		body, err := json.Marshal(forged)
		require.NoError(t, err)
		env.Payload = base64.StdEncoding.EncodeToString(body)
		tampered, err := json.Marshal(env)
		require.NoError(t, err)

		err = prepare(t, tampered)
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrAttestationSignatureInvalid)
	})

	t.Run("wrong-key attestation is rejected", func(t *testing.T) {
		other := newAttestationTestSigner(t, "other")
		envelope := other.signEnvelope(t, signer.id, attestationTestStatement(expected, at))
		err := prepare(t, envelope)
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrAttestationSignatureInvalid)
	})
}
