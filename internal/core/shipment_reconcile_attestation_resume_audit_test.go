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

func resumeAuditFixture(t *testing.T) (*Workspace, string, map[string]any, ShipmentShippedReconcileRequest) {
	t.Helper()
	ws := setupShipmentWorkspace(t)
	const shipmentID = "169-S-resume-audit"
	require.NoError(t, os.MkdirAll(WorkspaceLogsRoot(ws.RootPath), 0o755))
	raw := signedClassifierEvent(t, shipmentID, "idem-001", "abc123", reconcileTestAttestation())
	fm := classifierFrontmatter("shipped", "idem-001", "abc123", raw, ShipmentReconciledShippedEventDigest(raw))
	req := ShipmentShippedReconcileRequest{IdempotencyKey: "idem-001", Attestation: reconcileTestAttestation()}
	req.AttestationVerifier = func(AttestationExpectedEvidence) error {
		return &AttestationVerifyError{Field: "expiry", Err: ErrAttestationSignatureInvalid}
	}
	return ws, shipmentID, fm, req
}

func TestReconcileResumeAuditsRejectionUnderHeldLock(t *testing.T) {
	ws, shipmentID, fm, req := resumeAuditFixture(t)
	req.AttestationRejectionAuditor = func(ctx context.Context, rejection error) error {
		return AuditAttestationRejection(ctx, ws, shipmentID, "k", rejection)
	}

	bg, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	heldCtx, unlock, err := events.LockItemLogCrossProcess(bg, WorkspaceLocksRoot(ws.RootPath), WorkspaceLogsRoot(ws.RootPath), shipmentID)
	require.NoError(t, err)
	defer unlock()

	_, err = reconcileShipmentPhaseBNoOp(heldCtx, ws, req, shipmentID, fm, nil, ShipmentShippedReconcileResult{})
	var ve *AttestationVerifyError
	require.ErrorAs(t, err, &ve)

	logged, err := events.ReadAllEvents(context.Background(), WorkspaceLogsRoot(ws.RootPath), shipmentID)
	require.NoError(t, err)
	require.Len(t, logged, 1)
	assert.Equal(t, EventAttestationRejected, logged[0].EventType)
}

func TestReconcileResumeDryRunVerifiesPersistedAttestation(t *testing.T) {
	ws, shipmentID, fm, req := resumeAuditFixture(t)
	req.DryRun = true
	called := false
	req.AttestationRejectionAuditor = func(context.Context, error) error { called = true; return nil }

	_, err := reconcileShipmentPhaseBNoOp(context.Background(), ws, req, shipmentID, fm, nil, ShipmentShippedReconcileResult{})
	var ve *AttestationVerifyError
	require.ErrorAs(t, err, &ve, "dry run must reject what live resume would reject")
	assert.False(t, called, "dry run stays write-free")
	_, statErr := os.Stat(events.LogPathForItem(WorkspaceLogsRoot(ws.RootPath), shipmentID))
	assert.True(t, os.IsNotExist(statErr))
}
