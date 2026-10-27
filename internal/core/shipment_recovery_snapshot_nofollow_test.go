package core

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	blerrors "github.com/softwaresalt/backlogit/internal/errors"
)

func writeValidBlockedSnapshotForTest(t *testing.T, path, shipmentID string, members map[string]string) {
	t.Helper()
	data, err := json.Marshal(ShipmentBlockedSnapshot{
		SchemaVersion: ShipmentBlockedSnapshotSchemaVersion,
		ShipmentID:    shipmentID,
		Target:        ShipmentBlocked,
		BlockedReason: "test",
		BlockedAt:     "2026-01-01T00:00:00Z",
		Members:       members,
	})
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, data, 0o644))
}

// TestReadShipmentBlockedSnapshot_PathComponentSwapIsRefused swaps a directory
// component of the snapshot path for a link to an outside location after the
// reference was resolved and containment-checked and before the bytes are
// read. The outside file holds a valid snapshot; it must never be returned.
func TestReadShipmentBlockedSnapshot_PathComponentSwapIsRefused(t *testing.T) {
	const shipmentID = "201-S"
	members := map[string]string{"201.001-T": "queued"}
	memberIDs := []string{"201.001-T"}

	t.Run("control without swap reads the snapshot", func(t *testing.T) {
		ws := setupShipmentWorkspace(t)
		snapshotDir := filepath.Join(workspaceStorageRoot(ws), "snapshots", "sub")
		writeValidBlockedSnapshotForTest(t, filepath.Join(snapshotDir, "snap.json"), shipmentID, members)

		snapshot, err := readShipmentBlockedSnapshot(ws, filepath.Join("snapshots", "sub", "snap.json"), shipmentID, memberIDs)
		require.NoError(t, err)
		assert.Equal(t, shipmentID, snapshot.ShipmentID)
	})

	t.Run("swap after containment check is refused", func(t *testing.T) {
		ws := setupShipmentWorkspace(t)
		snapshotDir := filepath.Join(workspaceStorageRoot(ws), "snapshots", "sub")
		writeValidBlockedSnapshotForTest(t, filepath.Join(snapshotDir, "snap.json"), shipmentID, members)
		outside := t.TempDir()
		writeValidBlockedSnapshotForTest(t, filepath.Join(outside, "snap.json"), shipmentID, members)

		t.Cleanup(func() { shipmentSnapshotReadSeam = nil })
		seamCalled := false
		shipmentSnapshotReadSeam = func(string) {
			seamCalled = true
			require.NoError(t, os.Rename(snapshotDir, snapshotDir+".orig"))
			if linkErr := linkDirForMemberSwapTest(snapshotDir, outside); linkErr != nil {
				t.Skipf("filesystem cannot create a directory link for the swap: %v", linkErr)
			}
		}

		_, err := readShipmentBlockedSnapshot(ws, filepath.Join("snapshots", "sub", "snap.json"), shipmentID, memberIDs)
		assert.True(t, seamCalled, "the snapshot read seam must run between the containment check and the read")
		require.Error(t, err, "a snapshot reached through a swapped path component must be refused")
		assert.ErrorIs(t, err, blerrors.ErrValidation)
	})
}
