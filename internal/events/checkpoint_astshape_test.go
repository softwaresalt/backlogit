package events

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"

	"github.com/stretchr/testify/require"
)

// Shared go/ast source-shape harness helpers (147-F / U1d, U15).
//
// A source-shape harness parses a not-yet-modified production file with
// go/parser and asserts the declared shape through go/ast, so it references
// no undeclared identifier. That is what lets the harness compile against
// the pre-declaration tree and fail on an assertion rather than a build
// error (plan cycle-31 test lifecycle; workflow-policies.md P-004).

// parseEventsSource parses the named file in this package directory and
// returns its AST for source-shape assertions.
func parseEventsSource(t *testing.T, filename string) *ast.File {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filename, nil, parser.AllErrors)
	require.NoError(t, err, "%s must parse as valid Go source", filename)
	return file
}

// tryParseEventsSource parses the named file without failing the test,
// returning the parse error (e.g. file-not-found) to the caller. Used by
// harnesses whose target file does not exist yet (147-F / U2, U11).
func tryParseEventsSource(t *testing.T, filename string) (*ast.File, error) {
	t.Helper()
	fset := token.NewFileSet()
	return parser.ParseFile(fset, filename, nil, parser.AllErrors)
}

func findPackageVarIn(file *ast.File, name string) *ast.ValueSpec {
	for _, decl := range file.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok || genDecl.Tok != token.VAR {
			continue
		}
		for _, spec := range genDecl.Specs {
			valueSpec, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			for _, n := range valueSpec.Names {
				if n.Name == name {
					return valueSpec
				}
			}
		}
	}
	return nil
}

func findPackageTypeIn(file *ast.File, name string) *ast.TypeSpec {
	for _, decl := range file.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok || genDecl.Tok != token.TYPE {
			continue
		}
		for _, spec := range genDecl.Specs {
			typeSpec, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}
			if typeSpec.Name.Name == name {
				return typeSpec
			}
		}
	}
	return nil
}

func findPackageFuncIn(file *ast.File, name string) *ast.FuncDecl {
	for _, decl := range file.Decls {
		funcDecl, ok := decl.(*ast.FuncDecl)
		if !ok || funcDecl.Recv != nil {
			continue
		}
		if funcDecl.Name.Name == name {
			return funcDecl
		}
	}
	return nil
}

// findStructFieldTag returns the raw (quoted) struct tag literal for the
// named field of the named struct type, or "" if the type, field, or tag is
// absent.
func findStructFieldTag(file *ast.File, typeName, fieldName string) string {
	typeSpec := findPackageTypeIn(file, typeName)
	if typeSpec == nil {
		return ""
	}
	structType, ok := typeSpec.Type.(*ast.StructType)
	if !ok || structType.Fields == nil {
		return ""
	}
	for _, field := range structType.Fields.List {
		for _, name := range field.Names {
			if name.Name == fieldName {
				if field.Tag == nil {
					return ""
				}
				return field.Tag.Value
			}
		}
	}
	return ""
}
