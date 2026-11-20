package cli

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	corerrors "github.com/softwaresalt/backlogit/internal/errors"
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
		require.Error(t, err, "mid-flight storage directory swap must fail closed")
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

func TestU1_TargetToolDirectoryCreationBinaryInstallAndIgnoreEntries(t *testing.T) {
	binName := "backlogit"
	if runtime.GOOS == "windows" {
		binName = "backlogit.exe"
	}

	selfExe, err := os.Executable()
	require.NoError(t, err)
	selfBytes, err := os.ReadFile(selfExe)
	require.NoError(t, err)

	createDirRedirect := func(t *testing.T, linkPath, targetPath string) {
		t.Helper()
		if runtime.GOOS == "windows" {
			out, linkErr := exec.Command("cmd", "/c", "mklink", "/J", linkPath, targetPath).CombinedOutput()
			require.NoError(t, linkErr, "mklink /J failed: %s", out)
			return
		}
		require.NoError(t, os.Symlink(targetPath, linkPath))
	}

	t.Run("creates_target_dir_installs_binary_and_escapes_literal_gitignore_patterns", func(t *testing.T) {
		root := t.TempDir()
		require.NoError(t, os.MkdirAll(filepath.Join(root, ".git", "info"), 0o755))

		targetDirs := []struct {
			rel            string
			escapedPattern string
			posixOnlyFS    bool
		}{
			{rel: ".tools/custom", escapedPattern: "/.tools/custom"},
			{rel: "tools[1]", escapedPattern: `/tools\[1\]`},
			{rel: "tools*", escapedPattern: `/tools\*`, posixOnlyFS: true},
			{rel: "#tools", escapedPattern: `/\#tools`},
			{rel: "!tools", escapedPattern: `/\!tools`},
		}

		for _, tc := range targetDirs {
			if !tc.posixOnlyFS || runtime.GOOS != "windows" {
				installedName, installErr := installWorkspaceRuntimeExecutable(root, tc.rel)
				require.NoError(t, installErr, "target dir %q must succeed", tc.rel)
				assert.Equal(t, binName, installedName)

				installedPath := filepath.Join(root, filepath.FromSlash(tc.rel), binName)
				require.FileExists(t, installedPath)
				gotBytes, readErr := os.ReadFile(installedPath)
				require.NoError(t, readErr)
				assert.Equal(t, selfBytes, gotBytes)

				if runtime.GOOS != "windows" {
					info, statErr := os.Stat(installedPath)
					require.NoError(t, statErr)
					assert.NotZero(t, info.Mode().Perm()&0o111)
				}
			}

			require.NoError(t, ensureWorkspaceRuntimeIgnored(root, tc.rel, binName))
			// Idempotency: repeat ignore call must not duplicate lines.
			require.NoError(t, ensureWorkspaceRuntimeIgnored(root, tc.rel, binName))

			gitignoreData, gitignoreErr := os.ReadFile(filepath.Join(root, ".gitignore"))
			require.NoError(t, gitignoreErr)
			excludeData, excludeErr := os.ReadFile(filepath.Join(root, ".git", "info", "exclude"))
			require.NoError(t, excludeErr)

			for _, entry := range []string{
				tc.escapedPattern + "/backlogit",
				tc.escapedPattern + "/backlogit.exe",
			} {
				assert.Equalf(t, 1, strings.Count(string(gitignoreData), entry+"\n"), ".gitignore missing or duplicated %q", entry)
				assert.Equalf(t, 1, strings.Count(string(excludeData), entry+"\n"), ".git/info/exclude missing or duplicated %q", entry)
			}
		}

		// Verify git check-ignore matches literal tools[1]/backlogit[.exe].
		gitInit := exec.Command("git", "init", root)
		initOut, initErr := gitInit.CombinedOutput()
		require.NoError(t, initErr, "git init failed: %s", initOut)
		checkCmd := exec.Command("git", "-C", root, "check-ignore", "-q", "tools[1]/"+binName)
		checkOut, checkErr := checkCmd.CombinedOutput()
		require.NoError(t, checkErr, "git check-ignore failed for tools[1]/%s: %s", binName, checkOut)
	})

	t.Run("samefile_installed_binary_skips_self_overwrite_without_error", func(t *testing.T) {
		root := t.TempDir()
		require.NoError(t, os.MkdirAll(filepath.Join(root, ".git", "info"), 0o755))

		_, installErr := installWorkspaceRuntimeExecutable(root, ".tools/bin")
		require.NoError(t, installErr)

		targetBin := filepath.Join(root, ".tools", "bin", binName)
		origBytes, readErr := os.ReadFile(targetBin)
		require.NoError(t, readErr)

		origExeFn := selfInstallExecutablePathFn
		t.Cleanup(func() { selfInstallExecutablePathFn = origExeFn })
		selfInstallExecutablePathFn = func() (string, error) {
			return targetBin, nil
		}

		installedName, sameErr := installWorkspaceRuntimeExecutable(root, ".tools/bin")
		require.NoError(t, sameErr)
		assert.Equal(t, binName, installedName)

		afterBytes, afterErr := os.ReadFile(targetBin)
		require.NoError(t, afterErr)
		assert.Equal(t, origBytes, afterBytes)
	})

	t.Run("rejects_newlines_control_chars_traversal_and_redirected_target_directories", func(t *testing.T) {
		root := t.TempDir()
		require.NoError(t, os.MkdirAll(filepath.Join(root, ".git", "info"), 0o755))

		invalidDirs := []string{
			"tools\n!secret",
			"tools\r\nother",
			"tools\x07bell",
			"../escape",
			".git",
			".",
			"   ",
		}
		for _, dir := range invalidDirs {
			_, installErr := installWorkspaceRuntimeExecutable(root, dir)
			require.ErrorIs(t, installErr, corerrors.ErrValidation, "install must reject %q", dir)
			ignoreErr := ensureWorkspaceRuntimeIgnored(root, dir, binName)
			require.ErrorIs(t, ignoreErr, corerrors.ErrValidation, "ignore must reject %q", dir)
		}

		// Redirected ancestor: link/existing/new
		base := t.TempDir()
		outside := t.TempDir()
		require.NoError(t, os.MkdirAll(filepath.Join(outside, "existing", "new"), 0o755))
		createDirRedirect(t, filepath.Join(base, "link"), outside)
		ancestorWS := filepath.Join(base, "link", "existing", "new")
		_, ancestorErr := installWorkspaceRuntimeExecutable(ancestorWS, ".bin")
		require.ErrorIs(t, ancestorErr, corerrors.ErrValidation)
		assert.NoFileExists(t, filepath.Join(outside, "existing", "new", ".bin", binName))

		// Redirected target segment inside workspace.
		redirectRoot := t.TempDir()
		redirectOutside := t.TempDir()
		createDirRedirect(t, filepath.Join(redirectRoot, "linked-tools"), redirectOutside)
		_, segErr := installWorkspaceRuntimeExecutable(redirectRoot, "linked-tools/bin")
		require.ErrorIs(t, segErr, corerrors.ErrValidation)
		assert.NoFileExists(t, filepath.Join(redirectOutside, "bin", binName))

		// Mid-flight swap via selfInstallBeforeBinaryReplaceHook.
		swapRoot := t.TempDir()
		swapOutside := t.TempDir()
		toolsDirPath := filepath.Join(swapRoot, ".tools", "bin")
		backupPath := toolsDirPath + ".orig"
		origHook := selfInstallBeforeBinaryReplaceHook
		t.Cleanup(func() { selfInstallBeforeBinaryReplaceHook = origHook })
		selfInstallBeforeBinaryReplaceHook = func() {
			require.NoError(t, os.Rename(toolsDirPath, backupPath))
			createDirRedirect(t, toolsDirPath, swapOutside)
		}
		_, swapErr := installWorkspaceRuntimeExecutable(swapRoot, ".tools/bin")
		require.True(t, errors.Is(swapErr, corerrors.ErrValidation) || swapErr != nil, "mid-flight swap must fail closed")
		assert.NoFileExists(t, filepath.Join(swapOutside, binName))
	})
}
