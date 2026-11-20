package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
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

func TestU3_DefaultTargetToolsDirAndNonShortCircuitingPreflight(t *testing.T) {
	binName := "backlogit"
	if runtime.GOOS == "windows" {
		binName = "backlogit.exe"
	}
	binBytes := []byte("pinned-binary")
	binSum := sha256.Sum256(binBytes)
	binSumHex := hex.EncodeToString(binSum[:])

	writeFile := func(t *testing.T, root, rel, body string) {
		t.Helper()
		full := filepath.Join(root, filepath.FromSlash(rel))
		require.NoError(t, os.MkdirAll(filepath.Dir(full), 0o755))
		require.NoError(t, os.WriteFile(full, []byte(body), 0o644))
	}

	openRoot := func(t *testing.T, root string) *os.Root {
		t.Helper()
		r, err := os.OpenRoot(root)
		require.NoError(t, err)
		t.Cleanup(func() { _ = r.Close() })
		return r
	}

	createDirLink := func(t *testing.T, linkPath, targetPath string) {
		t.Helper()
		if runtime.GOOS == "windows" {
			out, err := exec.Command("cmd", "/c", "mklink", "/J", linkPath, targetPath).CombinedOutput()
			require.NoError(t, err, "mklink /J failed: %s", out)
			return
		}
		require.NoError(t, os.Symlink(targetPath, linkPath))
	}

	validToolchainYAML := func(wsRoot string, extraToolYAML string) string {
		return fmt.Sprintf(`schema_version: "1.0"
containment_level: level-1-workspace-local-pinned
os_container_sandbox_deferred: true
workspace_root: %q
tools_dir: ".bin"
tools:
  - id: backlogit
    kind: mcp
    containment_class: workspace-local-pinned
    executable_path: "./.bin/%s"
    pinned_version: "1.0.0"
    sha256: %q
    workspace_binding_args:
      - "--cwd"
      - %q
%shost_prerequisites:
  - id: shell
    command: shell
    purpose: bootstrap
    required: false
    reason: host shell
`, wsRoot, binName, binSumHex, wsRoot, extraToolYAML)
	}

	t.Run("rejects_line_separators_and_resolves_split_servers_and_empty_workspace_dir_override", func(t *testing.T) {
		for _, bad := range []string{"tools\n!secret", "tools\r\nother", "tools\x07bell"} {
			_, err := validateDaemonToolsDirRel(bad)
			require.ErrorIs(t, err, corerrors.ErrValidation, "validateDaemonToolsDirRel must reject %q", bad)
		}

		// BACKLOGIT_WORKSPACE_DIR="" must be treated as unset.
		singleRoot := t.TempDir()
		writeFile(t, singleRoot, ".backlogit/config.yaml", "tools_dir: \".custom\"\n")
		wsHandle := openRoot(t, singleRoot)

		t.Setenv("BACKLOGIT_WORKSPACE_DIR", "")
		storageDir, hasStorage, err := resolveDaemonStorageDirName(wsHandle, singleRoot)
		require.NoError(t, err)
		require.True(t, hasStorage)
		assert.Equal(t, ".backlogit", storageDir)

		rel, found, err := resolveConfiguredDaemonBinaryRel(wsHandle, singleRoot)
		require.NoError(t, err)
		require.True(t, found)
		assert.Equal(t, filepath.Join(".custom", binName), rel)

		for _, badEnv := range []string{"   ", "invalid"} {
			t.Setenv("BACKLOGIT_WORKSPACE_DIR", badEnv)
			_, _, badErr := resolveDaemonStorageDirName(wsHandle, singleRoot)
			require.ErrorIs(t, badErr, corerrors.ErrValidation)
		}

		// Dual-root workspace honors explicit BACKLOGIT_WORKSPACE_DIR=".backlog".
		dualRoot := t.TempDir()
		writeFile(t, dualRoot, ".backlog/config.yaml", "tools_dir: \".dual-tools\"\n")
		writeFile(t, dualRoot, ".backlogit/config.yaml", "tools_dir: \".legacy-tools\"\n")
		dualHandle := openRoot(t, dualRoot)
		t.Setenv("BACKLOGIT_WORKSPACE_DIR", ".backlog")
		dualStorage, hasDual, dualErr := resolveDaemonStorageDirName(dualHandle, dualRoot)
		require.NoError(t, dualErr)
		require.True(t, hasDual)
		assert.Equal(t, ".backlog", dualStorage)

		// Split mcpServers + servers: single backlogit in servers is found; duplicate across both is rejected.
		splitRoot := t.TempDir()
		writeFile(t, splitRoot, ".mcp.json", fmt.Sprintf(`{"mcpServers":{"other":{"type":"stdio","command":"./.bin/other"}},"servers":{"backlogit":{"type":"stdio","command":"./.tools/%s","args":["mcp","--cwd",%q]}}}`, binName, splitRoot))
		splitHandle := openRoot(t, splitRoot)
		splitRel, splitFound, splitErr := resolveConfiguredDaemonBinaryRel(splitHandle, splitRoot)
		require.NoError(t, splitErr)
		require.True(t, splitFound)
		assert.Equal(t, filepath.Join(".tools", binName), splitRel)

		dupRoot := t.TempDir()
		writeFile(t, dupRoot, ".mcp.json", fmt.Sprintf(`{"mcpServers":{"backlogit":{"type":"stdio","command":"./.bin/%s"}},"servers":{"backlog-alt":{"type":"stdio","command":"./bin/%s"}}}`, binName, binName))
		dupHandle := openRoot(t, dupRoot)
		_, _, dupErr := resolveConfiguredDaemonBinaryRel(dupHandle, dupRoot)
		require.ErrorIs(t, dupErr, corerrors.ErrValidation)
	})

	t.Run("config_yaml_filename_attribution_and_strict_typed_toolchain_preflight", func(t *testing.T) {
		t.Setenv("BACKLOGIT_WORKSPACE_DIR", "")
		pkgDir, wdErr := os.Getwd()
		require.NoError(t, wdErr)

		// Duplicate key in config.yaml must report config.yaml, not toolchain.yaml.
		dupCfgRoot := t.TempDir()
		writeFile(t, dupCfgRoot, ".backlog/config.yaml", "tools_dir: \".bin\"\nTOOLS_DIR: \"bin\"\n")
		dupCfgHandle := openRoot(t, dupCfgRoot)
		_, _, dupErr := resolveConfiguredDaemonBinaryRel(dupCfgHandle, dupCfgRoot)
		require.ErrorIs(t, dupErr, corerrors.ErrValidation)
		assert.Contains(t, dupErr.Error(), "config.yaml")
		assert.NotContains(t, dupErr.Error(), "toolchain.yaml")

		// Non-string scalar tools_dir in config.yaml fails closed.
		nonStrRoot := t.TempDir()
		writeFile(t, nonStrRoot, ".backlog/config.yaml", "tools_dir: 123\n")
		nonStrHandle := openRoot(t, nonStrRoot)
		_, _, nonStrErr := resolveConfiguredDaemonBinaryRel(nonStrHandle, nonStrRoot)
		require.ErrorIs(t, nonStrErr, corerrors.ErrValidation)

		// Relative workspace_root: "." resolves against absWorkspace even from a different cwd.
		relWSRoot := t.TempDir()
		otherCWD := t.TempDir()
		t.Chdir(otherCWD)
		writeFile(t, relWSRoot, ".bin/"+binName, string(binBytes))
		writeFile(t, relWSRoot, ".backlog/toolchain.yaml", validToolchainYAML(".", ""))
		relWSHandle := openRoot(t, relWSRoot)
		relBin, relFound, relErr := resolveConfiguredDaemonBinaryRel(relWSHandle, relWSRoot)
		require.NoError(t, relErr)
		require.True(t, relFound)
		assert.Equal(t, filepath.Join(".bin", binName), relBin)

		// Unknown field in toolchain.yaml fails closed.
		unkRoot := t.TempDir()
		writeFile(t, unkRoot, ".bin/"+binName, string(binBytes))
		writeFile(t, unkRoot, ".backlog/toolchain.yaml", validToolchainYAML(".", "")+"unknown_extra: true\n")
		unkHandle := openRoot(t, unkRoot)
		_, _, unkErr := resolveConfiguredDaemonBinaryRel(unkHandle, unkRoot)
		require.ErrorIs(t, unkErr, corerrors.ErrValidation)

		// Unrelated tool SHA-256 mismatch fails closed in preflight.
		engramMismatch := fmt.Sprintf(`  - id: engram
    kind: mcp
    containment_class: workspace-local-pinned
    executable_path: "./.bin/engram.exe"
    pinned_version: "1.0.0"
    sha256: %q
    workspace_binding_args: []
`, strings.Repeat("0", 64))
		mismatchRoot := t.TempDir()
		writeFile(t, mismatchRoot, ".bin/"+binName, string(binBytes))
		writeFile(t, mismatchRoot, ".bin/engram.exe", "actual-engram-bytes")
		writeFile(t, mismatchRoot, ".backlog/toolchain.yaml", validToolchainYAML(mismatchRoot, engramMismatch))
		mismatchHandle := openRoot(t, mismatchRoot)
		_, _, mismatchErr := resolveConfiguredDaemonBinaryRel(mismatchHandle, mismatchRoot)
		require.ErrorIs(t, mismatchErr, corerrors.ErrValidation)

		// Verify defaultTargetToolsDir is declared in mcp_daemon.go.
		daemonAST, parseErr := parser.ParseFile(token.NewFileSet(), filepath.Join(pkgDir, "mcp_daemon.go"), nil, 0)
		require.NoError(t, parseErr)
		hasDefaultHelper := false
		for _, decl := range daemonAST.Decls {
			if fn, ok := decl.(*ast.FuncDecl); ok && fn.Recv == nil && fn.Name.Name == "defaultTargetToolsDir" {
				hasDefaultHelper = true
			}
		}
		assert.True(t, hasDefaultHelper, "mcp_daemon.go must define defaultTargetToolsDir")
	})

	t.Run("non_short_circuiting_lower_priority_config_and_directory_validation", func(t *testing.T) {
		t.Setenv("BACKLOGIT_WORKSPACE_DIR", "")

		// Valid .mcp.json must NOT short-circuit invalid toolchain.yaml.
		badToolchainRoot := t.TempDir()
		writeFile(t, badToolchainRoot, ".mcp.json", fmt.Sprintf(`{"mcpServers":{"backlogit":{"type":"stdio","command":"./.bin/%s","args":["mcp","--cwd",%q]}}}`, binName, badToolchainRoot))
		writeFile(t, badToolchainRoot, ".bin/"+binName, string(binBytes))
		writeFile(t, badToolchainRoot, ".backlog/toolchain.yaml", validToolchainYAML(badToolchainRoot, "")+"unknown_field: 1\n")
		_, _, err1 := resolveConfiguredDaemonBinaryRel(openRoot(t, badToolchainRoot), badToolchainRoot)
		require.ErrorIs(t, err1, corerrors.ErrValidation)

		// Valid .mcp.json must NOT short-circuit invalid config.yaml.
		badConfigRoot := t.TempDir()
		writeFile(t, badConfigRoot, ".mcp.json", fmt.Sprintf(`{"mcpServers":{"backlogit":{"type":"stdio","command":"./.bin/%s","args":["mcp","--cwd",%q]}}}`, binName, badConfigRoot))
		writeFile(t, badConfigRoot, ".backlog/config.yaml", "tools_dir: \".bin\"\nTOOLS_DIR: \"bin\"\n")
		_, _, err2 := resolveConfiguredDaemonBinaryRel(openRoot(t, badConfigRoot), badConfigRoot)
		require.ErrorIs(t, err2, corerrors.ErrValidation)

		// Valid .mcp.json must NOT short-circuit redirected <storage>/templates.
		badTemplatesRoot := t.TempDir()
		outside := t.TempDir()
		writeFile(t, badTemplatesRoot, ".mcp.json", fmt.Sprintf(`{"mcpServers":{"backlogit":{"type":"stdio","command":"./.bin/%s","args":["mcp","--cwd",%q]}}}`, binName, badTemplatesRoot))
		require.NoError(t, os.MkdirAll(filepath.Join(badTemplatesRoot, ".backlog"), 0o755))
		createDirLink(t, filepath.Join(badTemplatesRoot, ".backlog", "templates"), outside)
		_, _, err3 := resolveConfiguredDaemonBinaryRel(openRoot(t, badTemplatesRoot), badTemplatesRoot)
		require.ErrorIs(t, err3, corerrors.ErrValidation)

		// Valid .mcp.json must NOT short-circuit regular file or redirected .bin / bin.
		badBinRoot := t.TempDir()
		writeFile(t, badBinRoot, ".mcp.json", fmt.Sprintf(`{"mcpServers":{"backlogit":{"type":"stdio","command":"./.bin/%s","args":["mcp","--cwd",%q]}}}`, binName, badBinRoot))
		writeFile(t, badBinRoot, "bin", "not-a-directory")
		_, _, err4 := resolveConfiguredDaemonBinaryRel(openRoot(t, badBinRoot), badBinRoot)
		require.ErrorIs(t, err4, corerrors.ErrValidation)
	})
}
