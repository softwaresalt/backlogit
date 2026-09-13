package core

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	blerrors "github.com/softwaresalt/backlogit/internal/errors"
	"github.com/softwaresalt/backlogit/internal/events"
	"github.com/softwaresalt/backlogit/internal/models"
)

func validShipmentReconcilePreconditionRequest(shipmentID string) ShipmentShippedReconcileRequest {
	return ShipmentShippedReconcileRequest{
		ShipmentID:      shipmentID,
		Reason:          "governed repair",
		Actor:           "operator",
		SecondApprover:  "auditor",
		IdempotencyKey:  "idem-167-014",
		MergeSHA:        strings.Repeat("a", 40),
		ClosureEvidence: filepath.Join("docs", "closure", "167-014-test.md"),
		EvidenceRefs:    []string{filepath.Join("docs", "evidence", "167-014-test.txt")},
	}
}

func mustShipmentReconcileRequestIdentityDigest(t *testing.T, req ShipmentShippedReconcileRequest) string {
	t.Helper()
	digest, err := shipmentReconcileRequestIdentityDigest(req)
	require.NoError(t, err)
	return digest
}

func createShipmentReconcileMemberWithStatus(t *testing.T, ws *Workspace, status models.ArtifactStatus) string {
	t.Helper()
	ctx := context.Background()
	if status == models.StatusActive {
		return newActiveTask(t, ws)
	}
	feature, err := CreateArtifact(ctx, ws, "167.014-T member feature", "feature")
	require.NoError(t, err)
	member, err := CreateArtifact(ctx, ws, "167.014-T member task", "task", WithParent(feature.ID))
	require.NoError(t, err)
	if status != models.StatusQueued {
		_, err = setArtifactStatus(ctx, ws, member.ID, status, "167.014-T test fixture")
		require.NoError(t, err)
	}
	return member.ID
}

func createArchivedShipmentReconcileMemberFromStatus(t *testing.T, ws *Workspace, status models.ArtifactStatus) string {
	t.Helper()
	ctx := context.Background()
	id := createShipmentReconcileMemberWithStatus(t, ws, status)
	_, err := ArchiveItem(ctx, ws.DB, ws, id)
	require.NoError(t, err)
	return id
}

func createArchivedShipmentReconcileFixture(t *testing.T, ws *Workspace, memberIDs []string) *models.Artifact {
	t.Helper()
	ctx := context.Background()
	shipment, err := CreateShipment(ctx, ws, "167.014-T archived shipment", memberIDs)
	require.NoError(t, err)
	_, err = ClaimShipment(ctx, ws, shipment.ID)
	require.NoError(t, err)
	_, err = ArchiveItem(ctx, ws.DB, ws, shipment.ID)
	require.NoError(t, err)
	archived, err := findArtifact(ctx, ws, shipment.ID)
	require.NoError(t, err)
	return archived
}

func rewriteArtifactFile(t *testing.T, ws *Workspace, id string, mutate func(string) string) {
	t.Helper()
	path, err := FindArtifactPath(context.Background(), ws, id)
	require.NoError(t, err)
	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, []byte(mutate(string(raw))), 0o644))
}

func writeShipmentReconcileConflictLog(t *testing.T, ws *Workspace, shipmentID string, req ShipmentShippedReconcileRequest, requestIdentityDigest string) {
	t.Helper()
	logPath := events.LogPathForItem(WorkspaceLogsRoot(ws.RootPath), shipmentID)
	require.NoError(t, os.MkdirAll(filepath.Dir(logPath), 0o755))
	delta := validReconciledShippedDelta()
	delta.IdempotencyKey = req.IdempotencyKey
	delta.RequestIdentityDigest = requestIdentityDigest
	require.NoError(t, os.WriteFile(logPath, marshalReconciledShippedEvent(t, delta, shipmentID), 0o644))
}

func TestValidateShipmentReconcilePreconditions_HappyPath(t *testing.T) {
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()
	directDone := createShipmentReconcileMemberWithStatus(t, ws, models.StatusDone)
	directAccepted := createShipmentReconcileMemberWithStatus(t, ws, models.StatusAccepted)
	archivedDone := createArchivedShipmentReconcileMemberFromStatus(t, ws, models.StatusDone)
	archivedAccepted := createArchivedShipmentReconcileMemberFromStatus(t, ws, models.StatusAccepted)
	shipment := createArchivedShipmentReconcileFixture(t, ws, []string{directDone, directAccepted, archivedDone, archivedAccepted})
	req := validShipmentReconcilePreconditionRequest(shipment.ID)
	digest := mustShipmentReconcileRequestIdentityDigest(t, req)

	reloaded, err := validateShipmentReconcilePreconditions(ctx, ws, req, digest)
	require.NoError(t, err)
	require.NotNil(t, reloaded)
	assert.Equal(t, shipment.ID, reloaded.ID)
	assert.Equal(t, models.StatusArchived, reloaded.Status)
	assert.Equal(t, string(ShipmentActive), reloaded.ArchivedStatus)
}

func TestValidateShipmentReconcilePreconditions_ShipmentNotFound(t *testing.T) {
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()
	req := validShipmentReconcilePreconditionRequest("999-S")

	_, err := validateShipmentReconcilePreconditions(ctx, ws, req, "digest-167-014")
	require.Error(t, err)
	assert.ErrorIs(t, err, blerrors.ErrNotFound)
}

func TestValidateShipmentReconcilePreconditions_ShipmentMustBeShipmentArtifact(t *testing.T) {
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()
	taskID := createArchivedShipmentReconcileMemberFromStatus(t, ws, models.StatusDone)
	req := validShipmentReconcilePreconditionRequest(taskID)
	digest := mustShipmentReconcileRequestIdentityDigest(t, req)

	_, err := validateShipmentReconcilePreconditions(ctx, ws, req, digest)
	require.Error(t, err)
	assert.ErrorIs(t, err, blerrors.ErrValidation)
	assert.Contains(t, err.Error(), "is not a shipment")
}

func TestValidateShipmentReconcilePreconditions_ShipmentMustResolveUnderArchive(t *testing.T) {
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()
	memberID := createShipmentReconcileMemberWithStatus(t, ws, models.StatusDone)
	shipment, err := CreateShipment(ctx, ws, "167.014-T queued shipment", []string{memberID})
	require.NoError(t, err)
	_, err = ClaimShipment(ctx, ws, shipment.ID)
	require.NoError(t, err)
	req := validShipmentReconcilePreconditionRequest(shipment.ID)
	digest := mustShipmentReconcileRequestIdentityDigest(t, req)

	_, err = validateShipmentReconcilePreconditions(ctx, ws, req, digest)
	require.Error(t, err)
	assert.ErrorIs(t, err, blerrors.ErrValidation)
	assert.Contains(t, err.Error(), ".backlogit\\archive")
}

func TestValidateShipmentReconcilePreconditions_ShipmentStatusMustBeArchived(t *testing.T) {
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()
	memberID := createShipmentReconcileMemberWithStatus(t, ws, models.StatusDone)
	shipment := createArchivedShipmentReconcileFixture(t, ws, []string{memberID})
	rewriteArtifactFile(t, ws, shipment.ID, func(raw string) string {
		require.Contains(t, raw, "status: archived")
		return strings.Replace(raw, "status: archived", "status: active", 1)
	})
	req := validShipmentReconcilePreconditionRequest(shipment.ID)
	digest := mustShipmentReconcileRequestIdentityDigest(t, req)

	_, err := validateShipmentReconcilePreconditions(ctx, ws, req, digest)
	require.Error(t, err)
	assert.ErrorIs(t, err, blerrors.ErrUnsupportedLegacyPreState)
}

func TestValidateShipmentReconcilePreconditions_ShipmentArchivedStatusMustBeLegacyActive(t *testing.T) {
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()
	memberID := createShipmentReconcileMemberWithStatus(t, ws, models.StatusDone)
	shipment := createArchivedShipmentReconcileFixture(t, ws, []string{memberID})
	rewriteArtifactFile(t, ws, shipment.ID, func(raw string) string {
		require.Contains(t, raw, "archived_status: active")
		return strings.Replace(raw, "archived_status: active", "archived_status: done", 1)
	})
	req := validShipmentReconcilePreconditionRequest(shipment.ID)
	digest := mustShipmentReconcileRequestIdentityDigest(t, req)

	_, err := validateShipmentReconcilePreconditions(ctx, ws, req, digest)
	require.Error(t, err)
	assert.ErrorIs(t, err, blerrors.ErrUnsupportedLegacyPreState)
}

func TestValidateShipmentReconcilePreconditions_ClassifierReconfirmationConflict(t *testing.T) {
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()
	memberID := createShipmentReconcileMemberWithStatus(t, ws, models.StatusDone)
	shipment := createArchivedShipmentReconcileFixture(t, ws, []string{memberID})
	req := validShipmentReconcilePreconditionRequest(shipment.ID)
	digest := mustShipmentReconcileRequestIdentityDigest(t, req)
	writeShipmentReconcileConflictLog(t, ws, shipment.ID, req, digest)

	_, err := validateShipmentReconcilePreconditions(ctx, ws, req, digest)
	require.Error(t, err)
	assert.ErrorIs(t, err, blerrors.ErrShipmentReconcileConflict)
}

func TestValidateShipmentReconcilePreconditions_EmptyManifestRejected(t *testing.T) {
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()
	shipment := createArchivedShipmentReconcileFixture(t, ws, nil)
	req := validShipmentReconcilePreconditionRequest(shipment.ID)
	digest := mustShipmentReconcileRequestIdentityDigest(t, req)

	_, err := validateShipmentReconcilePreconditions(ctx, ws, req, digest)
	require.Error(t, err)
	assert.ErrorIs(t, err, blerrors.ErrValidation)
	assert.Contains(t, err.Error(), "manifest must be non-empty")
}

func TestValidateShipmentReconcilePreconditions_DirectRejectedMemberFailsClosed(t *testing.T) {
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()
	rejected := createShipmentReconcileMemberWithStatus(t, ws, models.StatusRejected)
	shipment := createArchivedShipmentReconcileFixture(t, ws, []string{rejected})
	req := validShipmentReconcilePreconditionRequest(shipment.ID)
	digest := mustShipmentReconcileRequestIdentityDigest(t, req)

	_, err := validateShipmentReconcilePreconditions(ctx, ws, req, digest)
	require.Error(t, err)
	assert.ErrorIs(t, err, blerrors.ErrUnsupportedLegacyDescope)
}

func TestValidateShipmentReconcilePreconditions_ArchivedRejectedMemberFailsClosed(t *testing.T) {
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()
	archivedRejected := archiveMemberFromStatus(t, ctx, ws, "rejected")
	shipment := createArchivedShipmentReconcileFixture(t, ws, []string{archivedRejected})
	req := validShipmentReconcilePreconditionRequest(shipment.ID)
	digest := mustShipmentReconcileRequestIdentityDigest(t, req)

	_, err := validateShipmentReconcilePreconditions(ctx, ws, req, digest)
	require.Error(t, err)
	assert.ErrorIs(t, err, blerrors.ErrUnsupportedLegacyDescope)
}

func TestValidateShipmentReconcilePreconditions_ArchivedActiveMemberFailsClosed(t *testing.T) {
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()
	archivedActive := archiveMemberFromStatus(t, ctx, ws, "active")
	shipment := createArchivedShipmentReconcileFixture(t, ws, []string{archivedActive})
	req := validShipmentReconcilePreconditionRequest(shipment.ID)
	digest := mustShipmentReconcileRequestIdentityDigest(t, req)

	_, err := validateShipmentReconcilePreconditions(ctx, ws, req, digest)
	require.Error(t, err)
	assert.ErrorIs(t, err, blerrors.ErrUnsupportedLegacyDescope)
}

func TestValidateShipmentReconcilePreconditions_ArchivedSelfProvenanceMemberFailsClosed(t *testing.T) {
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()
	archivedArchived := archiveMemberFromStatus(t, ctx, ws, "archived")
	shipment := createArchivedShipmentReconcileFixture(t, ws, []string{archivedArchived})
	req := validShipmentReconcilePreconditionRequest(shipment.ID)
	digest := mustShipmentReconcileRequestIdentityDigest(t, req)

	_, err := validateShipmentReconcilePreconditions(ctx, ws, req, digest)
	require.Error(t, err)
	assert.ErrorIs(t, err, blerrors.ErrUnsupportedLegacyDescope)
}

func TestValidateShipmentReconcilePreconditions_ArchivedUnknownProvenanceMemberFailsClosed(t *testing.T) {
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()
	cases := []struct {
		name           string
		archivedStatus string
	}{
		{name: "empty", archivedStatus: ""},
		{name: "unknown", archivedStatus: "mystery"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			archivedMember := archiveMemberFromStatus(t, ctx, ws, tc.archivedStatus)
			shipment := createArchivedShipmentReconcileFixture(t, ws, []string{archivedMember})
			req := validShipmentReconcilePreconditionRequest(shipment.ID)
			digest := mustShipmentReconcileRequestIdentityDigest(t, req)

			_, err := validateShipmentReconcilePreconditions(ctx, ws, req, digest)
			require.Error(t, err)
			assert.ErrorIs(t, err, blerrors.ErrUnsupportedLegacyDescope)
		})
	}
}

func TestValidateShipmentReconcilePreconditions_NonTerminalMemberRejected(t *testing.T) {
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()
	cases := []struct {
		name   string
		status models.ArtifactStatus
	}{
		{name: "queued", status: models.StatusQueued},
		{name: "active", status: models.StatusActive},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			memberID := createShipmentReconcileMemberWithStatus(t, ws, tc.status)
			shipment := createArchivedShipmentReconcileFixture(t, ws, []string{memberID})
			_, setErr := setArtifactStatus(ctx, ws, memberID, tc.status, "167.014-T non-terminal fixture")
			require.NoError(t, setErr)
			req := validShipmentReconcilePreconditionRequest(shipment.ID)
			digest := mustShipmentReconcileRequestIdentityDigest(t, req)

			_, err := validateShipmentReconcilePreconditions(ctx, ws, req, digest)
			require.Error(t, err)
			assert.ErrorIs(t, err, blerrors.ErrValidation)
			assert.Contains(t, err.Error(), string(tc.status))
		})
	}
}

func TestValidateShipmentReconcilePreconditions_SecondApproverMustDifferFromActor(t *testing.T) {
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()
	memberID := createShipmentReconcileMemberWithStatus(t, ws, models.StatusDone)
	shipment := createArchivedShipmentReconcileFixture(t, ws, []string{memberID})
	req := validShipmentReconcilePreconditionRequest(shipment.ID)
	req.SecondApprover = req.Actor

	_, err := validateShipmentReconcilePreconditions(ctx, ws, req, "digest-167-014")
	require.Error(t, err)
	assert.ErrorIs(t, err, blerrors.ErrValidation)
}
