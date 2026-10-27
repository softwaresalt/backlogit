//go:build linux || darwin || freebsd || netbsd || openbsd || dragonfly

package core

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"

	blerrors "github.com/softwaresalt/backlogit/internal/errors"
)

// TestReadShipmentReconcileMemberFile_RefusesNonRegularAndSymlinkedAncestors
// proves a FIFO neither blocks nor is read, and that a symlinked intermediate
// directory is refused by the descriptor-relative walk.
func TestReadShipmentReconcileMemberFile_RefusesNonRegularAndSymlinkedAncestors(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "queue"), 0o755))

	fifo := filepath.Join(root, "queue", "pipe.md")
	require.NoError(t, unix.Mkfifo(fifo, 0o644))
	done := make(chan error, 1)
	go func() {
		_, err := readShipmentReconcileMemberFile(root, fifo)
		done <- err
	}()
	select {
	case err := <-done:
		require.Error(t, err)
		assert.ErrorIs(t, err, blerrors.ErrValidation)
	case <-time.After(5 * time.Second):
		t.Fatal("reading a FIFO blocked")
	}

	outside := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(outside, "x.md"), []byte("secret"), 0o644))
	require.NoError(t, os.Symlink(outside, filepath.Join(root, "linked")))
	_, err := readShipmentReconcileMemberFile(root, filepath.Join(root, "linked", "x.md"))
	require.Error(t, err)
	assert.ErrorIs(t, err, blerrors.ErrValidation)
}
