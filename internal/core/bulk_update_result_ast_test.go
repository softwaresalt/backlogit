package core

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestU172_BulkUpdateResultTypedConflictSurface(t *testing.T) {
	file := parseCoreSourceFile(t, "queue.go")

	resultType := findStructType(t, file, "BulkUpdateResult")
	assertStructFieldType(t, resultType, "Failed", "[]string")
	assertStructFieldType(t, resultType, "FailedDetails", "[]BulkUpdateConflict")

	conflictType := findStructType(t, file, "BulkUpdateConflict")
	for _, field := range []string{"ID", "Err", "FromStatus", "ToStatus"} {
		assert.True(t, hasStructField(conflictType, field), "BulkUpdateConflict.%s must be declared", field)
	}
}

func parseCoreSourceFile(t *testing.T, name string) *ast.File {
	t.Helper()
	wd, err := os.Getwd()
	require.NoError(t, err)
	path := filepath.Join(wd, name)
	set := token.NewFileSet()
	file, err := parser.ParseFile(set, path, nil, parser.ParseComments)
	require.NoError(t, err)
	return file
}

func findStructType(t *testing.T, file *ast.File, name string) *ast.StructType {
	t.Helper()
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.TYPE {
			continue
		}
		for _, spec := range gen.Specs {
			typeSpec, ok := spec.(*ast.TypeSpec)
			if !ok || typeSpec.Name.Name != name {
				continue
			}
			structType, ok := typeSpec.Type.(*ast.StructType)
			require.Truef(t, ok, "%s must be declared as a struct", name)
			return structType
		}
	}
	require.Failf(t, "%s is not declared in parsed source", name)
	return nil
}

func assertStructFieldType(t *testing.T, st *ast.StructType, name, want string) {
	t.Helper()
	got, ok := structFieldType(st, name)
	require.Truef(t, ok, "%s field must be declared", name)
	assert.Equal(t, want, got, "%s field type must stay source-compatible", name)
}

func hasStructField(st *ast.StructType, name string) bool {
	_, ok := structFieldType(st, name)
	return ok
}

func structFieldType(st *ast.StructType, name string) (string, bool) {
	for _, field := range st.Fields.List {
		for _, fieldName := range field.Names {
			if fieldName.Name == name {
				return exprString(field.Type), true
			}
		}
	}
	return "", false
}

func exprString(expr ast.Expr) string {
	switch typed := expr.(type) {
	case *ast.Ident:
		return typed.Name
	case *ast.ArrayType:
		return "[]" + exprString(typed.Elt)
	case *ast.SelectorExpr:
		return exprString(typed.X) + "." + typed.Sel.Name
	default:
		return ""
	}
}
