package events

import (
	"go/ast"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCleanupCheckpoints_NoPreRemoveInAST is a P-002 FC-1 structural assertion
// (150-F / 150.001-T / stash 11FFF601).
// It parses checkpoint_lifecycle.go and asserts that CleanupCheckpoints contains
// no os.Remove(dst) call targeting the archive destination. The pre-Remove
// block was a data-loss window: if os.Rename fails after os.Remove succeeds,
// any previously archived copy at dst is permanently destroyed (the source at
// path remains). Go 1.24.0 os.Rename on Windows calls MoveFileExW with
// MOVEFILE_REPLACE_EXISTING, which handles an existing dst without requiring a
// pre-Remove; the fix relies on this property, not full crash-atomic replacement.
// This test is RED against the pre-change source because os.Remove(dst) IS present;
// it turns GREEN after the block is removed.
func TestCleanupCheckpoints_NoPreRemoveInAST(t *testing.T) {
	file := parseEventsSource(t, "checkpoint_lifecycle.go")
	funcDecl := findPackageFuncIn(file, "CleanupCheckpoints")
	require.NotNil(t, funcDecl, "CleanupCheckpoints must be declared in checkpoint_lifecycle.go")

	// Walk the function body and detect any os.Remove call whose sole argument
	// is the identifier "dst" (the archive destination).
	var foundPreRemove bool
	ast.Inspect(funcDecl.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		pkgIdent, ok := sel.X.(*ast.Ident)
		if !ok || pkgIdent.Name != "os" || sel.Sel.Name != "Remove" {
			return true
		}
		if len(call.Args) == 1 {
			if argIdent, ok := call.Args[0].(*ast.Ident); ok && argIdent.Name == "dst" {
				foundPreRemove = true
			}
		}
		return true
	})

	assert.False(t, foundPreRemove,
		"CleanupCheckpoints must not contain os.Remove(dst): if os.Rename fails after os.Remove "+
			"succeeds, any previously archived copy at dst is permanently destroyed. "+
			"Go 1.24.0 os.Rename on Windows calls MoveFileExW(MOVEFILE_REPLACE_EXISTING), "+
			"which handles an existing dst without requiring a pre-Remove. "+
			"RED marker: PREREMOVE_FOUND_IN_CLEANUPCHECKPOINTS")
}
