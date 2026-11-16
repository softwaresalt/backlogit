package toolchain

import (
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
