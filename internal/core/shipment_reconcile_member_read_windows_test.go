//go:build windows

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
	"github.com/softwaresalt/backlogit/internal/models"
)

// TestWindowsContainedRead_InRootJunctionSwapIsRefused swaps an intermediate
// directory for a junction to ANOTHER directory inside the storage root. The
// substituted file is still contained, so only an exact final-path match
// refuses it.
func TestWindowsContainedRead_InRootJunctionSwapIsRefused(t *testing.T) {
	t.Run("manifest member read", func(t *testing.T) {
		ws := setupShipmentWorkspace(t)
		ctx := context.Background()
		memberID := createShipmentReconcileMemberWithStatus(t, ws, models.StatusDone)

		originalPath, err := FindArtifactPath(ctx, ws, memberID)
		require.NoError(t, err)
		raw, err := os.ReadFile(originalPath)
		require.NoError(t, err)

		swapDir := filepath.Join(filepath.Dir(originalPath), "inroot-swap")
		require.NoError(t, os.MkdirAll(swapDir, 0o755))
		memberPath := filepath.Join(swapDir, filepath.Base(originalPath))
		require.NoError(t, os.Rename(originalPath, memberPath))

		substituted := strings.Replace(string(raw), "status: done", "status: rejected", 1)
		require.NotEqual(t, string(raw), substituted)
		inRoot := filepath.Join(workspaceStorageRoot(ws), "inroot-other")
		require.NoError(t, os.MkdirAll(inRoot, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(inRoot, filepath.Base(memberPath)), []byte(substituted), 0o644))

		t.Cleanup(func() { shipmentReconcileMemberReadSeam = nil })
		seamCalled := false
		shipmentReconcileMemberReadSeam = func(string) {
			seamCalled = true
			require.NoError(t, os.Rename(swapDir, swapDir+".orig"))
			require.NoError(t, linkDirForMemberSwapTest(swapDir, inRoot))
		}

		err = validateShipmentReconcileManifestMembers(ctx, ws, []string{memberID})
		assert.True(t, seamCalled)
		require.Error(t, err)
		assert.NotErrorIs(t, err, blerrors.ErrUnsupportedLegacyDescope, "the substituted in-root file must never be read")
		assert.ErrorIs(t, err, blerrors.ErrValidation)
	})

	t.Run("blocked snapshot read", func(t *testing.T) {
		const shipmentID = "201-S"
		members := map[string]string{"201.001-T": "queued"}
		ws := setupShipmentWorkspace(t)
		snapshotDir := filepath.Join(workspaceStorageRoot(ws), "snapshots", "sub")
		writeValidBlockedSnapshotForTest(t, filepath.Join(snapshotDir, "snap.json"), shipmentID, members)
		inRoot := filepath.Join(workspaceStorageRoot(ws), "snapshots", "other")
		writeValidBlockedSnapshotForTest(t, filepath.Join(inRoot, "snap.json"), shipmentID, members)

		t.Cleanup(func() { shipmentSnapshotReadSeam = nil })
		seamCalled := false
		shipmentSnapshotReadSeam = func(string) {
			seamCalled = true
			require.NoError(t, os.Rename(snapshotDir, snapshotDir+".orig"))
			require.NoError(t, linkDirForMemberSwapTest(snapshotDir, inRoot))
		}

		_, err := readShipmentBlockedSnapshot(ws, filepath.Join("snapshots", "sub", "snap.json"), shipmentID, []string{"201.001-T"})
		assert.True(t, seamCalled)
		require.Error(t, err)
		assert.ErrorIs(t, err, blerrors.ErrValidation)
	})
}
