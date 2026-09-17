package config

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

const trustAnchorSourceFile = "schema.go"

type trustAnchorFieldShape struct {
	name     string
	typeName string
	yamlTag  string
}

func TestTrustAnchorSourceShape(t *testing.T) {
	file, err := parser.ParseFile(token.NewFileSet(), trustAnchorSourceFile, nil, parser.AllErrors)
	if err != nil {
		t.Fatalf("parse %s: %v", trustAnchorSourceFile, err)
	}

	trustAnchor := findStructType(file, "TrustAnchor")
	if trustAnchor == nil {
		t.Errorf("TrustAnchor struct is not declared in %s", trustAnchorSourceFile)
	} else {
		assertTrustAnchorFields(t, trustAnchor)
	}

	workspaceConfig := findStructType(file, "WorkspaceConfig")
	if workspaceConfig == nil {
		t.Fatalf("WorkspaceConfig struct is not declared in %s", trustAnchorSourceFile)
	}
	assertStructFieldShape(t, workspaceConfig, trustAnchorFieldShape{
		name:     "TrustAnchors",
		typeName: "[]TrustAnchor",
		yamlTag:  `yaml:"trust_anchors"`,
	})
}

func assertTrustAnchorFields(t *testing.T, trustAnchor *ast.StructType) {
	t.Helper()

	expected := []trustAnchorFieldShape{
		{name: "ID", typeName: "string", yamlTag: `yaml:"id"`},
		{name: "Role", typeName: "string", yamlTag: `yaml:"role"`},
		{name: "Algo", typeName: "string", yamlTag: `yaml:"algo"`},
		{name: "PublicKeyRef", typeName: "string", yamlTag: `yaml:"public_key_ref"`},
		{name: "Fingerprint", typeName: "string", yamlTag: `yaml:"fingerprint"`},
		{name: "Status", typeName: "string", yamlTag: `yaml:"status"`},
		{name: "NotBefore", typeName: "time.Time", yamlTag: `yaml:"not_before"`},
		{name: "NotAfter", typeName: "time.Time", yamlTag: `yaml:"not_after"`},
	}

	if len(trustAnchor.Fields.List) != len(expected) {
		t.Errorf("TrustAnchor must declare exactly %d fields, got %d", len(expected), len(trustAnchor.Fields.List))
	}
	for _, field := range expected {
		assertStructFieldShape(t, trustAnchor, field)
	}
}

func assertStructFieldShape(t *testing.T, structType *ast.StructType, expected trustAnchorFieldShape) {
	t.Helper()

	for _, field := range structType.Fields.List {
		if len(field.Names) != 1 || field.Names[0].Name != expected.name {
			continue
		}
		if !matchesTypeName(field.Type, expected.typeName) {
			t.Errorf("%s field must have type %s", expected.name, expected.typeName)
		}
		if field.Tag == nil || field.Tag.Value != "`"+expected.yamlTag+"`" {
			t.Errorf("%s field must have exact struct tag `%s`", expected.name, expected.yamlTag)
		}
		return
	}

	t.Errorf("%s field is not declared", expected.name)
}

func matchesTypeName(expr ast.Expr, expected string) bool {
	switch typed := expr.(type) {
	case *ast.Ident:
		return typed.Name == expected
	case *ast.SelectorExpr:
		qualifier, ok := typed.X.(*ast.Ident)
		return ok && qualifier.Name+"."+typed.Sel.Name == expected
	case *ast.ArrayType:
		element, ok := typed.Elt.(*ast.Ident)
		return typed.Len == nil && ok && "[]"+element.Name == expected
	default:
		return false
	}
}

func findStructType(file *ast.File, name string) *ast.StructType {
	for _, declaration := range file.Decls {
		general, ok := declaration.(*ast.GenDecl)
		if !ok || general.Tok != token.TYPE {
			continue
		}
		for _, specification := range general.Specs {
			typeSpec, ok := specification.(*ast.TypeSpec)
			if !ok || typeSpec.Name.Name != name {
				continue
			}
			structType, _ := typeSpec.Type.(*ast.StructType)
			return structType
		}
	}
	return nil
}
