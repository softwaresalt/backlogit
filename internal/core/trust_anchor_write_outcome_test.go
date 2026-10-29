package core_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/config"
	"github.com/softwaresalt/backlogit/internal/core"
)

// woFixture records ordered protocol steps and injects failures per step.
type woFixture struct {
	log                                       []string
	anchors                                   []config.TrustAnchor
	markerPresent                             bool
	markerErr, configErr, statusErr, auditErr error
	clearErr                                  error
}

func (f *woFixture) LoadAnchors(context.Context) ([]config.TrustAnchor, error) {
	return append([]config.TrustAnchor(nil), f.anchors...), nil
}

func (f *woFixture) WriteConfig(_ context.Context, a []config.TrustAnchor) error {
	f.log = append(f.log, "config")
	if f.configErr != nil {
		return f.configErr
	}
	f.anchors = append([]config.TrustAnchor(nil), a...)
	return nil
}

func (f *woFixture) WriteStatus(_ context.Context, c core.TrustAnchorStatusChange) error {
	f.log = append(f.log, "status:"+c.ID)
	if f.statusErr != nil {
		return f.statusErr
	}
	for i := range f.anchors {
		if f.anchors[i].ID == c.ID {
			f.anchors[i].Status = c.Status
		}
	}
	return nil
}

func (f *woFixture) RecordAudit(_ context.Context, ev core.TrustAnchorAuditEvent) error {
	f.log = append(f.log, "audit:"+ev.Outcome)
	return f.auditErr
}

func (f *woFixture) WriteMarker(context.Context, core.TrustAnchorMarker) error {
	f.log = append(f.log, "marker:write")
	if f.markerErr != nil {
		return f.markerErr
	}
	f.markerPresent = true
	return nil
}

func (f *woFixture) ClearMarker(context.Context) error {
	f.log = append(f.log, "marker:clear")
	if f.clearErr != nil {
		return f.clearErr
	}
	f.markerPresent = false
	return nil
}

func TestTrustAnchorWriteOutcome(t *testing.T) {
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	base := config.TrustAnchor{ID: "old", Role: "attestation-signer", Algo: "ed25519", PublicKeyRef: "keys/old.pem",
		Fingerprint: "sha256:old", Status: config.TrustAnchorStatusActive}
	add := core.TrustAnchorMutation{Action: core.TrustAnchorActionAdd, Anchor: config.TrustAnchor{ID: "new", Role: "attestation-signer",
		Algo: "ed25519", PublicKeyRef: "keys/new.pem", Fingerprint: "sha256:new"}}
	boom := errors.New("injected failure")

	setup := func(pins ...string) (*woFixture, *core.TrustAnchorMutator) {
		f := &woFixture{anchors: []config.TrustAnchor{base}}
		return f, &core.TrustAnchorMutator{State: f, Audit: f, Marker: f,
			Pins: pinProv{set: core.NewExternalPinSet(pins...)}, Now: func() time.Time { return now }}
	}

	t.Run("happy path follows the ordered protocol", func(t *testing.T) {
		f, m := setup("sha256:new")
		out, err := m.Apply(context.Background(), add)
		require.NoError(t, err)
		require.True(t, out.AuditPersisted)
		require.False(t, out.MarkerOutstanding)
		require.Equal(t, []string{"marker:write", "config", "status:new", "audit:applied", "marker:clear"}, f.log)
		require.False(t, f.markerPresent)
	})

	t.Run("failure before the mutation is ErrWriteNotApplied with no state change and marker absent", func(t *testing.T) {
		for name, inject := range map[string]func(*woFixture){
			"marker write fails": func(f *woFixture) { f.markerErr = boom },
			"config write fails": func(f *woFixture) { f.configErr = boom },
		} {
			t.Run(name, func(t *testing.T) {
				f, m := setup("sha256:new")
				inject(f)
				out, err := m.Apply(context.Background(), add)
				require.ErrorIs(t, err, core.ErrWriteNotApplied)
				require.NotErrorIs(t, err, core.ErrWriteIndeterminate)
				require.ErrorIs(t, err, boom)
				require.Equal(t, []config.TrustAnchor{base}, f.anchors)
				require.False(t, f.markerPresent, "marker must be absent after a not-applied failure")
				require.False(t, out.MarkerOutstanding)
				require.False(t, out.AuditPersisted)
			})
		}
	})

	t.Run("failure after the mutation is ErrWriteIndeterminate with the marker left in place", func(t *testing.T) {
		for name, inject := range map[string]func(*woFixture){
			"status write fails after config": func(f *woFixture) { f.statusErr = boom },
			"audit persist fails":             func(f *woFixture) { f.auditErr = boom },
			"marker clear fails":              func(f *woFixture) { f.clearErr = boom },
		} {
			t.Run(name, func(t *testing.T) {
				f, m := setup("sha256:new")
				inject(f)
				out, err := m.Apply(context.Background(), add)
				require.ErrorIs(t, err, core.ErrWriteIndeterminate)
				require.NotErrorIs(t, err, core.ErrWriteNotApplied)
				require.ErrorIs(t, err, boom)
				require.True(t, f.markerPresent, "marker must remain for operator visibility")
				require.True(t, out.MarkerOutstanding)
				require.True(t, out.ConfigWritten)
				// No automatic roll-forward or retry: each step ran at most once.
				counts := map[string]int{}
				for _, s := range f.log {
					counts[s]++
				}
				for step, n := range counts {
					require.Equal(t, 1, n, "step %s repeated (no auto roll-forward)", step)
				}
			})
		}
	})

	t.Run("denied mutation whose denial audit fails is ErrAuditNotPersisted with no state change", func(t *testing.T) {
		f, m := setup() // new fingerprint not pinned => denied
		f.auditErr = boom
		out, err := m.Apply(context.Background(), add)
		require.ErrorIs(t, err, core.ErrAuditNotPersisted)
		require.ErrorIs(t, err, core.ErrTrustAnchorNotPinned, "the denial stands")
		require.ErrorIs(t, err, boom, "the audit error is never ignored")
		require.False(t, out.AuditPersisted)
		require.Equal(t, []config.TrustAnchor{base}, f.anchors)
		require.False(t, f.markerPresent)
		require.Equal(t, []string{"audit:denied"}, f.log, "no marker or state step on the denied path")
	})

	t.Run("file marker store round-trips and is detectable", func(t *testing.T) {
		dir := t.TempDir()
		got, err := core.OutstandingTrustAnchorMarker(dir)
		require.NoError(t, err)
		require.Nil(t, got)

		store := core.FileTrustAnchorMarkerStore{Path: filepath.Join(dir, core.TrustAnchorMarkerFile)}
		require.NoError(t, store.WriteMarker(context.Background(), core.TrustAnchorMarker{Action: core.TrustAnchorActionRevoke, AnchorID: "old", At: now}))
		got, err = core.OutstandingTrustAnchorMarker(dir)
		require.NoError(t, err)
		require.NotNil(t, got)
		require.Equal(t, "old", got.AnchorID)

		require.NoError(t, store.ClearMarker(context.Background()))
		require.NoError(t, store.ClearMarker(context.Background()), "clearing an absent marker is not an error")
		got, err = core.OutstandingTrustAnchorMarker(dir)
		require.NoError(t, err)
		require.Nil(t, got)
	})
}
