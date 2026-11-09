//go:build windows

package core

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/windows"

	blerrors "github.com/softwaresalt/backlogit/internal/errors"
)

// TestU1j_ShipmentOpsJournalWindowsTransientContentionRetryAndSecurityPreservation
// keeps every seam-changing scenario sequential, including its subtests.
func TestU1j_ShipmentOpsJournalWindowsTransientContentionRetryAndSecurityPreservation(t *testing.T) {
	transients := []struct {
		name string
		err  error
	}{
		{"errno_access", windows.ERROR_ACCESS_DENIED},
		{"errno_sharing", windows.ERROR_SHARING_VIOLATION},
		{"status_access", windows.STATUS_ACCESS_DENIED},
		{"status_sharing", windows.STATUS_SHARING_VIOLATION},
		{"status_delete_pending", windows.STATUS_DELETE_PENDING},
	}
	for _, operation := range []string{"rename", "remove"} {
		for _, transient := range transients {
			for _, wrapped := range []bool{false, true} {
				for _, exhaust := range []bool{false, true} {
					t.Run(fmt.Sprintf("%s/%s/wrapped_%t/exhaust_%t",
						operation, transient.name, wrapped, exhaust), func(t *testing.T) {
						failure := transient.err
						if wrapped {
							failure = fmt.Errorf("injected syscall: %w", failure)
						}
						runU1jMutation(t, operation, failure, exhaust, false)
					})
				}
			}
		}
		for _, failure := range []error{
			windows.STATUS_FILE_IS_A_DIRECTORY,
			windows.ERROR_FILE_NOT_FOUND,
			errors.New("permanent syscall failure"),
			blerrors.ErrValidation,
		} {
			for _, wrapped := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/immediate_%v/wrapped_%t", operation, failure, wrapped), func(t *testing.T) {
					if wrapped {
						failure = fmt.Errorf("injected syscall: %w", failure)
					}
					runU1jImmediateFailure(t, operation, failure)
				})
			}
		}
		t.Run(operation+"/directory_object_binding_during_backoff", func(t *testing.T) {
			runU1jMutation(t, operation, windows.STATUS_SHARING_VIOLATION, false, true)
		})
	}
	for _, temp := range []bool{false, true} {
		for _, exhaust := range []bool{false, true} {
			t.Run(fmt.Sprintf("open/temp_%t/exhaust_%t", temp, exhaust), func(t *testing.T) {
				runU1jOpenContention(t, temp, exhaust)
			})
		}
	}
	t.Run("validation_and_reparse_never_retried", runU1jValidation)
}

type u1jFixture struct {
	dir      *os.File
	opsRoot  string
	tempName string
}

func newU1jFixture(t *testing.T) u1jFixture {
	t.Helper()
	root := t.TempDir()
	storageRoot, err := filepath.EvalSymlinks(root)
	require.NoError(t, err)
	opsRoot := filepath.Join(storageRoot, "ops")
	require.NoError(t, os.Mkdir(opsRoot, 0o700))
	tempName, err := shipmentOperationJournalTempName(testShipmentOperationJournalName)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(opsRoot, tempName), []byte("replacement"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(opsRoot, testShipmentOperationJournalName), []byte("original"), 0o600))
	dir, err := openShipmentOpsDirectory(storageRoot, opsRoot)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, dir.Close()) })
	renameFn, dispositionFn, sleepFn := shipmentOpsNtRenameFn, shipmentOpsSetDispositionFn, shipmentOpsRetrySleep
	t.Cleanup(func() {
		shipmentOpsNtRenameFn, shipmentOpsSetDispositionFn, shipmentOpsRetrySleep = renameFn, dispositionFn, sleepFn
	})
	return u1jFixture{dir: dir, opsRoot: opsRoot, tempName: tempName}
}

func (f u1jFixture) mutate(operation string) error {
	if operation == "rename" {
		return renameShipmentOpsWindowsChild(f.dir, f.tempName, testShipmentOperationJournalName)
	}
	return removeShipmentOperationJournalTempFile(f.dir, f.opsRoot, f.tempName)
}

// installU1jMutation checks that retries retain the validated source handle and
// the same directory-relative rename buffer, not a reopened canonical path.
func installU1jMutation(t *testing.T, dir *os.File, operation string, failure error, succeedAt int) *int {
	t.Helper()
	attempts := new(int)
	var firstHandle windows.Handle
	var firstInfo *byte
	renameFn, dispositionFn := shipmentOpsNtRenameFn, shipmentOpsSetDispositionFn
	shipmentOpsNtRenameFn = func(handle windows.Handle, iosb *windows.IO_STATUS_BLOCK, info *byte, length, class uint32) error {
		*attempts++
		assert.Equal(t, "rename", operation)
		assert.Equal(t, uint32(windows.FileRenameInformation), class)
		// Check the shape and retain the actual buffer pointer. The real
		// syscall on success plus the path-swap assertions verify its binding
		// without unsafe memory inspection.
		want, err := shipmentOpsWindowsRenameInfo(windows.Handle(dir.Fd()), testShipmentOperationJournalName)
		require.NoError(t, err)
		assert.Equal(t, uint32(len(want)), length)
		assert.Equal(t, byte(1), *info)
		if *attempts == 1 {
			firstHandle = handle
			firstInfo = info
		}
		assert.Equal(t, firstHandle, handle, "retry must use the validated source handle")
		assert.True(t, firstInfo == info, "retry must retain the directory-relative rename buffer")
		if *attempts == succeedAt {
			return renameFn(handle, iosb, info, length, class)
		}
		return failure
	}
	shipmentOpsSetDispositionFn = func(handle windows.Handle, class uint32, info *byte, length uint32) error {
		*attempts++
		assert.Equal(t, "remove", operation)
		assert.Equal(t, uint32(windows.FileDispositionInfo), class)
		assert.Equal(t, uint32(1), length)
		assert.Equal(t, byte(1), *info)
		if *attempts == 1 {
			firstHandle = handle
		}
		assert.Equal(t, firstHandle, handle, "retry must use the validated source handle")
		if *attempts == succeedAt {
			return dispositionFn(handle, class, info, length)
		}
		return failure
	}
	return attempts
}

func runU1jMutation(t *testing.T, operation string, failure error, exhaust, swap bool) {
	t.Helper()
	f := newU1jFixture(t)
	succeedAt := 8
	if exhaust {
		succeedAt = 0
	}
	attempts := installU1jMutation(t, f.dir, operation, failure, succeedAt)
	sleeps := make([]time.Duration, 0, 7)
	actualRoot := f.opsRoot
	shipmentOpsRetrySleep = func(delay time.Duration) {
		sleeps = append(sleeps, delay)
		assert.Equal(t, len(sleeps), *attempts, "sleep must follow each failed attempt")
		assert.Equal(t, "original", string(requireReadFile(t, filepath.Join(actualRoot, testShipmentOperationJournalName))))
		assert.Equal(t, "replacement", string(requireReadFile(t, filepath.Join(actualRoot, f.tempName))))
		if swap && len(sleeps) == 1 {
			swapShipmentOpsDirectoryObject(t, f.opsRoot)
			actualRoot = f.opsRoot + "-original"
			require.NoError(t, os.WriteFile(filepath.Join(f.opsRoot, f.tempName), []byte("decoy-temp"), 0o600))
			require.NoError(t, os.WriteFile(filepath.Join(f.opsRoot, testShipmentOperationJournalName), []byte("decoy-target"), 0o600))
		}
	}
	err := f.mutate(operation)
	assert.Equal(t, 8, *attempts, "U1j: bounded retry requires eight total attempts")
	assertU1jBackoff(t, sleeps)
	if exhaust {
		assert.ErrorIs(t, shipmentOpsWindowsError(err), shipmentOpsWindowsError(failure))
		assert.Equal(t, "replacement", string(requireReadFile(t, filepath.Join(actualRoot, f.tempName))))
	} else {
		assert.NoError(t, err, "U1j: seventh retry must apply the operation")
		_, statErr := os.Stat(filepath.Join(actualRoot, f.tempName))
		assert.ErrorIs(t, statErr, os.ErrNotExist)
	}
	want := "original"
	if !exhaust && operation == "rename" {
		want = "replacement"
	}
	assert.Equal(t, want, string(requireReadFile(t, filepath.Join(actualRoot, testShipmentOperationJournalName))))
	if swap && len(sleeps) > 0 {
		assert.Equal(t, "decoy-temp", string(requireReadFile(t, filepath.Join(f.opsRoot, f.tempName))))
		assert.Equal(t, "decoy-target", string(requireReadFile(t, filepath.Join(f.opsRoot, testShipmentOperationJournalName))))
	}
}

func runU1jImmediateFailure(t *testing.T, operation string, failure error) {
	t.Helper()
	f := newU1jFixture(t)
	attempts := installU1jMutation(t, f.dir, operation, failure, 0)
	sleeps := 0
	shipmentOpsRetrySleep = func(time.Duration) { sleeps++ }
	err := f.mutate(operation)
	assert.ErrorIs(t, shipmentOpsWindowsError(err), shipmentOpsWindowsError(failure))
	assert.Equal(t, 1, *attempts, "nontransient errors must fail immediately")
	assert.Zero(t, sleeps)
	assert.Equal(t, "original", string(requireReadFile(t, filepath.Join(f.opsRoot, testShipmentOperationJournalName))))
	assert.Equal(t, "replacement", string(requireReadFile(t, filepath.Join(f.opsRoot, f.tempName))))
}

func runU1jOpenContention(t *testing.T, temp, exhaust bool) {
	t.Helper()
	f := newU1jFixture(t)
	name := testShipmentOperationJournalName
	if temp {
		name = f.tempName
	}
	path, err := windows.UTF16PtrFromString(filepath.Join(f.opsRoot, name))
	require.NoError(t, err)
	// Deny DELETE sharing until the seventh injected sleep; no goroutines,
	// timing races, or wall-clock sleeps are involved.
	blocker, err := windows.CreateFile(path, windows.GENERIC_READ,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE, nil, windows.OPEN_EXISTING,
		windows.FILE_ATTRIBUTE_NORMAL, 0)
	require.NoError(t, err)
	t.Cleanup(func() {
		if blocker != windows.InvalidHandle {
			require.NoError(t, windows.CloseHandle(blocker))
		}
	})
	probe, probeErr := windows.CreateFile(path, windows.DELETE|windows.FILE_READ_ATTRIBUTES,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil, windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if probeErr == nil {
		require.NoError(t, windows.CloseHandle(probe))
	}
	require.ErrorIs(t, probeErr, windows.ERROR_SHARING_VIOLATION, "fixture must actually deny DELETE sharing")
	sleeps := make([]time.Duration, 0, 7)
	shipmentOpsRetrySleep = func(delay time.Duration) {
		sleeps = append(sleeps, delay)
		if !exhaust && len(sleeps) == 7 {
			require.NoError(t, windows.CloseHandle(blocker))
			blocker = windows.InvalidHandle
		}
	}
	handle, openErr := openShipmentOpsWindowsChild(f.dir, name, windows.DELETE|windows.FILE_READ_ATTRIBUTES, windows.FILE_OPEN)
	if openErr == nil {
		require.NoError(t, windows.CloseHandle(handle))
	}
	assertU1jBackoff(t, sleeps)
	if exhaust {
		assert.ErrorIs(t, shipmentOpsWindowsError(openErr), windows.ERROR_SHARING_VIOLATION)
		assert.Zero(t, handle)
	} else {
		assert.NoError(t, openErr, "U1j: existing regular child must open after bounded contention")
	}
	assert.Equal(t, "original", string(requireReadFile(t, filepath.Join(f.opsRoot, testShipmentOperationJournalName))))
	assert.Equal(t, "replacement", string(requireReadFile(t, filepath.Join(f.opsRoot, f.tempName))))
}

func assertU1jBackoff(t *testing.T, sleeps []time.Duration) {
	t.Helper()
	want := []time.Duration{5 * time.Millisecond, 10 * time.Millisecond, 20 * time.Millisecond,
		40 * time.Millisecond, 80 * time.Millisecond, 160 * time.Millisecond, 320 * time.Millisecond}
	assert.Equal(t, want, sleeps, "U1j: exactly seven exponential sleeps, no sleep after final attempt")
	var total time.Duration
	for _, delay := range sleeps {
		total += delay
	}
	assert.Equal(t, 635*time.Millisecond, total)
}

func runU1jValidation(t *testing.T) {
	t.Helper()
	f := newU1jFixture(t)
	calls, sleeps := 0, 0
	shipmentOpsRetrySleep = func(time.Duration) { sleeps++ }
	shipmentOpsNtRenameFn = func(windows.Handle, *windows.IO_STATUS_BLOCK, *byte, uint32, uint32) error {
		calls++
		return windows.ERROR_ACCESS_DENIED
	}
	shipmentOpsSetDispositionFn = func(windows.Handle, uint32, *byte, uint32) error {
		calls++
		return windows.ERROR_ACCESS_DENIED
	}
	for _, name := range []string{"", ".", "..", "../escape", `..\escape`} {
		handle, err := openShipmentOpsWindowsChild(f.dir, name, windows.DELETE, windows.FILE_OPEN)
		assert.ErrorIs(t, err, blerrors.ErrValidation)
		assert.Zero(t, handle)
		assert.ErrorIs(t, removeShipmentOperationJournalTempFile(f.dir, "", name), blerrors.ErrValidation)
		assert.ErrorIs(t, renameShipmentOpsWindowsChild(f.dir, name, testShipmentOperationJournalName), blerrors.ErrValidation)
	}
	_, err := openShipmentOpsWindowsChild(nil, f.tempName, windows.DELETE, windows.FILE_OPEN)
	assert.ErrorIs(t, err, blerrors.ErrValidation)
	assert.ErrorIs(t, renameShipmentOpsWindowsChild(f.dir, f.tempName, "wrong.json"), blerrors.ErrValidation)
	assert.ErrorIs(t, removeShipmentOperationJournalTempFile(f.dir, "", testShipmentOperationJournalName), blerrors.ErrValidation)
	assert.ErrorIs(t, validateShipmentOpsWindowsFileHandle(windows.Handle(f.dir.Fd()), "directory"), blerrors.ErrValidation)

	// A writer-shaped directory is not a regular temp and must never reach
	// either mutation syscall or the backoff seam.
	directoryTarget := "shipment-operation-11111111111111111111111111111111.json"
	directoryTemp, err := shipmentOperationJournalTempName(directoryTarget)
	require.NoError(t, err)
	require.NoError(t, os.Mkdir(filepath.Join(f.opsRoot, directoryTemp), 0o700))
	assert.Error(t, renameShipmentOpsWindowsChild(f.dir, directoryTemp, directoryTarget))
	assert.Error(t, removeShipmentOperationJournalTempFile(f.dir, "", directoryTemp))
	assert.Zero(t, calls)
	assert.Zero(t, sleeps)
	t.Run("reparse_temp", func(t *testing.T) {
		outside := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(outside, "preimage"), []byte("outside"), 0o600))
		reparseTarget := "shipment-operation-22222222222222222222222222222222.json"
		reparseTemp, err := shipmentOperationJournalTempName(reparseTarget)
		require.NoError(t, err)
		require.NoError(t, createReconcileTestJunction(t, filepath.Join(f.opsRoot, reparseTemp), outside))
		handle, err := openShipmentOpsWindowsHandle(filepath.Join(f.opsRoot, reparseTemp),
			windows.FILE_READ_ATTRIBUTES, windows.OPEN_EXISTING,
			windows.FILE_FLAG_BACKUP_SEMANTICS|windows.FILE_FLAG_OPEN_REPARSE_POINT)
		require.NoError(t, err)
		assert.ErrorIs(t, validateShipmentOpsWindowsFileHandle(handle, reparseTemp), blerrors.ErrValidation)
		require.NoError(t, windows.CloseHandle(handle))
		assert.Error(t, renameShipmentOpsWindowsChild(f.dir, reparseTemp, reparseTarget))
		assert.Error(t, removeShipmentOperationJournalTempFile(f.dir, "", reparseTemp))
		assert.Zero(t, calls)
		assert.Zero(t, sleeps)
		assert.Equal(t, "outside", string(requireReadFile(t, filepath.Join(outside, "preimage"))))
	})
}

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
