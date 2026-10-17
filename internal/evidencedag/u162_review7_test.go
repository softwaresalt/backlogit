package evidencedag

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReview7_IdenticalRetryRechecksValidityUnderLock(t *testing.T) {
	d := decl([]Node{pnode("t", true)})

	t.Run("retry that expires between prepare and lock is denied", func(t *testing.T) {
		f := newFixture(t)
		tok := signClaims(f.alice, f.claims())
		_, err := f.svc.Accept(tok, ship(), lookupFor(d))
		require.NoError(t, err)

		p, err := f.svc.prepare(tok, ship(), lookupFor(d))
		require.NoError(t, err)
		f.now = f.now.Add(2 * time.Hour)
		w, err := f.svc.commit(p)
		assert.ErrorIs(t, err, ErrWaiverDenied)
		assert.Equal(t, Waiver{}, w)
	})

	t.Run("retry within validity returns the original acceptance timestamp", func(t *testing.T) {
		f := newFixture(t)
		tok := signClaims(f.alice, f.claims())
		first := f.now.Unix()
		_, err := f.svc.Accept(tok, ship(), lookupFor(d))
		require.NoError(t, err)

		f.now = f.now.Add(10 * time.Minute)
		_, err = f.svc.Accept(tok, ship(), lookupFor(d))
		require.NoError(t, err)

		recs, err := f.ledger.Records(nil)
		require.NoError(t, err)
		require.Len(t, recs, 1)
		assert.Equal(t, first, recs[0].AcceptedAt)
	})

	t.Run("not-yet-valid retry under the lock is denied", func(t *testing.T) {
		f := newFixture(t)
		tok := signClaims(f.alice, f.claims())
		_, err := f.svc.Accept(tok, ship(), lookupFor(d))
		require.NoError(t, err)
		p, err := f.svc.prepare(tok, ship(), lookupFor(d))
		require.NoError(t, err)
		f.now = f.now.Add(-2 * time.Hour)
		_, err = f.svc.commit(p)
		assert.ErrorIs(t, err, ErrWaiverDenied)
	})
}
