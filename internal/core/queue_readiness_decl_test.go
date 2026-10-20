package core

import (
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fieldTypeStrings expands a field list into one type string per declared name.
func fieldTypeStrings(list *ast.FieldList) []string {
	if list == nil {
		return nil
	}
	var out []string
	for _, f := range list.List {
		n := len(f.Names)
		if n < 1 {
			n = 1
		}
		for i := 0; i < n; i++ {
			out = append(out, types.ExprString(f.Type))
		}
	}
	return out
}

// TestQueueReadinessDeclarations asserts the declaration shape of the queue
// readiness resolver field and workspace entry point without naming them at
// compile time.
func TestQueueReadinessDeclarations(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}
	fset := token.NewFileSet()
	var files []*ast.File
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, perr := parser.ParseFile(fset, filepath.Join(".", name), nil, 0)
		if perr != nil {
			t.Fatalf("parse %s: %v", name, perr)
		}
		files = append(files, f)
	}
	if len(files) == 0 {
		t.Fatal("no non-test files parsed")
	}

	t.Run("QueryQueueForWorkspace", func(t *testing.T) {
		var fn *ast.FuncDecl
		for _, f := range files {
			for _, d := range f.Decls {
				if fd, ok := d.(*ast.FuncDecl); ok && fd.Recv == nil && fd.Name.Name == "QueryQueueForWorkspace" {
					fn = fd
				}
			}
		}
		if fn == nil {
			t.Fatal("QueryQueueForWorkspace: missing")
		}
		got := "(" + strings.Join(fieldTypeStrings(fn.Type.Params), ", ") + ") (" + strings.Join(fieldTypeStrings(fn.Type.Results), ", ") + ")"
		want := "(context.Context, *Workspace, *QueueFilter) (*QueueView, error)"
		if got != want {
			t.Fatalf("QueryQueueForWorkspace: wrong signature: got %s, want %s", got, want)
		}
	})

	t.Run("shipmentPredecessorResolver", func(t *testing.T) {
		var field *ast.Field
		for _, f := range files {
			for _, d := range f.Decls {
				gd, ok := d.(*ast.GenDecl)
				if !ok {
					continue
				}
				for _, s := range gd.Specs {
					ts, ok := s.(*ast.TypeSpec)
					if !ok || ts.Name.Name != "QueueFilter" {
						continue
					}
					st, ok := ts.Type.(*ast.StructType)
					if !ok {
						continue
					}
					for _, fl := range st.Fields.List {
						for _, n := range fl.Names {
							if n.Name == "shipmentPredecessorResolver" {
								field = fl
							}
						}
					}
				}
			}
		}
		if field == nil {
			t.Fatal("QueueFilter.shipmentPredecessorResolver: missing")
		}
		ft, ok := field.Type.(*ast.FuncType)
		if !ok {
			t.Fatalf("QueueFilter.shipmentPredecessorResolver: wrong signature: got %s, want func(context.Context, string) (bool, string, error)", types.ExprString(field.Type))
		}
		got := "func(" + strings.Join(fieldTypeStrings(ft.Params), ", ") + ") (" + strings.Join(fieldTypeStrings(ft.Results), ", ") + ")"
		want := "func(context.Context, string) (bool, string, error)"
		if got != want {
			t.Fatalf("QueueFilter.shipmentPredecessorResolver: wrong signature: got %s, want %s", got, want)
		}
	})
}
