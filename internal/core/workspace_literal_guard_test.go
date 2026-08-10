package core

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWorkspaceLiteralGuard_NoHardcodedLegacyStorageRoot(t *testing.T) {
	root := repoRootFromCurrentFile(t)
	fset := token.NewFileSet()
	var violations []string

	for _, dir := range []string{"core", "cli", "mcp", "config"} {
		err := filepath.WalkDir(filepath.Join(root, "internal", dir), func(path string, d os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if d.IsDir() || filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
				return nil
			}

			file, err := parser.ParseFile(fset, path, nil, 0)
			if err != nil {
				return err
			}

			ast.Inspect(file, func(node ast.Node) bool {
				lit, ok := node.(*ast.BasicLit)
				if !ok || lit.Kind != token.STRING {
					return true
				}

				value, err := strconv.Unquote(lit.Value)
				if err != nil || value != ".backlogit" {
					return true
				}
				if path == filepath.Join(root, "internal", "core", "workspace.go") {
					return true
				}

				pos := fset.Position(lit.Pos())
				rel, relErr := filepath.Rel(root, pos.Filename)
				if relErr != nil {
					rel = pos.Filename
				}
				violations = append(violations, rel+":"+strconv.Itoa(pos.Line))
				return true
			})
			return nil
		})
		require.NoError(t, err)
	}
	require.Empty(t, violations, "found hardcoded legacy storage-root literals outside workspaceRootCandidates")
}

func repoRootFromCurrentFile(t *testing.T) string {
	t.Helper()

	_, currentFile, _, ok := runtimeCaller(0)
	require.True(t, ok, "resolve caller")
	return filepath.Clean(filepath.Join(filepath.Dir(currentFile), "..", ".."))
}

var runtimeCaller = func(skip int) (uintptr, string, int, bool) {
	return runtime.Caller(skip + 1)
}
