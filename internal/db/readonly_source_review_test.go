package db

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRound3SnapshotRejectsRedirectedSource(t *testing.T) {
	for _, leaf := range []string{"db", "wal", "dangling-wal", "hardlink-db"} {
		t.Run(leaf, func(t *testing.T) {
			_, source := reviewReadOnlyWriter(t)
			root := t.TempDir()
			path := filepath.Join(root, "backlogit.db")
			base, err := os.ReadFile(source)
			require.NoError(t, err)
			require.NoError(t, os.WriteFile(path, base, 0o600))
			switch leaf {
			case "db":
				require.NoError(t, os.Remove(path))
				require.NoError(t, os.Symlink(source, path))
			case "hardlink-db":
				require.NoError(t, os.Remove(path))
				require.NoError(t, os.Link(source, path))
			case "wal":
				require.NoError(t, os.Symlink(source+"-wal", path+"-wal"))
			case "dangling-wal":
				require.NoError(t, os.Symlink(filepath.Join(t.TempDir(), "missing"), path+"-wal"))
			}

			before, err := os.ReadDir(root)
			require.NoError(t, err)
			base, wal, err := readSnapshotBracket(path)
			require.Error(t, err, "source leaves must be opened without following redirects")
			require.Nil(t, base)
			require.Nil(t, wal)
			after, err := os.ReadDir(root)
			require.NoError(t, err)
			names := func(entries []os.DirEntry) []string {
				out := make([]string, 0, len(entries))
				for _, entry := range entries {
					out = append(out, entry.Name())
				}
				return out
			}
			require.Equal(t, names(before), names(after), "rejection must not create source sidecars")

			cache := t.TempDir()
			t.Setenv("TEMP", cache)
			t.Setenv("TMP", cache)
			t.Setenv("TMPDIR", cache)
			snapshot, err := openReadOnlySnapshot(path)
			if snapshot != nil {
				require.NoError(t, snapshot.Close())
			}
			require.Error(t, err, "unsafe source must be rejected before snapshot allocation")
			cacheEntries, err := os.ReadDir(cache)
			require.NoError(t, err)
			require.Empty(t, cacheEntries, "unsafe source must not allocate a snapshot directory")
		})
	}
}

func TestRound3SnapshotRejectsReplacedOpenLeaf(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "backlogit.db")
	require.NoError(t, os.WriteFile(path, []byte("original"), 0o600))
	source, err := openSnapshotSource(path)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, source.close()) })
	file, err := source.open(source.name)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, file.Close()) })
	require.NoError(t, source.check(file, source.name))
	require.NoError(t, os.Rename(path, path+".old"))
	require.NoError(t, os.WriteFile(path, []byte("replacement"), 0o600))
	require.Error(t, source.check(file, source.name), "replaced basename must not pass handle validation")
}

func TestSnapshotRejectsRedirectedAncestor(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "target")
	require.NoError(t, os.Mkdir(target, 0o700))
	link := filepath.Join(root, "link")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	source, err := openSnapshotSource(filepath.Join(link, "backlogit.db"))
	require.Error(t, err)
	require.Nil(t, source)
}
