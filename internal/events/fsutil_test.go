package events

// Harness for Unit 1: syncWriteFile helpers (040.001-T dependency, 040.002-T dependency).
//
// These tests use package events (internal) to access the unexported helper
// functions directly.

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---- syncAppendLine ----------------------------------------------------------

func TestSyncAppendLine_WritesDataAndIsReadable(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.jsonl")

	data := []byte(`{"key":"value"}` + "\n")
	require.NoError(t, syncAppendLine(path, data))

	got, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, data, got)
}

func TestSyncAppendLine_AppendsMultipleLines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.jsonl")

	line1 := []byte(`{"seq":1}` + "\n")
	line2 := []byte(`{"seq":2}` + "\n")

	require.NoError(t, syncAppendLine(path, line1))
	require.NoError(t, syncAppendLine(path, line2))

	got, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, append(line1, line2...), got)
}

func TestSyncAppendLine_CreatesFileIfMissing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "new.jsonl")

	require.NoError(t, syncAppendLine(path, []byte("data\n")))

	_, err := os.Stat(path)
	assert.NoError(t, err, "syncAppendLine must create the file if it does not exist")
}

func TestSyncAppendLine_ErrorOnUnwritablePath(t *testing.T) {
	dir := t.TempDir()
	err := syncAppendLine(filepath.Join(dir, "no_such_dir", "file.jsonl"), []byte("data\n"))
	assert.Error(t, err, "writing to an unwritable path must return an error")
}

// ---- syncWriteFileAtomic ----------------------------------------------------

func TestSyncWriteFileAtomic_WritesDataAndIsReadable(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "checkpoint.json")

	data := []byte(`{"seq":42}`)
	require.NoError(t, syncWriteFileAtomic(path, data, 0o644))

	got, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, data, got)
}

func TestSyncWriteFileAtomic_NoDotTmpAfterSuccess(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.json")

	require.NoError(t, syncWriteFileAtomic(path, []byte(`{}`), 0o644))

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	for _, e := range entries {
		assert.NotContains(t, e.Name(), ".tmp",
			"no temp files must remain after syncWriteFileAtomic succeeds")
	}
}

func TestSyncWriteFileAtomic_OverwritesExistingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "data.json")

	require.NoError(t, syncWriteFileAtomic(path, []byte(`{"v":1}`), 0o644))
	require.NoError(t, syncWriteFileAtomic(path, []byte(`{"v":2}`), 0o644))

	got, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, []byte(`{"v":2}`), got, "syncWriteFileAtomic must replace the existing file")
}

func TestSyncWriteFileAtomic_ErrorOnUnwritablePath(t *testing.T) {
	dir := t.TempDir()
	err := syncWriteFileAtomic(filepath.Join(dir, "no_such_dir", "out.json"), []byte("{}"), 0o644)
	assert.Error(t, err, "writing to an unwritable path must return an error")
}
