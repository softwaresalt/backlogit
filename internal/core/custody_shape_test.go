package core

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

// custodyDecls indexes the top-level declarations of one production file by name.
type custodyDecls struct {
	consts  map[string]ast.Expr
	vars    map[string]struct{}
	structs map[string]*ast.StructType
	funcs   map[string]*ast.FuncDecl
}

// custodyPackageDir returns the directory holding this test file, so source
// lookups do not depend on the directory the test binary runs from.
func custodyPackageDir(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller(0) failed; cannot locate the internal/core source directory")
	}
	return filepath.Dir(thisFile)
}

// parseCustodySource parses one production file from disk. A missing or
// unparsable file is reported as a test error and nil is returned, so the
// harness fails on an assertion rather than panicking or failing to build.
func parseCustodySource(t *testing.T, name string) *ast.File {
	t.Helper()
	path := filepath.Join(custodyPackageDir(t), name)
	file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.AllErrors)
	if err != nil {
		t.Errorf("%s is not declared yet or does not parse: %v", name, err)
		return nil
	}
	return file
}

// indexCustodyDecls records consts, vars, struct types, and receiver-less
// functions declared at package level in file.
func indexCustodyDecls(file *ast.File) custodyDecls {
	d := custodyDecls{
		consts:  map[string]ast.Expr{},
		vars:    map[string]struct{}{},
		structs: map[string]*ast.StructType{},
		funcs:   map[string]*ast.FuncDecl{},
	}
	for _, decl := range file.Decls {
		switch dd := decl.(type) {
		case *ast.FuncDecl:
			if dd.Recv == nil {
				d.funcs[dd.Name.Name] = dd
			}
		case *ast.GenDecl:
			for _, spec := range dd.Specs {
				switch s := spec.(type) {
				case *ast.ValueSpec:
					for i, name := range s.Names {
						switch dd.Tok {
						case token.CONST:
							var value ast.Expr
							if i < len(s.Values) {
								value = s.Values[i]
							}
							d.consts[name.Name] = value
						case token.VAR:
							d.vars[name.Name] = struct{}{}
						}
					}
				case *ast.TypeSpec:
					if st, ok := s.Type.(*ast.StructType); ok {
						d.structs[s.Name.Name] = st
					}
				}
			}
		}
	}
	return d
}

// assertCustodyFuncs reports each named function that is not a package-level
// function declared in fileName.
func assertCustodyFuncs(t *testing.T, fileName string, names []string) {
	t.Helper()
	assertCustodyFuncsInFiles(t, []string{fileName}, names)
}

// assertCustodyFuncsInFiles reports each named function that is not a package-level
// function declared in at least one of fileNames. Each name must still exist in
// one of the files; the union only lets the declaration live in either file.
func assertCustodyFuncsInFiles(t *testing.T, fileNames []string, names []string) {
	t.Helper()
	declared := make(map[string]struct{})
	for _, fileName := range fileNames {
		file := parseCustodySource(t, fileName)
		if file == nil {
			return
		}
		for name := range indexCustodyDecls(file).funcs {
			declared[name] = struct{}{}
		}
	}
	for _, name := range names {
		if _, ok := declared[name]; !ok {
			t.Errorf("%s is not declared in %s", name, strings.Join(fileNames, " or "))
		}
	}
}

// TestU2_WorkflowCustodyDeclarationsSourceShape is the source-shape harness for
// the workflow-custody/v1 declarations (210.002-T). It parses custody_types.go
// and custody_exec.go with go/parser and inspects them with go/ast, so it
// compiles before those files exist and fails on an assertion, never a build
// error. Function bodies are deliberately not frozen: U3 and U4 replace the
// ErrNotImplemented stubs with real behavior.
func TestU2_WorkflowCustodyDeclarationsSourceShape(t *testing.T) {
	t.Run("custody_types_and_constants_declared", func(t *testing.T) {
		file := parseCustodySource(t, "custody_types.go")
		if file == nil {
			return
		}
		d := indexCustodyDecls(file)

		stringConsts := []struct {
			name string
			want string
		}{
			{"WorkflowCustodySchemaVersion", "workflow-custody/v1"},
			{"CustodyRequestIdentityDomain", "backlogit/workflow-custody/request-identity/v1"},
		}
		for _, c := range stringConsts {
			value, ok := d.consts[c.name]
			if !ok {
				t.Errorf("%s is not declared in custody_types.go", c.name)
				continue
			}
			lit, isLit := value.(*ast.BasicLit)
			if !isLit || lit.Kind != token.STRING {
				t.Errorf("%s is not a string literal in custody_types.go; want %q", c.name, c.want)
				continue
			}
			if got, err := strconv.Unquote(lit.Value); err != nil || got != c.want {
				t.Errorf("%s = %s in custody_types.go; want %q", c.name, lit.Value, c.want)
			}
		}

		consts := []string{
			"DefaultCustodyReceiptExpiry",
			"MaxCustodyApprovalTokenWindow",
			"EventWorkflowCustodyBound",
			"EventCustodyHandoffOffered",
			"EventCustodyHandoffAccepted",
			"EventCustodyAdoptionRecorded",
			"EventCustodyReceiptSuperseded",
			"EventCustodyReceiptInvalidated",
		}
		for _, name := range consts {
			if _, ok := d.consts[name]; !ok {
				t.Errorf("%s is not declared in custody_types.go", name)
			}
		}

		sentinels := []string{
			"ErrCustodyDefaultBranchForbidden",
			"ErrPartialRenameAdoptionRejected",
			"ErrUnsupportedCustodyChangeKind",
			"ErrOperatorApprovalRequired",
			"ErrApprovalChannelUnavailable",
			"ErrMixedIndexWorktreeDivergence",
			"ErrCustodyStaleDigest",
			"ErrCustodyWrongBranch",
		}
		for _, name := range sentinels {
			if _, ok := d.vars[name]; !ok {
				t.Errorf("%s is not declared in custody_types.go", name)
			}
		}

		structs := []string{
			"WorkflowCustodyEnvelope",
			"CustodyReceipt",
			"CustodyEntry",
			"CustodyApprovalProof",
			"CustodyCandidateSpec",
			"CustodyPreviewOptions",
			"CustodyPreviewResult",
			"CustodyOfferOptions",
			"CustodyAcceptOptions",
			"CustodyMutationResult",
			"CustodyVerifyOptions",
			"CustodyPathVerdict",
			"CustodyVerifyResult",
		}
		for _, name := range structs {
			if _, ok := d.structs[name]; !ok {
				t.Errorf("%s is not declared in custody_types.go", name)
			}
		}
	})

	t.Run("custody_validator_and_token_signatures_declared", func(t *testing.T) {
		assertCustodyFuncsInFiles(t, []string{"custody_types.go", "custody_validate.go"}, []string{
			"ValidateCustodyEnvelope",
			"ValidateCustodyEntryPath",
			"CustodyRequestIdentityDigest",
			"MintCustodyApprovalToken",
			"VerifyCustodyApprovalToken",
		})
	})

	t.Run("custody_execution_and_verify_signatures_declared", func(t *testing.T) {
		assertCustodyFuncs(t, "custody_exec.go", []string{
			"PreviewCustodyCandidates",
			"OfferCustodyHandoff",
			"AcceptCustodyHandoff",
			"VerifyCustodyState",
		})
	})
}
