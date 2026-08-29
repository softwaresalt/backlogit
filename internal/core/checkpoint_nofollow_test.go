package core

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestReadFileNoFollow_U4_SymlinkIsRejected (148.003-T / U4) verifies that
// readFileNoFollow rejects a path that resolves to a symlink, protecting
// against TOCTOU replacement between confinement check and actual file read.
// Skipped on Windows where creating symlinks requires elevated privileges.
func TestReadFileNoFollow_U4_SymlinkIsRejected(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation requires elevated privileges on Windows")
	}

	dir := t.TempDir()
	realFile := filepath.Join(dir, "real.json")
	require.NoError(t, os.WriteFile(realFile, []byte(`{"test":1}`), 0o644))

	linkFile := filepath.Join(dir, "link.json")
	require.NoError(t, os.Symlink(realFile, linkFile))

	_, err := readFileNoFollow(linkFile)
	require.Error(t, err, "readFileNoFollow must refuse to read through a symlink")
}

// TestReadFileNoFollow_U4_RegularFileSucceeds verifies that readFileNoFollow
// reads a regular (non-symlink) file successfully and returns its bytes.
func TestReadFileNoFollow_U4_RegularFileSucceeds(t *testing.T) {
	dir := t.TempDir()
	realFile := filepath.Join(dir, "real.json")
	content := []byte(`{"test":1}`)
	require.NoError(t, os.WriteFile(realFile, content, 0o644))

	data, err := readFileNoFollow(realFile)
	require.NoError(t, err)
	assert.Equal(t, content, data)
}

// TestReadFileNoFollow_U4_NonExistentFileErrors verifies standard not-found
// behavior is preserved — readFileNoFollow is not a symlink-only filter.
func TestReadFileNoFollow_U4_NonExistentFileErrors(t *testing.T) {
	dir := t.TempDir()
	nonexistent := filepath.Join(dir, "does_not_exist.json")

	_, err := readFileNoFollow(nonexistent)
	require.Error(t, err)
	require.True(t, os.IsNotExist(err), "non-existent path must return os.IsNotExist error")
}
