package atomicfile

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriteFileAtomic_OverwriteContentByteIdentical(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "doc.md")
	require.NoError(t, os.WriteFile(path, []byte("orig"), 0o644))

	want := []byte("updated body\r\nwith CRLF\n")
	require.NoError(t, WriteFileAtomic(path, want))

	got, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, want, got, "overwritten content must be byte-identical")
}

func TestWriteFileAtomic_OverwritePreservesExistingMode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permission bits are not represented on Windows filesystems")
	}
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "doc.md")
	require.NoError(t, os.WriteFile(path, []byte("orig"), 0o600))

	require.NoError(t, WriteFileAtomic(path, []byte("updated")))

	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm(),
		"an in-place rewrite preserves the source 0600 mode, not the 0600/0644 temp default")
	got, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "updated", string(got))
}

func TestWriteFileAtomic_NewFileCreatedAt0644(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "new.md")

	require.NoError(t, WriteFileAtomic(path, []byte("data")))

	got, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "data", string(got), "new file must hold the exact content")

	if runtime.GOOS == "windows" {
		t.Skip("POSIX permission bits are not represented on Windows filesystems")
	}
	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o644), info.Mode().Perm(), "a new file gets the 0644 default, not 0600")
}

// TestWriteFileAtomic_OverPermissiveSourceClampedTo0644 proves the F9 mode-clamp:
// a 0666 source is written back at 0644 (group/world write bits stripped via
// perm &^ 0o022), never perpetuating an over-permissive record.
func TestWriteFileAtomic_OverPermissiveSourceClampedTo0644(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permission bits are not represented on Windows filesystems")
	}
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "loose.md")
	require.NoError(t, os.WriteFile(path, []byte("orig"), 0o666))
	// os.WriteFile honors umask; force the on-disk mode to a true 0666 so the
	// clamp is what tightens it, not the umask.
	require.NoError(t, os.Chmod(path, 0o666))

	require.NoError(t, WriteFileAtomic(path, []byte("clamped")))

	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o644), info.Mode().Perm(),
		"an over-permissive 0666 source must be clamped to 0644, not perpetuated")
}

// shortWriter is the falsifiable fake for the io.Writer seam: it accepts at most
// limit bytes per Write and reports that (smaller) count with a nil error,
// exactly the silent short-write a real os.File.Write would never produce. It
// forces the writeAll short-write guard to be exercised deterministically.
type shortWriter struct {
	limit int
	wrote []byte
}

func (s *shortWriter) Write(p []byte) (int, error) {
	n := len(p)
	if n > s.limit {
		n = s.limit
	}
	s.wrote = append(s.wrote, p[:n]...)
	return n, nil
}

// TestWriteAll_ShortWriteSurfacedAsError drives the io.Writer seam with a fake
// that writes fewer bytes than requested and asserts the short-write guard turns
// it into a hard error (a real os.File.Write never short-writes, so this branch
// is otherwise unreachable in a unit test).
func TestWriteAll_ShortWriteSurfacedAsError(t *testing.T) {
	t.Parallel()
	data := []byte("twelve bytes")
	require.Len(t, data, 12)

	sw := &shortWriter{limit: 5}
	err := writeAll(sw, data)
	require.Error(t, err, "a short write must surface as an error, not be silently dropped")
	assert.Contains(t, err.Error(), "short write")
}

// TestWriteAll_FullWriteSucceeds is the positive companion: a writer that
// accepts every byte returns no error.
func TestWriteAll_FullWriteSucceeds(t *testing.T) {
	t.Parallel()
	data := []byte("twelve bytes")
	sw := &shortWriter{limit: len(data)}
	require.NoError(t, writeAll(sw, data))
	assert.Equal(t, data, sw.wrote)
}
