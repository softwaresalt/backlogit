package evidencedag

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestU163_005_Remediation8ReadWorkspaceFileIsBoundedAndContained(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".github", "policies"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, ".github", "policies", "r.yaml"), []byte("hello"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(root, "big.json"), []byte(strings.Repeat("a", 64)), 0o600))
	require.NoError(t, os.Mkdir(filepath.Join(root, "dir"), 0o755))

	t.Run("hidden component allowed", func(t *testing.T) {
		got, err := ReadWorkspaceFile(root, ".github/policies/r.yaml", 1024)
		require.NoError(t, err)
		assert.Equal(t, "hello", string(got))
	})
	t.Run("rejected inputs", func(t *testing.T) {
		for name, rel := range map[string]string{
			"empty": "", "parent": "../x", "nested parent": "dir/../../x", "absolute": filepath.Join(root, "big.json"),
			"rooted": "/etc/passwd", "backslash rooted": `\x`, "dot": "./big.json", "double slash": "dir//x", "nul": "a\x00b",
		} {
			_, err := ReadWorkspaceFile(root, rel, 1024)
			assert.Error(t, err, name)
		}
	})
	t.Run("directory and missing", func(t *testing.T) {
		_, err := ReadWorkspaceFile(root, "dir", 1024)
		assert.Error(t, err)
		_, err = ReadWorkspaceFile(root, "absent.json", 1024)
		assert.Error(t, err)
	})
	t.Run("oversize", func(t *testing.T) {
		_, err := ReadWorkspaceFile(root, "big.json", 8)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "exceeds")
	})
	t.Run("symlink", func(t *testing.T) {
		outside := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(outside, "s.json"), []byte("x"), 0o600))
		if err := os.Symlink(filepath.Join(outside, "s.json"), filepath.Join(root, "link.json")); err != nil {
			t.Skip("symlinks unavailable: " + err.Error())
		}
		_, err := ReadWorkspaceFile(root, "link.json", 1024)
		assert.Error(t, err)
	})
	t.Run("fifo does not hang", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("no FIFOs on Windows")
		}
		if err := mkfifo(filepath.Join(root, "pipe.json")); err != nil {
			t.Skip("mkfifo unavailable")
		}
		done := make(chan error, 1)
		go func() {
			_, err := ReadWorkspaceFile(root, "pipe.json", 1024)
			done <- err
		}()
		select {
		case err := <-done:
			assert.Error(t, err)
		case <-time.After(5 * time.Second):
			t.Fatal("reading a FIFO hung")
		}
	})
}

func TestU163_005_Remediation9LoadUsesDecodeStrict(t *testing.T) {
	// Load and DecodeStrict must agree on every defect class.
	for name, doc := range map[string]string{
		"duplicate key": `{"schema_version":1,"schema_version":1,"nodes":[],"edges":[]}`,
		"unknown field": `{"schema_version":1,"nodes":[],"edges":[],"x":1}`,
		"trailing data": `{"schema_version":1,"nodes":[],"edges":[]} {}`,
		"empty":         "  ",
		"yaml anchor":   "schema_version: 1\nnodes: &a []\nedges: *a\n",
	} {
		t.Run(name, func(t *testing.T) {
			var d Declaration
			strictErr := DecodeStrict([]byte(doc), &d)
			require.Error(t, strictErr)
			_, loadErr := Load([]byte(doc))
			require.Error(t, loadErr)
			assert.ErrorIs(t, loadErr, ErrInvalidDeclaration)
		})
	}
	assert.True(t, ValidTaskID("163.005-T"))
	assert.False(t, ValidTaskID("163.005-t"))
	assert.False(t, ValidTaskID(strings.Repeat("1", MaxIDLength+1)+"-T"))
}
