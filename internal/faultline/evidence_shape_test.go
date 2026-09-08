package faultline

// TestU4aDeclSignatureShape is the permanent regression guard for the
// internal/faultline contract function signatures (156.004-T + 156.006-T).
// Originally a red-deliverable harness, it is now GREEN: all six required
// contract functions (Canonical, DecodeAndValidate, Validate, RegisterFamily,
// knownFamilies, freeze) exist in evidence.go with full implementations.

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
)

// TestU4aDeclSignatureShape parses evidence.go and asserts that each of the six
// contract functions/methods exists AND has the expected signature shape:
// receiver (for methods), parameter types, and result types. It is an AST text
// check (not a type-checker), so it compares rendered type-expression strings
// and field counts. It is GREEN: all six functions exist with full
// implementations as delivered by 156.006-T.
func TestU4aDeclSignatureShape(t *testing.T) {
	src := evidenceSourcePath(t)

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, src, nil, parser.SkipObjectResolution)
	if err != nil {
		t.Fatalf("parse %s: %v", src, err)
	}

	var decls []*ast.FuncDecl
	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok {
			decls = append(decls, fn)
		}
	}

	// sigSpec describes the expected shape of one required declaration. recv is
	// "" for a package-level func or the receiver base type for a method.
	type sigSpec struct {
		recv    string
		params  []string
		results []string
	}
	specs := map[string]sigSpec{
		"Canonical":         {recv: "EvidenceArtifact", params: nil, results: []string{"[]byte", "error"}},
		"DecodeAndValidate": {recv: "", params: []string{"[]byte"}, results: []string{"EvidenceArtifact", "FamilyPayload", "error"}},
		"Validate":          {recv: "", params: []string{"EvidenceArtifact"}, results: []string{"error"}},
		"RegisterFamily":    {recv: "", params: []string{"int", "string", "func() FamilyPayload"}, results: []string{"error"}},
		"knownFamilies":     {recv: "", params: []string{"int"}, results: []string{"[]string"}},
		"freeze":            {recv: "", params: nil, results: nil},
	}

	// Stable iteration for deterministic failure output.
	names := make([]string, 0, len(specs))
	for name := range specs {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		spec := specs[name]
		fn := findFuncDecl(decls, name, spec.recv)
		if fn == nil {
			t.Errorf("evidence.go is missing required contract declaration %q with receiver %q; "+
				"these are implemented by 156.006-T", name, spec.recv)
			continue
		}
		gotParams := flattenFieldTypes(fn.Type.Params)
		if !equalStrings(gotParams, spec.params) {
			t.Errorf("%q params = %v, want %v", name, gotParams, spec.params)
		}
		gotResults := flattenFieldTypes(fn.Type.Results)
		if !equalStrings(gotResults, spec.results) {
			t.Errorf("%q results = %v, want %v", name, gotResults, spec.results)
		}
	}
}

// findFuncDecl returns the FuncDecl matching name and receiver base type (recv
// == "" matches a package-level function; a non-empty recv matches a method
// whose single receiver base type renders to recv).
func findFuncDecl(decls []*ast.FuncDecl, name, recv string) *ast.FuncDecl {
	for _, fn := range decls {
		if fn.Name.Name != name {
			continue
		}
		got := ""
		if fn.Recv != nil {
			if rt := flattenFieldTypes(fn.Recv); len(rt) == 1 {
				// Strip a leading pointer marker so method receivers match by base type.
				got = strings.TrimPrefix(rt[0], "*")
			}
		}
		if got == recv {
			return fn
		}
	}
	return nil
}

// flattenFieldTypes renders each field's type to a string, expanding fields
// that declare multiple names (e.g. `a, b int`) into one entry per name.
func flattenFieldTypes(fl *ast.FieldList) []string {
	var out []string
	if fl == nil {
		return out
	}
	for _, f := range fl.List {
		n := len(f.Names)
		if n == 0 {
			n = 1
		}
		ts := exprString(f.Type)
		for i := 0; i < n; i++ {
			out = append(out, ts)
		}
	}
	return out
}

// exprString renders the subset of AST type expressions used by the contract
// signatures to a canonical string form.
func exprString(e ast.Expr) string {
	switch t := e.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		return "*" + exprString(t.X)
	case *ast.ArrayType:
		if t.Len == nil {
			return "[]" + exprString(t.Elt)
		}
		return "[...]" + exprString(t.Elt)
	case *ast.SelectorExpr:
		return exprString(t.X) + "." + t.Sel.Name
	case *ast.Ellipsis:
		return "..." + exprString(t.Elt)
	case *ast.MapType:
		return "map[" + exprString(t.Key) + "]" + exprString(t.Value)
	case *ast.InterfaceType:
		return "interface{}"
	case *ast.FuncType:
		params := flattenFieldTypes(t.Params)
		s := "func(" + strings.Join(params, ", ") + ")"
		results := flattenFieldTypes(t.Results)
		switch len(results) {
		case 0:
			return s
		case 1:
			return s + " " + results[0]
		default:
			return s + " (" + strings.Join(results, ", ") + ")"
		}
	default:
		return "?"
	}
}

// equalStrings reports whether two string slices are element-wise equal.
func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// evidenceSourcePath resolves the absolute path to evidence.go relative to this
// test file so the harness is independent of the working directory.
func evidenceSourcePath(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller(0) failed to locate the test file")
	}
	return filepath.Join(filepath.Dir(thisFile), "evidence.go")
}
