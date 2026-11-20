package toolchain

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/atomicfile"
	corerrors "github.com/softwaresalt/backlogit/internal/errors"
)

func TestU1_WriteToolchainManifestUsesAtomicFileReplacementAndRetrySemantics(t *testing.T) {
	t.Run("replacement-failure-wraps-ErrWriteNotApplied", func(t *testing.T) {
		root, _ := u9Workspace(t)
		manifest := u9PinnedWorkspace(t, root)
		require.NoError(t, os.MkdirAll(filepath.Join(root, ".backlogit"), 0o755))
		require.NoError(t, WriteToolchainManifest(root, manifest))

		dstPath := filepath.Join(root, ".backlogit", "toolchain.yaml")
		origBytes, err := os.ReadFile(dstPath)
		require.NoError(t, err)

		if runtime.GOOS == "windows" {
			require.NoError(t, os.Chmod(dstPath, 0o400))
			t.Cleanup(func() { _ = os.Chmod(dstPath, 0o600) })
		} else {
			origHook := replaceToolchainManifestFile
			t.Cleanup(func() { replaceToolchainManifestFile = origHook })
			replaceToolchainManifestFile = func(_ *os.Root, _, _, _ string) error {
				return fmt.Errorf("injected posix replace failure: %w", corerrors.ErrWriteNotApplied)
			}
		}

		updated := u9Manifest(root)
		updated.Tools[0].PinnedVersion = "2.0.0"
		err = WriteToolchainManifest(root, updated)
		require.Error(t, err)
		assert.ErrorIs(t, err, corerrors.ErrWriteNotApplied,
			"WriteToolchainManifest replacement failure must wrap ErrWriteNotApplied")

		afterBytes, readErr := os.ReadFile(dstPath)
		require.NoError(t, readErr)
		assert.Equal(t, origBytes, afterBytes, "failed replacement must preserve existing toolchain.yaml bytes")
	})

	t.Run("ast-delegates-to-ReplaceTempFileInRoot", func(t *testing.T) {
		launcherFile, err := parser.ParseFile(token.NewFileSet(), "launcher.go", nil, 0)
		require.NoError(t, err)

		var writeFn *ast.FuncDecl
		var hasReplaceSeam bool
		for _, decl := range launcherFile.Decls {
			switch d := decl.(type) {
			case *ast.FuncDecl:
				if d.Recv == nil && d.Name.Name == "WriteToolchainManifest" {
					writeFn = d
				}
			case *ast.GenDecl:
				if d.Tok != token.VAR {
					continue
				}
				for _, spec := range d.Specs {
					vs, ok := spec.(*ast.ValueSpec)
					if !ok {
						continue
					}
					for _, name := range vs.Names {
						if name.Name == "replaceToolchainManifestFile" {
							hasReplaceSeam = true
						}
					}
				}
			}
		}
		require.NotNil(t, writeFn, "WriteToolchainManifest must exist in launcher.go")
		assert.True(t, hasReplaceSeam, "launcher.go must declare replaceToolchainManifestFile seam")

		var foundOSRename bool
		var callsReplaceSeam bool
		ast.Inspect(writeFn.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			switch fun := call.Fun.(type) {
			case *ast.SelectorExpr:
				if pkg, ok := fun.X.(*ast.Ident); ok && pkg.Name == "os" && fun.Sel.Name == "Rename" {
					foundOSRename = true
				}
			case *ast.Ident:
				if fun.Name == "replaceToolchainManifestFile" {
					callsReplaceSeam = true
				}
			}
			return true
		})
		assert.False(t, foundOSRename, "WriteToolchainManifest must not call raw os.Rename")
		assert.True(t, callsReplaceSeam, "WriteToolchainManifest must delegate replacement to replaceToolchainManifestFile")

		atomicFileAST, err := parser.ParseFile(token.NewFileSet(), filepath.Join("..", "atomicfile", "atomicfile.go"), nil, 0)
		require.NoError(t, err)
		var foundExportedReplace bool
		for _, decl := range atomicFileAST.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if ok && fn.Recv == nil && fn.Name.Name == "ReplaceTempFileInRoot" {
				foundExportedReplace = true
			}
		}
		assert.True(t, foundExportedReplace, "internal/atomicfile/atomicfile.go must export ReplaceTempFileInRoot")
	})

	t.Run("non-cwd-workspace-succeeds", func(t *testing.T) {
		root := t.TempDir()
		manifest := u9PinnedWorkspace(t, root)
		require.NoError(t, os.MkdirAll(filepath.Join(root, ".backlogit"), 0o755))

		cwd, err := os.Getwd()
		require.NoError(t, err)
		require.NotEqual(t, strings.ToLower(cwd), strings.ToLower(root),
			"fixture root must differ from ambient process CWD")

		require.NoError(t, WriteToolchainManifest(root, manifest))
		data, err := os.ReadFile(filepath.Join(root, ".backlogit", "toolchain.yaml"))
		require.NoError(t, err)
		assert.Contains(t, string(data), "level-1-workspace-local-pinned")
	})

	t.Run("parent-storage-directory-swap-rejected", func(t *testing.T) {
		root, _ := u9Workspace(t)
		manifest := u9PinnedWorkspace(t, root)
		storageDir := filepath.Join(root, ".backlogit")
		require.NoError(t, os.MkdirAll(storageDir, 0o755))
		require.NoError(t, WriteToolchainManifest(root, manifest))

		outsideDir := t.TempDir()
		swappedBackup := filepath.Join(root, ".backlogit.orig")

		origHook := replaceToolchainManifestFile
		t.Cleanup(func() { replaceToolchainManifestFile = origHook })
		replaceToolchainManifestFile = func(storageHandle *os.Root, storageDirAbs, tmpRel, dstRel string) error {
			require.NoError(t, os.Rename(storageDir, swappedBackup))
			if runtime.GOOS == "windows" {
				out, linkErr := exec.Command("cmd", "/c", "mklink", "/J", storageDir, outsideDir).CombinedOutput()
				require.NoError(t, linkErr, "mklink /J failed: %s", out)
			} else {
				require.NoError(t, os.Symlink(outsideDir, storageDir))
			}
			return atomicfile.ReplaceTempFileInRoot(storageHandle, storageDirAbs, tmpRel, dstRel, true)
		}

		updated := u9Manifest(root)
		updated.Tools[0].PinnedVersion = "2.0.0"
		err := WriteToolchainManifest(root, updated)
		require.Error(t, err, "parent directory swap must fail closed")
		assert.ErrorIs(t, err, corerrors.ErrWriteNotApplied)
		_, outsideErr := os.Stat(filepath.Join(outsideDir, "toolchain.yaml"))
		assert.True(t, os.IsNotExist(outsideErr), "parent directory swap must never write toolchain.yaml into outsideDir")
	})
}

// TestU2_LauncherAndProvisioningSupportDotBinAndConfigurableToolsDir gates 217.002-T (U2).
func TestU2_LauncherAndProvisioningSupportDotBinAndConfigurableToolsDir(t *testing.T) {
	const binaryBytes = "pinned binary"
	sum := sha256.Sum256([]byte(binaryBytes))
	sumHex := hex.EncodeToString(sum[:])

	newManifest := func(root, manifestToolsDir, execDir string) *ToolchainManifest {
		return &ToolchainManifest{
			SchemaVersion:              "1.0",
			ContainmentLevel:           Level1WorkspaceLocalPinned,
			OSContainerSandboxDeferred: true,
			WorkspaceRoot:              root,
			ToolsDir:                   manifestToolsDir,
			Tools: []ToolEntry{
				{
					ID:                   "backlogit",
					Kind:                 "mcp",
					ContainmentClass:     WorkspaceLocalPinned,
					ExecutablePath:       "./" + execDir + "/backlogit.exe",
					PinnedVersion:        "1.0.0",
					SHA256:               sumHex,
					WorkspaceBindingArgs: []string{"--cwd", root},
				},
				{
					ID:               "remote",
					Kind:             "mcp",
					ContainmentClass: RemoteDeclared,
					RemotePolicy:     "approved",
				},
				{
					ID:               "host-built-in",
					Kind:             "agent",
					ContainmentClass: BuiltInHost,
					Reason:           "provided by host",
				},
			},
			HostPrerequisites: []HostPrerequisiteEntry{
				{ID: "shell", Command: "shell", Purpose: "bootstrap", Reason: "host shell", Required: false},
			},
		}
	}

	writeLaunchers := func(t *testing.T, root, execDir string) {
		t.Helper()
		u9Write(t, root, "start.ps1", fmt.Sprintf(`$workspaceRoot = $PSScriptRoot
$workspaceBinDirectory = Join-Path $workspaceRoot %q
$backlogitPath = Join-Path $workspaceBinDirectory "backlogit.exe"
if (-not (Test-Path -LiteralPath $backlogitPath -PathType Leaf)) { throw "Missing ./%s/backlogit.exe" }
$env:COPILOT_HOME = Join-Path $workspaceRoot ".copilot"
$env:ENGRAM_DATA_DIR = Join-Path $workspaceRoot ".engram"
& $backlogitPath --cwd $workspaceRoot mcp status
`, execDir, execDir))
		u9Write(t, root, "start.sh", fmt.Sprintf(`#!/bin/sh
workspace_root=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
workspace_bin_directory="$workspace_root/%s"
backlogit_path="$workspace_bin_directory/backlogit.exe"
if [ ! -f "$backlogit_path" ]; then echo "Missing ./%s/backlogit.exe" >&2; exit 1; fi
export COPILOT_HOME="$workspace_root/.copilot"
export ENGRAM_DATA_DIR="$workspace_root/.engram"
"$backlogit_path" --cwd "$workspace_root" mcp status
`, execDir, execDir))
	}

	t.Run("schema-declares-tools-dir", func(t *testing.T) {
		data, err := os.ReadFile("../../schemas/workspace-toolchain.schema.json")
		require.NoError(t, err)

		var schema struct {
			Properties map[string]struct {
				Type      string `json:"type"`
				MinLength int    `json:"minLength"`
			} `json:"properties"`
		}
		require.NoError(t, json.Unmarshal(data, &schema))

		toolsDirProp, ok := schema.Properties["tools_dir"]
		require.True(t, ok, "schema properties must declare tools_dir")
		assert.Equal(t, "string", toolsDirProp.Type)
		assert.Equal(t, 1, toolsDirProp.MinLength)
	})

	t.Run("dot-bin-and-custom-tools-dir-audit-pass", func(t *testing.T) {
		for _, tc := range []struct {
			name             string
			execDir          string
			manifestToolsDir string
		}{
			{name: "dot-bin-explicit", execDir: ".bin", manifestToolsDir: ".bin"},
			{name: "dot-bin-omitted", execDir: ".bin", manifestToolsDir: ""},
			{name: "custom-bin", execDir: "custom-bin", manifestToolsDir: "custom-bin"},
		} {
			t.Run(tc.name, func(t *testing.T) {
				root, _ := u9Workspace(t)
				u9Write(t, root, tc.execDir+"/backlogit.exe", binaryBytes)
				manifest := newManifest(root, tc.manifestToolsDir, tc.execDir)
				writeLaunchers(t, root, tc.execDir)

				require.NoError(t, WriteToolchainManifest(root, manifest))
				rendered, err := RenderPinnedMCPConfig(root, manifest)
				require.NoError(t, err)
				assert.Contains(t, string(rendered), "./"+tc.execDir+"/backlogit.exe")
				u9Write(t, root, ".mcp.json", string(rendered))

				findings, err := VerifyLauncherScripts(root)
				require.NoError(t, err)
				require.NotEmpty(t, findings)
				for _, f := range findings {
					assert.NotEqual(t, "violation", f.Status, "%+v", f)
					assert.NotEqual(t, NonConformant, f.ContainmentClass, "%+v", f)
					assert.NotEqual(t, Unsupported, f.ContainmentClass, "%+v", f)
				}

				report, err := AuditRepositoryToolchain(context.Background(), root)
				require.NoError(t, err)
				require.NotNil(t, report)
				assert.True(t, report.Valid, "%+v", report.Tools)
				assert.Zero(t, report.Violations)
			})
		}
	})

	t.Run("remediation-strings-match-manifest-tools-dir", func(t *testing.T) {
		for _, tc := range []struct {
			name             string
			manifestToolsDir string
			execDir          string
			wantPrefix       string
			forbidPrefix     string
			omitManifest     bool
		}{
			{
				name:             "dot-bin-manifest",
				manifestToolsDir: ".bin",
				execDir:          ".bin",
				wantPrefix:       "./.bin/",
				forbidPrefix:     "./bin/",
			},
			{
				name:         "default-empty-manifest",
				execDir:      ".bin",
				wantPrefix:   "./.bin/",
				forbidPrefix: "./bin/",
				omitManifest: true,
			},
			{
				name:             "custom-tools-dir-manifest",
				manifestToolsDir: "custom-bin",
				execDir:          "custom-bin",
				wantPrefix:       "./custom-bin/",
				forbidPrefix:     "./bin/",
			},
			{
				name:             "legacy-bin-manifest",
				manifestToolsDir: "",
				execDir:          "bin",
				wantPrefix:       "./bin/",
				forbidPrefix:     "./.bin/",
			},
		} {
			t.Run(tc.name, func(t *testing.T) {
				t.Run("missing-start-sh-and-path-lookup", func(t *testing.T) {
					root, _ := u9Workspace(t)
					if !tc.omitManifest {
						u9Write(t, root, tc.execDir+"/backlogit.exe", binaryBytes)
						manifest := newManifest(root, tc.manifestToolsDir, tc.execDir)
						require.NoError(t, WriteToolchainManifest(root, manifest))
						rendered, err := RenderPinnedMCPConfig(root, manifest)
						require.NoError(t, err)
						u9Write(t, root, ".mcp.json", string(rendered))
					}
					u9Write(t, root, "start.ps1", fmt.Sprintf(`$workspaceRoot = $PSScriptRoot
$workspaceBinDirectory = Join-Path $workspaceRoot %q
$backlogitPath = Join-Path $workspaceBinDirectory "backlogit.exe"
if (-not (Test-Path -LiteralPath $backlogitPath -PathType Leaf)) { throw "Missing ./%s/backlogit.exe" }
$env:COPILOT_HOME = Join-Path $workspaceRoot ".copilot"
$env:ENGRAM_DATA_DIR = Join-Path $workspaceRoot ".engram"
$companion = (Get-Command "engram")
& $backlogitPath --cwd $workspaceRoot mcp status
`, tc.execDir, tc.execDir))

					report, err := AuditRepositoryToolchain(context.Background(), root)
					require.NoError(t, err)
					require.NotNil(t, report)
					assert.False(t, report.Valid)
					assert.Positive(t, report.Violations)

					foundStartSH := false
					foundEngram := false
					for _, f := range report.Tools {
						if f.Status != "violation" {
							continue
						}
						norm := strings.ReplaceAll(f.Remediation, `\`, "/")
						assert.NotContains(t, norm, tc.forbidPrefix, "finding %+v must not use %s", f, tc.forbidPrefix)
						if f.ToolID == "start.sh" {
							foundStartSH = true
							assert.Contains(t, norm, tc.wantPrefix+"backlogit.exe", "start.sh finding %+v", f)
						}
						if f.ToolID == "engram" {
							foundEngram = true
							assert.Contains(t, norm, tc.wantPrefix+"engram.exe", "engram finding %+v", f)
						}
					}
					assert.True(t, foundStartSH, "expected start.sh violation in %+v", report.Tools)
					assert.True(t, foundEngram, "expected engram lookup violation in %+v", report.Tools)
				})

				if !tc.omitManifest {
					t.Run("missing-engram-binary", func(t *testing.T) {
						root, _ := u9Workspace(t)
						u9Write(t, root, tc.execDir+"/backlogit.exe", binaryBytes)
						manifest := newManifest(root, tc.manifestToolsDir, tc.execDir)
						manifest.Tools = append(manifest.Tools, ToolEntry{
							ID:               "engram",
							Kind:             "mcp",
							ContainmentClass: WorkspaceLocalPinned,
							ExecutablePath:   "./" + tc.execDir + "/engram.exe",
							PinnedVersion:    "1.0.0",
							SHA256:           sumHex,
						})
						require.NoError(t, WriteToolchainManifest(root, manifest))
						rendered, err := RenderPinnedMCPConfig(root, manifest)
						require.NoError(t, err)
						u9Write(t, root, ".mcp.json", string(rendered))
						writeLaunchers(t, root, tc.execDir)

						report, err := AuditRepositoryToolchain(context.Background(), root)
						require.NoError(t, err)
						require.NotNil(t, report)
						assert.False(t, report.Valid)
						assert.Positive(t, report.Violations)

						foundEngram := false
						for _, f := range report.Tools {
							if f.Status != "violation" {
								continue
							}
							norm := strings.ReplaceAll(f.Remediation, `\`, "/")
							assert.NotContains(t, norm, tc.forbidPrefix, "finding %+v must not use %s", f, tc.forbidPrefix)
							if f.ToolID == "engram" {
								foundEngram = true
								assert.Contains(t, norm, tc.wantPrefix+"engram.exe", "engram finding %+v", f)
							}
						}
						assert.True(t, foundEngram, "expected engram violation in %+v", report.Tools)
					})
				}
			})
		}
	})

	t.Run("provisioning-rejects-invalid-tools-dir-and-mismatch", func(t *testing.T) {
		for _, tc := range []struct {
			name     string
			toolsDir string
			execDir  string
		}{
			{name: "traversal", toolsDir: "../escape", execDir: ".bin"},
			{name: "reserved-dot-backlogit", toolsDir: ".backlogit", execDir: ".bin"},
			{name: "whitespace", toolsDir: "   ", execDir: ".bin"},
			{name: "mismatched-exec-dir", toolsDir: "custom-bin", execDir: "bin"},
		} {
			t.Run(tc.name, func(t *testing.T) {
				root, _ := u9Workspace(t)
				u9Write(t, root, tc.execDir+"/backlogit.exe", binaryBytes)
				manifest := newManifest(root, tc.toolsDir, tc.execDir)

				err := WriteToolchainManifest(root, manifest)
				require.Error(t, err)
				assert.ErrorIs(t, err, corerrors.ErrValidation)

				_, err = RenderPinnedMCPConfig(root, manifest)
				require.Error(t, err)
				assert.ErrorIs(t, err, corerrors.ErrValidation)
			})
		}
	})

	t.Run("effective-tools-dir-normalizes-relative-workspace-root-and-casefold-reserved", func(t *testing.T) {
		absCwd, err := filepath.Abs(".")
		require.NoError(t, err)
		m := &ToolchainManifest{
			WorkspaceRoot: ".",
			Tools: []ToolEntry{
				{
					ID:               "backlogit",
					ContainmentClass: WorkspaceLocalPinned,
					ExecutablePath:   filepath.Join(absCwd, ".tools", "bin", "backlogit.exe"),
				},
			},
		}
		assert.Equal(t, ".tools/bin", effectiveManifestToolsDir(m))
		f := toolRemediation(ToolVerificationFinding{ToolID: "backlogit", Status: "violation"}, m)
		assert.Contains(t, f.Remediation, "./.tools/bin/backlogit.exe")

		reserved := &ToolchainManifest{
			WorkspaceRoot: ".",
			ToolsDir:      ".GIT/bin",
			Tools: []ToolEntry{
				{
					ID:               "backlogit",
					ContainmentClass: WorkspaceLocalPinned,
					ExecutablePath:   ".Backlog/backlogit.exe",
				},
			},
		}
		assert.Equal(t, ".bin", effectiveManifestToolsDir(reserved))
	})
}

func TestU2b_WriteAndValidateToolchainManifestWithRelativeWorkspaceRootAndDifferentCWD(t *testing.T) {
	binBytes := []byte("pinned binary")
	binSum := sha256.Sum256(binBytes)
	binSumHex := hex.EncodeToString(binSum[:])

	newRelManifest := func(execDir string) *ToolchainManifest {
		return &ToolchainManifest{
			SchemaVersion:              "1.0",
			ContainmentLevel:           Level1WorkspaceLocalPinned,
			OSContainerSandboxDeferred: true,
			WorkspaceRoot:              ".",
			ToolsDir:                   execDir,
			Tools: []ToolEntry{
				{
					ID:                   "backlogit",
					Kind:                 "mcp",
					ContainmentClass:     WorkspaceLocalPinned,
					ExecutablePath:       "./" + execDir + "/backlogit.exe",
					PinnedVersion:        "1.0.0",
					SHA256:               binSumHex,
					WorkspaceBindingArgs: []string{"--cwd", "."},
				},
			},
			HostPrerequisites: []HostPrerequisiteEntry{
				{ID: "shell", Command: "shell", Purpose: "bootstrap", Reason: "host shell", Required: false},
			},
		}
	}

	launcherPS1 := func(execDir string) string {
		return fmt.Sprintf(`$workspaceRoot = $PSScriptRoot
$workspaceBinDirectory = Join-Path $workspaceRoot %q
$backlogitPath = Join-Path $workspaceBinDirectory "backlogit.exe"
if (-not (Test-Path -LiteralPath $backlogitPath -PathType Leaf)) { throw "Missing ./%s/backlogit.exe" }
$env:COPILOT_HOME = Join-Path $workspaceRoot ".copilot"
$env:ENGRAM_DATA_DIR = Join-Path $workspaceRoot ".engram"
& $backlogitPath --cwd $workspaceRoot mcp status
`, execDir, execDir)
	}

	launcherSH := func(execDir string) string {
		return fmt.Sprintf(`#!/bin/sh
workspace_root=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
workspace_bin_directory="$workspace_root/%s"
backlogit_path="$workspace_bin_directory/backlogit.exe"
if [ ! -f "$backlogit_path" ]; then echo "Missing ./%s/backlogit.exe" >&2; exit 1; fi
export COPILOT_HOME="$workspace_root/.copilot"
export ENGRAM_DATA_DIR="$workspace_root/.engram"
"$backlogit_path" --cwd "$workspace_root" mcp status
`, execDir, execDir)
	}

	t.Run("relative_workspace_root_dot_and_dual_root_override_succeed_from_different_cwd", func(t *testing.T) {
		pkgDir, wdErr := os.Getwd()
		require.NoError(t, wdErr)
		_ = pkgDir

		// Single-root workspace with BACKLOGIT_WORKSPACE_DIR=""
		singleRoot, otherDir := u9Workspace(t)
		u9Write(t, singleRoot, ".bin/backlogit.exe", string(binBytes))
		u9Write(t, singleRoot, "start.ps1", launcherPS1(".bin"))
		u9Write(t, singleRoot, "start.sh", launcherSH(".bin"))
		u9Write(t, singleRoot, ".mcp.json", fmt.Sprintf(`{"mcpServers":{"backlogit":{"type":"stdio","command":"./.bin/backlogit.exe","args":["mcp","--cwd",%q]}}}`, singleRoot))

		t.Chdir(otherDir)
		t.Setenv("BACKLOGIT_WORKSPACE_DIR", "")

		manifest := newRelManifest(".bin")
		wsHandle, openErr := os.OpenRoot(singleRoot)
		require.NoError(t, openErr)
		t.Cleanup(func() { _ = wsHandle.Close() })

		require.NoError(t, validateManifestForProvisioning(wsHandle, singleRoot, manifest))
		storageName, storageErr := resolveProvisioningStorageDir(wsHandle)
		require.NoError(t, storageErr)
		assert.Equal(t, ".backlogit", storageName)

		require.NoError(t, WriteToolchainManifest(singleRoot, manifest))
		findings, verifyErr := VerifyLauncherScripts(singleRoot)
		require.NoError(t, verifyErr)
		for _, f := range findings {
			assert.NotEqual(t, "violation", f.Status, "%+v", f)
		}
		auditReport, auditErr := AuditRepositoryToolchain(context.Background(), singleRoot)
		require.NoError(t, auditErr)
		assert.True(t, auditReport.Valid, "%+v", auditReport.Tools)
		assert.Zero(t, auditReport.Violations)

		// Dual-root workspace (.backlog + .backlogit) with BACKLOGIT_WORKSPACE_DIR=".backlog"
		dualRoot, _ := u9Workspace(t)
		require.NoError(t, os.MkdirAll(filepath.Join(dualRoot, ".backlog"), 0o755))
		require.NoError(t, os.MkdirAll(filepath.Join(dualRoot, ".backlogit"), 0o755))
		u9Write(t, dualRoot, ".bin/backlogit.exe", string(binBytes))
		u9Write(t, dualRoot, "start.ps1", launcherPS1(".bin"))
		u9Write(t, dualRoot, "start.sh", launcherSH(".bin"))
		u9Write(t, dualRoot, ".mcp.json", fmt.Sprintf(`{"mcpServers":{"backlogit":{"type":"stdio","command":"./.bin/backlogit.exe","args":["mcp","--cwd",%q]}}}`, dualRoot))

		t.Setenv("BACKLOGIT_WORKSPACE_DIR", ".backlog")
		dualHandle, openDualErr := os.OpenRoot(dualRoot)
		require.NoError(t, openDualErr)
		t.Cleanup(func() { _ = dualHandle.Close() })

		dualStorage, dualStorageErr := resolveProvisioningStorageDir(dualHandle)
		require.NoError(t, dualStorageErr)
		assert.Equal(t, ".backlog", dualStorage)

		require.NoError(t, WriteToolchainManifest(dualRoot, newRelManifest(".bin")))
		dualAudit, dualAuditErr := AuditRepositoryToolchain(context.Background(), dualRoot)
		require.NoError(t, dualAuditErr)
		assert.True(t, dualAudit.Valid, "%+v", dualAudit.Tools)
		assert.Zero(t, dualAudit.Violations)
	})

	t.Run("mismatched_workspace_root_or_unoverridden_dual_roots_fail_closed", func(t *testing.T) {
		singleRoot, otherDir := u9Workspace(t)
		u9Write(t, singleRoot, ".bin/backlogit.exe", string(binBytes))
		t.Chdir(otherDir)

		wsHandle, openErr := os.OpenRoot(singleRoot)
		require.NoError(t, openErr)
		t.Cleanup(func() { _ = wsHandle.Close() })

		for _, badWSRoot := range []string{"./subdir", "../escape", otherDir, " . "} {
			m := newRelManifest(".bin")
			m.WorkspaceRoot = badWSRoot
			require.ErrorIs(t, validateManifestForProvisioning(wsHandle, singleRoot, m), corerrors.ErrValidation)
			require.ErrorIs(t, WriteToolchainManifest(singleRoot, m), corerrors.ErrValidation)
		}

		dualRoot, _ := u9Workspace(t)
		require.NoError(t, os.MkdirAll(filepath.Join(dualRoot, ".backlog"), 0o755))
		require.NoError(t, os.MkdirAll(filepath.Join(dualRoot, ".backlogit"), 0o755))
		u9Write(t, dualRoot, ".bin/backlogit.exe", string(binBytes))
		dualHandle, openDualErr := os.OpenRoot(dualRoot)
		require.NoError(t, openDualErr)
		t.Cleanup(func() { _ = dualHandle.Close() })

		for _, envVal := range []string{"", "invalid"} {
			t.Setenv("BACKLOGIT_WORKSPACE_DIR", envVal)
			_, storErr := resolveProvisioningStorageDir(dualHandle)
			require.ErrorIs(t, storErr, corerrors.ErrValidation)
			require.ErrorIs(t, WriteToolchainManifest(dualRoot, newRelManifest(".bin")), corerrors.ErrValidation)
		}
	})

	t.Run("verify_proposed_launcher_scripts_validates_in_memory_bytes_without_disk_reload_and_rejects_mismatched_or_unpinned_launchers", func(t *testing.T) {
		pkgDir, wdErr := os.Getwd()
		require.NoError(t, wdErr)

		// AST check: VerifyProposedLauncherScripts is exported and delegates to inspectLauncher without LoadToolchainManifest.
		launcherAST, parseErr := parser.ParseFile(token.NewFileSet(), filepath.Join(pkgDir, "launcher.go"), nil, 0)
		require.NoError(t, parseErr)
		var verifyProposedFn *ast.FuncDecl
		for _, decl := range launcherAST.Decls {
			if fn, ok := decl.(*ast.FuncDecl); ok && fn.Recv == nil && fn.Name.Name == "VerifyProposedLauncherScripts" {
				verifyProposedFn = fn
				break
			}
		}
		require.NotNil(t, verifyProposedFn, "launcher.go must export VerifyProposedLauncherScripts")
		callsInspect := false
		callsLoad := false
		ast.Inspect(verifyProposedFn.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			if id, ok := call.Fun.(*ast.Ident); ok {
				if id.Name == "inspectLauncher" {
					callsInspect = true
				}
				if id.Name == "LoadToolchainManifest" || id.Name == "loadToolchainManifest" {
					callsLoad = true
				}
			}
			return true
		})
		assert.True(t, callsInspect, "VerifyProposedLauncherScripts must delegate to inspectLauncher")
		assert.False(t, callsLoad, "VerifyProposedLauncherScripts must not reload toolchain.yaml from disk")

		// Runtime check: while on-disk files still point to ./bin/backlogit.exe, inspectLauncher validates proposed .tools/bin bytes in-memory.
		root, otherDir := u9Workspace(t)
		u9Write(t, root, "bin/backlogit.exe", string(binBytes))
		u9Write(t, root, "start.ps1", launcherPS1("bin"))
		u9Write(t, root, "start.sh", launcherSH("bin"))
		u9Write(t, root, ".mcp.json", fmt.Sprintf(`{"mcpServers":{"backlogit":{"type":"stdio","command":"./bin/backlogit.exe","args":["mcp","--cwd",%q]}}}`, root))
		require.NoError(t, WriteToolchainManifest(root, newRelManifest("bin")))

		t.Chdir(otherDir)
		proposed := newRelManifest(".tools/bin")
		proposed.WorkspaceRoot = root

		wsHandle, openErr := os.OpenRoot(root)
		require.NoError(t, openErr)
		t.Cleanup(func() { _ = wsHandle.Close() })

		// Pre-install proposed launcher bytes targeting .tools/bin (not yet on disk) pass VerifyProposedLauncherScripts and inspectLauncher with zero violations.
		proposedFindings, vpErr := VerifyProposedLauncherScripts(root, proposed, []byte(launcherPS1(".tools/bin")), []byte(launcherSH(".tools/bin")))
		require.NoError(t, vpErr)
		for _, f := range proposedFindings {
			assert.NotEqual(t, "violation", f.Status, "VerifyProposedLauncherScripts must pass without violation: %+v", f)
		}
		for _, item := range []struct {
			name string
			body string
		}{
			{name: "start.ps1", body: launcherPS1(".tools/bin")},
			{name: "start.sh", body: launcherSH(".tools/bin")},
		} {
			findings := inspectLauncher(wsHandle, root, item.name, item.body, proposed, true)
			for _, f := range findings {
				assert.NotEqual(t, "violation", f.Status, "proposed %s must pass without violation: %+v", item.name, f)
			}
		}

		// Disk-based VerifyLauncherScripts must still fail closed when the executable is missing on disk.
		missingBinRoot, _ := u9Workspace(t)
		u9Write(t, missingBinRoot, ".bin/backlogit.exe", string(binBytes))
		u9Write(t, missingBinRoot, "start.ps1", launcherPS1(".bin"))
		u9Write(t, missingBinRoot, "start.sh", launcherSH(".bin"))
		u9Write(t, missingBinRoot, ".mcp.json", fmt.Sprintf(`{"mcpServers":{"backlogit":{"type":"stdio","command":"./.bin/backlogit.exe","args":["mcp","--cwd",%q]}}}`, missingBinRoot))
		require.NoError(t, WriteToolchainManifest(missingBinRoot, newRelManifest(".bin")))
		require.NoError(t, os.Remove(filepath.Join(missingBinRoot, ".bin", "backlogit.exe")))
		diskFindings, diskErr := VerifyLauncherScripts(missingBinRoot)
		require.NoError(t, diskErr)
		hasDiskViolation := false
		for _, f := range diskFindings {
			if f.Status == "violation" {
				hasDiskViolation = true
				break
			}
		}
		assert.True(t, hasDiskViolation, "VerifyLauncherScripts must fail closed when backlogit binary is missing on disk")

		// Un-migrated (still pointing to bin), missing guard, missing --cwd, or ambient PATH lookup produce violation findings.
		badPS1Cases := []string{
			launcherPS1("bin"),
			"$workspaceRoot = $PSScriptRoot\n$backlogitPath = Join-Path $workspaceRoot \".tools/bin/backlogit.exe\"\n$env:COPILOT_HOME = Join-Path $workspaceRoot \".copilot\"\n$env:ENGRAM_DATA_DIR = Join-Path $workspaceRoot \".engram\"\n& $backlogitPath --cwd $workspaceRoot mcp status\n",
			"$workspaceRoot = $PSScriptRoot\n$backlogitPath = Join-Path $workspaceRoot \".tools/bin/backlogit.exe\"\nif (-not (Test-Path -LiteralPath $backlogitPath -PathType Leaf)) { throw \"missing\" }\n$env:COPILOT_HOME = Join-Path $workspaceRoot \".copilot\"\n$env:ENGRAM_DATA_DIR = Join-Path $workspaceRoot \".engram\"\n& $backlogitPath mcp status\n",
			launcherPS1(".tools/bin") + "$companion = (Get-Command \"engram\")\n",
		}
		for _, badBody := range badPS1Cases {
			findings := inspectLauncher(wsHandle, root, "start.ps1", badBody, proposed, true)
			hasViolation := false
			for _, f := range findings {
				if f.Status == "violation" {
					hasViolation = true
					break
				}
			}
			assert.True(t, hasViolation, "invalid proposed launcher must produce violation finding:\n%s", badBody)
		}
	})
}
