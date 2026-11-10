//go:build windows

package core

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
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
		t.Run(operation+"/directory_object_binding_after_path_swap_across_backoff", func(t *testing.T) {
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

func TestU1j_RawErrorPreservation(t *testing.T) {
	for _, exhaust := range []bool{false, true} {
		t.Run(fmt.Sprintf("retry/exhaust_%t", exhaust), func(t *testing.T) {
			original := fmt.Errorf("original syscall: %w", windows.STATUS_FILE_IS_A_DIRECTORY)
			if exhaust {
				original = fmt.Errorf("original syscall: %w", windows.STATUS_SHARING_VIOLATION)
			}
			sleepFn := shipmentOpsRetrySleep
			t.Cleanup(func() { shipmentOpsRetrySleep = sleepFn })
			sleeps := 0
			shipmentOpsRetrySleep = func(time.Duration) { sleeps++ }
			attempts := 0
			err := retryShipmentOpsWindowsOperation(func() error {
				attempts++
				if attempts == 1 {
					return original
				}
				return windows.STATUS_ACCESS_DENIED
			}, nil)
			assert.True(t, err == original, "retry must return the original raw error, including its wrapper")
			var status windows.NTStatus
			assert.ErrorAs(t, err, &status)
			if exhaust {
				assert.Equal(t, windows.STATUS_SHARING_VIOLATION, status)
				assert.Equal(t, 7, attempts)
				assert.Equal(t, 6, sleeps)
			} else {
				assert.Equal(t, windows.STATUS_FILE_IS_A_DIRECTORY, status)
				assert.Equal(t, 1, attempts)
				assert.Zero(t, sleeps)
			}
		})
	}
	t.Run("missing_child_retains_status_through_open_wrapper", func(t *testing.T) {
		f := newU1jFixture(t)
		handle, err := openShipmentOpsWindowsChild(f.dir, "missing.json", windows.GENERIC_READ, windows.FILE_OPEN)
		if err == nil {
			require.NoError(t, windows.CloseHandle(handle))
		}
		require.Error(t, err)
		var status windows.NTStatus
		assert.ErrorAs(t, err, &status)
		assert.True(t, shipmentOpsWindowsIsNotExist(err), "absent journal must remain recognizable through the open wrapper")
	})
}

func TestU1j_PermanentTargetDenialFastFail(t *testing.T) {
	for _, directory := range []bool{false, true} {
		for _, failure := range []error{nil, windows.STATUS_ACCESS_DENIED, windows.ERROR_ACCESS_DENIED} {
			for _, wrapped := range []bool{false, true} {
				t.Run(fmt.Sprintf("directory_%t/error_%v/wrapped_%t", directory, failure, wrapped), func(t *testing.T) {
					f := newU1jFixture(t)
					target := filepath.Join(f.opsRoot, testShipmentOperationJournalName)
					if directory {
						require.NoError(t, os.Remove(target))
						require.NoError(t, os.Mkdir(target, 0o700))
						require.NoError(t, os.WriteFile(filepath.Join(target, "preimage"), []byte("original"), 0o600))
					} else {
						require.NoError(t, os.Chmod(target, 0o400))
						t.Cleanup(func() { require.NoError(t, os.Chmod(target, 0o600)) })
					}
					renameFn := shipmentOpsNtRenameFn
					attempts, sleeps := 0, 0
					shipmentOpsNtRenameFn = func(handle windows.Handle, iosb *windows.IO_STATUS_BLOCK, info *byte, length, class uint32) error {
						attempts++
						err := renameFn(handle, iosb, info, length, class)
						require.Error(t, err, "real replacement of a permanent-denial target must fail")
						if failure != nil {
							err = failure
						}
						if wrapped {
							err = fmt.Errorf("injected wrapper: %w", err)
						}
						return err
					}
					shipmentOpsRetrySleep = func(time.Duration) { sleeps++ }
					err := f.mutate("rename")
					require.Error(t, err)
					if failure != nil {
						assert.ErrorIs(t, shipmentOpsWindowsError(err), shipmentOpsWindowsError(failure))
					}
					assert.Equal(t, 1, attempts, "permanent target denial must fail on the first attempt")
					assert.Zero(t, sleeps, "permanent target denial must never back off")
					preimage := target
					if directory {
						preimage = filepath.Join(target, "preimage")
					}
					assert.Equal(t, "original", string(requireReadFile(t, preimage)))
					assert.Equal(t, "replacement", string(requireReadFile(t, filepath.Join(f.opsRoot, f.tempName))))
				})
			}
		}
	}
}

func TestU1j_PermanentDenialProbeUsesBoundDirectory(t *testing.T) {
	for _, boundReadOnly := range []bool{false, true} {
		t.Run(fmt.Sprintf("bound_readonly_%t", boundReadOnly), func(t *testing.T) {
			f := newU1jFixture(t)
			swapShipmentOpsDirectoryObject(t, f.opsRoot)
			boundTarget := filepath.Join(f.opsRoot+"-original", testShipmentOperationJournalName)
			decoyTarget := filepath.Join(f.opsRoot, testShipmentOperationJournalName)
			require.NoError(t, os.WriteFile(decoyTarget, []byte("decoy"), 0o600))
			readOnly := decoyTarget
			if boundReadOnly {
				readOnly = boundTarget
			}
			require.NoError(t, os.Chmod(readOnly, 0o400))
			t.Cleanup(func() { require.NoError(t, os.Chmod(readOnly, 0o600)) })
			attempts := installU1jMutation(t, f.dir, "rename", windows.STATUS_ACCESS_DENIED, 0)
			sleeps := make([]time.Duration, 0, 6)
			shipmentOpsRetrySleep = func(delay time.Duration) { sleeps = append(sleeps, delay) }
			err := f.mutate("rename")
			assert.ErrorIs(t, shipmentOpsWindowsError(err), windows.ERROR_ACCESS_DENIED)
			if boundReadOnly {
				assert.Equal(t, 1, *attempts)
				assert.Empty(t, sleeps, "the bound read-only target must fast-fail despite a writable decoy")
			} else {
				assert.Equal(t, 7, *attempts)
				assertU1jBackoff(t, sleeps)
			}
			assert.Equal(t, "original", string(requireReadFile(t, boundTarget)))
			assert.Equal(t, "decoy", string(requireReadFile(t, decoyTarget)))
			assert.Equal(t, "replacement", string(requireReadFile(t, filepath.Join(f.opsRoot+"-original", f.tempName))))
		})
	}
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

func readU1jSharedFile(t *testing.T, path string) []byte {
	t.Helper()
	name, err := windows.UTF16PtrFromString(path)
	require.NoError(t, err)
	handle, err := windows.CreateFile(name, windows.GENERIC_READ,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil, windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL, 0)
	require.NoError(t, err)
	file := os.NewFile(uintptr(handle), path)
	require.NotNil(t, file)
	data, readErr := io.ReadAll(file)
	closeErr := file.Close()
	require.NoError(t, readErr)
	require.NoError(t, closeErr)
	return data
}

func runU1jMutation(t *testing.T, operation string, failure error, exhaust, swap bool) {
	t.Helper()
	f := newU1jFixture(t)
	succeedAt := 7
	if exhaust {
		succeedAt = 0
	}
	attempts := installU1jMutation(t, f.dir, operation, failure, succeedAt)
	sleeps := make([]time.Duration, 0, 6)
	actualRoot := f.opsRoot
	if swap {
		// Swap after directory validation but before mutate pins its source
		// child. Windows may reject a parent rename while that child is open.
		swapShipmentOpsDirectoryObject(t, f.opsRoot)
		actualRoot = f.opsRoot + "-original"
		require.NoError(t, os.WriteFile(filepath.Join(f.opsRoot, f.tempName), []byte("decoy-temp"), 0o600))
		require.NoError(t, os.WriteFile(filepath.Join(f.opsRoot, testShipmentOperationJournalName), []byte("decoy-target"), 0o600))
	}
	shipmentOpsRetrySleep = func(delay time.Duration) {
		sleeps = append(sleeps, delay)
		assert.Equal(t, len(sleeps), *attempts, "sleep must follow each failed attempt")
		if swap && len(sleeps) == 1 {
			// Observe the in-flight restriction without making OS rejection
			// a contract on filesystems that permit this parent rename.
			probeRoot := actualRoot + "-in-flight"
			swapErr := os.Rename(actualRoot, probeRoot)
			if swapErr == nil {
				require.NoError(t, os.Rename(probeRoot, actualRoot))
				t.Log("filesystem permits in-flight parent rename; original directory restored")
			} else {
				require.True(t, errors.Is(swapErr, windows.ERROR_ACCESS_DENIED) ||
					errors.Is(swapErr, windows.ERROR_SHARING_VIOLATION),
					"in-flight parent rename must fail only for access/sharing contention: %v", swapErr)
				t.Logf("in-flight parent rename rejected while source handle is pinned: %v", swapErr)
			}
		}
		assert.Equal(t, "original", string(readU1jSharedFile(t, filepath.Join(actualRoot, testShipmentOperationJournalName))))
		assert.Equal(t, "replacement", string(readU1jSharedFile(t, filepath.Join(actualRoot, f.tempName))))
		if swap {
			assert.Equal(t, "decoy-temp", string(readU1jSharedFile(t, filepath.Join(f.opsRoot, f.tempName))))
			assert.Equal(t, "decoy-target", string(readU1jSharedFile(t, filepath.Join(f.opsRoot, testShipmentOperationJournalName))))
		}
	}
	err := f.mutate(operation)
	assert.Equal(t, 7, *attempts, "U1j: bounded retry requires seven total attempts")
	assertU1jBackoff(t, sleeps)
	if exhaust {
		assert.ErrorIs(t, shipmentOpsWindowsError(err), shipmentOpsWindowsError(failure))
		assert.Equal(t, "replacement", string(requireReadFile(t, filepath.Join(actualRoot, f.tempName))))
	} else {
		assert.NoError(t, err, "U1j: sixth retry must apply the operation")
		_, statErr := os.Stat(filepath.Join(actualRoot, f.tempName))
		assert.ErrorIs(t, statErr, os.ErrNotExist)
	}
	want := "original"
	if !exhaust && operation == "rename" {
		want = "replacement"
	}
	assert.Equal(t, want, string(requireReadFile(t, filepath.Join(actualRoot, testShipmentOperationJournalName))))
	if swap {
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
	// Deny DELETE sharing until the sixth injected sleep; no goroutines,
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
	sleeps := make([]time.Duration, 0, 6)
	shipmentOpsRetrySleep = func(delay time.Duration) {
		sleeps = append(sleeps, delay)
		if !exhaust && len(sleeps) == 6 {
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
	want := []time.Duration{time.Millisecond, 2 * time.Millisecond, 4 * time.Millisecond,
		8 * time.Millisecond, 16 * time.Millisecond, 32 * time.Millisecond}
	assert.Equal(t, want, sleeps, "U1j: exactly six exponential sleeps, no sleep after final attempt")
	var total time.Duration
	for _, delay := range sleeps {
		total += delay
	}
	assert.Equal(t, 63*time.Millisecond, total)
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

	err := os.Rename(opsRoot, opsRoot+"-original")
	for retry := 0; retry < 6 && isTransientShipmentOpsWindowsError(err); retry++ {
		time.Sleep(time.Millisecond << retry)
		err = os.Rename(opsRoot, opsRoot+"-original")
	}
	require.NoError(t, err,
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
