package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestU6_SelfInstallMidFlightParentAndLeafSwapRejected(t *testing.T) {
	t.Run("binary-parent-swap-rejected", func(t *testing.T) {
		root := t.TempDir()
		storageDir := filepath.Join(root, ".backlog")
		require.NoError(t, os.MkdirAll(storageDir, 0o755))
		outsideDir := t.TempDir()
		backupDir := storageDir + ".orig"

		origHook := selfInstallBeforeBinaryReplaceHook
		t.Cleanup(func() { selfInstallBeforeBinaryReplaceHook = origHook })
		selfInstallBeforeBinaryReplaceHook = func() {
			require.NoError(t, os.Rename(storageDir, backupDir))
			if runtime.GOOS == "windows" {
				out, err := exec.Command("cmd", "/c", "mklink", "/J", storageDir, outsideDir).CombinedOutput()
				require.NoError(t, err, "mklink /J failed: %s", out)
			} else {
				require.NoError(t, os.Symlink(outsideDir, storageDir))
			}
		}

		_, err := installWorkspaceRuntimeExecutable(root, ".backlog")
		if runtime.GOOS == "windows" {
			require.Error(t, err, "Windows mid-flight storage directory swap must fail closed")
		}
		binName := "backlogit"
		if runtime.GOOS == "windows" {
			binName = "backlogit.exe"
		}
		assert.NoFileExists(t, filepath.Join(outsideDir, binName))
	})

	t.Run("ignore-leaf-swap-rejected", func(t *testing.T) {
		root := t.TempDir()
		require.NoError(t, os.MkdirAll(filepath.Join(root, ".git", "info"), 0o755))
		outsideFile := filepath.Join(t.TempDir(), "outside-exclude")
		require.NoError(t, os.WriteFile(outsideFile, []byte("untouched\n"), 0o600))

		origHook := selfInstallBeforeIgnoreReplaceHook
		t.Cleanup(func() { selfInstallBeforeIgnoreReplaceHook = origHook })
		selfInstallBeforeIgnoreReplaceHook = func(fileName string) {
			if fileName != "exclude" {
				return
			}
			excludePath := filepath.Join(root, ".git", "info", "exclude")
			if runtime.GOOS == "windows" {
				out, linkErr := exec.Command("cmd", "/c", "mklink", excludePath, outsideFile).CombinedOutput()
				if linkErr != nil {
					t.Skipf("file symlink requires privilege on Windows: %v (%s)", linkErr, out)
				}
			} else {
				require.NoError(t, os.Symlink(outsideFile, excludePath))
			}
		}

		err := ensureWorkspaceRuntimeIgnored(root, ".backlog", "backlogit")
		require.Error(t, err, "swapped symlinked exclude leaf must be rejected")
		got, readErr := os.ReadFile(outsideFile)
		require.NoError(t, readErr)
		assert.Equal(t, "untouched\n", string(got))
	})
}
