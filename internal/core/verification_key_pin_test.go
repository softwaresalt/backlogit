package core_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/config"
	"github.com/softwaresalt/backlogit/internal/core"
)

func TestVerificationKeyFingerprintPin(t *testing.T) {
	root := t.TempDir()
	pub, _, fp := edKeyPEM(t)
	anchor := config.TrustAnchor{ID: "pin1", Algo: "ed25519", PublicKeyRef: pub, Fingerprint: fp}
	key, err := core.LoadVerificationKey(root, anchor)
	require.NoError(t, err)
	require.Equal(t, fp, key.Fingerprint)

	t.Run("matching pinned fingerprint is accepted", func(t *testing.T) {
		require.NoError(t, core.VerifyVerificationKeyPin(key, anchor, core.NewExternalPinSet(fp, "sha256:other")))
	})

	t.Run("mismatching fingerprint fails closed", func(t *testing.T) {
		// Workspace config claims a different (pinned) fingerprint than the
		// key material actually has.
		swapped := anchor
		swapped.Fingerprint = "sha256:deadbeef"
		err := core.VerifyVerificationKeyPin(key, swapped, core.NewExternalPinSet("sha256:deadbeef"))
		require.ErrorIs(t, err, core.ErrVerificationKeyFingerprintMismatch)
		var vke *core.VerificationKeyError
		require.ErrorAs(t, err, &vke)
		require.Equal(t, "pin1", vke.AnchorID)

		// Computed fingerprint equals the anchor claim but is not pinned.
		err = core.VerifyVerificationKeyPin(key, anchor, core.NewExternalPinSet("sha256:other"))
		require.ErrorIs(t, err, core.ErrVerificationKeyFingerprintMismatch)

		// Absent pin set and absent key both fail closed.
		require.ErrorIs(t, core.VerifyVerificationKeyPin(key, anchor, nil), core.ErrTrustPinSetAbsent)
		require.ErrorIs(t, core.VerifyVerificationKeyPin(nil, anchor, core.NewExternalPinSet(fp)), core.ErrVerificationKeyFingerprintMismatch)
	})
}
