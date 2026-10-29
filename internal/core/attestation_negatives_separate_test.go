package core

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/config"
)

// TestAttestationNegativesSeparate asserts that expired-key, revoked-key and
// wrong-shipment rejections are three distinct typed fail-closed outcomes
// (169.012-T).
func TestAttestationNegativesSeparate(t *testing.T) {
	const shipmentID = "048-S"
	d := validReconciledShippedDelta()

	reverify := func(t *testing.T, signer attestationTestSigner, mutate func(*config.TrustAnchor), st func(*AttestationStatement)) error {
		t.Helper()
		anchor := signer.anchor
		if mutate != nil {
			mutate(&anchor)
		}
		statement := attestationTestStatement(expectedFromDelta(shipmentID, d), reconcileEventTime)
		if st != nil {
			st(&statement)
		}
		env := signer.signEnvelope(t, signer.id, statement)
		claim := attestationTestClaim(t, env, signer.id)
		claim.Envelope = string(env)
		rv := &AttestationReverifier{Anchors: []config.TrustAnchor{anchor}, Pins: signer.pins}
		_, err := rv.Reverify(context.Background(), AttestationReverifyInput{ShipmentID: shipmentID, At: reconcileEventTime, Attestation: claim, Evidence: d.Evidence})
		return err
	}
	signer := newAttestationTestSigner(t, "ci-signer")

	t.Run("expired key is rejected with a typed expired-key error", func(t *testing.T) {
		past := reconcileEventTime.Add(-time.Hour)
		err := reverify(t, signer, func(a *config.TrustAnchor) { a.NotAfter = &past }, nil)
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrAttestationKeyExpired)
		assert.NotErrorIs(t, err, ErrAttestationKeyRevoked)
		assert.NotErrorIs(t, err, ErrAttestationWrongShipment)
	})

	t.Run("revoked key is rejected with a typed revoked-key error", func(t *testing.T) {
		err := reverify(t, signer, func(a *config.TrustAnchor) { a.Status = config.TrustAnchorStatusRevoked }, nil)
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrAttestationKeyRevoked)
		assert.NotErrorIs(t, err, ErrAttestationKeyExpired)
		assert.NotErrorIs(t, err, ErrAttestationWrongShipment)
	})

	t.Run("attestation naming another shipment is rejected with a typed wrong-shipment error", func(t *testing.T) {
		err := reverify(t, signer, nil, func(s *AttestationStatement) {
			s.Predicate.ShipmentID = "049-S"
			s.Subject[0].Name = "049-S"
		})
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrAttestationWrongShipment)
		assert.NotErrorIs(t, err, ErrAttestationKeyExpired)
		assert.NotErrorIs(t, err, ErrAttestationKeyRevoked)
	})
}
