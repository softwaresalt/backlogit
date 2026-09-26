//go:build windows

package core

import (
	"bytes"
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

func TestShipmentOpsJournalRead_BindsToValidatedDirectoryObjectAfterPathSwap(t *testing.T) {
	root := newShipmentOpsSecurityWorkspace(t)
	storageRoot, err := filepath.EvalSymlinks(filepath.Join(root, ".backlogit"))
	require.NoError(t, err)
	opsRoot := filepath.Join(storageRoot, "ops")
	require.NoError(t, os.Mkdir(opsRoot, 0o755))

	const originalPayload = "validated-directory-object"
	require.NoError(t, os.WriteFile(
		filepath.Join(opsRoot, testShipmentOperationJournalName),
		[]byte(originalPayload),
		0o600,
	))

	dir, err := openShipmentOpsDirectory(storageRoot, opsRoot)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, dir.Close()) })

	swapShipmentOpsDirectoryObject(t, opsRoot)
	const swappedPayload = "swapped-in-directory-must-not-be-read"
	require.NoError(t, os.WriteFile(
		filepath.Join(opsRoot, testShipmentOperationJournalName),
		[]byte(swappedPayload),
		0o600,
	))

	data, readErr := readShipmentOperationJournalFile(dir, opsRoot, testShipmentOperationJournalName)
	if readErr == nil {
		require.Equal(t, originalPayload, string(data))
	}
	require.Equal(t, swappedPayload, string(requireReadFile(t,
		filepath.Join(opsRoot, testShipmentOperationJournalName))))
}

func TestShipmentOpsJournalTempRemove_BindsToValidatedDirectoryObjectAfterPathSwap(t *testing.T) {
	root := newShipmentOpsSecurityWorkspace(t)
	storageRoot, err := filepath.EvalSymlinks(filepath.Join(root, ".backlogit"))
	require.NoError(t, err)
	opsRoot := filepath.Join(storageRoot, "ops")
	require.NoError(t, os.Mkdir(opsRoot, 0o755))

	tempName, err := shipmentOperationJournalTempName(testShipmentOperationJournalName)
	require.NoError(t, err)
	const originalPayload = "validated-directory-temp"
	require.NoError(t, os.WriteFile(filepath.Join(opsRoot, tempName), []byte(originalPayload), 0o600))

	dir, err := openShipmentOpsDirectory(storageRoot, opsRoot)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, dir.Close()) })

	swapShipmentOpsDirectoryObject(t, opsRoot)
	const swappedPayload = "swapped-in-temp-must-not-be-removed"
	swappedTempPath := filepath.Join(opsRoot, tempName)
	require.NoError(t, os.WriteFile(swappedTempPath, []byte(swappedPayload), 0o600))

	removeErr := removeShipmentOperationJournalTempFile(dir, opsRoot, tempName)
	if removeErr == nil {
		_, statErr := os.Lstat(filepath.Join(opsRoot+"-original", tempName))
		require.ErrorIs(t, statErr, os.ErrNotExist)
	}
	require.Equal(t, swappedPayload, string(requireReadFile(t, swappedTempPath)))
}

func TestShipmentOpsJournalWrite_BindsCreateAndRenameToValidatedDirectoryObjectAfterPathSwap(t *testing.T) {
	root := newShipmentOpsSecurityWorkspace(t)
	storageRoot, err := filepath.EvalSymlinks(filepath.Join(root, ".backlogit"))
	require.NoError(t, err)
	opsRoot := filepath.Join(storageRoot, "ops")
	require.NoError(t, os.Mkdir(opsRoot, 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(opsRoot, testShipmentOperationJournalName),
		[]byte("original-target-preimage"),
		0o600,
	))

	dir, err := openShipmentOpsDirectory(storageRoot, opsRoot)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, dir.Close()) })

	swapShipmentOpsDirectoryObject(t, opsRoot)
	const swappedPayload = "swapped-in-target-must-not-be-replaced"
	swappedTargetPath := filepath.Join(opsRoot, testShipmentOperationJournalName)
	require.NoError(t, os.WriteFile(swappedTargetPath, []byte(swappedPayload), 0o600))

	requestedPayload := []byte("requested-payload-must-stay-with-validated-directory-object")
	writeErr := writeShipmentOperationJournalFile(
		dir,
		opsRoot,
		testShipmentOperationJournalName,
		requestedPayload,
	)
	if writeErr == nil {
		require.Equal(t, requestedPayload, requireReadFile(t,
			filepath.Join(opsRoot+"-original", testShipmentOperationJournalName)))
	}
	require.Equal(t, swappedPayload, string(requireReadFile(t, swappedTargetPath)))
	requireShipmentOpsPayloadAbsent(t, opsRoot, requestedPayload)
}

func swapShipmentOpsDirectoryObject(t *testing.T, opsRoot string) {
	t.Helper()

	require.NoError(t, os.Rename(opsRoot, opsRoot+"-original"),
		"the validated directory handle must permit a canonical-path object swap")
	require.NoError(t, os.Mkdir(opsRoot, 0o755))
}

func requireShipmentOpsPayloadAbsent(t *testing.T, opsRoot string, payload []byte) {
	t.Helper()

	entries, err := os.ReadDir(opsRoot)
	require.NoError(t, err)
	for _, entry := range entries {
		if entry.Type().IsRegular() {
			require.False(t,
				bytes.Contains(requireReadFile(t, filepath.Join(opsRoot, entry.Name())), payload),
				"requested payload appeared in swapped-in directory entry %s", entry.Name())
		}
	}
}
