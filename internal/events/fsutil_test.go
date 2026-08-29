package events

// Harness for Unit 1: syncWriteFile helpers (040.001-T dependency, 040.002-T dependency).
//
// These tests use package events (internal) to access the unexported helper
// functions directly.

import (
	"errors"
	"go/ast"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSyncAppendLineDetailed_ClassifiesOpenVsPostWrite proves the shared append
// primitive (reused by both syncAppendLine and EventWriter.appendDurable)
// distinguishes a pre-write open failure from a post-write fsync failure so the
// two-class error contract can be mapped consistently at both call sites.
func TestSyncAppendLineDetailed_ClassifiesOpenVsPostWrite(t *testing.T) {
	dir := t.TempDir()

	// Pre-write: open fails because the parent directory is missing.
	res := syncAppendLineDetailed(filepath.Join(dir, "no_such_dir", "f.jsonl"), []byte("x\n"), nil)
	require.Error(t, res.err)
	assert.True(t, res.preWrite, "an open failure must be classified as a pre-write failure")

	// Post-write: the write succeeds but the injected fsync fails.
	path := filepath.Join(dir, "f.jsonl")
	res2 := syncAppendLineDetailed(path, []byte("x\n"), func(*os.File) error {
		return errors.New("simulated fsync failure")
	})
	require.Error(t, res2.err)
	assert.False(t, res2.preWrite, "a post-write fsync failure must not be classified as pre-write")

	// Success path returns a zero-value result.
	res3 := syncAppendLineDetailed(path, []byte("y\n"), nil)
	require.NoError(t, res3.err)
	assert.False(t, res3.preWrite)
}

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

// TestSyncWriteFileAtomic_NoPreRemoveInAST is a P-002 FC-1 structural assertion
// (149-F / 149.001-T / stash CB71B412 / INC-P002-131S-148F).
// It parses fsutil.go and asserts that syncWriteFileAtomic contains no
// os.Remove(path) call targeting the destination parameter. The pre-Remove
// block was a data-loss window: if os.Rename fails after os.Remove succeeds
// both the original and the temp file are gone. This test is RED against
// the pre-change source because os.Remove(path) IS present; it turns GREEN
// after the block is removed.
func TestSyncWriteFileAtomic_NoPreRemoveInAST(t *testing.T) {
	file := parseEventsSource(t, "fsutil.go")
	funcDecl := findPackageFuncIn(file, "syncWriteFileAtomic")
	require.NotNil(t, funcDecl, "syncWriteFileAtomic must be declared in fsutil.go")

	// Walk the function body and detect any os.Remove call whose sole argument
	// is the identifier "path" (the destination parameter, not "tmp").
	// os.Remove("tmp") calls are expected on error paths and must not be flagged.
	var foundPreRemove bool
	ast.Inspect(funcDecl.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		pkgIdent, ok := sel.X.(*ast.Ident)
		if !ok || pkgIdent.Name != "os" || sel.Sel.Name != "Remove" {
			return true
		}
		if len(call.Args) == 1 {
			if argIdent, ok := call.Args[0].(*ast.Ident); ok && argIdent.Name == "path" {
				foundPreRemove = true
			}
		}
		return true
	})

	assert.False(t, foundPreRemove,
		"syncWriteFileAtomic must not call os.Remove(path): "+
			"the pre-Remove block creates a data-loss window where the destination "+
			"is deleted before Rename succeeds (CB71B412 / 149-F / INC-P002-131S-148F)")
}
