package cli

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

func parseSourceFile(t *testing.T, path string) *ast.File {
	t.Helper()
	f, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ParseComments)
	if err != nil {
		t.Errorf("parse %s: %v", path, err)
		return nil
	}
	return f
}

// TestU5a_DaemonAndShimDeclarationsSourceShape gates 211.019-T metadata and
// signatures only, allowing U5/U6 to replace bodies without changing this gate.
func TestU5a_DaemonAndShimDeclarationsSourceShape(t *testing.T) {
	const path = "mcp_daemon.go"
	file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
	if err != nil {
		t.Fatalf("U5a: daemon source shape cannot be parsed in %s: %v", path, err)
	}
	for _, name := range []string{"MCPDaemonInfo", "MCPDaemonStatus"} {
		var structure *ast.StructType
		for _, decl := range file.Decls {
			group, ok := decl.(*ast.GenDecl)
			if !ok || group.Tok != token.TYPE {
				continue
			}
			for _, spec := range group.Specs {
				typ, ok := spec.(*ast.TypeSpec)
				if ok && typ.Name.Name == name && !typ.Assign.IsValid() && typ.TypeParams == nil {
					structure, _ = typ.Type.(*ast.StructType)
				}
			}
		}
		if structure == nil {
			t.Errorf("U5a: %s defined struct is not declared in %s", name, path)
			continue
		}
		fields := []struct{ name, typ, tag string }{
			{"IPCTransport", "string", `json:"ipc_transport,omitempty"`},
			{"Endpoint", "string", `json:"endpoint,omitempty"`},
			{"ActiveSessions", "int", `json:"active_sessions"`},
			{"ProtocolVersion", "string", `json:"protocol_version,omitempty"`},
		}
		for _, want := range fields {
			found := false
			for _, field := range structure.Fields.List {
				for _, fieldName := range field.Names {
					if fieldName.Name != want.name || cliShapeExpr(file, field.Type) != want.typ || field.Tag == nil {
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
				t.Errorf("U5a: %s.%s %s with %s is not declared in %s", name, want.name, want.typ, want.tag, path)
			}
		}
	}
	cliDeclarationSignatures(t, file, path, "U5a", []cliShapeSignature{
		{"VerifyMCPDaemonAttachment", "MCPDaemonInfo,string,string,string,string", "error"},
		{"RunMCPDaemon", "context.Context,string,bool,time.Duration", "error"},
		{"PrewarmMCPDaemon", "context.Context,string,bool,time.Duration", "MCPDaemonStatus,error"},
		{"RunMCPSessionShim", "context.Context,string,bool,io.Reader,io.Writer", "error"},
	})
}

func structJSONTags(f *ast.File, name string) (map[string]bool, bool) {
	for _, decl := range f.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok || gd.Tok != token.TYPE {
			continue
		}
		for _, spec := range gd.Specs {
			ts, ok := spec.(*ast.TypeSpec)
			if !ok || ts.Name.Name != name {
				continue
			}
			st, ok := ts.Type.(*ast.StructType)
			if !ok {
				return nil, false
			}
			tags := map[string]bool{}
			for _, field := range st.Fields.List {
				if field.Tag == nil {
					continue
				}
				raw, err := strconv.Unquote(field.Tag.Value)
				if err != nil {
					continue
				}
				tag := reflect.StructTag(raw).Get("json")
				if i := indexComma(tag); i >= 0 {
					tag = tag[:i]
				}
				tags[tag] = true
			}
			return tags, true
		}
	}
	return nil, false
}

func indexComma(s string) int {
	for i := range s {
		if s[i] == ',' {
			return i
		}
	}
	return -1
}

func findFunc(f *ast.File, name string) *ast.FuncDecl {
	for _, decl := range f.Decls {
		if fd, ok := decl.(*ast.FuncDecl); ok && fd.Recv == nil && fd.Name.Name == name {
			return fd
		}
	}
	return nil
}

func TestU1_MCPDaemonDeclarationsSourceShape(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	cliDir := filepath.Dir(thisFile)
	errorsPath := filepath.Join(cliDir, "..", "errors", "errors.go")
	daemonPath := filepath.Join(cliDir, "mcp_daemon.go")

	t.Run("sentinel", func(t *testing.T) {
		f := parseSourceFile(t, errorsPath)
		if f == nil {
			return
		}
		found := false
		ast.Inspect(f, func(n ast.Node) bool {
			if vs, ok := n.(*ast.ValueSpec); ok {
				for _, id := range vs.Names {
					if id.Name == "ErrDaemonAlreadyRunning" {
						found = true
					}
				}
			}
			return true
		})
		if !found {
			t.Errorf("internal/errors/errors.go must declare ErrDaemonAlreadyRunning")
		}
	})

	f := parseSourceFile(t, daemonPath)
	if f == nil {
		return
	}

	structs := map[string][]string{
		"MCPDaemonInfo":       {"pid", "process_start_time", "executable_path", "started_at", "workspace_root", "storage_root", "version", "lock_path"},
		"MCPDaemonStatus":     {"running", "pid", "started_at", "workspace_root", "storage_root", "version", "lock_path"},
		"MCPDaemonStopResult": {"stopped", "running", "pid", "workspace_root", "storage_root", "lock_path"},
	}
	for name, want := range structs {
		t.Run(name, func(t *testing.T) {
			tags, found := structJSONTags(f, name)
			if !found {
				t.Errorf("mcp_daemon.go must declare struct %s", name)
				return
			}
			for _, tag := range want {
				if !tags[tag] {
					t.Errorf("%s missing json tag %q", name, tag)
				}
			}
		})
	}

	funcs := map[string]struct{ params, results int }{
		"AcquireMCPDaemonLock":   {3, 2},
		"InspectMCPDaemonStatus": {1, 2},
		"StopMCPDaemon":          {2, 2},
	}
	for name, sig := range funcs {
		t.Run(name, func(t *testing.T) {
			fd := findFunc(f, name)
			if fd == nil {
				t.Errorf("mcp_daemon.go must declare func %s", name)
				return
			}
			if got := fd.Type.Params.NumFields(); got != sig.params {
				t.Errorf("%s params = %d, want %d", name, got, sig.params)
			}
			if fd.Type.Results == nil || fd.Type.Results.NumFields() != sig.results {
				t.Errorf("%s must return %d values", name, sig.results)
				return
			}
			last := fd.Type.Results.List[len(fd.Type.Results.List)-1].Type
			if id, ok := last.(*ast.Ident); !ok || id.Name != "error" {
				t.Errorf("%s last result must be error", name)
			}
		})
	}
}
