package compatcorpus

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

type declShapeSignature struct {
	receiver string
	params   []string
	results  []string
}

type declShapeField struct {
	name string
	typ  string
}

// TestCompatcorpusDeclShape asserts the declaration-only contract owned by
// 158.001-T without importing symbols that do not exist yet.
func TestCompatcorpusDeclShape(t *testing.T) {
	files := parseCompatcorpusPackage(t)

	assertNamedStringType(t, files, "Category")
	assertNamedStringType(t, files, "Expectation")

	assertTypedConstants(t, files, "Category", map[string]string{
		"CatMalformedJSON":    `"malformed_json"`,
		"CatTruncatedJSON":    `"truncated_json"`,
		"CatMalformedYAML":    `"malformed_yaml"`,
		"CatTruncatedYAML":    `"truncated_yaml"`,
		"CatDuplicateKey":     `"duplicate_key"`,
		"CatCaseFoldedKey":    `"case_folded_key"`,
		"CatCRLF":             `"crlf"`,
		"CatLF":               `"lf"`,
		"CatOversizedToken":   `"oversized_token"`,
		"CatOldIndexVersion":  `"old_index_version"`,
		"CatWindowsSemantics": `"windows_semantics"`,
	})
	assertTypedConstants(t, files, "Expectation", map[string]string{
		"ExpectRejected":   `"rejected"`,
		"ExpectAccepted":   `"accepted"`,
		"ExpectNormalized": `"normalized"`,
	})

	assertSentinelErrors(t, files, map[string]string{
		"ErrTruncated":         `"compatcorpus: truncated input"`,
		"ErrMalformed":         `"compatcorpus: malformed input"`,
		"ErrDuplicateKey":      `"compatcorpus: duplicate key"`,
		"ErrCaseFoldCollision": `"compatcorpus: case-folded key collision"`,
		"ErrOldVersion":        `"compatcorpus: unsupported/old index version"`,
		"ErrTokenTooLong":      `"compatcorpus: token exceeds bound"`,
		"ErrInvalidPath":       `"compatcorpus: invalid/reserved path"`,
		"ErrUnclosedFront":     `"compatcorpus: unclosed frontmatter fence"`,
	})

	assertStructShape(t, files, "Entry", []declShapeField{
		{name: "ID", typ: "string"},
		{name: "Category", typ: "Category"},
		{name: "Adapter", typ: "string"},
		{name: "Input", typ: "[]byte"},
		{name: "Expect", typ: "Expectation"},
		{name: "WantErr", typ: "error"},
		{name: "WantNormalized", typ: "[]byte"},
	})
	assertStructShape(t, files, "DecodeResult", []declShapeField{
		{name: "Normalized", typ: "[]byte"},
	})
	assertInterfaceShape(t, files, "ParserAdapter", map[string]declShapeSignature{
		"Name": {
			results: []string{"string"},
		},
		"Decode": {
			params:  []string{"context.Context", "[]byte"},
			results: []string{"DecodeResult", "error"},
		},
	})
	assertStructShape(t, files, "Result", []declShapeField{
		{name: "EntryID", typ: "string"},
		{name: "Adapter", typ: "string"},
		{name: "Passed", typ: "bool"},
		{name: "GotErr", typ: "string"},
		{name: "Reason", typ: "string"},
	})
	assertStructShape(t, files, "Report", []declShapeField{
		{name: "SchemaVersion", typ: "string"},
		{name: "Total", typ: "int"},
		{name: "Passed", typ: "int"},
		{name: "Failed", typ: "int"},
		{name: "Deterministic", typ: "bool"},
		{name: "Results", typ: "[]Result"},
	})

	assertFunctionShape(t, files, "Run", declShapeSignature{
		params:  []string{"context.Context", "[]Entry", "map[string]ParserAdapter"},
		results: []string{"Report"},
	})
	assertFunctionShape(t, files, "DefaultCorpus", declShapeSignature{
		results: []string{"[]Entry", "error"},
	})
	assertFunctionShape(t, files, "DefaultAdapters", declShapeSignature{
		results: []string{"map[string]ParserAdapter"},
	})
	assertFunctionShape(t, files, "LoadCorpus", declShapeSignature{
		params:  []string{"fs.FS"},
		results: []string{"[]Entry", "error"},
	})
	assertFunctionShape(t, files, "JSON", declShapeSignature{
		receiver: "Report",
		results:  []string{"[]byte", "error"},
	})
}

func parseCompatcorpusPackage(t *testing.T) []*ast.File {
	t.Helper()

	packages, err := parser.ParseDir(
		token.NewFileSet(),
		".",
		nil,
		parser.SkipObjectResolution,
	)
	if err != nil {
		t.Fatalf("parse compatcorpus package source: %v", err)
	}
	pkg := packages["compatcorpus"]
	if pkg == nil {
		t.Fatal("compatcorpus package declarations are not present")
	}

	files := make([]*ast.File, 0, len(pkg.Files))
	for _, file := range pkg.Files {
		files = append(files, file)
	}
	return files
}

func assertNamedStringType(t *testing.T, files []*ast.File, name string) {
	t.Helper()

	spec := findTypeSpec(files, name)
	if spec == nil {
		t.Errorf("%s type is not declared in internal/faultline/compatcorpus", name)
		return
	}
	if got := declShapeExpr(spec.Type); got != "string" {
		t.Errorf("%s underlying type = %s, want string", name, got)
	}
}

func assertTypedConstants(
	t *testing.T,
	files []*ast.File,
	typeName string,
	expected map[string]string,
) {
	t.Helper()

	actual := collectValueSpecs(files, token.CONST)
	for name, wantValue := range expected {
		spec, ok := actual[name]
		if !ok {
			t.Errorf("%s constant is not declared in internal/faultline/compatcorpus", name)
			continue
		}
		if got := declShapeExpr(spec.typ); got != typeName {
			t.Errorf("%s type = %s, want %s", name, got, typeName)
		}
		if got := declShapeExpr(spec.value); got != wantValue {
			t.Errorf("%s value = %s, want %s", name, got, wantValue)
		}
	}
}

func assertSentinelErrors(t *testing.T, files []*ast.File, expected map[string]string) {
	t.Helper()

	actual := collectValueSpecs(files, token.VAR)
	for name, wantMessage := range expected {
		spec, ok := actual[name]
		if !ok {
			t.Errorf("%s sentinel is not declared in internal/faultline/compatcorpus", name)
			continue
		}
		call, ok := spec.value.(*ast.CallExpr)
		if !ok || declShapeExpr(call.Fun) != "errors.New" || len(call.Args) != 1 {
			t.Errorf("%s must be initialized with errors.New(%s)", name, wantMessage)
			continue
		}
		if got := declShapeExpr(call.Args[0]); got != wantMessage {
			t.Errorf("%s error message = %s, want %s", name, got, wantMessage)
		}
	}
}

func assertStructShape(
	t *testing.T,
	files []*ast.File,
	typeName string,
	expected []declShapeField,
) {
	t.Helper()

	spec := findTypeSpec(files, typeName)
	if spec == nil {
		t.Errorf("%s struct is not declared in internal/faultline/compatcorpus", typeName)
		return
	}
	structType, ok := spec.Type.(*ast.StructType)
	if !ok {
		t.Errorf("%s is %T, want struct", typeName, spec.Type)
		return
	}
	actual := flattenNamedFields(structType.Fields)
	if !equalDeclShapeFields(actual, expected) {
		t.Errorf("%s fields = %v, want %v", typeName, actual, expected)
	}
}

func assertInterfaceShape(
	t *testing.T,
	files []*ast.File,
	typeName string,
	expected map[string]declShapeSignature,
) {
	t.Helper()

	spec := findTypeSpec(files, typeName)
	if spec == nil {
		t.Errorf("%s interface is not declared in internal/faultline/compatcorpus", typeName)
		return
	}
	interfaceType, ok := spec.Type.(*ast.InterfaceType)
	if !ok {
		t.Errorf("%s is %T, want interface", typeName, spec.Type)
		return
	}

	actual := make(map[string]declShapeSignature)
	for _, field := range interfaceType.Methods.List {
		if len(field.Names) != 1 {
			continue
		}
		funcType, ok := field.Type.(*ast.FuncType)
		if !ok {
			continue
		}
		actual[field.Names[0].Name] = declShapeSignature{
			params:  flattenFieldTypes(funcType.Params),
			results: flattenFieldTypes(funcType.Results),
		}
	}
	for name, want := range expected {
		got, ok := actual[name]
		if !ok {
			t.Errorf("%s.%s method is not declared", typeName, name)
			continue
		}
		if !equalStrings(got.params, want.params) || !equalStrings(got.results, want.results) {
			t.Errorf(
				"%s.%s signature = (%v) (%v), want (%v) (%v)",
				typeName,
				name,
				got.params,
				got.results,
				want.params,
				want.results,
			)
		}
	}
	if len(actual) != len(expected) {
		t.Errorf("%s method count = %d, want %d", typeName, len(actual), len(expected))
	}
}

func assertFunctionShape(
	t *testing.T,
	files []*ast.File,
	name string,
	expected declShapeSignature,
) {
	t.Helper()

	fn := findFunction(files, name, expected.receiver)
	if fn == nil {
		if expected.receiver == "" {
			t.Errorf("%s function is not declared in internal/faultline/compatcorpus", name)
		} else {
			t.Errorf("%s.%s method is not declared in internal/faultline/compatcorpus", expected.receiver, name)
		}
		return
	}
	gotParams := flattenFieldTypes(fn.Type.Params)
	gotResults := flattenFieldTypes(fn.Type.Results)
	if !equalStrings(gotParams, expected.params) || !equalStrings(gotResults, expected.results) {
		t.Errorf(
			"%s signature = (%v) (%v), want (%v) (%v)",
			name,
			gotParams,
			gotResults,
			expected.params,
			expected.results,
		)
	}
}

type declShapeValueSpec struct {
	typ   ast.Expr
	value ast.Expr
}

func collectValueSpecs(files []*ast.File, kind token.Token) map[string]declShapeValueSpec {
	result := make(map[string]declShapeValueSpec)
	for _, file := range files {
		for _, decl := range file.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != kind {
				continue
			}
			for _, rawSpec := range gen.Specs {
				spec, ok := rawSpec.(*ast.ValueSpec)
				if !ok {
					continue
				}
				for i, name := range spec.Names {
					var value ast.Expr
					if i < len(spec.Values) {
						value = spec.Values[i]
					} else if len(spec.Values) == 1 {
						value = spec.Values[0]
					}
					result[name.Name] = declShapeValueSpec{
						typ:   spec.Type,
						value: value,
					}
				}
			}
		}
	}
	return result
}

func findTypeSpec(files []*ast.File, name string) *ast.TypeSpec {
	for _, file := range files {
		for _, decl := range file.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.TYPE {
				continue
			}
			for _, rawSpec := range gen.Specs {
				spec, ok := rawSpec.(*ast.TypeSpec)
				if ok && spec.Name.Name == name {
					return spec
				}
			}
		}
	}
	return nil
}

func findFunction(files []*ast.File, name, receiver string) *ast.FuncDecl {
	for _, file := range files {
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Name.Name != name {
				continue
			}
			if functionReceiver(fn) == receiver {
				return fn
			}
		}
	}
	return nil
}

func functionReceiver(fn *ast.FuncDecl) string {
	if fn.Recv == nil {
		return ""
	}
	types := flattenFieldTypes(fn.Recv)
	if len(types) != 1 {
		return ""
	}
	return strings.TrimPrefix(types[0], "*")
}

func flattenNamedFields(fields *ast.FieldList) []declShapeField {
	if fields == nil {
		return nil
	}
	var result []declShapeField
	for _, field := range fields.List {
		typ := declShapeExpr(field.Type)
		for _, name := range field.Names {
			result = append(result, declShapeField{name: name.Name, typ: typ})
		}
	}
	return result
}

func flattenFieldTypes(fields *ast.FieldList) []string {
	if fields == nil {
		return nil
	}
	var result []string
	for _, field := range fields.List {
		count := len(field.Names)
		if count == 0 {
			count = 1
		}
		for range count {
			result = append(result, declShapeExpr(field.Type))
		}
	}
	return result
}

func declShapeExpr(expr ast.Expr) string {
	switch value := expr.(type) {
	case nil:
		return ""
	case *ast.Ident:
		return value.Name
	case *ast.BasicLit:
		return value.Value
	case *ast.SelectorExpr:
		return declShapeExpr(value.X) + "." + value.Sel.Name
	case *ast.StarExpr:
		return "*" + declShapeExpr(value.X)
	case *ast.ArrayType:
		if value.Len == nil {
			return "[]" + declShapeExpr(value.Elt)
		}
		return "[...]" + declShapeExpr(value.Elt)
	case *ast.MapType:
		return "map[" + declShapeExpr(value.Key) + "]" + declShapeExpr(value.Value)
	default:
		return "?"
	}
}

func equalDeclShapeFields(left, right []declShapeField) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

func equalStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}
