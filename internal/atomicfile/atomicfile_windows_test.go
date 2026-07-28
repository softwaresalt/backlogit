//go:build windows

package atomicfile

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/windows"

	blerrors "github.com/softwaresalt/backlogit/internal/errors"
)

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
