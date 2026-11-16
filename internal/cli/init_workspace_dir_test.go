package cli_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/cli"
	"github.com/softwaresalt/backlogit/internal/config"
)

func TestInitCommand_CreatesBacklogDirectory(t *testing.T) {
	root := t.TempDir()

	buf := new(bytes.Buffer)
	cmd := cli.NewRootCommand()
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"init", "--cwd", root})

	require.NoError(t, cmd.Execute())
	assert.DirExists(t, filepath.Join(root, ".backlog"))
	assert.FileExists(t, filepath.Join(root, ".backlog", "config.yaml"))
	assert.NoDirExists(t, filepath.Join(root, ".backlogit"))
}

func TestInitCommand_RefusesLegacyWorkspaceRoot(t *testing.T) {
	root := t.TempDir()
	legacyDir := filepath.Join(root, ".backlogit")
	require.NoError(t, os.MkdirAll(legacyDir, 0o755))
	require.NoError(t, config.WriteDefaults(legacyDir))

	buf := new(bytes.Buffer)
	cmd := cli.NewRootCommand()
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"init", "--cwd", root})

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "workspace already exists at .backlogit")
}

func TestU6_InitAndInstallSelfInstallRuntimeExecutableAndIgnoreEntries(t *testing.T) {
	exePath, err := os.Executable()
	require.NoError(t, err)
	exeBytes, err := os.ReadFile(exePath)
	require.NoError(t, err)
	binName := "backlogit"
	if runtime.GOOS == "windows" {
		binName = "backlogit.exe"
	}

	assertIgnoreEntriesOnce := func(t *testing.T, content string) {
		t.Helper()
		for _, entry := range []string{
			"/.backlog/backlogit",
			"/.backlog/backlogit.exe",
			"/.backlog/.locks/",
			"/.backlogit/backlogit",
			"/.backlogit/backlogit.exe",
			"/.backlogit/.locks/",
		} {
			assert.Equalf(t, 1, strings.Count(content, entry+"\n"),
				"ignore file must contain %q exactly once; got:\n%s", entry, content)
		}
	}

	t.Run("init-installs-runtime-binary-and-ignore-entries", func(t *testing.T) {
		root := t.TempDir()
		require.NoError(t, os.MkdirAll(filepath.Join(root, ".git", "info"), 0o755))

		buf := new(bytes.Buffer)
		cmd := cli.NewRootCommand()
		cmd.SetOut(buf)
		cmd.SetErr(buf)
		cmd.SetArgs([]string{"init", "--cwd", root})
		require.NoError(t, cmd.Execute())

		installedBin := filepath.Join(root, ".backlog", binName)
		require.FileExists(t, installedBin)
		gotBytes, readErr := os.ReadFile(installedBin)
		require.NoError(t, readErr)
		assert.Equal(t, exeBytes, gotBytes)
		if runtime.GOOS != "windows" {
			info, statErr := os.Stat(installedBin)
			require.NoError(t, statErr)
			assert.NotZero(t, info.Mode().Perm()&0o111, "installed binary must be executable on POSIX")
		}

		// Preserve U12 negative-pinning invariant: bin/backlogit.exe must remain absent.
		assert.NoFileExists(t, filepath.Join(root, "bin", "backlogit.exe"))

		gitignoreBytes, err := os.ReadFile(filepath.Join(root, ".gitignore"))
		require.NoError(t, err)
		assertIgnoreEntriesOnce(t, string(gitignoreBytes))

		excludeBytes, err := os.ReadFile(filepath.Join(root, ".git", "info", "exclude"))
		require.NoError(t, err)
		assertIgnoreEntriesOnce(t, string(excludeBytes))
	})

	t.Run("install-fresh-and-existing-backlogit-and-idempotent", func(t *testing.T) {
		root := t.TempDir()
		require.NoError(t, os.MkdirAll(filepath.Join(root, ".git", "info"), 0o755))

		// Fresh install defaults to .backlogit when BACKLOGIT_WORKSPACE_DIR is unset.
		t.Setenv("BACKLOGIT_WORKSPACE_DIR", "")
		cmd := cli.NewRootCommand()
		cmd.SetOut(new(bytes.Buffer))
		cmd.SetErr(new(bytes.Buffer))
		cmd.SetArgs([]string{"install", "--cwd", root})
		require.NoError(t, cmd.Execute())
		require.FileExists(t, filepath.Join(root, ".backlogit", binName))
		assert.NoFileExists(t, filepath.Join(root, "bin", "backlogit.exe"))

		// Repeat install on existing .backlogit must succeed without duplicating ignore entries.
		cmd2 := cli.NewRootCommand()
		cmd2.SetOut(new(bytes.Buffer))
		cmd2.SetErr(new(bytes.Buffer))
		cmd2.SetArgs([]string{"install", "--cwd", root})
		require.NoError(t, cmd2.Execute())

		gitignoreBytes, err := os.ReadFile(filepath.Join(root, ".gitignore"))
		require.NoError(t, err)
		assertIgnoreEntriesOnce(t, string(gitignoreBytes))

		excludeBytes, err := os.ReadFile(filepath.Join(root, ".git", "info", "exclude"))
		require.NoError(t, err)
		assertIgnoreEntriesOnce(t, string(excludeBytes))
	})

	t.Run("rejects-redirected-storage-and-git-info-directories", func(t *testing.T) {
		root := t.TempDir()
		outside := t.TempDir()
		require.NoError(t, os.MkdirAll(filepath.Join(root, ".git"), 0o755))
		gitInfo := filepath.Join(root, ".git", "info")
		if runtime.GOOS == "windows" {
			out, linkErr := exec.Command("cmd", "/c", "mklink", "/J", gitInfo, outside).CombinedOutput()
			require.NoError(t, linkErr, "mklink /J failed: %s", out)
		} else {
			require.NoError(t, os.Symlink(outside, gitInfo))
		}

		cmd := cli.NewRootCommand()
		cmd.SetOut(new(bytes.Buffer))
		cmd.SetErr(new(bytes.Buffer))
		cmd.SetArgs([]string{"install", "--cwd", root})
		err := cmd.Execute()
		require.Error(t, err, "redirected .git/info must be rejected")
		assert.NoFileExists(t, filepath.Join(outside, "exclude"))
	})

	t.Run("install-honors-storage-override-even-when-both-roots-exist", func(t *testing.T) {
		root := t.TempDir()
		require.NoError(t, os.MkdirAll(filepath.Join(root, ".git", "info"), 0o755))
		require.NoError(t, os.MkdirAll(filepath.Join(root, ".backlog"), 0o755))
		require.NoError(t, os.MkdirAll(filepath.Join(root, ".backlogit"), 0o755))

		t.Setenv("BACKLOGIT_WORKSPACE_DIR", "")
		cmdConflict := cli.NewRootCommand()
		cmdConflict.SetOut(new(bytes.Buffer))
		cmdConflict.SetErr(new(bytes.Buffer))
		cmdConflict.SetArgs([]string{"install", "--cwd", root})
		require.Error(t, cmdConflict.Execute(), "dual roots without override must fail")

		t.Setenv("BACKLOGIT_WORKSPACE_DIR", ".backlog")
		cmdOverride := cli.NewRootCommand()
		cmdOverride.SetOut(new(bytes.Buffer))
		cmdOverride.SetErr(new(bytes.Buffer))
		cmdOverride.SetArgs([]string{"install", "--cwd", root})
		require.NoError(t, cmdOverride.Execute())
		require.FileExists(t, filepath.Join(root, ".backlog", binName))
		assert.NoFileExists(t, filepath.Join(root, ".backlogit", binName))

		t.Setenv("BACKLOGIT_WORKSPACE_DIR", "../escape")
		cmdInvalid := cli.NewRootCommand()
		cmdInvalid.SetOut(new(bytes.Buffer))
		cmdInvalid.SetErr(new(bytes.Buffer))
		cmdInvalid.SetArgs([]string{"install", "--cwd", root})
		require.Error(t, cmdInvalid.Execute(), "invalid override must fail closed")
	})
}
