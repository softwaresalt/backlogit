package scannerdiscipline

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestScaffoldShape asserts the declaration and build scaffolding owned by
// 158.003-T without importing the not-yet-declared analyzer or command.
func TestScaffoldShape(t *testing.T) {
	root := filepath.Join("..", "..", "..", "..")

	assertAnalyzerDeclarationShape(t)
	assertMulticheckerShape(t, filepath.Join(root, "cmd", "faultline-analyze", "main.go"))
	assertModuleShape(t, filepath.Join(root, "go.mod"))
	assertCheckTargetShape(t, filepath.Join(root, "Makefile"))
}

func assertAnalyzerDeclarationShape(t *testing.T) {
	t.Helper()

	packages, err := parser.ParseDir(
		token.NewFileSet(),
		".",
		nil,
		parser.SkipObjectResolution,
	)
	if err != nil {
		t.Errorf("parse scannerdiscipline package source: %v", err)
		return
	}
	pkg := packages["scannerdiscipline"]
	if pkg == nil {
		t.Error("scannerdiscipline analyzer package is not declared")
		return
	}

	files := make([]*ast.File, 0, len(pkg.Files))
	for _, file := range pkg.Files {
		files = append(files, file)
	}

	spec := findScaffoldValue(files, "Analyzer", token.VAR)
	if spec == nil {
		t.Error("scannerdiscipline.Analyzer var is not declared")
		return
	}
	if got := scaffoldExpr(spec.Type); got != "*analysis.Analyzer" {
		t.Errorf("scannerdiscipline.Analyzer type = %s, want *analysis.Analyzer", got)
	}
	if len(spec.Values) != 1 {
		t.Errorf("scannerdiscipline.Analyzer initializer count = %d, want 1", len(spec.Values))
		return
	}
	unary, ok := spec.Values[0].(*ast.UnaryExpr)
	if !ok || unary.Op != token.AND {
		t.Error("scannerdiscipline.Analyzer must be initialized with &analysis.Analyzer{...}")
		return
	}
	literal, ok := unary.X.(*ast.CompositeLit)
	if !ok || scaffoldExpr(literal.Type) != "analysis.Analyzer" {
		t.Error("scannerdiscipline.Analyzer must be initialized with &analysis.Analyzer{...}")
		return
	}

	fields := keyedCompositeFields(literal)
	if got := scaffoldExpr(fields["Name"]); got != `"FL001scannerdiscipline"` {
		t.Errorf("scannerdiscipline.Analyzer Name = %s, want %q", got, "FL001scannerdiscipline")
	}
	if got := scaffoldExpr(fields["Run"]); got != "run" {
		t.Errorf("scannerdiscipline.Analyzer Run = %s, want run", got)
	}
	doc, ok := fields["Doc"].(*ast.BasicLit)
	if !ok || doc.Kind != token.STRING || doc.Value == `""` {
		t.Error("scannerdiscipline.Analyzer Doc must be a non-empty string literal")
	}

	run := findScaffoldFunction(files, "run")
	if run == nil {
		t.Error("scannerdiscipline run function is not declared")
		return
	}
	if run.Recv != nil {
		t.Error("scannerdiscipline run must be receiver-less")
	}
	if got := flattenScaffoldTypes(run.Type.Params); !equalScaffoldStrings(got, []string{"*analysis.Pass"}) {
		t.Errorf("scannerdiscipline run params = %v, want [*analysis.Pass]", got)
	}
	if got := flattenScaffoldTypes(run.Type.Results); !equalScaffoldStrings(got, []string{"any", "error"}) {
		t.Errorf("scannerdiscipline run results = %v, want [any error]", got)
	}
	if !isNoopRunBody(run.Body) {
		t.Error("scannerdiscipline run must remain a declaration-only skeleton returning nil, nil")
	}
}

func assertMulticheckerShape(t *testing.T, path string) {
	t.Helper()

	source, err := os.ReadFile(path)
	if err != nil {
		t.Errorf("cmd/faultline-analyze/main.go is not declared: %v", err)
		return
	}
	file, err := parser.ParseFile(
		token.NewFileSet(),
		path,
		source,
		parser.SkipObjectResolution|parser.ParseComments,
	)
	if err != nil {
		t.Errorf("parse cmd/faultline-analyze/main.go: %v", err)
		return
	}
	if file.Name.Name != "main" {
		t.Errorf("cmd/faultline-analyze package = %s, want main", file.Name.Name)
	}

	mainFn := findScaffoldFunction([]*ast.File{file}, "main")
	if mainFn == nil {
		t.Error("cmd/faultline-analyze main function is not declared")
		return
	}
	calls := findCalls(mainFn, "multichecker.Main")
	if len(calls) != 1 {
		t.Errorf("multichecker.Main call count = %d, want 1", len(calls))
		return
	}
	if len(calls[0].Args) != 1 || scaffoldExpr(calls[0].Args[0]) != "scannerdiscipline.Analyzer" {
		t.Errorf(
			"live multichecker arguments = %v, want [scannerdiscipline.Analyzer]",
			callArgumentShapes(calls[0]),
		)
	}

	text := string(source)
	liveOffset := strings.Index(text, "scannerdiscipline.Analyzer")
	if liveOffset < 0 {
		t.Error("FL001 scannerdiscipline.Analyzer slot is not present")
	}
	previous := liveOffset
	for _, slot := range []struct {
		id  string
		ref string
	}{
		{id: "FL002", ref: "errwrap.Analyzer"},
		{id: "FL003", ref: "failopen.Analyzer"},
		{id: "FL004", ref: "auditsuccess.Analyzer"},
		{id: "FL005", ref: "locktimeout.Analyzer"},
	} {
		offset := commentedSlotOffset(text, slot.id, slot.ref)
		if offset < 0 {
			t.Errorf("%s reserved commented slot for %s is not present", slot.id, slot.ref)
			continue
		}
		if offset <= previous {
			t.Errorf("%s reserved slot is not ordered after the preceding analyzer slot", slot.id)
		}
		previous = offset
	}
}

func assertModuleShape(t *testing.T, path string) {
	t.Helper()

	source, err := os.ReadFile(path)
	if err != nil {
		t.Errorf("read go.mod: %v", err)
		return
	}
	lines := strings.Split(string(source), "\n")

	toolsMatches := 0
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) < 2 || fields[0] != "golang.org/x/tools" {
			continue
		}
		toolsMatches++
		if fields[1] != "v0.39.0" {
			t.Errorf("golang.org/x/tools version = %s, want v0.39.0", fields[1])
		}
		if len(fields) != 2 {
			t.Error("golang.org/x/tools v0.39.0 must be a direct requirement without // indirect")
		}
	}
	if toolsMatches != 1 {
		t.Errorf("direct golang.org/x/tools v0.39.0 requirement count = %d, want 1", toolsMatches)
	}

	if !moduleLineHasVersion(lines, "golang.org/x/text", "v0.32.0") {
		t.Error("golang.org/x/text must remain at the pre-scaffold version v0.32.0")
	}
}

func assertCheckTargetShape(t *testing.T, path string) {
	t.Helper()

	source, err := os.ReadFile(path)
	if err != nil {
		t.Errorf("read Makefile: %v", err)
		return
	}
	lines := strings.Split(strings.ReplaceAll(string(source), "\r\n", "\n"), "\n")

	phonyCheck := false
	checkLine := -1
	for i, line := range lines {
		if strings.HasPrefix(line, ".PHONY:") {
			for _, target := range strings.Fields(strings.TrimPrefix(line, ".PHONY:")) {
				if target == "check" {
					phonyCheck = true
				}
			}
		}
		if strings.TrimSpace(line) == "check:" {
			checkLine = i
		}
	}
	if !phonyCheck {
		t.Error("Makefile .PHONY declaration does not include check")
	}
	if checkLine < 0 {
		t.Error("Makefile check target is not declared")
		return
	}
	if checkLine+2 >= len(lines) {
		t.Error("Makefile check target does not contain both required commands")
		return
	}
	if lines[checkLine+1] != "\tgo build ./cmd/faultline-analyze" {
		t.Errorf("Makefile check build command = %q, want %q", lines[checkLine+1], "\tgo build ./cmd/faultline-analyze")
	}
	if lines[checkLine+2] != "\tgo test ./internal/faultline/analyzer/..." {
		t.Errorf(
			"Makefile check test command = %q, want %q",
			lines[checkLine+2],
			"\tgo test ./internal/faultline/analyzer/...",
		)
	}
}

func findScaffoldValue(files []*ast.File, name string, kind token.Token) *ast.ValueSpec {
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
				for _, candidate := range spec.Names {
					if candidate.Name == name {
						return spec
					}
				}
			}
		}
	}
	return nil
}

func findScaffoldFunction(files []*ast.File, name string) *ast.FuncDecl {
	for _, file := range files {
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if ok && fn.Name.Name == name {
				return fn
			}
		}
	}
	return nil
}

func keyedCompositeFields(literal *ast.CompositeLit) map[string]ast.Expr {
	result := make(map[string]ast.Expr)
	for _, element := range literal.Elts {
		kv, ok := element.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		key, ok := kv.Key.(*ast.Ident)
		if ok {
			result[key.Name] = kv.Value
		}
	}
	return result
}

func isNoopRunBody(body *ast.BlockStmt) bool {
	if body == nil || len(body.List) != 1 {
		return false
	}
	ret, ok := body.List[0].(*ast.ReturnStmt)
	if !ok || len(ret.Results) != 2 {
		return false
	}
	return scaffoldExpr(ret.Results[0]) == "nil" && scaffoldExpr(ret.Results[1]) == "nil"
}

func findCalls(fn *ast.FuncDecl, target string) []*ast.CallExpr {
	var result []*ast.CallExpr
	ast.Inspect(fn.Body, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if ok && scaffoldExpr(call.Fun) == target {
			result = append(result, call)
		}
		return true
	})
	return result
}

func callArgumentShapes(call *ast.CallExpr) []string {
	result := make([]string, 0, len(call.Args))
	for _, arg := range call.Args {
		result = append(result, scaffoldExpr(arg))
	}
	return result
}

func commentedSlotOffset(source, id, ref string) int {
	offset := 0
	for _, line := range strings.Split(source, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "//") &&
			strings.Contains(trimmed, id) &&
			strings.Contains(trimmed, ref) {
			return offset
		}
		offset += len(line) + 1
	}
	return -1
}

func moduleLineHasVersion(lines []string, module, version string) bool {
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[0] == module && fields[1] == version {
			return true
		}
	}
	return false
}

func flattenScaffoldTypes(fields *ast.FieldList) []string {
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
			result = append(result, scaffoldExpr(field.Type))
		}
	}
	return result
}

func scaffoldExpr(expr ast.Expr) string {
	switch value := expr.(type) {
	case nil:
		return ""
	case *ast.Ident:
		return value.Name
	case *ast.BasicLit:
		return value.Value
	case *ast.SelectorExpr:
		return scaffoldExpr(value.X) + "." + value.Sel.Name
	case *ast.StarExpr:
		return "*" + scaffoldExpr(value.X)
	default:
		return "?"
	}
}

func equalScaffoldStrings(left, right []string) bool {
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
