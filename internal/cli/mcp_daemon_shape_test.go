package cli

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
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
