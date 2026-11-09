//go:build windows

package atomicfile

import (
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

// TestU1i_AtomicReplaceRetriesTransientWindowsErrorsWithExponentialBackoff
// gates the retry implementation; all seam mutations are sequential.
func TestU1i_AtomicReplaceRetriesTransientWindowsErrorsWithExponentialBackoff(t *testing.T) {
	transients := []struct {
		name string
		err  error
	}{
		{"access", windows.ERROR_ACCESS_DENIED},
		{"sharing", windows.ERROR_SHARING_VIOLATION},
		{"wrapped_access", fmt.Errorf("injected: %w", windows.ERROR_ACCESS_DENIED)},
		{"wrapped_sharing", fmt.Errorf("injected: %w", windows.ERROR_SHARING_VIOLATION)},
	}
	for _, transient := range transients {
		for _, durable := range []bool{false, true} {
			for _, exhaust := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/durable_%t/exhaust_%t", transient.name, durable, exhaust), func(t *testing.T) {
					dir := t.TempDir()
					source, target := filepath.Join(dir, "source.tmp"), filepath.Join(dir, "target.md")
					require.NoError(t, os.WriteFile(source, []byte("replacement"), 0o600))
					require.NoError(t, os.WriteFile(target, []byte("original"), 0o600))
					originalMove, originalSleep := moveFileEx, atomicReplaceSleep
					t.Cleanup(func() { moveFileEx, atomicReplaceSleep = originalMove, originalSleep })
					attempts := 0
					sleeps := make([]time.Duration, 0, 7)
					atomicReplaceSleep = func(delay time.Duration) {
						sleeps = append(sleeps, delay)
						assert.Equal(t, len(sleeps), attempts, "sleep must follow each failed attempt")
						assert.Equal(t, "original", string(readU1iFile(t, target)),
							"destination must survive every backoff")
					}
					moveFileEx = func(from, to string, flags uint32) error {
						attempts++
						assert.Equal(t, source, from)
						assert.Equal(t, target, to)
						assert.Equal(t, moveFileExFlags(durable), flags)
						if exhaust || attempts < 8 {
							return transient.err
						}
						return originalMove(from, to, flags)
					}
					err := atomicReplace(source, target, durable)
					assert.Equal(t, 8, attempts, "U1i: bounded retry requires eight total attempts")
					assertU1iBackoff(t, sleeps)
					if exhaust {
						assert.ErrorIs(t, err, blerrors.ErrWriteNotApplied)
						assert.ErrorIs(t, err, transient.err)
						assert.Equal(t, "original", string(readU1iFile(t, target)))
						assert.Equal(t, "replacement", string(readU1iFile(t, source)))
					} else {
						assert.NoError(t, err, "U1i: seventh retry must commit replacement")
						assert.Equal(t, "replacement", string(readU1iFile(t, target)))
					}
				})
			}
		}
	}
	for _, failure := range []error{windows.ERROR_FILE_NOT_FOUND, errors.New("permanent replace failure")} {
		t.Run(fmt.Sprintf("immediate_%v", failure), func(t *testing.T) {
			originalMove, originalSleep := moveFileEx, atomicReplaceSleep
			t.Cleanup(func() { moveFileEx, atomicReplaceSleep = originalMove, originalSleep })
			attempts, sleeps := 0, 0
			moveFileEx = func(_, _ string, _ uint32) error { attempts++; return failure }
			atomicReplaceSleep = func(time.Duration) { sleeps++ }
			err := atomicReplace("source.tmp", "target.md", false)
			assert.ErrorIs(t, err, failure)
			assert.ErrorIs(t, err, blerrors.ErrWriteNotApplied)
			assert.Equal(t, 1, attempts)
			assert.Zero(t, sleeps)
		})
	}
}

func readU1iFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	return data
}

func assertU1iBackoff(t *testing.T, sleeps []time.Duration) {
	t.Helper()
	want := []time.Duration{5 * time.Millisecond, 10 * time.Millisecond, 20 * time.Millisecond,
		40 * time.Millisecond, 80 * time.Millisecond, 160 * time.Millisecond, 320 * time.Millisecond}
	assert.Equal(t, want, sleeps, "U1i: exactly seven exponential sleeps, no sleep after final attempt")
	var total time.Duration
	for _, delay := range sleeps {
		total += delay
	}
	assert.Equal(t, 635*time.Millisecond, total)
}

// TestMoveFileExFlags_Gating asserts the safety/durability flag split: the
// atomic-replace safety flag (MOVEFILE_REPLACE_EXISTING) is always set, while
// the write-through durability flush is added only in durable mode.
func TestMoveFileExFlags_Gating(t *testing.T) {
	t.Parallel()

	off := moveFileExFlags(false)
	assert.Equal(t, uint32(windows.MOVEFILE_REPLACE_EXISTING),
		off&windows.MOVEFILE_REPLACE_EXISTING,
		"REPLACE_EXISTING must be set unconditionally")
	assert.Zero(t, off&windows.MOVEFILE_WRITE_THROUGH,
		"WRITE_THROUGH must NOT be set when durable is off")

	on := moveFileExFlags(true)
	assert.Equal(t, uint32(windows.MOVEFILE_REPLACE_EXISTING),
		on&windows.MOVEFILE_REPLACE_EXISTING,
		"REPLACE_EXISTING must be set in durable mode too")
	assert.Equal(t, uint32(windows.MOVEFILE_WRITE_THROUGH),
		on&windows.MOVEFILE_WRITE_THROUGH,
		"WRITE_THROUGH must be set when durable is on")
}

// TestAtomicReplace_PassesGatedFlagsThroughSeam drives the injected MoveFileEx
// seam and asserts atomicReplace forwards exactly the gated flags.
//
// Must not run with t.Parallel: this test swaps a package-global seam
// (moveFileEx) read on the production write path.
func TestAtomicReplace_PassesGatedFlagsThroughSeam(t *testing.T) {
	var captured uint32
	orig := moveFileEx
	moveFileEx = func(_, _ string, flags uint32) error {
		captured = flags
		return nil
	}
	t.Cleanup(func() { moveFileEx = orig })

	require.NoError(t, atomicReplace("from.tmp", "to.md", false))
	assert.Equal(t, uint32(windows.MOVEFILE_REPLACE_EXISTING), captured,
		"durable=false must pass REPLACE_EXISTING only")

	require.NoError(t, atomicReplace("from.tmp", "to.md", true))
	assert.Equal(t,
		uint32(windows.MOVEFILE_REPLACE_EXISTING|windows.MOVEFILE_WRITE_THROUGH), captured,
		"durable=true must pass REPLACE_EXISTING | WRITE_THROUGH")
}

// TestAtomicReplace_FailureReturnsNotApplied asserts a MoveFileEx failure is
// classified as ErrWriteNotApplied (the rename never committed).
func TestAtomicReplace_FailureReturnsNotApplied(t *testing.T) {
	orig := moveFileEx
	moveFileEx = func(_, _ string, _ uint32) error {
		return errors.New("simulated MoveFileEx failure")
	}
	t.Cleanup(func() { moveFileEx = orig })

	err := atomicReplace("from.tmp", "to.md", false)
	require.Error(t, err)
	assert.True(t, blerrors.IsWriteNotApplied(err),
		"a failed atomic replace must be classified not-applied")
}

// TestWriteFileAtomic_ReplaceFailureLeavesOriginalIntact asserts the Windows
// path never removes the destination before the replacement lands: on a
// simulated replace failure the original content is still present.
func TestWriteFileAtomic_ReplaceFailureLeavesOriginalIntact(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "doc.md")
	require.NoError(t, os.WriteFile(path, []byte("original"), 0o644))

	orig := moveFileEx
	moveFileEx = func(_, _ string, _ uint32) error {
		return errors.New("simulated MoveFileEx failure")
	}
	t.Cleanup(func() { moveFileEx = orig })

	err := WriteFileAtomic(path, []byte("replacement"))
	require.Error(t, err)
	assert.True(t, blerrors.IsWriteNotApplied(err))

	got, readErr := os.ReadFile(path)
	require.NoError(t, readErr, "destination must never be observed missing")
	assert.Equal(t, "original", string(got),
		"the destination must retain its original content on a failed replace")
}

// TestWriteFileAtomic_WindowsReplaceOverExistingSucceeds is the happy path: a
// real MoveFileEx replace over an existing destination lands the new content.
func TestWriteFileAtomic_WindowsReplaceOverExistingSucceeds(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "doc.md")
	require.NoError(t, os.WriteFile(path, []byte("orig"), 0o644))

	require.NoError(t, WriteFileAtomic(path, []byte("replacement content")))

	got, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "replacement content", string(got))
}

// TestWriteFileAtomic_WindowsAbsentDestCreates asserts the replace path also
// creates a brand-new destination (REPLACE_EXISTING tolerates an absent dest).
func TestWriteFileAtomic_WindowsAbsentDestCreates(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "new.md")

	require.NoError(t, WriteFileAtomic(path, []byte("fresh")))

	got, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "fresh", string(got))
}
