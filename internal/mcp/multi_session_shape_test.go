package mcp

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

// TestU1_MultiSessionCoordinatorSourceShape gates declarations, not stub bodies.
func TestU1_MultiSessionCoordinatorSourceShape(t *testing.T) {
	assertMultiSessionCoordinatorSourceShape(t)
}

// TestFeature211_MultiSessionCoordinatorSourceShape is the feature-root U1 witness.
// Later-wave behavior remains governed by each task's own harness.
func TestFeature211_MultiSessionCoordinatorSourceShape(t *testing.T) {
	assertMultiSessionCoordinatorSourceShape(t)
}

func assertMultiSessionCoordinatorSourceShape(t *testing.T) {
	t.Helper()

	scenarios := []struct {
		name  string
		path  string
		check func(*testing.T, *ast.File)
	}{
		{
			name: "daemon_and_admission_sentinels",
			path: "../errors/errors.go",
			check: func(t *testing.T, file *ast.File) {
				t.Helper()
				sentinels := []string{
					"ErrMCPMutationQueueFull", "ErrMCPAdmissionTimeout",
					"ErrDaemonWorkspaceMismatch", "ErrDaemonVersionMismatch",
					"ErrDaemonBinaryMissing",
				}
				errorPackage := ""
				for _, imp := range file.Imports {
					if imp.Path.Value == `"errors"` {
						errorPackage = "errors"
						if imp.Name != nil {
							errorPackage = imp.Name.Name
						}
					}
				}
				found := make(map[string]bool, len(sentinels))
				for _, decl := range file.Decls {
					group, ok := decl.(*ast.GenDecl)
					if !ok || group.Tok != token.VAR {
						continue
					}
					for _, spec := range group.Specs {
						value, ok := spec.(*ast.ValueSpec)
						if !ok {
							continue
						}
						for i, name := range value.Names {
							if i >= len(value.Values) {
								continue
							}
							call, ok := value.Values[i].(*ast.CallExpr)
							if !ok || len(call.Args) != 1 {
								continue
							}
							if errorPackage != "" && multiSessionShapeExpr(call.Fun) == errorPackage+".New" {
								found[name.Name] = true
							}
						}
					}
				}
				for _, name := range sentinels {
					if !found[name] {
						t.Errorf("%s sentinel initialized with errors.New is not declared in internal/errors/errors.go", name)
					}
				}
			},
		},
		{
			name: "coordinator_types_and_busy_json_tags",
			path: "coordinator.go",
			check: func(t *testing.T, file *ast.File) {
				t.Helper()
				types := make(map[string]*ast.TypeSpec, 4)
				for _, decl := range file.Decls {
					group, ok := decl.(*ast.GenDecl)
					if !ok || group.Tok != token.TYPE {
						continue
					}
					for _, spec := range group.Specs {
						if typ, ok := spec.(*ast.TypeSpec); ok {
							types[typ.Name.Name] = typ
						}
					}
				}
				for _, name := range []string{"OperationKind", "CoordinatorConfig", "BusyErrorResponse", "OperationCoordinator"} {
					typ := types[name]
					if typ == nil || typ.Assign.IsValid() {
						t.Errorf("%s defined type is not declared in coordinator.go", name)
						continue
					}
					if name != "OperationKind" {
						if _, ok := typ.Type.(*ast.StructType); !ok {
							t.Errorf("%s is not a struct in coordinator.go", name)
						}
					}
				}
				busy := types["BusyErrorResponse"]
				if busy == nil {
					return
				}
				structure, ok := busy.Type.(*ast.StructType)
				if !ok {
					return
				}
				fields := []struct {
					name, typ, tag string
				}{
					{"Error", "string", `json:"error"`},
					{"Code", "string", `json:"code"`},
					{"Message", "string", `json:"message"`},
					{"Retryable", "bool", `json:"retryable"`},
					{"Remediation", "string", `json:"remediation"`},
				}
				for _, want := range fields {
					found := false
					for _, field := range structure.Fields.List {
						for _, name := range field.Names {
							if name.Name != want.name || multiSessionShapeExpr(field.Type) != want.typ || field.Tag == nil {
								continue
							}
							for _, tag := range strings.Fields(strings.Trim(field.Tag.Value, "`")) {
								if tag == want.tag {
									found = true
								}
							}
						}
					}
					if !found {
						t.Errorf("BusyErrorResponse.%s %s with %s is not declared in coordinator.go", want.name, want.typ, want.tag)
					}
				}
			},
		},
		{
			name: "exact_coordinator_signatures",
			path: "coordinator.go",
			check: func(t *testing.T, file *ast.File) {
				t.Helper()
				signatures := []struct {
					name, receiver, params, results string
				}{
					{"NewOperationCoordinator", "", "CoordinatorConfig", "*OperationCoordinator"},
					{"ClassifyToolOperation", "", "string,map[string]any", "OperationKind"},
					{"Execute", "*OperationCoordinator", "context.Context,*mcpserver.MCPServer,string,mcplib.CallToolRequest,mcpserver.ToolHandlerFunc", "*mcplib.CallToolResult,error"},
					{"Close", "*OperationCoordinator", "", "error"},
				}
				for _, want := range signatures {
					found := false
					for _, decl := range file.Decls {
						fn, ok := decl.(*ast.FuncDecl)
						if !ok || fn.Name.Name != want.name || fn.Type.TypeParams != nil {
							continue
						}
						lists := []*ast.FieldList{fn.Recv, fn.Type.Params, fn.Type.Results}
						got := make([]string, 0, len(lists))
						for _, list := range lists {
							var builder strings.Builder
							if list != nil {
								for _, field := range list.List {
									count := len(field.Names)
									if count == 0 {
										count = 1
									}
									for range count {
										if builder.Len() > 0 {
											builder.WriteString(",")
										}
										builder.WriteString(multiSessionShapeExpr(field.Type))
									}
								}
							}
							got = append(got, builder.String())
						}
						if got[0] == want.receiver && got[1] == want.params && got[2] == want.results {
							found = true
						}
					}
					if !found {
						t.Errorf("%s receiver=%q (%s) (%s) is not declared in coordinator.go", want.name, want.receiver, want.params, want.results)
					}
				}
			},
		},
	}
	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			file, err := parser.ParseFile(token.NewFileSet(), scenario.path, nil, 0)
			if err != nil {
				// Missing source is an assertion RED, never an undeclared Go symbol.
				t.Errorf("%s source shape is not declared or cannot be parsed in %s: %v", scenario.name, scenario.path, err)
				file = &ast.File{}
			}
			scenario.check(t, file)
		})
	}
}

func multiSessionShapeExpr(expr ast.Expr) string {
	switch node := expr.(type) {
	case *ast.Ident:
		return node.Name
	case *ast.StarExpr:
		return "*" + multiSessionShapeExpr(node.X)
	case *ast.SelectorExpr:
		return multiSessionShapeExpr(node.X) + "." + node.Sel.Name
	case *ast.MapType:
		return "map[" + multiSessionShapeExpr(node.Key) + "]" + multiSessionShapeExpr(node.Value)
	default:
		return "<unsupported shape>"
	}
}

// TestU3a_MultiSessionStreamDispatcherSourceShape gates 211.017-T declarations.
// Bodies deliberately remain unconstrained for the Wave 6 behavior successor.
func TestU3a_MultiSessionStreamDispatcherSourceShape(t *testing.T) {
	file, err := parser.ParseFile(token.NewFileSet(), "session_mux.go", nil, 0)
	if err != nil {
		t.Fatalf("U3a: stream dispatcher source shape is not declared or cannot be parsed in session_mux.go: %v", err)
	}
	foundStruct, foundAssertion := false, false
	for _, decl := range file.Decls {
		group, ok := decl.(*ast.GenDecl)
		if !ok {
			continue
		}
		for _, spec := range group.Specs {
			if typ, ok := spec.(*ast.TypeSpec); ok && group.Tok == token.TYPE && typ.Name.Name == "MultiSessionConn" {
				_, foundStruct = typ.Type.(*ast.StructType)
				foundStruct = foundStruct && !typ.Assign.IsValid() && typ.TypeParams == nil
			}
			value, ok := spec.(*ast.ValueSpec)
			if !ok || group.Tok != token.VAR || len(value.Names) != 1 || value.Names[0].Name != "_" ||
				len(value.Values) != 1 || u3aShapeExpr(file, value.Type) != "mcpserver.ClientSession" {
				continue
			}
			call, ok := value.Values[0].(*ast.CallExpr)
			if !ok || len(call.Args) != 1 || call.Ellipsis.IsValid() {
				continue
			}
			conversion, ok := call.Fun.(*ast.ParenExpr)
			nilArg, nilOK := call.Args[0].(*ast.Ident)
			if ok && u3aShapeExpr(file, conversion.X) == "*MultiSessionConn" && nilOK && nilArg.Name == "nil" {
				foundAssertion = true
			}
		}
	}
	if !foundStruct {
		t.Error("U3a: MultiSessionConn defined struct is not declared in session_mux.go")
	}
	if !foundAssertion {
		t.Error("U3a: var _ mcpserver.ClientSession = (*MultiSessionConn)(nil) is not declared in session_mux.go")
	}
	signatures := []struct{ name, receiver, params, results string }{
		{"SessionID", "*MultiSessionConn", "", "string"},
		{"NotificationChannel", "*MultiSessionConn", "", "chan mcplib.JSONRPCNotification"},
		{"Initialize", "*MultiSessionConn", "", ""},
		{"Initialized", "*MultiSessionConn", "", "bool"},
		{"ServeSessionStream", "*Server", "context.Context,string,io.Reader,io.Writer", "error"},
		{"ActiveSessionCount", "*Server", "", "int"},
		{"CloseAllSessions", "*Server", "", "error"},
	}
	for _, want := range signatures {
		found := false
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if ok && fn.Name.Name == want.name && fn.Type.TypeParams == nil &&
				u3aShapeFields(file, fn.Recv) == want.receiver &&
				u3aShapeFields(file, fn.Type.Params) == want.params &&
				u3aShapeFields(file, fn.Type.Results) == want.results {
				found = true
			}
		}
		if !found {
			t.Errorf("U3a: %s receiver=%q (%s) (%s) is not declared in session_mux.go", want.name, want.receiver, want.params, want.results)
		}
	}
}

func u3aShapeFields(file *ast.File, fields *ast.FieldList) string {
	var parts []string
	if fields != nil {
		for _, field := range fields.List {
			count := len(field.Names)
			if count == 0 {
				count = 1
			}
			for range count {
				parts = append(parts, u3aShapeExpr(file, field.Type))
			}
		}
	}
	return strings.Join(parts, ",")
}

func u3aShapeExpr(file *ast.File, expr ast.Expr) string {
	switch node := expr.(type) {
	case *ast.Ident:
		return node.Name
	case *ast.StarExpr:
		return "*" + u3aShapeExpr(file, node.X)
	case *ast.ChanType:
		if node.Dir == ast.SEND|ast.RECV {
			return "chan " + u3aShapeExpr(file, node.Value)
		}
	case *ast.SelectorExpr:
		id, ok := node.X.(*ast.Ident)
		if !ok {
			break
		}
		for _, imp := range file.Imports {
			path := strings.Trim(imp.Path.Value, `"`)
			canonical := map[string]string{
				"context": "context", "io": "io",
				"github.com/mark3labs/mcp-go/mcp":    "mcplib",
				"github.com/mark3labs/mcp-go/server": "mcpserver",
			}[path]
			alias := path[strings.LastIndex(path, "/")+1:]
			if imp.Name != nil {
				alias = imp.Name.Name
			}
			if canonical != "" && alias == id.Name {
				return canonical + "." + node.Sel.Name
			}
		}
	}
	return "<unsupported or unbound shape>"
}
