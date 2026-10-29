package core

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/config"
	"github.com/softwaresalt/backlogit/internal/events"
)

// 169.008-T persistent RED harness: genuine pre-169.007 unsigned-metadata
// acceptance. Both tests are driven green only by the 169.007-T signed
// envelope persistence and doctor re-verification.

// TestAttestationUnsignedAcceptNegative_ForgedMetadataIsNotShippedEvidence
// proves a reconcile event carrying only forged attestation metadata (no
// durable signed envelope) must not suppress the missing-shipped-event
// finding.
func TestAttestationUnsignedAcceptNegative_ForgedMetadataIsNotShippedEvidence(t *testing.T) {
	ws := setupShipmentWorkspace(t)
	shipment := seedArchivedShippedNoEvent(t, ws, "forged attestation metadata", nil)

	delta := validReconciledShippedDelta()
	delta.Attestation = reconcileTestAttestation()
	logPath := events.LogPathForItem(WorkspaceLogsRoot(ws.RootPath), shipment.ID)
	require.NoError(t, os.MkdirAll(WorkspaceLogsRoot(ws.RootPath), 0o755))
	require.NoError(t, os.WriteFile(logPath, marshalReconciledShippedEvent(t, delta, shipment.ID), 0o644))

	report := runShippedEventAudit(t, ws)
	require.Len(t, findingsOfType(report, FindingMissingShippedEvent), 1,
		"an unsigned-metadata-only attestation must fail closed, not count as shipped evidence")
}

// TestAttestationUnsignedAcceptNegative_SignedEnvelopeIsPersisted proves the
// reconcile event persists the signed envelope so doctor has signed material
// to re-verify offline.
func TestAttestationUnsignedAcceptNegative_SignedEnvelopeIsPersisted(t *testing.T) {
	fixture := setupShipmentReconcileEvidenceFixture(t)
	fixture.ws.Config.Reconcile = &config.ReconcileConfig{}
	signer := newAttestationTestSigner(t, "ci-signer")
	envelope := signer.signEnvelope(t, signer.id, attestationTestStatement(AttestationExpectedEvidence{ShipmentID: "048-S"}, time.Now().UTC()))

	claimJSON, err := json.Marshal(attestationTestClaim(t, envelope, signer.id))
	require.NoError(t, err)
	var claim map[string]any
	require.NoError(t, json.Unmarshal(claimJSON, &claim))
	claim["envelope"] = string(envelope)
	withEnvelope, err := json.Marshal(claim)
	require.NoError(t, err)
	var att ShipmentReconciledShippedAttestation
	require.NoError(t, json.Unmarshal(withEnvelope, &att))

	req := fixture.request("048-S", fixture.merge048)
	req.Attestation = &att
	prepared, err := prepareShipmentReconcileEvidence(context.Background(), fixture.ws, req, fixedShipmentReconcileEvidenceInput())
	require.NoError(t, err)

	var wrapper events.Event
	require.NoError(t, json.Unmarshal(prepared.EventBytes, &wrapper))
	persisted, ok := wrapper.Delta["attestation"].(map[string]any)
	require.True(t, ok, "event must carry an attestation member")
	require.Equal(t, string(envelope), persisted["envelope"], "signed envelope must be persisted in the event binding")
}
