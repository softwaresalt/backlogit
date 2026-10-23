package core

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

type u175_023CloseTarget struct {
	file      string
	functions []string
}

func TestU175_023_LintFix(t *testing.T) {
	u175_023AssertCloseErrorsHandled(t, []u175_023CloseTarget{
		{
			file: "shipment_test.go",
			functions: []string{
				"TestNewWorkspace_RecoversPendingReturnBlockedJournal",
				"TestShipment_RehydrationConsistency",
				"setupShipmentWorkspace",
			},
		},
		{
			file: "archive_consistency_harness_042_test.go",
			functions: []string{
				"TestArchiveItem_DBFailureAfterFileMove_OriginalFileRestored",
				"TestArchiveItem_DBFailure_DBStatusUnchanged",
				"setupArchiveConsistencyWorkspace",
			},
		},
	})
}

func u175_023AssertCloseErrorsHandled(t *testing.T, targets []u175_023CloseTarget) {
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
				if u175_023HasUncheckedClose(node) {
					position := fileSet.Position(node.Pos())
					t.Errorf("%s:%d: unchecked Close error in %s", target.file, position.Line, functionName)
				}
				return true
			})
		}
	}
}

func u175_023HasUncheckedClose(node ast.Node) bool {
	switch statement := node.(type) {
	case *ast.ExprStmt:
		return u175_023IsCloseCall(statement.X)
	case *ast.DeferStmt:
		return u175_023IsCloseCall(statement.Call)
	case *ast.AssignStmt:
		for _, right := range statement.Rhs {
			if !u175_023IsCloseCall(right) {
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

func u175_023IsCloseCall(expression ast.Expr) bool {
	call, ok := expression.(*ast.CallExpr)
	if !ok || len(call.Args) != 0 {
		return false
	}
	selector, ok := call.Fun.(*ast.SelectorExpr)
	return ok && selector.Sel.Name == "Close"
}
