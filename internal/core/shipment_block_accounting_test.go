package core

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	blerrors "github.com/softwaresalt/backlogit/internal/errors"
	"github.com/softwaresalt/backlogit/internal/models"
)

// TestUCYA3_BlockShipmentMemberRelocationRemovalReportsWriteIndeterminate queues
// an active member whose file sits outside its canonical queue directory, so the
// queue write relocates it. The queued copy lands at the target, then removing the
// old file fails. That outcome is indeterminate and must report ErrWriteIndeterminate.
func TestUCYA3_BlockShipmentMemberRelocationRemovalReportsWriteIndeterminate(t *testing.T) {
	ctx := context.Background()
	_, ws := setupUR3Workspace(t)
	t.Cleanup(func() { require.NoError(t, ws.Close()) })
	fixture := newURBlockedActiveFixture(t, ws)
	shipmentID := fixture.shipment.ID
	memberID := fixture.members[0].ID
	require.Equal(t, models.StatusActive, loadURCanonicalArtifact(t, ws, memberID).Status,
		"the relocated member must be active so BlockShipment queues it")

	currentPath, err := FindArtifactPath(ctx, ws, memberID)
	require.NoError(t, err)
	archiveDir := filepath.Join(WorkspaceStorageRoot(ws.RootPath), "archive")
	require.NoError(t, os.MkdirAll(archiveDir, 0o755))
	relocatedFrom := filepath.Join(archiveDir, filepath.Base(currentPath))
	require.NoError(t, os.Rename(currentPath, relocatedFrom))

	realWrite := persistArtifactWriteFn
	armed := false
	persistArtifactWriteFn = func(artifact *models.Artifact, path string, durable bool) error {
		if !armed && artifact.ID == memberID && artifact.Status == models.StatusQueued {
			if err := realWrite(artifact, path, durable); err != nil {
				return err
			}
			armed = true
			// Replace the old member file with a non-empty directory so removing
			// the old path fails after the queued copy has already landed.
			require.NoError(t, os.Remove(relocatedFrom))
			require.NoError(t, os.Mkdir(relocatedFrom, 0o755))
			require.NoError(t, os.WriteFile(filepath.Join(relocatedFrom, "obstruction.txt"), []byte("u-cya3"), 0o644))
			return nil
		}
		return realWrite(artifact, path, durable)
	}
	t.Cleanup(func() { persistArtifactWriteFn = realWrite })

	shipment, blockErr := BlockShipment(ctx, ws, shipmentID, BlockOptions{
		Reason:    "u-cya3 relocation accounting",
		BlockedBy: "UCYA3",
	})

	assert.True(t, armed, "the relocated member's queued write must arm the old-file removal obstruction")
	require.Error(t, blockErr)
	assert.Nil(t, shipment, "a failed block must return no shipment")
	assert.True(t, errors.Is(blockErr, blerrors.ErrWriteIndeterminate),
		"an old-file removal failure after the relocated copy landed must report write_indeterminate, got: %v", blockErr)
}

// TestUCYA3_BlockShipmentCompensationRestoreDoubleFaultReportsWriteIndeterminate
// fails the blocked shipment write and then fails the compensation restore of the
// shipment. The double fault must report ErrWriteIndeterminate and leave the
// lifecycle journal at intent so recovery can finish the operation.
func TestUCYA3_BlockShipmentCompensationRestoreDoubleFaultReportsWriteIndeterminate(t *testing.T) {
	ctx := context.Background()
	_, ws := setupUR3Workspace(t)
	t.Cleanup(func() { require.NoError(t, ws.Close()) })
	fixture := newURBlockedActiveFixture(t, ws)
	shipmentID := fixture.shipment.ID
	require.Equal(t, models.StatusActive, loadURCanonicalArtifact(t, ws, shipmentID).Status)

	realWrite := persistArtifactWriteFn
	persistArtifactWriteFn = func(artifact *models.Artifact, path string, durable bool) error {
		if artifact.ID == shipmentID && artifact.ArtifactType == "shipment" {
			switch artifact.Status {
			case models.StatusBlocked:
				return errors.New("u-cya3 injected blocked shipment write failure")
			case models.StatusActive:
				return errors.New("u-cya3 injected compensation restore failure")
			}
		}
		return realWrite(artifact, path, durable)
	}
	t.Cleanup(func() { persistArtifactWriteFn = realWrite })

	shipment, blockErr := BlockShipment(ctx, ws, shipmentID, BlockOptions{
		Reason:    "u-cya3 double fault",
		BlockedBy: "UCYA3",
	})

	require.Error(t, blockErr)
	assert.Nil(t, shipment, "a failed block must return no shipment")
	assert.Contains(t, blockErr.Error(), "compensation failed",
		"the restore failure must be reported as a compensation failure")
	assert.True(t, errors.Is(blockErr, blerrors.ErrWriteIndeterminate),
		"a compensation restore double fault must report write_indeterminate, got: %v", blockErr)

	records, err := loadShipmentOperationJournals(ws)
	require.NoError(t, err)
	var phases []string
	for _, record := range records {
		if record.kind == shipmentLifecycleJournalKind &&
			record.lifecycle.ShipmentID == shipmentID &&
			record.lifecycle.Operation == "block" {
			phases = append(phases, record.lifecycle.Phase)
		}
	}
	require.Len(t, phases, 1, "the failed block must leave exactly one lifecycle journal")
	assert.Equal(t, "intent", phases[0], "a failed compensation restore must leave the journal at intent")
}
