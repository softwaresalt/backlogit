package core_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/core"
)

func TestAttestationBindingExpiry(t *testing.T) {
	t.Run("binding mismatch fails closed", func(t *testing.T) {
		cases := map[string]func(st *core.AttestationStatement){
			"predicate shipment id": func(st *core.AttestationStatement) { st.Predicate.ShipmentID = "999-S" },
			"subject name":          func(st *core.AttestationStatement) { st.Subject[0].Name = "999-S" },
			"no subject":            func(st *core.AttestationStatement) { st.Subject = nil },
		}
		for name, mutate := range cases {
			t.Run(name, func(t *testing.T) {
				f := newAttFixture(t)
				st := f.statement()
				mutate(&st)
				_, err := core.VerifyAttestation(f.input(t, f.sign(t, f.priv, "att", "ed25519", st)))
				require.ErrorIs(t, err, core.ErrAttestationBindingMismatch)
				var ve *core.AttestationVerifyError
				require.ErrorAs(t, err, &ve)
			})
		}
	})

	t.Run("expiry mismatch fails closed", func(t *testing.T) {
		t.Run("expired statement", func(t *testing.T) {
			f := newAttFixture(t)
			st := f.statement()
			st.Predicate.ExpiresAt = attVerifyNow.Add(-time.Minute).Format(time.RFC3339)
			_, err := core.VerifyAttestation(f.input(t, f.sign(t, f.priv, "att", "ed25519", st)))
			require.ErrorIs(t, err, core.ErrAttestationExpired)
		})
		t.Run("missing or unparsable statement expiry", func(t *testing.T) {
			for _, bad := range []string{"", "not-a-time"} {
				f := newAttFixture(t)
				st := f.statement()
				st.Predicate.ExpiresAt = bad
				_, err := core.VerifyAttestation(f.input(t, f.sign(t, f.priv, "att", "ed25519", st)))
				require.ErrorIs(t, err, core.ErrAttestationExpired, bad)
			}
		})
		t.Run("expired signing key", func(t *testing.T) {
			f := newAttFixture(t)
			f.anchors[0].NotAfter = tptr(attVerifyNow.Add(time.Hour))
			in := f.input(t, f.sign(t, f.priv, "att", "ed25519", f.statement()))
			in.At = attVerifyNow.Add(2 * time.Hour)
			_, err := core.VerifyAttestation(in)
			require.ErrorIs(t, err, core.ErrAttestationExpired)
		})
		t.Run("not yet valid signing key", func(t *testing.T) {
			f := newAttFixture(t)
			f.anchors[0].NotBefore = tptr(attVerifyNow.Add(-time.Hour))
			in := f.input(t, f.sign(t, f.priv, "att", "ed25519", f.statement()))
			in.At = attVerifyNow.Add(-2 * time.Hour)
			_, err := core.VerifyAttestation(in)
			require.ErrorIs(t, err, core.ErrAttestationExpired)
		})
	})
}
