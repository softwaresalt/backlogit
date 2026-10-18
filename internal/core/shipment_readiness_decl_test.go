package core

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// readinessParseDir parses the non-test Go files of dir into a flat file list.
// The harness reads source text only, so it compiles before the declarations
// it pins exist and fails on an assertion rather than on a build error.
func readinessParseDir(t *testing.T, dir string) []*ast.File {
	t.Helper()
	fset := token.NewFileSet()
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	var files []*ast.File
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, parseErr := parser.ParseFile(fset, filepath.Join(dir, name), nil, 0)
		require.NoError(t, parseErr)
		files = append(files, f)
	}
	require.NotEmpty(t, files, "no Go files parsed in %s", dir)
	return files
}

// readinessExprString renders a type expression compactly for signature comparison.
func readinessExprString(e ast.Expr) string {
	switch v := e.(type) {
	case *ast.Ident:
		return v.Name
	case *ast.StarExpr:
		return "*" + readinessExprString(v.X)
	case *ast.SelectorExpr:
		return readinessExprString(v.X) + "." + v.Sel.Name
	default:
		return "?"
	}
}

func readinessFieldTypes(fl *ast.FieldList) []string {
	var out []string
	if fl == nil {
		return out
	}
	for _, f := range fl.List {
		n := len(f.Names)
		if n == 0 {
			n = 1
		}
		for i := 0; i < n; i++ {
			out = append(out, readinessExprString(f.Type))
		}
	}
	return out
}

// TestShipmentReadinessDeclarations pins the permanent readiness API
// declarations by signature only (A-U1 / 187.001-T). It deliberately does not
// pin function bodies, so later successor code stays allowed.
func TestShipmentReadinessDeclarations(t *testing.T) {
	t.Run("predicate", func(t *testing.T) {
		var found *ast.FuncDecl
		for _, f := range readinessParseDir(t, ".") {
			for _, d := range f.Decls {
				if fd, ok := d.(*ast.FuncDecl); ok && fd.Recv == nil && fd.Name.Name == "ShipmentPredecessorShipped" {
					found = fd
				}
			}
		}
		require.NotNil(t, found, "func ShipmentPredecessorShipped is not declared in internal/core")
		require.Equal(t, []string{"context.Context", "*Workspace", "string"}, readinessFieldTypes(found.Type.Params))
		require.Equal(t, []string{"bool", "string", "error"}, readinessFieldTypes(found.Type.Results))
	})

	t.Run("sentinel", func(t *testing.T) {
		var found bool
		for _, f := range readinessParseDir(t, "../errors") {
			for _, d := range f.Decls {
				gd, ok := d.(*ast.GenDecl)
				if !ok || gd.Tok != token.VAR {
					continue
				}
				for _, s := range gd.Specs {
					if vs, ok := s.(*ast.ValueSpec); ok {
						for _, n := range vs.Names {
							if n.Name == "ErrShipmentPredecessorNotShipped" {
								found = true
							}
						}
					}
				}
			}
		}
		require.True(t, found, "ErrShipmentPredecessorNotShipped is not declared in internal/errors")
	})
}
