package core_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/core"
)

func goldenReconcileRequest() core.ShipmentShippedReconcileRequest {
	return core.ShipmentShippedReconcileRequest{
		ShipmentID:      "151-S",
		Reason:          "operator approved reconcile",
		Actor:           "alice",
		SecondApprover:  "bob",
		IdempotencyKey:  "idem-1",
		MergeSHA:        "0123456789ABCDEF0123456789abcdef01234567",
		ClosureEvidence: "docs/closure.md",
		EvidenceRefs:    []string{"b-ref", "a-ref"},
	}
}

// goldenRequestIdentityDigest is the producer output (167.008-T) for
// goldenReconcileRequest. Producer-encoding drift breaks this test.
const goldenRequestIdentityDigest = "68a20034eb49ef73716ddc245488df0619ad765eefc7ab9a56b7abf91662b55a"

func TestTokenRequestIdentityBindingGolden(t *testing.T) {
	t.Run("matching digest is accepted", func(t *testing.T) {
		digest, err := core.ReconcileRequestIdentityDigest(goldenReconcileRequest())
		require.NoError(t, err)
		f := newTokenVerifyFixture(t)
		tok := f.token()
		tok.Binding.RequestIdentityDigest = digest
		require.NoError(t, core.VerifyTokenRequestIdentity(tok, digest))
	})

	t.Run("mismatched digest fails closed with typed error", func(t *testing.T) {
		f := newTokenVerifyFixture(t)
		tok := f.token()
		err := core.VerifyTokenRequestIdentity(tok, "different")
		require.Error(t, err)
		require.True(t, errors.Is(err, core.ErrTokenRequestIdentityMismatch))
		var verr *core.TokenVerifyError
		require.True(t, errors.As(err, &verr))
		require.Equal(t, "binding.request_identity_digest", verr.Field)
	})

	t.Run("empty digests never bind", func(t *testing.T) {
		f := newTokenVerifyFixture(t)
		tok := f.token()
		tok.Binding.RequestIdentityDigest = ""
		require.ErrorIs(t, core.VerifyTokenRequestIdentity(tok, ""), core.ErrTokenRequestIdentityMismatch)
	})

	t.Run("golden producer cross-check", func(t *testing.T) {
		digest, err := core.ReconcileRequestIdentityDigest(goldenReconcileRequest())
		require.NoError(t, err)
		require.Equal(t, goldenRequestIdentityDigest, digest)
	})

	t.Run("digest ignores shipment id and normalizes scalars", func(t *testing.T) {
		a := goldenReconcileRequest()
		b := goldenReconcileRequest()
		b.ShipmentID = "other"
		b.Actor = "  alice  "
		b.MergeSHA = "0123456789abcdef0123456789abcdef01234567"
		da, err := core.ReconcileRequestIdentityDigest(a)
		require.NoError(t, err)
		db, err := core.ReconcileRequestIdentityDigest(b)
		require.NoError(t, err)
		require.Equal(t, da, db)
	})
}
