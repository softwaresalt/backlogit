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
