package events

import (
	"context"
	"errors"
	"os"
	"testing"

	blerrors "github.com/softwaresalt/backlogit/internal/errors"
	"github.com/stretchr/testify/require"
)

// TestCreateCheckpoint_U3_IndeterminateWrite_ClassifiedCorrectly (148.002-T / U3)
// uses the syncWriteFileAtomicHook seam to simulate an ErrWriteIndeterminate
// outcome and asserts that CreateCheckpoint propagates the classified error
// via errors.Is rather than returning a bare opaque message.
func TestCreateCheckpoint_U3_IndeterminateWrite_ClassifiedCorrectly(t *testing.T) {
	dir := t.TempDir()
	stateDump := `{"schema_version":1,"agent":"ship","session_id":"s1","phase":"build"}`

	// Install an indeterminate-simulating seam.
	original := syncWriteFileAtomicHook
	syncWriteFileAtomicHook = func(_ string, _ []byte, _ os.FileMode) error {
		return errors.Join(blerrors.ErrWriteIndeterminate, errors.New("simulated post-rename fsync failure"))
	}
	t.Cleanup(func() { syncWriteFileAtomicHook = original })

	_, err := CreateCheckpoint(context.Background(), dir, stateDump)
	require.Error(t, err, "indeterminate write must return an error")
	require.True(t, errors.Is(err, blerrors.ErrWriteIndeterminate),
		"error must satisfy errors.Is(err, ErrWriteIndeterminate), got: %v", err)
}

// TestCreateCheckpoint_U3_NotAppliedWrite_ClassifiedCorrectly asserts that
// an ErrWriteNotApplied error from the write is propagated and classifiable.
func TestCreateCheckpoint_U3_NotAppliedWrite_ClassifiedCorrectly(t *testing.T) {
	dir := t.TempDir()
	stateDump := `{"schema_version":1,"agent":"ship","session_id":"s1","phase":"build"}`

	original := syncWriteFileAtomicHook
	syncWriteFileAtomicHook = func(_ string, _ []byte, _ os.FileMode) error {
		return errors.Join(blerrors.ErrWriteNotApplied, errors.New("simulated create-temp failure"))
	}
	t.Cleanup(func() { syncWriteFileAtomicHook = original })

	_, err := CreateCheckpoint(context.Background(), dir, stateDump)
	require.Error(t, err, "not-applied write must return an error")
	require.True(t, errors.Is(err, blerrors.ErrWriteNotApplied),
		"error must satisfy errors.Is(err, ErrWriteNotApplied), got: %v", err)
}
