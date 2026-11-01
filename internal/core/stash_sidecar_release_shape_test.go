package core

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// This compiling pre-declaration harness checks Unix contracts on Windows.
// It is not a substitute for execution of the Unix behavioral regressions.
func TestReviewStashReleaseUnixSourceContract(t *testing.T) {
	source, err := parser.ParseFile(token.NewFileSet(), "task_lock_unix.go", nil, 0)
	require.NoError(t, err)
	functions := make(map[string]*ast.FuncDecl)
	for _, declaration := range source.Decls {
		if function, ok := declaration.(*ast.FuncDecl); ok {
			functions[function.Name.Name] = function
		}
	}
	t.Run("retained-parent", func(t *testing.T) {
		open := functions["openStashLockHandle"]
		require.NotNil(t, open)
		lastOpen, lastClose := token.NoPos, token.NoPos
		ast.Inspect(open.Body, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			selector, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || len(call.Args) == 0 {
				return true
			}
			argument, ok := call.Args[0].(*ast.Ident)
			if !ok || argument.Name != "dirFD" {
				return true
			}
			switch selector.Sel.Name {
			case "Openat":
				lastOpen = call.Pos()
			case "Close":
				lastClose = call.Pos()
			}
			return true
		})
		require.NotEqual(t, token.NoPos, lastOpen)
		require.Less(t, lastClose, lastOpen,
			"the final parent pin must transfer to deterministic ownership, not close after leaf open")
	})
	t.Run("relative-owned-unlink", func(t *testing.T) {
		release := functions["releaseStashSidecarFile"]
		require.NotNil(t, release)
		calls := make(map[string]token.Pos)
		fields := make(map[string]bool)
		ast.Inspect(release.Body, func(node ast.Node) bool {
			if selector, ok := node.(*ast.SelectorExpr); ok {
				fields[selector.Sel.Name] = true
			}
			if call, ok := node.(*ast.CallExpr); ok {
				switch function := call.Fun.(type) {
				case *ast.SelectorExpr:
					calls[function.Sel.Name] = call.Pos()
				case *ast.Ident:
					calls[function.Name] = call.Pos()
				}
			}
			return true
		})
		require.Contains(t, calls, "Fstat", "release must inspect the owned inode")
		require.Contains(t, calls, "Fstatat", "release must inspect the pinned basename without following links")
		require.Contains(t, calls, "Unlinkat", "release must unlink relative to the retained parent")
		require.Less(t, calls["Fstatat"], calls["Unlinkat"])
		for _, field := range []string{"Dev", "Ino", "Nlink", "Mode", "AT_SYMLINK_NOFOLLOW"} {
			require.True(t, fields[field], "release must validate %s", field)
		}
		require.NotContains(t, calls, "Remove")
		require.NotContains(t, calls, "Name")
		require.NotContains(t, calls, "unlockStashFile")
	})
}

func TestReviewStashDirectCloseIsDeterministic(t *testing.T) {
	file, err := acquireStashSidecarFile(filepath.Join(t.TempDir(), "stash.lock"))
	require.NoError(t, err)
	require.NoError(t, file.Close())
	require.NoError(t, file.Close(), "owned resource closure must run exactly once")
	_, err = file.WriteString("must not write through a closed handle")
	require.ErrorIs(t, err, os.ErrClosed)
}
