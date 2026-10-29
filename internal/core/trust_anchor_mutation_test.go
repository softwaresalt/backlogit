package core_test

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/config"
	"github.com/softwaresalt/backlogit/internal/core"
)

type mutFixture struct {
	log      []string
	anchors  []config.TrustAnchor
	audits   []core.TrustAnchorAuditEvent
	auditErr error
}

func (f *mutFixture) LoadAnchors(context.Context) ([]config.TrustAnchor, error) {
	return append([]config.TrustAnchor(nil), f.anchors...), nil
}

func (f *mutFixture) WriteConfig(_ context.Context, a []config.TrustAnchor) error {
	f.log = append(f.log, "config")
	f.anchors = append([]config.TrustAnchor(nil), a...)
	return nil
}

func (f *mutFixture) WriteStatus(_ context.Context, c core.TrustAnchorStatusChange) error {
	f.log = append(f.log, "status:"+c.ID+"="+string(c.Status))
	for i := range f.anchors {
		if f.anchors[i].ID == c.ID {
			f.anchors[i].Status = c.Status
		}
	}
	return nil
}

func (f *mutFixture) RecordAudit(_ context.Context, ev core.TrustAnchorAuditEvent) error {
	f.log = append(f.log, "audit:"+ev.Outcome)
	if f.auditErr != nil {
		return f.auditErr
	}
	f.audits = append(f.audits, ev)
	return nil
}

type pinProv struct{ set *core.ExternalPinSet }

func (p pinProv) ExternalPinSet(context.Context) (*core.ExternalPinSet, error) { return p.set, nil }

func TestTrustAnchorMutationCore(t *testing.T) {
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	_, otherPriv, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)

	evidence := func(action core.TrustAnchorAction, id string, expiry time.Time, key ed25519.PrivateKey) *core.ApprovalEvidence {
		ev := core.ApprovalEvidence{Action: action, AnchorID: id, Approver: "operator-1", Expiry: expiry, Nonce: "n-1"}
		ev.Signature = ed25519.Sign(key, ev.SigningPayload())
		return &ev
	}
	rotateEvidence := func(id string, repl config.TrustAnchor, expiry time.Time, key ed25519.PrivateKey) *core.ApprovalEvidence {
		ev := core.ApprovalEvidence{Action: core.TrustAnchorActionRotate, AnchorID: id, Approver: "operator-1", Expiry: expiry, Nonce: "n-1", Replacement: &repl}
		ev.Signature = ed25519.Sign(key, ev.SigningPayload())
		return &ev
	}
	oldAnchor := config.TrustAnchor{ID: "old", Role: "attestation-signer", Algo: "ed25519", PublicKeyRef: "keys/old.pem",
		Fingerprint: "sha256:old", Status: config.TrustAnchorStatusActive}
	newAnchor := config.TrustAnchor{ID: "new", Role: "attestation-signer", Algo: "ed25519", PublicKeyRef: "keys/new.pem",
		Fingerprint: "sha256:new", Status: config.TrustAnchorStatusActive}

	setup := func(pins ...string) (*mutFixture, *core.TrustAnchorMutator) {
		f := &mutFixture{anchors: []config.TrustAnchor{oldAnchor}}
		return f, &core.TrustAnchorMutator{
			State: f, Audit: f,
			Pins:     pinProv{set: core.NewExternalPinSet(pins...)},
			Approver: core.NewEd25519ApproverVerifier(pub),
			Now:      func() time.Time { return now },
		}
	}

	t.Run("add rotate revoke share ordered path", func(t *testing.T) {
		f, m := setup("sha256:new")
		out, err := m.Apply(context.Background(), core.TrustAnchorMutation{Action: core.TrustAnchorActionAdd, Anchor: newAnchor})
		require.NoError(t, err)
		require.Equal(t, core.WriteOutcome{ConfigWritten: true, StatusWritten: true, AuditPersisted: true}, out)
		require.Equal(t, []string{"config", "status:new=active", "audit:applied"}, f.log)
		require.Len(t, f.anchors, 2)

		f, m = setup("sha256:new")
		rotAuth := &fakeRevocation{}
		m.Revocation = rotAuth
		rot := core.TrustAnchorMutation{Action: core.TrustAnchorActionRotate, AnchorID: "old", Anchor: newAnchor,
			Approval: rotateEvidence("old", newAnchor, now.Add(time.Hour), priv)}
		out, err = m.Apply(context.Background(), rot)
		require.NoError(t, err)
		require.True(t, out.ConfigWritten && out.StatusWritten && out.AuditPersisted && out.RevocationRecorded)
		require.Equal(t, []string{"sha256:old"}, rotAuth.tombstones)
		require.Equal(t, []string{"config", "status:old=revoked", "status:new=active", "audit:applied"}, f.log)
		require.Equal(t, "operator-1", f.audits[0].Approver)
		require.Equal(t, "old", f.audits[0].AnchorID)
		require.Equal(t, "new", f.audits[0].NewAnchorID)

		f, m = setup()
		auth := &fakeRevocation{}
		m.Revocation = auth // 168.009-T: revoke succeeds only via the external authority
		rev := core.TrustAnchorMutation{Action: core.TrustAnchorActionRevoke, AnchorID: "old",
			Approval: evidence(core.TrustAnchorActionRevoke, "old", now.Add(time.Hour), priv)}
		out, err = m.Apply(context.Background(), rev)
		require.NoError(t, err)
		require.True(t, out.StatusWritten && out.AuditPersisted && out.RevocationRecorded)
		require.Equal(t, []string{"status:old=revoked", "audit:applied"}, f.log)
		require.Equal(t, []string{"sha256:old"}, auth.tombstones)
		require.Equal(t, config.TrustAnchorStatusRevoked, f.anchors[0].Status)
	})

	t.Run("unpinned fingerprint rejected before state change with denial audit", func(t *testing.T) {
		f, m := setup() // empty (but present) pin set
		out, err := m.Apply(context.Background(), core.TrustAnchorMutation{Action: core.TrustAnchorActionAdd, Anchor: newAnchor})
		require.ErrorIs(t, err, core.ErrTrustAnchorNotPinned)
		var me *core.TrustAnchorMutationError
		require.ErrorAs(t, err, &me)
		require.Equal(t, core.WriteOutcome{AuditPersisted: true}, out)
		require.Equal(t, []string{"audit:denied"}, f.log)
		require.Equal(t, core.TrustAnchorAuditDenied, f.audits[0].Outcome)
		require.Equal(t, "sha256:new", f.audits[0].Fingerprint)
		require.Equal(t, []config.TrustAnchor{oldAnchor}, f.anchors)

		f, m = setup()
		f.auditErr = errors.New("disk full")
		_, err = m.Apply(context.Background(), core.TrustAnchorMutation{Action: core.TrustAnchorActionAdd, Anchor: newAnchor})
		require.ErrorIs(t, err, core.ErrTrustAnchorNotPinned)
		require.ErrorIs(t, err, f.auditErr)
		require.Equal(t, []string{"audit:denied"}, f.log)
	})

	t.Run("rotate and revoke refuse missing invalid or forged approval", func(t *testing.T) {
		cases := []struct {
			name string
			ev   *core.ApprovalEvidence
			want error
		}{
			{"absent", nil, core.ErrApprovalRequired},
			{"forged signature", evidence(core.TrustAnchorActionRevoke, "old", now.Add(time.Hour), otherPriv), core.ErrApprovalInvalid},
			{"wrong action", evidence(core.TrustAnchorActionRotate, "old", now.Add(time.Hour), priv), core.ErrApprovalInvalid},
			{"wrong anchor", evidence(core.TrustAnchorActionRevoke, "other", now.Add(time.Hour), priv), core.ErrApprovalInvalid},
			{"expired", evidence(core.TrustAnchorActionRevoke, "old", now.Add(-time.Second), priv), core.ErrApprovalInvalid},
			{"unsigned", &core.ApprovalEvidence{Action: core.TrustAnchorActionRevoke, AnchorID: "old", Expiry: now.Add(time.Hour), Nonce: "n"}, core.ErrApprovalInvalid},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				f, m := setup("sha256:new")
				out, err := m.Apply(context.Background(), core.TrustAnchorMutation{Action: core.TrustAnchorActionRevoke, AnchorID: "old", Approval: tc.ev})
				require.ErrorIs(t, err, tc.want)
				require.Equal(t, core.WriteOutcome{}, out)
				require.Empty(t, f.log)
				require.Equal(t, []config.TrustAnchor{oldAnchor}, f.anchors)

				if tc.name == "wrong action" {
					return // this evidence is legitimately a rotate approval
				}
				f, m = setup("sha256:new")
				_, err = m.Apply(context.Background(), core.TrustAnchorMutation{Action: core.TrustAnchorActionRotate, AnchorID: "old", Anchor: newAnchor, Approval: tc.ev})
				require.Error(t, err)
				require.Empty(t, f.log)
			})
		}

		f, m := setup()
		m.Approver = nil
		_, err := m.Apply(context.Background(), core.TrustAnchorMutation{Action: core.TrustAnchorActionRevoke, AnchorID: "old",
			Approval: evidence(core.TrustAnchorActionRevoke, "old", now.Add(time.Hour), priv)})
		require.ErrorIs(t, err, core.ErrApprovalInvalid)
		require.Empty(t, f.log)
	})

	t.Run("rotate approval binds the replacement anchor", func(t *testing.T) {
		other := newAnchor
		other.Fingerprint = "sha256:evil"
		f, m := setup("sha256:new", "sha256:evil")
		m.Revocation = &fakeRevocation{}
		_, err := m.Apply(context.Background(), core.TrustAnchorMutation{Action: core.TrustAnchorActionRotate, AnchorID: "old", Anchor: other,
			Approval: rotateEvidence("old", newAnchor, now.Add(time.Hour), priv)})
		require.ErrorIs(t, err, core.ErrApprovalInvalid)
		require.Empty(t, f.log)
		// evidence without a replacement is also refused
		_, err = m.Apply(context.Background(), core.TrustAnchorMutation{Action: core.TrustAnchorActionRotate, AnchorID: "old", Anchor: newAnchor,
			Approval: evidence(core.TrustAnchorActionRotate, "old", now.Add(time.Hour), priv)})
		require.ErrorIs(t, err, core.ErrApprovalInvalid)
		require.Empty(t, f.log)
	})

	t.Run("approval lifetime is bounded", func(t *testing.T) {
		f, m := setup()
		m.Revocation = &fakeRevocation{}
		_, err := m.Apply(context.Background(), core.TrustAnchorMutation{Action: core.TrustAnchorActionRevoke, AnchorID: "old",
			Approval: evidence(core.TrustAnchorActionRevoke, "old", now.Add(25*time.Hour), priv)})
		require.ErrorIs(t, err, core.ErrApprovalInvalid)
		require.Empty(t, f.log)
	})

	t.Run("rotate without revocation authority fails closed", func(t *testing.T) {
		f, m := setup("sha256:new")
		out, err := m.Apply(context.Background(), core.TrustAnchorMutation{Action: core.TrustAnchorActionRotate, AnchorID: "old", Anchor: newAnchor,
			Approval: rotateEvidence("old", newAnchor, now.Add(time.Hour), priv)})
		require.ErrorIs(t, err, core.ErrRevocationAuthorityRequired)
		require.Contains(t, err.Error(), "sha256:old")
		require.False(t, out.ConfigWritten || out.StatusWritten || out.RevocationRecorded)
		require.Equal(t, []string{"audit:denied"}, f.log)
		require.Equal(t, []config.TrustAnchor{oldAnchor}, f.anchors)

		auth := &fakeRevocation{recordErr: errors.New("authority down")}
		f, m = setup("sha256:new")
		m.Revocation = auth
		_, err = m.Apply(context.Background(), core.TrustAnchorMutation{Action: core.TrustAnchorActionRotate, AnchorID: "old", Anchor: newAnchor,
			Approval: rotateEvidence("old", newAnchor, now.Add(time.Hour), priv)})
		require.ErrorIs(t, err, core.ErrWriteNotApplied)
		require.Empty(t, f.log)
	})

	t.Run("invalid requests are rejected without state change", func(t *testing.T) {
		f, m := setup("sha256:new")
		_, err := m.Apply(context.Background(), core.TrustAnchorMutation{Action: "bogus"})
		require.ErrorIs(t, err, core.ErrTrustAnchorMutationInvalid)
		dup := newAnchor
		dup.ID = "old"
		_, err = m.Apply(context.Background(), core.TrustAnchorMutation{Action: core.TrustAnchorActionAdd, Anchor: dup})
		require.ErrorIs(t, err, core.ErrTrustAnchorMutationInvalid)
		_, err = m.Apply(context.Background(), core.TrustAnchorMutation{Action: core.TrustAnchorActionRevoke, AnchorID: "ghost",
			Approval: evidence(core.TrustAnchorActionRevoke, "ghost", now.Add(time.Hour), priv)})
		require.ErrorIs(t, err, core.ErrTrustAnchorMutationInvalid)
		require.Empty(t, f.log)
	})
}
