//go:build windows

package core

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewWorkspace_RejectsRedirectedShipmentOperationsDirectory(t *testing.T) {
	root := newShipmentOpsSecurityWorkspace(t)
	opsRoot := filepath.Join(root, ".backlogit", "ops")
	outsideRoot := t.TempDir()
	const outsidePreimage = "outside-ops-preimage-must-not-be-exposed"
	require.NoError(t, os.WriteFile(
		filepath.Join(outsideRoot, testShipmentOperationJournalName),
		[]byte(outsidePreimage),
		0o644,
	))

	if err := createReconcileTestJunction(t, opsRoot, outsideRoot); err != nil {
		t.Skipf("filesystem cannot create an operations-directory junction: %v", err)
	}

	ws, err := NewWorkspace(context.Background(), root)
	require.Error(t, err)
	require.Nil(t, ws)
	require.NotContains(t, err.Error(), outsidePreimage)
	require.Equal(t, outsidePreimage, string(requireReadFile(t,
		filepath.Join(outsideRoot, testShipmentOperationJournalName))))
}

func TestWriteShipmentLifecycleJournal_RejectsRedirectedOperationsDirectory(t *testing.T) {
	root := newShipmentOpsSecurityWorkspace(t)
	ws, err := NewWorkspace(context.Background(), root)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, ws.Close()) })

	opsRoot := filepath.Join(root, ".backlogit", "ops")
	outsideRoot := t.TempDir()
	if err := createReconcileTestJunction(t, opsRoot, outsideRoot); err != nil {
		t.Skipf("filesystem cannot create an operations-directory junction: %v", err)
	}

	journal := shipmentLifecycleJournal{
		SchemaVersion: "shipment-operation/v1",
		CorrelationID: "0123456789abcdef0123456789abcdef",
		Phase:         "intent",
		Reason:        "secret-preimage-marker",
	}
	_, err = writeShipmentLifecycleJournalForWorkspace(ws, testShipmentOperationJournalName, journal)
	require.Error(t, err)
	entries, readErr := os.ReadDir(outsideRoot)
	require.NoError(t, readErr)
	require.Empty(t, entries, "no temp file or preimage may be created outside the workspace")
}
