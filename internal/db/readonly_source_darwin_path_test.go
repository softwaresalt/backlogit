//go:build darwin

package db

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSnapshotDarwinSystemVarAlias(t *testing.T) {
	dir := t.TempDir()
	real, err := filepath.EvalSymlinks(dir)
	require.NoError(t, err)
	if !strings.HasPrefix(real, "/private/var/") {
		t.Skip("test directory is not under the macOS /var alias")
	}
	require.NoError(t, os.WriteFile(filepath.Join(dir, "backlogit.db"), []byte("fixture"), 0o600))
	source, err := openSnapshotSource("/var/" + strings.TrimPrefix(real, "/private/var/") + "/backlogit.db")
	require.NoError(t, err)
	require.NoError(t, source.close())
}
