package gateevidence

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

func TestU175_029_LintFix(t *testing.T) {
	fileSet := token.NewFileSet()
	parsed, err := parser.ParseFile(fileSet, "formal.go", nil, parser.AllErrors)
	if err != nil {
		t.Fatalf("parse formal.go: %v", err)
	}

	var function *ast.FuncDecl
	for _, declaration := range parsed.Decls {
		candidate, ok := declaration.(*ast.FuncDecl)
		if ok && candidate.Name.Name == "envelopeFromEvent" {
			function = candidate
			break
		}
	}
	if function == nil || function.Body == nil {
		t.Fatal("envelopeFromEvent is missing")
	}

	hasTaggedSchemaSwitch := false
	ast.Inspect(function.Body, func(node ast.Node) bool {
		switchStatement, ok := node.(*ast.SwitchStmt)
		if !ok {
			return true
		}
		tag, ok := switchStatement.Tag.(*ast.Ident)
		if ok && tag.Name == "schema" {
			hasTaggedSchemaSwitch = true
		}
		return true
	})
	if !hasTaggedSchemaSwitch {
		t.Error("envelopeFromEvent does not use a tagged switch on schema (QF1003)")
	}
}
