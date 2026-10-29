package core

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/events"
)

// resumeAuthFixture builds persisted resume markers (prepared event, no
// logged event) for the seam fixture's shipment. approver is recorded in the
// prepared event; nil records none.
func resumeAuthFixture(t *testing.T, f authSeamFixture, approver *ShipmentReconciledShippedApprover) map[string]any {
	t.Helper()
	require.NoError(t, os.MkdirAll(WorkspaceLogsRoot(f.ws.RootPath), 0o755))
	return f.resumeFixtureMutated(t, approver, nil)
}

// resumeFixtureMutated is resumeAuthFixture whose prepared event request
// fields mirror the fixture request, with mutate applied afterwards (a forged
// prepared event).
func (f authSeamFixture) resumeFixtureMutated(t *testing.T, approver *ShipmentReconciledShippedApprover, mutate func(*ShipmentReconciledShippedDelta)) map[string]any {
	t.Helper()
	require.NoError(t, os.MkdirAll(WorkspaceLogsRoot(f.ws.RootPath), 0o755))
	raw := classifierEvent(t, f.shipmentID, f.req.IdempotencyKey, f.digest, func(d *ShipmentReconciledShippedDelta) {
		d.Approver = approver
		d.Reason = f.req.Reason
		d.Actor = f.req.Actor
		d.SecondApprover = f.req.SecondApprover
		d.Evidence.MergeSHA = strings.ToLower(f.req.MergeSHA)
		d.Evidence.ClosureEvidence = f.req.ClosureEvidence
		d.Evidence.EvidenceRefs = append([]string{}, f.req.EvidenceRefs...)
		if mutate != nil {
			mutate(d)
		}
	})
	return classifierFrontmatter("shipped", f.req.IdempotencyKey, f.digest, raw, ShipmentReconciledShippedEventDigest(raw))
}

type failingConsumeAuditor struct{}

func (failingConsumeAuditor) RecordNonceConsume(NonceConsumeEvent) error {
	return errors.New("audit sink down")
}

// withFailingAudit returns req whose nonce consumer consumes at the provider
// but fails the consume audit (indeterminate).
func (f authSeamFixture) withFailingAudit(t *testing.T, req ShipmentShippedReconcileRequest) ShipmentShippedReconcileRequest {
	t.Helper()
	projection, err := OpenNonceLedger(filepath.Join(t.TempDir(), "nonces-fail.jsonl"))
	require.NoError(t, err)
	consumer, err := NewNonceConsumer(f.provider, projection, failingConsumeAuditor{})
	require.NoError(t, err)
	auth := *req.Authorization
	auth.Nonces = consumer
	req.Authorization = &auth
	return req
}

func TestReconcileResumeForgedPreparedEvent(t *testing.T) {
	ctx := context.Background()
	approver := &ShipmentReconciledShippedApprover{KeyID: "op-1", Algo: "ed25519", Nonce: "0f01"}
	forgeries := map[string]func(*ShipmentReconciledShippedDelta){
		"reason":          func(d *ShipmentReconciledShippedDelta) { d.Reason = "forged reason" },
		"actor":           func(d *ShipmentReconciledShippedDelta) { d.Actor = "mallory" },
		"merge sha":       func(d *ShipmentReconciledShippedDelta) { d.Evidence.MergeSHA = strings.Repeat("a", 40) },
		"closure path":    func(d *ShipmentReconciledShippedDelta) { d.Evidence.ClosureEvidence = "docs/other.md" },
		"idempotency key": func(d *ShipmentReconciledShippedDelta) { d.IdempotencyKey = "forged-key" },
	}
	for name, mutate := range forgeries {
		t.Run("consumed token with forged prepared "+name+" is rejected", func(t *testing.T) {
			f := newAuthSeamFixture(t)
			fm := f.resumeFixtureMutated(t, approver, mutate)
			require.NoError(t, f.nonces.ConsumeBound(ctx, "0f01", time.Now().Add(time.Hour), f.digest))
			req := f.authorized(f.token(f.shipmentID, f.digest, "0f01"))
			_, err := reconcileShipmentPhaseBNoOp(ctx, f.ws, req, f.shipmentID, fm, nil, ShipmentShippedReconcileResult{})
			require.ErrorIs(t, err, ErrAuthorizationRejected)
			assert.Empty(t, f.resumeLogged(t))
		})
	}
}

func TestReconcileResumeConsumedNonceBinding(t *testing.T) {
	ctx := context.Background()
	approver := &ShipmentReconciledShippedApprover{KeyID: "op-1", Algo: "ed25519", Nonce: "0f21"}
	for name, digest := range map[string]string{"unbound consume": "", "consumed against another digest": "deadbeef"} {
		t.Run(name+" is rejected on resume", func(t *testing.T) {
			f := newAuthSeamFixture(t)
			fm := resumeAuthFixture(t, f, approver)
			require.NoError(t, f.nonces.ConsumeBound(ctx, "0f21", time.Now().Add(time.Hour), digest))
			req := f.authorized(f.token(f.shipmentID, f.digest, "0f21"))
			_, err := reconcileShipmentPhaseBNoOp(ctx, f.ws, req, f.shipmentID, fm, nil, ShipmentShippedReconcileResult{})
			require.ErrorIs(t, err, ErrAuthorizationRejected)
			assert.Empty(t, f.resumeLogged(t))
		})
	}
}

func TestReconcileIndeterminateNonceConsumeOutcome(t *testing.T) {
	ctx := context.Background()
	t.Run("main gate reports the indeterminate outcome", func(t *testing.T) {
		f := newAuthSeamFixture(t)
		req := f.withFailingAudit(t, f.authorized(f.token(f.shipmentID, f.digest, "0f11")))
		res, err := ReconcileShipmentToShipped(ctx, f.ws, req)
		require.ErrorIs(t, err, ErrNonceConsumeIndeterminate)
		assert.Equal(t, ShipmentReconcileOutcomeIndeterminate, res.Outcome)
	})
	t.Run("resume path reports the indeterminate outcome", func(t *testing.T) {
		f := newAuthSeamFixture(t)
		fm := resumeAuthFixture(t, f, &ShipmentReconciledShippedApprover{KeyID: "op-1", Algo: "ed25519", Nonce: "0f12"})
		req := f.withFailingAudit(t, f.authorized(f.token(f.shipmentID, f.digest, "0f12")))
		res, err := reconcileShipmentPhaseBNoOp(ctx, f.ws, req, f.shipmentID, fm, nil, ShipmentShippedReconcileResult{})
		require.ErrorIs(t, err, ErrNonceConsumeIndeterminate)
		assert.Equal(t, ShipmentReconcileOutcomeIndeterminate, res.Outcome)
	})
}

func (f authSeamFixture) resumeLogged(t *testing.T) []events.Event {
	t.Helper()
	logged, err := events.ReadAllEvents(context.Background(), WorkspaceLogsRoot(f.ws.RootPath), f.shipmentID)
	require.NoError(t, err)
	var reconciled []events.Event
	for _, ev := range logged {
		if ev.EventType == EventShipmentReconciledShipped {
			reconciled = append(reconciled, ev)
		}
	}
	return reconciled
}

func TestReconcileResumeAuthorization(t *testing.T) {
	ctx := context.Background()
	approverFor := func(nonce string) *ShipmentReconciledShippedApprover {
		return &ShipmentReconciledShippedApprover{KeyID: "op-1", Algo: "ed25519", Nonce: nonce}
	}

	for _, dry := range []bool{false, true} {
		name := "live"
		if dry {
			name = "dry-run"
		}
		t.Run(name+": authenticated resume without a token is rejected", func(t *testing.T) {
			f := newAuthSeamFixture(t)
			fm := resumeAuthFixture(t, f, approverFor("00aa"))
			req := f.req
			req.AuthEnforcement = AuthEnforcementAuthenticated
			req.DryRun = dry
			_, err := reconcileShipmentPhaseBNoOp(ctx, f.ws, req, f.shipmentID, fm, nil, ShipmentShippedReconcileResult{})
			require.ErrorIs(t, err, ErrAuthorizationRequired)
			assert.Empty(t, f.resumeLogged(t))
		})

		t.Run(name+": authenticated resume with a rejected token writes nothing", func(t *testing.T) {
			f := newAuthSeamFixture(t)
			fm := resumeAuthFixture(t, f, approverFor("00bb"))
			tok := f.token(f.shipmentID, "tampered", "00bb")
			req := f.authorized(tok)
			req.DryRun = dry
			_, err := reconcileShipmentPhaseBNoOp(ctx, f.ws, req, f.shipmentID, fm, nil, ShipmentShippedReconcileResult{})
			require.ErrorIs(t, err, ErrAuthorizationRejected)
			assert.Empty(t, f.resumeLogged(t))
			assert.Zero(t, f.provider.count())
		})

		t.Run(name+": prepared event approver differing from the token is rejected", func(t *testing.T) {
			f := newAuthSeamFixture(t)
			fm := resumeAuthFixture(t, f, approverFor("00cc"))
			req := f.authorized(f.token(f.shipmentID, f.digest, "00dd"))
			req.DryRun = dry
			_, err := reconcileShipmentPhaseBNoOp(ctx, f.ws, req, f.shipmentID, fm, nil, ShipmentShippedReconcileResult{})
			require.ErrorIs(t, err, ErrAuthorizationRejected)
			assert.Empty(t, f.resumeLogged(t))
			assert.Zero(t, f.provider.count(), "a mismatched token must not burn its nonce")
		})

		t.Run(name+": prepared event without approver is rejected under authenticated mode", func(t *testing.T) {
			f := newAuthSeamFixture(t)
			fm := resumeAuthFixture(t, f, nil)
			req := f.authorized(f.token(f.shipmentID, f.digest, "00ee"))
			req.DryRun = dry
			_, err := reconcileShipmentPhaseBNoOp(ctx, f.ws, req, f.shipmentID, fm, nil, ShipmentShippedReconcileResult{})
			require.ErrorIs(t, err, ErrAuthorizationRejected)
			assert.Empty(t, f.resumeLogged(t))
		})
	}

	t.Run("live: nonce already consumed for the same prepared approver resumes", func(t *testing.T) {
		f := newAuthSeamFixture(t)
		fm := resumeAuthFixture(t, f, approverFor("00ff"))
		require.NoError(t, f.nonces.ConsumeBound(ctx, "00ff", time.Now().Add(time.Hour), f.digest))
		req := f.authorized(f.token(f.shipmentID, f.digest, "00ff"))
		res, err := reconcileShipmentPhaseBNoOp(ctx, f.ws, req, f.shipmentID, fm, nil, ShipmentShippedReconcileResult{})
		require.NoError(t, err)
		assert.Equal(t, AuthorizationOutcomeConsumed, res.Authorization)
		assert.Len(t, f.resumeLogged(t), 1)
	})

	t.Run("live: replayed nonce with a different prepared approver stays rejected", func(t *testing.T) {
		f := newAuthSeamFixture(t)
		fm := resumeAuthFixture(t, f, approverFor("0011"))
		require.NoError(t, f.provider.CompareAndConsumeNonce(ctx, "0022", time.Now().Add(time.Hour)))
		req := f.authorized(f.token(f.shipmentID, f.digest, "0022"))
		_, err := reconcileShipmentPhaseBNoOp(ctx, f.ws, req, f.shipmentID, fm, nil, ShipmentShippedReconcileResult{})
		require.ErrorIs(t, err, ErrAuthorizationRejected)
		assert.Empty(t, f.resumeLogged(t))
	})

	t.Run("dry run: valid token verifies without consuming or appending", func(t *testing.T) {
		f := newAuthSeamFixture(t)
		fm := resumeAuthFixture(t, f, approverFor("0033"))
		req := f.authorized(f.token(f.shipmentID, f.digest, "0033"))
		req.DryRun = true
		res, err := reconcileShipmentPhaseBNoOp(ctx, f.ws, req, f.shipmentID, fm, nil, ShipmentShippedReconcileResult{})
		require.NoError(t, err)
		assert.Equal(t, AuthorizationOutcomeVerified, res.Authorization)
		assert.Zero(t, f.provider.count())
		assert.Empty(t, f.resumeLogged(t))
	})

	t.Run("live: fresh valid token matching the prepared approver appends", func(t *testing.T) {
		f := newAuthSeamFixture(t)
		fm := resumeAuthFixture(t, f, approverFor("0044"))
		req := f.authorized(f.token(f.shipmentID, f.digest, "0044"))
		res, err := reconcileShipmentPhaseBNoOp(ctx, f.ws, req, f.shipmentID, fm, nil, ShipmentShippedReconcileResult{})
		require.NoError(t, err)
		assert.Equal(t, AuthorizationOutcomeConsumed, res.Authorization)
		assert.Len(t, f.resumeLogged(t), 1)
	})

	t.Run("confirmation mode resume is unchanged", func(t *testing.T) {
		f := newAuthSeamFixture(t)
		fm := resumeAuthFixture(t, f, nil)
		res, err := reconcileShipmentPhaseBNoOp(ctx, f.ws, f.req, f.shipmentID, fm, nil, ShipmentShippedReconcileResult{})
		require.NoError(t, err)
		assert.Empty(t, res.Authorization)
		assert.Len(t, f.resumeLogged(t), 1)
	})
}

func TestAuthorizeApprovalDryRunWritesNoRejectionAudit(t *testing.T) {
	ctx := context.Background()
	t.Run("dry run: verifier and digest rejections are not audited", func(t *testing.T) {
		f := newAuthSeamFixture(t)
		bad := f.token(f.shipmentID, "tampered", "0a01")
		_, err := AuthorizeApproval(ctx, *f.authorized(bad).Authorization, f.shipmentID, f.digest, true)
		require.ErrorIs(t, err, ErrAuthorizationRejected)
		wrongShip := f.token("other-S", f.digest, "0a02")
		_, err = AuthorizeApproval(ctx, *f.authorized(wrongShip).Authorization, f.shipmentID, f.digest, true)
		require.ErrorIs(t, err, ErrAuthorizationRejected)
		assert.Empty(t, f.rejections.events)
	})
	t.Run("live: rejections remain audited", func(t *testing.T) {
		f := newAuthSeamFixture(t)
		bad := f.token(f.shipmentID, "tampered", "0a03")
		_, err := AuthorizeApproval(ctx, *f.authorized(bad).Authorization, f.shipmentID, f.digest, false)
		require.ErrorIs(t, err, ErrAuthorizationRejected)
		wrongShip := f.token("other-S", f.digest, "0a04")
		_, err = AuthorizeApproval(ctx, *f.authorized(wrongShip).Authorization, f.shipmentID, f.digest, false)
		require.ErrorIs(t, err, ErrAuthorizationRejected)
		assert.Len(t, f.rejections.events, 2)
	})
}
