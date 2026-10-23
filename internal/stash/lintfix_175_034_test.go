package stash

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

type u175_034Target struct {
	file      string
	functions []string
}

func TestU175_034_LintFix(t *testing.T) {
	u175_034AssertErrorsHandled(t, []u175_034Target{
		{file: "jsonl.go", functions: []string{"MigrateStashMDToJSONL"}},
	})
}

func u175_034AssertErrorsHandled(t *testing.T, targets []u175_034Target) {
	t.Helper()

	for _, target := range targets {
		fileSet := token.NewFileSet()
		parsed, err := parser.ParseFile(fileSet, target.file, nil, parser.AllErrors)
		if err != nil {
			t.Fatalf("parse %s: %v", target.file, err)
		}

		for _, functionName := range target.functions {
			var function *ast.FuncDecl
			for _, declaration := range parsed.Decls {
				candidate, ok := declaration.(*ast.FuncDecl)
				if ok && candidate.Name.Name == functionName {
					function = candidate
					break
				}
			}
			if function == nil || function.Body == nil {
				t.Errorf("%s: function %s is missing", target.file, functionName)
				continue
			}

			ast.Inspect(function.Body, func(node ast.Node) bool {
				if u175_034HasUncheckedCall(node) {
					position := fileSet.Position(node.Pos())
					t.Errorf("%s:%d: unchecked Close/Remove error in %s", target.file, position.Line, functionName)
				}
				return true
			})
		}
	}
}

func u175_034HasUncheckedCall(node ast.Node) bool {
	switch statement := node.(type) {
	case *ast.ExprStmt:
		return u175_034IsErrorCall(statement.X)
	case *ast.DeferStmt:
		return u175_034IsErrorCall(statement.Call)
	case *ast.AssignStmt:
		for _, right := range statement.Rhs {
			if !u175_034IsErrorCall(right) {
				continue
			}
			for _, left := range statement.Lhs {
				identifier, ok := left.(*ast.Ident)
				if ok && identifier.Name == "_" {
					return true
				}
			}
		}
	}
	return false
}

func u175_034IsErrorCall(expression ast.Expr) bool {
	call, ok := expression.(*ast.CallExpr)
	if !ok {
		return false
	}
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	if selector.Sel.Name == "Close" && len(call.Args) == 0 {
		return true
	}
	receiver, ok := selector.X.(*ast.Ident)
	return ok && receiver.Name == "os" && selector.Sel.Name == "Remove"
}
