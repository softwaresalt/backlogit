package cli_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestU175_009_LintFix asserts status_cmd.go discards no Close/fmt.Fprint*
// error and jsonrpc_test.go has no redundant explicit func var type (QF1011).
func TestU175_009_LintFix(t *testing.T) {
	fset := token.NewFileSet()

	status, err := parser.ParseFile(fset, "status_cmd.go", nil, 0)
	require.NoError(t, err)
	var unchecked []string
	ast.Inspect(status, func(n ast.Node) bool {
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
			unchecked = append(unchecked, fset.Position(call.Pos()).String())
		}
		return true
	})
	assert.Empty(t, unchecked, "unchecked error returns remain in status_cmd.go")

	jsonrpc, err := parser.ParseFile(fset, "jsonrpc_test.go", nil, 0)
	require.NoError(t, err)
	var redundant []string
	ast.Inspect(jsonrpc, func(n ast.Node) bool {
		spec, ok := n.(*ast.ValueSpec)
		if !ok || spec.Type == nil || len(spec.Values) == 0 {
			return true
		}
		if _, isFunc := spec.Type.(*ast.FuncType); isFunc {
			redundant = append(redundant, fset.Position(spec.Pos()).String())
		}
		return true
	})
	assert.Empty(t, redundant, "redundant func type in var declaration (QF1011)")
}
