package config

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

// shapeExprString renders the small set of type expressions used by the
// TrustAnchor declaration without importing go/printer.
func shapeExprString(e ast.Expr) string {
	switch t := e.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		return "*" + shapeExprString(t.X)
	case *ast.ArrayType:
		return "[]" + shapeExprString(t.Elt)
	case *ast.SelectorExpr:
		return shapeExprString(t.X) + "." + t.Sel.Name
	default:
		return "?"
	}
}

// shapeStructFields returns field name -> (type string, raw tag) for a struct.
func shapeStructFields(t *testing.T, typeName string) map[string][2]string {
	t.Helper()
	fset := token.NewFileSet()
	//nolint:staticcheck // ParseDir is sufficient for a package-local source-shape assertion.
	pkgs, err := parser.ParseDir(fset, ".", nil, 0)
	if err != nil {
		t.Fatalf("parse package source: %v", err)
	}
	pkg, ok := pkgs["config"]
	if !ok {
		t.Fatalf("package config not found in source")
	}
	for _, f := range pkg.Files {
		for _, d := range f.Decls {
			gd, ok := d.(*ast.GenDecl)
			if !ok || gd.Tok != token.TYPE {
				continue
			}
			for _, s := range gd.Specs {
				ts, ok := s.(*ast.TypeSpec)
				if !ok || ts.Name.Name != typeName {
					continue
				}
				st, ok := ts.Type.(*ast.StructType)
				if !ok {
					t.Fatalf("%s is not a struct", typeName)
				}
				out := map[string][2]string{}
				for _, fld := range st.Fields.List {
					tag := ""
					if fld.Tag != nil && len(fld.Tag.Value) >= 2 {
						tag = fld.Tag.Value[1 : len(fld.Tag.Value)-1]
					}
					for _, n := range fld.Names {
						out[n.Name] = [2]string{shapeExprString(fld.Type), tag}
					}
				}
				return out
			}
		}
	}
	return nil
}

func TestTrustAnchorSourceShape(t *testing.T) {
	t.Run("TrustAnchor struct fields and yaml tags", func(t *testing.T) {
		fields := shapeStructFields(t, "TrustAnchor")
		if fields == nil {
			t.Fatalf("TrustAnchor struct is not declared in package config")
		}
		want := map[string][2]string{
			"ID":           {"string", `yaml:"id"`},
			"Role":         {"string", `yaml:"role"`},
			"Algo":         {"string", `yaml:"algo"`},
			"PublicKeyRef": {"string", `yaml:"public_key_ref"`},
			"Fingerprint":  {"string", `yaml:"fingerprint"`},
			"Status":       {"TrustAnchorStatus", `yaml:"status"`},
			"NotBefore":    {"*time.Time", `yaml:"not_before,omitempty"`},
			"NotAfter":     {"*time.Time", `yaml:"not_after,omitempty"`},
		}
		if len(fields) != len(want) {
			t.Errorf("TrustAnchor has %d fields, want %d", len(fields), len(want))
		}
		for name, w := range want {
			got, ok := fields[name]
			if !ok {
				t.Errorf("TrustAnchor missing field %s", name)
				continue
			}
			if got[0] != w[0] {
				t.Errorf("TrustAnchor.%s type = %q, want %q", name, got[0], w[0])
			}
			if got[1] != w[1] {
				t.Errorf("TrustAnchor.%s tag = %q, want %q", name, got[1], w[1])
			}
		}
	})

	t.Run("WorkspaceConfig declares trust_anchors", func(t *testing.T) {
		fields := shapeStructFields(t, "WorkspaceConfig")
		got, ok := fields["TrustAnchors"]
		if !ok {
			t.Fatalf("WorkspaceConfig.TrustAnchors is not declared")
		}
		if got[0] != "[]TrustAnchor" {
			t.Errorf("WorkspaceConfig.TrustAnchors type = %q, want []TrustAnchor", got[0])
		}
		if got[1] != `yaml:"trust_anchors,omitempty"` {
			t.Errorf("WorkspaceConfig.TrustAnchors tag = %q, want yaml:\"trust_anchors,omitempty\"", got[1])
		}
	})
}
