package core_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/core"
)

type recordingTokenAuditor struct {
	events []core.TokenRejectionEvent
	err    error
}

func (r *recordingTokenAuditor) RecordTokenRejection(ev core.TokenRejectionEvent) error {
	if r.err != nil {
		return r.err
	}
	r.events = append(r.events, ev)
	return nil
}

func TestTokenVerifyRoleAlgoAudit(t *testing.T) {
	t.Run("cross-role key is rejected with typed role error", func(t *testing.T) {
		f := newTokenVerifyFixture(t)
		in := f.input(t, f.sign(f.token()))
		in.Key.Anchor.Role = "attestation-signer"
		err := core.VerifyAuthorizationToken(in)
		require.ErrorIs(t, err, core.ErrTokenRoleMismatch)
		var verr *core.TokenVerifyError
		require.True(t, errors.As(err, &verr))
		require.Equal(t, "key.role", verr.Field)
	})

	t.Run("header alg differing from pinned algo is rejected typed", func(t *testing.T) {
		for _, alg := range []string{"none", "None", "hs256", "rsa", ""} {
			f := newTokenVerifyFixture(t)
			tok := f.token()
			tok.Header.Algo = alg
			err := core.VerifyAuthorizationToken(f.input(t, f.sign(tok)))
			require.ErrorIs(t, err, core.ErrTokenAlgoMismatch, "alg %q", alg)
		}
	})

	t.Run("fail-closed rejection persists an audit event", func(t *testing.T) {
		f := newTokenVerifyFixture(t)
		tok := f.token()
		tok.Header.Algo = "none"
		in := f.input(t, f.sign(tok))
		auditor := &recordingTokenAuditor{}
		err := core.VerifyAuthorizationTokenAudited(in, auditor)
		require.ErrorIs(t, err, core.ErrTokenAlgoMismatch)
		require.False(t, errors.Is(err, core.ErrTokenRejectionAuditFailed))
		require.Len(t, auditor.events, 1)
		ev := auditor.events[0]
		require.Equal(t, "151-S", ev.ShipmentID)
		require.Equal(t, "op-1", ev.KeyID)
		require.Equal(t, "header.alg", ev.Field)
		require.NotEmpty(t, ev.Reason)
	})

	t.Run("accepted token writes no rejection audit", func(t *testing.T) {
		f := newTokenVerifyFixture(t)
		auditor := &recordingTokenAuditor{}
		require.NoError(t, core.VerifyAuthorizationTokenAudited(f.input(t, f.sign(f.token())), auditor))
		require.Empty(t, auditor.events)
	})

	t.Run("audit write failure surfaces typed error and keeps the rejection", func(t *testing.T) {
		f := newTokenVerifyFixture(t)
		in := f.input(t, f.sign(f.token()))
		in.ShipmentID = "other"
		boom := errors.New("disk full")
		err := core.VerifyAuthorizationTokenAudited(in, &recordingTokenAuditor{err: boom})
		require.Error(t, err)
		require.ErrorIs(t, err, core.ErrTokenRejectionAuditFailed)
		require.ErrorIs(t, err, core.ErrTokenWrongShipment)
		require.ErrorIs(t, err, boom)
	})

	t.Run("nil auditor fails closed", func(t *testing.T) {
		f := newTokenVerifyFixture(t)
		in := f.input(t, f.sign(f.token()))
		in.ShipmentID = "other"
		err := core.VerifyAuthorizationTokenAudited(in, nil)
		require.ErrorIs(t, err, core.ErrTokenRejectionAuditFailed)
	})
}
