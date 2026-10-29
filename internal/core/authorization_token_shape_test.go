package core

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

// authorizationTokenShape lists every struct the operator authorization token
// declares (170.001-T) and the exact JSON tag on each field, per the 170.011-T
// request-identity-digest contract. The harness inspects package source only,
// so it compiles whether or not the declaration exists.
var authorizationTokenShape = map[string][]struct{ field, goType, tag string }{
	"AuthorizationToken": {
		{"Header", "AuthorizationTokenHeader", "header"},
		{"Claims", "AuthorizationTokenClaims", "claims"},
		{"Binding", "AuthorizationTokenBinding", "binding"},
		{"Signature", "string", "signature"},
	},
	"AuthorizationTokenHeader": {
		{"Algo", "string", "alg"},
		{"KeyID", "string", "key_id"},
	},
	"AuthorizationTokenClaims": {
		{"NotBefore", "time.Time", "not_before"},
		{"NotAfter", "time.Time", "not_after"},
		{"Nonce", "string", "nonce"},
	},
	"AuthorizationTokenBinding": {
		{"ShipmentID", "string", "shipment_id"},
		{"RequestIdentityDigest", "string", "request_identity_digest"},
	},
}

// TestAuthorizationTokenSourceShape asserts, over the package source, that the
// authorization token structs and each field's exact JSON tag are declared.
func TestAuthorizationTokenSourceShape(t *testing.T) {
	structs := parseTokenShapeStructDecls(t)
	for name, fields := range authorizationTokenShape {
		t.Run(name, func(t *testing.T) {
			st, ok := structs[name]
			if !ok {
				t.Fatalf("struct %s is not declared in internal/core", name)
			}
			got := map[string]*ast.Field{}
			for _, f := range st.Fields.List {
				for _, n := range f.Names {
					got[n.Name] = f
				}
			}
			for _, want := range fields {
				f, ok := got[want.field]
				if !ok {
					t.Errorf("%s.%s is not declared", name, want.field)
					continue
				}
				if typ := tokenShapeTypeString(f.Type); typ != want.goType {
					t.Errorf("%s.%s type = %q, want %q", name, want.field, typ, want.goType)
				}
				if f.Tag == nil {
					t.Errorf("%s.%s has no struct tag", name, want.field)
					continue
				}
				raw, err := strconv.Unquote(f.Tag.Value)
				if err != nil {
					t.Errorf("%s.%s tag unquote: %v", name, want.field, err)
					continue
				}
				if tag := reflect.StructTag(raw).Get("json"); tag != want.tag {
					t.Errorf("%s.%s json tag = %q, want %q", name, want.field, tag, want.tag)
				}
			}
			if len(st.Fields.List) != len(fields) {
				t.Errorf("%s declares %d fields, want exactly %d", name, len(st.Fields.List), len(fields))
			}
		})
	}
}

func parseTokenShapeStructDecls(t *testing.T) map[string]*ast.StructType {
	t.Helper()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read package dir: %v", err)
	}
	fset := token.NewFileSet()
	out := map[string]*ast.StructType{}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, filepath.Join(".", name), nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		for _, decl := range file.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.TYPE {
				continue
			}
			for _, spec := range gen.Specs {
				ts, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}
				if st, ok := ts.Type.(*ast.StructType); ok {
					out[ts.Name.Name] = st
				}
			}
		}
	}
	return out
}

func tokenShapeTypeString(e ast.Expr) string {
	switch v := e.(type) {
	case *ast.Ident:
		return v.Name
	case *ast.SelectorExpr:
		return tokenShapeTypeString(v.X) + "." + v.Sel.Name
	case *ast.StarExpr:
		return "*" + tokenShapeTypeString(v.X)
	}
	return ""
}
