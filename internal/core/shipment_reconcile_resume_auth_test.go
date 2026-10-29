package core

import (
	"context"
	"os"
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
	raw := classifierEvent(t, f.shipmentID, f.req.IdempotencyKey, f.digest, func(d *ShipmentReconciledShippedDelta) {
		d.Approver = approver
	})
	return classifierFrontmatter("shipped", f.req.IdempotencyKey, f.digest, raw, ShipmentReconciledShippedEventDigest(raw))
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
		require.NoError(t, f.provider.CompareAndConsumeNonce(ctx, "00ff", time.Now().Add(time.Hour)))
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
