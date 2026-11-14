package toolchain

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

// TestU11a_LauncherDeclarationsSourceShape gates the declarations in 213.004-T.
func TestU11a_LauncherDeclarationsSourceShape(t *testing.T) {
	file, err := parser.ParseFile(token.NewFileSet(), "launcher.go", nil, 0)
	if err != nil {
		t.Fatalf("launcher declarations are not available in launcher.go: %v", err)
	}
	if file.Name.Name != "toolchain" {
		t.Errorf("launcher.go package = %q, want toolchain", file.Name.Name)
	}

	// Resolve context by import path, without requiring an import alias or style.
	contextQualifier := ""
	for _, spec := range file.Imports {
		if strings.Trim(spec.Path.Value, "\"`") == "context" {
			contextQualifier = "context"
			if spec.Name != nil {
				contextQualifier = spec.Name.Name
			}
		}
	}
	var typeShape func(ast.Expr) string
	typeShape = func(expr ast.Expr) string {
		switch expr := expr.(type) {
		case *ast.Ident:
			if contextQualifier == "." && expr.Name == "Context" {
				return "context.Context"
			}
			return expr.Name
		case *ast.SelectorExpr:
			if qualifier, ok := expr.X.(*ast.Ident); ok &&
				contextQualifier != "" && contextQualifier != "_" &&
				qualifier.Name == contextQualifier && expr.Sel.Name == "Context" {
				return "context.Context"
			}
		case *ast.StarExpr:
			return "*" + typeShape(expr.X)
		case *ast.ArrayType:
			if expr.Len == nil {
				return "[]" + typeShape(expr.Elt)
			}
		case *ast.ParenExpr:
			return typeShape(expr.X)
		}
		return "<unsupported type>"
	}
	fieldShape := func(fields *ast.FieldList, includeNames bool) string {
		var shape strings.Builder
		if fields != nil {
			for _, field := range fields.List {
				count := len(field.Names)
				if count == 0 {
					count = 1
				}
				for i := 0; i < count; i++ {
					if shape.Len() != 0 {
						shape.WriteString(", ")
					}
					if includeNames && len(field.Names) != 0 {
						shape.WriteString(field.Names[i].Name)
						shape.WriteByte(' ')
					}
					shape.WriteString(typeShape(field.Type))
				}
			}
		}
		return shape.String()
	}

	for _, tc := range []struct {
		name    string
		params  string
		results string
	}{
		{"VerifyLauncherScripts", "rootPath string", "[]ToolVerificationFinding, error"},
		{"AuditRepositoryToolchain", "ctx context.Context, rootPath string", "*ToolchainVerificationReport, error"},
		{"WriteToolchainManifest", "rootPath string, manifest *ToolchainManifest", "error"},
		{"RenderPinnedMCPConfig", "rootPath string, manifest *ToolchainManifest", "[]byte, error"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			matches := 0
			for _, decl := range file.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok || fn.Name.Name != tc.name || fn.Recv != nil {
					continue
				}
				matches++
				if fn.Type.TypeParams != nil && len(fn.Type.TypeParams.List) != 0 {
					t.Errorf("%s in launcher.go must not have type parameters", tc.name)
				}
				if got := fieldShape(fn.Type.Params, true); got != tc.params {
					t.Errorf("%s parameters in launcher.go = (%s), want (%s)", tc.name, got, tc.params)
				}
				if got := fieldShape(fn.Type.Results, false); got != tc.results {
					t.Errorf("%s results in launcher.go = (%s), want (%s)", tc.name, got, tc.results)
				}
			}
			if matches != 1 {
				t.Errorf("%s must be declared exactly once as a receiver-less function in launcher.go; found %d", tc.name, matches)
			}
		})
	}
}
