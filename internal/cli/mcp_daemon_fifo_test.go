//go:build !windows

package cli

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	corerrors "github.com/softwaresalt/backlogit/internal/errors"
)

func TestU2_RejectsNonRegularSidecar(t *testing.T) {
	_, _, runtimeDir := u2Workspace(t)
	require.NoError(t, os.MkdirAll(runtimeDir, 0o700))
	require.NoError(t, syscall.Mkfifo(filepath.Join(runtimeDir, u2SidecarName), 0o600))

	root, err := os.OpenRoot(runtimeDir)
	require.NoError(t, err)
	defer func() { _ = root.Close() }()

	_, err = readMCPDaemonInfo(root)
	assert.True(t, errors.Is(err, corerrors.ErrValidation), "got %v", err)
}
