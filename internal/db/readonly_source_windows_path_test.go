//go:build windows

package db

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSnapshotExtendedDriveDirectory(t *testing.T) {
	dir := t.TempDir()
	parent, _, err := pinSnapshotDirectory(`\\?\` + filepath.Clean(dir))
	require.NoError(t, err)
	require.NoError(t, parent.Close())
}

func TestNormalizeSnapshotWindowsPath(t *testing.T) {
	for _, tt := range []struct{ input, want string }{
		{`\\?\UNC\server\share\dir`, `\\server\share\dir`},
		{`\\?\unc\server\share\dir`, `\\server\share\dir`},
		{`\\?\C:\dir`, `C:\dir`},
		{`\\server\share\dir`, `\\server\share\dir`},
	} {
		t.Run(tt.input, func(t *testing.T) {
			require.Equal(t, tt.want, normalizeSnapshotWindowsPath(tt.input))
		})
	}
}
