package cli_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestU175_048_LintFix asserts the owned functions discard no ws.Close/fmt.Fprint* error.
func TestU175_048_LintFix(t *testing.T) {
	owned := map[string][]string{
		"delete.go":             {"newDeleteCommand"},
		"delete_search_test.go": {"TestDeleteCommand_Force", "TestSearchCommand_FindsResults", "TestSearchCommand_WithLimit"},
	}
	fset := token.NewFileSet()
	var offenders []string
	for name, funcs := range owned {
		file, err := parser.ParseFile(fset, name, nil, 0)
		require.NoError(t, err)
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || !slices.Contains(funcs, fn.Name.Name) {
				continue
			}
			ast.Inspect(fn, func(n ast.Node) bool {
				var call *ast.CallExpr
				switch s := n.(type) {
				case *ast.ExprStmt:
					call, _ = s.X.(*ast.CallExpr)
				case *ast.DeferStmt:
					call = s.Call
				}
				if call == nil {
					return true
				}
				sel, ok := call.Fun.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				recv, isIdent := sel.X.(*ast.Ident)
				if isIdent && ((recv.Name == "ws" && sel.Sel.Name == "Close") || (recv.Name == "fmt" && strings.HasPrefix(sel.Sel.Name, "Fprint"))) {
					offenders = append(offenders, fset.Position(call.Pos()).String())
				}
				return true
			})
		}
	}
	assert.Empty(t, offenders, "unchecked error returns remain")
}
