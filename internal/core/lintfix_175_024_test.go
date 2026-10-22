package core

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

type u175_024CloseTarget struct {
	file      string
	functions []string
}

func TestU175_024_LintFix(t *testing.T) {
	u175_024AssertCloseErrorsHandled(t, []u175_024CloseTarget{
		{
			file:      "workspace.go",
			functions: []string{"newWorkspace"},
		},
		{
			file: "migrate_links_canonical_test.go",
			functions: []string{
				"BenchmarkMigrateDBOnlyLinksCanonicalIndex",
				"TestMigrateDBOnlyLinksBuildsOneCanonicalIndex",
				"TestMigrateDBOnlyLinksCountsUnresolvedSourceAsSkipped",
				"TestMigrateDBOnlyLinksFailsClosedOnParseFailure",
				"TestMigrateDBOnlyLinksIgnoresUnlinkedDuplicateIDs",
				"TestMigrateDBOnlyLinksRejectsDuplicateArtifactIDs",
				"TestNewWorkspaceDefersDBOnlyLinkMigration",
				"TestRemoveArtifactLinkDeletesDBOnlyLink",
				"TestRemoveArtifactLinkKeepsCacheWhenMarkdownWriteIsNotApplied",
			},
		},
	})
}

func u175_024AssertCloseErrorsHandled(t *testing.T, targets []u175_024CloseTarget) {
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
				if u175_024HasUncheckedClose(node) {
					position := fileSet.Position(node.Pos())
					t.Errorf("%s:%d: unchecked Close error in %s", target.file, position.Line, functionName)
				}
				return true
			})
		}
	}
}

func u175_024HasUncheckedClose(node ast.Node) bool {
	switch statement := node.(type) {
	case *ast.ExprStmt:
		return u175_024IsCloseCall(statement.X)
	case *ast.DeferStmt:
		return u175_024IsCloseCall(statement.Call)
	case *ast.AssignStmt:
		for _, right := range statement.Rhs {
			if !u175_024IsCloseCall(right) {
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

func u175_024IsCloseCall(expression ast.Expr) bool {
	call, ok := expression.(*ast.CallExpr)
	if !ok || len(call.Args) != 0 {
		return false
	}
	selector, ok := call.Fun.(*ast.SelectorExpr)
	return ok && selector.Sel.Name == "Close"
}
