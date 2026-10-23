package stash

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const tempCleanupStashContent = `---
stash_version: "1"
---

- id: AAAA1111
  priority: high
  kind: feature
  text: "Add shipment type"
`

// forceRenameFailure returns src/dst paths where the final rename must fail
// because dst is an existing directory.
func forceRenameFailure(t *testing.T) (srcPath, dstPath string) {
	t.Helper()
	tmpDir := t.TempDir()
	srcPath = filepath.Join(tmpDir, ".stash.md")
	dstPath = filepath.Join(tmpDir, "stash.jsonl")
	require.NoError(t, os.WriteFile(srcPath, []byte(tempCleanupStashContent), 0o644))
	require.NoError(t, os.Mkdir(dstPath, 0o755))
	return srcPath, dstPath
}

// TestMigrateStashMDToJSONL_JoinsTempRemovalError forces the rename to fail and
// the temp-file removal to fail, and requires both failures in the returned error.
func TestMigrateStashMDToJSONL_JoinsTempRemovalError(t *testing.T) {
	srcPath, dstPath := forceRenameFailure(t)
	errRemove := errors.New("injected remove failure")
	orig := removeTempFile
	t.Cleanup(func() { removeTempFile = orig })
	var removed string
	removeTempFile = func(path string) error {
		removed = path
		return errRemove
	}

	count, err := MigrateStashMDToJSONL(srcPath, dstPath)

	require.Error(t, err)
	assert.Equal(t, 0, count)
	assert.ErrorIs(t, err, errRemove, "removal error must be joined")
	assert.Contains(t, err.Error(), "rename jsonl", "original cause must be preserved")
	assert.Contains(t, err.Error(), "remove temp jsonl")
	assert.Equal(t, dstPath+".tmp", removed)
}

// TestMigrateStashMDToJSONL_IgnoresMissingTempOnCleanup requires that an
// already-absent temp file does not add a removal error to the cause.
func TestMigrateStashMDToJSONL_IgnoresMissingTempOnCleanup(t *testing.T) {
	srcPath, dstPath := forceRenameFailure(t)
	orig := removeTempFile
	t.Cleanup(func() { removeTempFile = orig })
	removeTempFile = func(string) error { return os.ErrNotExist }

	_, err := MigrateStashMDToJSONL(srcPath, dstPath)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "rename jsonl")
	assert.NotContains(t, err.Error(), "remove temp jsonl")
}

// TestMigrateStashMDToJSONL_RemovesTempOnRenameFailure documents the happy
// cleanup path with the real remover: no temp file remains.
func TestMigrateStashMDToJSONL_RemovesTempOnRenameFailure(t *testing.T) {
	srcPath, dstPath := forceRenameFailure(t)

	_, err := MigrateStashMDToJSONL(srcPath, dstPath)

	require.Error(t, err)
	assert.NotContains(t, err.Error(), "remove temp jsonl")
	_, statErr := os.Stat(dstPath + ".tmp")
	assert.True(t, os.IsNotExist(statErr), "temp file must be cleaned up")
}
