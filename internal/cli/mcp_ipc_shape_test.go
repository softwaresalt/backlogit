package cli

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

// TestU4a_PlatformIPCSignaturesSourceShape gates 211.018-T on both platforms,
// regardless of the host running this test. No temporary bodies are frozen.
func TestU4a_PlatformIPCSignaturesSourceShape(t *testing.T) {
	for _, platform := range []struct{ path, constraint string }{
		{"mcp_ipc_windows.go", "//go:build windows"},
		{"mcp_ipc_unix.go", "//go:build !windows"},
	} {
		t.Run(platform.path, func(t *testing.T) {
			file, err := parser.ParseFile(token.NewFileSet(), platform.path, nil, parser.ParseComments)
			if err != nil {
				t.Fatalf("U4a: platform IPC source shape is not declared or cannot be parsed in %s: %v", platform.path, err)
			}
			var constraints []string
			for _, group := range file.Comments {
				for _, comment := range group.List {
					if strings.HasPrefix(comment.Text, "//go:build") {
						if comment.End() >= file.Package {
							t.Errorf("U4a: platform build constraint must precede package in %s", platform.path)
						}
						constraints = append(constraints, comment.Text)
					}
				}
			}
			if len(constraints) != 1 || constraints[0] != platform.constraint {
				t.Errorf("U4a: exact %s constraint is not declared in %s", platform.constraint, platform.path)
			}
			cliDeclarationSignatures(t, file, platform.path, "U4a", []cliShapeSignature{
				{"DeriveMCPDaemonEndpoint", "string,string,string", "string,string,error"},
				{"ListenMCPDaemonIPC", "context.Context,*os.Root,string,string,string", "net.Listener,error"},
				{"DialMCPDaemonIPC", "context.Context,string,string,time.Duration", "net.Conn,error"},
				{"isVerifiedProcessAlive", "int,int64,string", "bool"},
			})
		})
	}
}

type cliShapeSignature struct {
	name, params, results string
}

// cliDeclarationSignatures checks receiver-less, non-generic declarations only.
func cliDeclarationSignatures(t *testing.T, file *ast.File, path, unit string, signatures []cliShapeSignature) {
	t.Helper()
	for _, want := range signatures {
		found := false
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if ok && fn.Name.Name == want.name && fn.Recv == nil && fn.Type.TypeParams == nil &&
				cliShapeFields(file, fn.Type.Params) == want.params &&
				cliShapeFields(file, fn.Type.Results) == want.results {
				found = true
			}
		}
		if !found {
			t.Errorf("%s: receiver-less %s(%s) (%s) is not declared in %s", unit, want.name, want.params, want.results, path)
		}
	}
}

func cliShapeFields(file *ast.File, fields *ast.FieldList) string {
	var parts []string
	if fields != nil {
		for _, field := range fields.List {
			count := len(field.Names)
			if count == 0 {
				count = 1
			}
			for range count {
				parts = append(parts, cliShapeExpr(file, field.Type))
			}
		}
	}
	return strings.Join(parts, ",")
}

func cliShapeExpr(file *ast.File, expr ast.Expr) string {
	switch node := expr.(type) {
	case *ast.Ident:
		return node.Name
	case *ast.StarExpr:
		return "*" + cliShapeExpr(file, node.X)
	case *ast.SelectorExpr:
		id, ok := node.X.(*ast.Ident)
		if !ok {
			break
		}
		for _, imp := range file.Imports {
			path := strings.Trim(imp.Path.Value, `"`)
			switch path {
			case "context", "os", "net", "time", "io":
				alias := path
				if imp.Name != nil {
					alias = imp.Name.Name
				}
				if alias == id.Name {
					return path + "." + node.Sel.Name
				}
			}
		}
	}
	return "<unsupported or unbound shape>"
}
