package integration_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRenameOverwritesExistingFile(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source.txt")
	destination := filepath.Join(dir, "destination.txt")

	require.NoError(t, os.WriteFile(source, []byte("source"), 0o644))
	require.NoError(t, os.WriteFile(destination, []byte("destination"), 0o644))
	require.NoError(t, os.Rename(source, destination))

	data, err := os.ReadFile(destination)
	require.NoError(t, err)
	require.Equal(t, "source", string(data))
}
