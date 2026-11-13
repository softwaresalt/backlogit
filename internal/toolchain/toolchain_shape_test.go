package toolchain

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

func TestU8_ToolchainDeclarationsSourceShape(t *testing.T) {
	t.Run("schema", func(t *testing.T) {
		fset := token.NewFileSet()
		// JSON is not Go source; syntax errors are expected when the file is read.
		file, _ := parser.ParseFile(fset, "../../schemas/workspace-toolchain.schema.json", nil, 0)
		if file == nil {
			t.Error("workspace-toolchain.schema.json is not readable")
		}
		found := false
		fset.Iterate(func(file *token.File) bool {
			if strings.HasSuffix(file.Name(), "workspace-toolchain.schema.json") {
				found = true
				if file.Size() <= 500 || file.LineCount() <= 20 {
					t.Errorf("workspace-toolchain.schema.json must exceed 500 bytes and 20 lines; got %d bytes and %d lines", file.Size(), file.LineCount())
				}
			}
			return true
		})
		if !found {
			t.Error("workspace-toolchain.schema.json token file is not registered")
		}
	})

	t.Run("declarations", func(t *testing.T) {
		file, err := parser.ParseFile(token.NewFileSet(), "types.go", nil, 0)
		if err != nil {
			t.Errorf("toolchain declarations are not available in types.go: %v", err)
			return
		}
		types := map[string]ast.Expr{}
		constants := map[string]bool{}
		functions := map[string]*ast.FuncDecl{}
		for _, decl := range file.Decls {
			switch decl := decl.(type) {
			case *ast.GenDecl:
				for _, spec := range decl.Specs {
					switch spec := spec.(type) {
					case *ast.TypeSpec:
						types[spec.Name.Name] = spec.Type
					case *ast.ValueSpec:
						if decl.Tok == token.CONST {
							for _, value := range spec.Values {
								if value, ok := value.(*ast.BasicLit); ok && value.Kind == token.STRING {
									constants[strings.Trim(value.Value, "`\"")] = true
								}
							}
						}
					}
				}
			case *ast.FuncDecl:
				if decl.Recv == nil {
					functions[decl.Name.Name] = decl
				}
			}
		}

		if typ, ok := types["ToolContainmentClass"].(*ast.Ident); !ok || typ.Name != "string" {
			t.Error("ToolContainmentClass string type is not declared in types.go")
		}
		for _, value := range []string{"workspace-local-pinned", "host-prerequisite", "remote-declared", "built-in-host", "unsupported", "non-conformant", "level-1-workspace-local-pinned"} {
			if !constants[value] {
				t.Errorf("constant %q is not declared in types.go", value)
			}
		}
		structs := map[string][]string{
			"ToolchainManifest":           {"schema_version", "containment_level", "os_container_sandbox_deferred", "workspace_root", "tools", "host_prerequisites"},
			"ToolEntry":                   {"id", "kind", "containment_class", "executable_path", "entrypoint_path", "pinned_version", "sha256", "lockfile_path", "workspace_binding_args", "remote_policy", "reason"},
			"HostPrerequisiteEntry":       {"id", "command", "purpose", "required", "reason"},
			"ToolVerificationFinding":     {"tool_id", "containment_class", "status", "code", "message", "remediation"},
			"ToolchainVerificationReport": {"valid", "containment_level", "os_container_sandbox_deferred", "workspace_root", "tools", "host_prerequisites", "violations"},
		}
		for structName, fields := range structs {
			structure, ok := types[structName].(*ast.StructType)
			if !ok {
				t.Errorf("%s struct is not declared in types.go", structName)
				continue
			}
			for _, name := range fields {
				found := false
				for _, field := range structure.Fields.List {
					if field.Tag == nil || len(field.Names) != 1 {
						continue
					}
					tags := strings.Fields(strings.Trim(field.Tag.Value, "`"))
					jsonTag, yamlTag := false, false
					for _, tag := range tags {
						jsonTag = jsonTag || tag == `json:"`+name+`"`
						yamlTag = yamlTag || tag == `yaml:"`+name+`"`
					}
					if jsonTag && yamlTag {
						found = true
					}
				}
				if !found {
					t.Errorf("%s field with exact json and yaml tags %q is not declared", structName, name)
				}
			}
		}
		signatures := map[string]struct {
			params  []string
			results []string
		}{
			"LoadToolchainManifest":             {[]string{"rootPath:string"}, []string{"*ToolchainManifest", "error"}},
			"DefaultWorkspaceToolchainManifest": {[]string{"rootPath:string"}, []string{"*ToolchainManifest"}},
			"VerifyWorkspaceToolchain":          {[]string{"ctx:context.Context", "rootPath:string", "manifest:*ToolchainManifest"}, []string{"*ToolchainVerificationReport", "error"}},
			"ValidateMCPConfigPinning":          {[]string{"rootPath:string", "mcpJSONPath:string", "manifest:*ToolchainManifest"}, []string{"[]ToolVerificationFinding", "error"}},
		}
		for name, signature := range signatures {
			fn := functions[name]
			if fn == nil || !equalFields(fn.Type.Params, signature.params, true) || !equalFields(fn.Type.Results, signature.results, false) {
				t.Errorf("%s receiver-less function with expected signature is not declared in types.go", name)
			}
		}
	})
}

func equalFields(fields *ast.FieldList, want []string, named bool) bool {
	if fields == nil {
		return len(want) == 0
	}

	var got []string
	for _, field := range fields.List {
		typ := typeName(field.Type)
		if named {
			for _, name := range field.Names {
				got = append(got, name.Name+":"+typ)
			}
		} else {
			count := len(field.Names)
			if count == 0 {
				count = 1
			}
			for range count {
				got = append(got, typ)
			}
		}
	}
	if len(got) != len(want) {
		return false
	}
	for i := range want {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

func typeName(expr ast.Expr) string {
	switch expr := expr.(type) {
	case *ast.Ident:
		return expr.Name
	case *ast.StarExpr:
		return "*" + typeName(expr.X)
	case *ast.ArrayType:
		if expr.Len == nil {
			return "[]" + typeName(expr.Elt)
		}
	case *ast.SelectorExpr:
		return typeName(expr.X) + "." + expr.Sel.Name
	}
	return "<unexpected type>"
}
