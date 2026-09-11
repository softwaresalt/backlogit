package main

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path"
	"strconv"
	"testing"
)

const multicheckerPackagePath = "golang.org/x/tools/go/analysis/multichecker"

var (
	errMulticheckerCallCount = errors.New("unexpected multichecker.Main call count")
	errMulticheckerPlacement = errors.New("multichecker.Main is not a direct top-level statement in main")
	errAnalyzerEnumeration   = errors.New("multichecker.Main does not directly enumerate five analyzers")
	errAnalyzerOrder         = errors.New("multichecker.Main analyzer order differs from the frozen order")
)

type selectorReference struct {
	packagePath  string
	selectorName string
	analyzerName string
}

func TestMulticheckerEnumeratesAll(t *testing.T) {
	file, err := parser.ParseFile(token.NewFileSet(), "main.go", nil, parser.SkipObjectResolution)
	if err != nil {
		t.Fatalf("parse main.go: %v", err)
	}

	if err := validateMulticheckerWiring(file); err != nil {
		t.Fatal(err)
	}
}

func TestValidateMulticheckerWiringAcceptsImportAliases(t *testing.T) {
	source := `package main

import (
	check "golang.org/x/tools/go/analysis/multichecker"
	audit "github.com/softwaresalt/backlogit/internal/faultline/analyzer/auditsuccess"
	wrap "github.com/softwaresalt/backlogit/internal/faultline/analyzer/errwrap"
	open "github.com/softwaresalt/backlogit/internal/faultline/analyzer/failopen"
	lock "github.com/softwaresalt/backlogit/internal/faultline/analyzer/locktimeout"
	scan "github.com/softwaresalt/backlogit/internal/faultline/analyzer/scannerdiscipline"
)

func main() {
	check.Main(
		scan.Analyzer,
		wrap.Analyzer,
		open.Analyzer,
		audit.Analyzer,
		lock.Analyzer,
	)
}`

	file := parseTestSource(t, source)
	if err := validateMulticheckerWiring(file); err != nil {
		t.Fatalf("validate aliased imports: %v", err)
	}
}

func TestValidateMulticheckerWiringRejectsInvalidShapes(t *testing.T) {
	tests := []struct {
		name    string
		source  string
		wantErr error
	}{
		{
			name: "nested closure",
			source: validTestProgram(`func() {
		multichecker.Main(
			scannerdiscipline.Analyzer,
			errwrap.Analyzer,
			failopen.Analyzer,
			auditsuccess.Analyzer,
			locktimeout.Analyzer,
		)
	}()`),
			wantErr: errMulticheckerPlacement,
		},
		{
			name: "unreachable branch",
			source: validTestProgram(`if false {
		multichecker.Main(
			scannerdiscipline.Analyzer,
			errwrap.Analyzer,
			failopen.Analyzer,
			auditsuccess.Analyzer,
			locktimeout.Analyzer,
		)
	}`),
			wantErr: errMulticheckerPlacement,
		},
		{
			name: "textual aliases bound to wrong imports",
			source: `package main

import (
	"golang.org/x/tools/go/analysis/multichecker"
	scannerdiscipline "github.com/softwaresalt/backlogit/internal/faultline/analyzer/auditsuccess"
	"github.com/softwaresalt/backlogit/internal/faultline/analyzer/errwrap"
	"github.com/softwaresalt/backlogit/internal/faultline/analyzer/failopen"
	"github.com/softwaresalt/backlogit/internal/faultline/analyzer/locktimeout"
	auditsuccess "github.com/softwaresalt/backlogit/internal/faultline/analyzer/scannerdiscipline"
)

func main() {
	multichecker.Main(
		scannerdiscipline.Analyzer,
		errwrap.Analyzer,
		failopen.Analyzer,
		auditsuccess.Analyzer,
		locktimeout.Analyzer,
	)
}`,
			wantErr: errAnalyzerOrder,
		},
		{
			name: "reordered canonical analyzers",
			source: validTestProgram(`multichecker.Main(
		scannerdiscipline.Analyzer,
		errwrap.Analyzer,
		auditsuccess.Analyzer,
		failopen.Analyzer,
		locktimeout.Analyzer,
	)`),
			wantErr: errAnalyzerOrder,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			file := parseTestSource(t, test.source)
			err := validateMulticheckerWiring(file)
			if !errors.Is(err, test.wantErr) {
				t.Fatalf("validateMulticheckerWiring() error = %v, want error matching %v", err, test.wantErr)
			}
		})
	}
}

func validateMulticheckerWiring(file *ast.File) error {
	bindings, err := importBindings(file)
	if err != nil {
		return fmt.Errorf("resolve imports: %w", err)
	}

	calls := findMulticheckerMainCalls(file, bindings)
	if len(calls) != 1 {
		return fmt.Errorf("%w: got %d, want 1", errMulticheckerCallCount, len(calls))
	}
	call := calls[0]

	mainFunction := findMainFunction(file)
	if mainFunction == nil || !isDirectTopLevelCall(mainFunction, call) {
		return errMulticheckerPlacement
	}
	if call.Ellipsis.IsValid() {
		return fmt.Errorf("%w: expanded slices are forbidden", errAnalyzerEnumeration)
	}

	want := expectedAnalyzers()
	if len(call.Args) != len(want) {
		return fmt.Errorf("%w: got %d arguments, want %d", errAnalyzerEnumeration, len(call.Args), len(want))
	}
	for index, argument := range call.Args {
		got, ok := resolveSelector(bindings, argument)
		if !ok {
			return fmt.Errorf(
				"%w: argument %d must be a direct imported-package.Analyzer selector",
				errAnalyzerEnumeration,
				index+1,
			)
		}
		if got.packagePath != want[index].packagePath || got.selectorName != want[index].selectorName {
			return fmt.Errorf(
				"%w: argument %d resolves to %s.%s, want %s.%s (%s)",
				errAnalyzerOrder,
				index+1,
				got.packagePath,
				got.selectorName,
				want[index].packagePath,
				want[index].selectorName,
				want[index].analyzerName,
			)
		}
	}

	return nil
}

func importBindings(file *ast.File) (map[string]string, error) {
	bindings := make(map[string]string, len(file.Imports))
	for _, spec := range file.Imports {
		importPath, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			return nil, fmt.Errorf("unquote import path %s: %w", spec.Path.Value, err)
		}

		localName := path.Base(importPath)
		if spec.Name != nil {
			localName = spec.Name.Name
		}
		if localName == "." || localName == "_" {
			continue
		}
		if previousPath, exists := bindings[localName]; exists {
			return nil, fmt.Errorf("import name %q binds both %q and %q", localName, previousPath, importPath)
		}
		bindings[localName] = importPath
	}
	return bindings, nil
}

func findMulticheckerMainCalls(file *ast.File, bindings map[string]string) []*ast.CallExpr {
	var calls []*ast.CallExpr
	ast.Inspect(file, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		reference, ok := resolveSelector(bindings, call.Fun)
		if ok && reference.packagePath == multicheckerPackagePath && reference.selectorName == "Main" {
			calls = append(calls, call)
		}
		return true
	})
	return calls
}

func findMainFunction(file *ast.File) *ast.FuncDecl {
	var mainFunction *ast.FuncDecl
	for _, declaration := range file.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if !ok || function.Recv != nil || function.Name.Name != "main" {
			continue
		}
		if mainFunction != nil {
			return nil
		}
		mainFunction = function
	}
	return mainFunction
}

func isDirectTopLevelCall(function *ast.FuncDecl, call *ast.CallExpr) bool {
	for _, statement := range function.Body.List {
		expression, ok := statement.(*ast.ExprStmt)
		if !ok {
			continue
		}
		if directCall, ok := expression.X.(*ast.CallExpr); ok && directCall == call {
			return true
		}
	}
	return false
}

func resolveSelector(bindings map[string]string, expression ast.Expr) (selectorReference, bool) {
	selector, ok := expression.(*ast.SelectorExpr)
	if !ok {
		return selectorReference{}, false
	}
	packageIdentifier, ok := selector.X.(*ast.Ident)
	if !ok {
		return selectorReference{}, false
	}
	packagePath, ok := bindings[packageIdentifier.Name]
	if !ok {
		return selectorReference{}, false
	}
	return selectorReference{
		packagePath:  packagePath,
		selectorName: selector.Sel.Name,
	}, true
}

func expectedAnalyzers() []selectorReference {
	return []selectorReference{
		{
			packagePath:  "github.com/softwaresalt/backlogit/internal/faultline/analyzer/scannerdiscipline",
			selectorName: "Analyzer",
			analyzerName: "FL001scannerdiscipline",
		},
		{
			packagePath:  "github.com/softwaresalt/backlogit/internal/faultline/analyzer/errwrap",
			selectorName: "Analyzer",
			analyzerName: "FL002errwrap",
		},
		{
			packagePath:  "github.com/softwaresalt/backlogit/internal/faultline/analyzer/failopen",
			selectorName: "Analyzer",
			analyzerName: "FL003failopen",
		},
		{
			packagePath:  "github.com/softwaresalt/backlogit/internal/faultline/analyzer/auditsuccess",
			selectorName: "Analyzer",
			analyzerName: "FL004auditsuccess",
		},
		{
			packagePath:  "github.com/softwaresalt/backlogit/internal/faultline/analyzer/locktimeout",
			selectorName: "Analyzer",
			analyzerName: "FL005locktimeout",
		},
	}
}

func parseTestSource(t *testing.T, source string) *ast.File {
	t.Helper()

	file, err := parser.ParseFile(token.NewFileSet(), "main.go", source, parser.SkipObjectResolution)
	if err != nil {
		t.Fatalf("parse test source: %v", err)
	}
	return file
}

func validTestProgram(mainBody string) string {
	return fmt.Sprintf(`package main

import (
	"golang.org/x/tools/go/analysis/multichecker"
	"github.com/softwaresalt/backlogit/internal/faultline/analyzer/auditsuccess"
	"github.com/softwaresalt/backlogit/internal/faultline/analyzer/errwrap"
	"github.com/softwaresalt/backlogit/internal/faultline/analyzer/failopen"
	"github.com/softwaresalt/backlogit/internal/faultline/analyzer/locktimeout"
	"github.com/softwaresalt/backlogit/internal/faultline/analyzer/scannerdiscipline"
)

func main() {
%s
}`, mainBody)
}
