package core_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestU175_021_LintFix asserts the owned files discard no Close/fmt.Fprint* error.
func TestU175_021_LintFix(t *testing.T) {
	for _, name := range []string{"shipment_reconcile_fs_windows.go", "060_archive_rollback_integrity_harness_test.go"} {
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, name, nil, 0)
		require.NoError(t, err)
		var offenders []string
		ast.Inspect(file, func(n ast.Node) bool {
			var call *ast.CallExpr
			switch s := n.(type) {
			case *ast.ExprStmt:
				call, _ = s.X.(*ast.CallExpr)
			case *ast.DeferStmt:
				call = s.Call
			case *ast.GoStmt:
				call = s.Call
			}
			if call == nil {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			pkg, isIdent := sel.X.(*ast.Ident)
			if sel.Sel.Name == "Close" || (isIdent && pkg.Name == "fmt" && strings.HasPrefix(sel.Sel.Name, "Fprint")) {
				offenders = append(offenders, fset.Position(call.Pos()).String())
			}
			return true
		})
		assert.Empty(t, offenders, "%s: unchecked error returns remain", name)
	}
}
