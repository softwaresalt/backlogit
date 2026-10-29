package core_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/core"
	"github.com/softwaresalt/backlogit/internal/events"
)

func TestAttestationVerifyRoleAlgoAudit(t *testing.T) {
	t.Run("cross-role anchor is rejected with a typed role-mismatch error", func(t *testing.T) {
		f := newAttFixture(t)
		parsed := f.sign(t, f.priv, "att", "ed25519", f.statement())
		in := f.input(t, parsed)
		f.anchors[0].Role = "token-issuer"
		key, err := core.ResolveVerificationKey(f.anchors, core.KeySelector{ID: "att"}, attVerifyNow, allPinned(f.anchors))
		require.NoError(t, err)
		in.Key = key
		_, err = core.VerifyAttestation(in)
		require.ErrorIs(t, err, core.ErrTrustAnchorRoleMismatch)
		var re *core.TrustAnchorResolveError
		require.ErrorAs(t, err, &re)
	})

	t.Run("payload-declared algo differing from the pinned algo is rejected", func(t *testing.T) {
		for _, declared := range []string{"none", "None", "ecdsa-p256", "hs256"} {
			f := newAttFixture(t)
			in := f.input(t, f.sign(t, f.priv, "att", declared, f.statement()))
			_, err := core.VerifyAttestation(in)
			require.ErrorIs(t, err, core.ErrTrustAnchorAlgoMismatch, declared)
		}
	})

	t.Run("fail-closed rejection persists an audit event", func(t *testing.T) {
		ws := setupReconcileWorkspace(t)
		f := newAttFixture(t)
		st := f.statement()
		st.Predicate.MergeSHA = "other"
		in := f.input(t, f.sign(t, f.priv, "att", "ed25519", st))

		_, err := core.VerifyAttestationAudited(context.Background(), ws, in)
		require.ErrorIs(t, err, core.ErrAttestationEvidenceMismatch)

		logged, err := events.ReadAllEvents(context.Background(), core.WorkspaceLogsRoot(ws.RootPath), "150-S")
		require.NoError(t, err)
		require.Len(t, logged, 1)
		require.Equal(t, core.EventAttestationRejected, logged[0].EventType)
		require.Equal(t, "att", logged[0].Delta["key_id"])
		require.Equal(t, "merge_sha", logged[0].Delta["field"])
		require.NotEmpty(t, logged[0].Delta["error"])
	})

	t.Run("successful verification persists no audit event", func(t *testing.T) {
		ws := setupReconcileWorkspace(t)
		f := newAttFixture(t)
		_, err := core.VerifyAttestationAudited(context.Background(), ws, f.input(t, f.sign(t, f.priv, "att", "ed25519", f.statement())))
		require.NoError(t, err)
		logged, err := events.ReadAllEvents(context.Background(), core.WorkspaceLogsRoot(ws.RootPath), "150-S")
		require.NoError(t, err)
		require.Empty(t, logged)
	})
}
