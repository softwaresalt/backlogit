package errors

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// parseCheckpointErrorsSource parses internal/errors/checkpoint_errors.go and
// returns its AST for source-shape assertions. U1's harness must not
// reference any symbol declared by this unit's own delta, so it inspects the
// file's syntax tree instead of importing the not-yet-declared identifiers.
// That is what lets this test compile against the pre-declaration tree and
// fail on an assertion rather than a build error.
func parseCheckpointErrorsSource(t *testing.T) *ast.File {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "checkpoint_errors.go", nil, parser.AllErrors)
	require.NoError(t, err, "checkpoint_errors.go must parse as valid Go source")
	return file
}

func findPackageVar(file *ast.File, name string) *ast.ValueSpec {
	for _, decl := range file.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok || genDecl.Tok != token.VAR {
			continue
		}
		for _, spec := range genDecl.Specs {
			valueSpec, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			for _, n := range valueSpec.Names {
				if n.Name == name {
					return valueSpec
				}
			}
		}
	}
	return nil
}

func findPackageType(file *ast.File, name string) *ast.TypeSpec {
	for _, decl := range file.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok || genDecl.Tok != token.TYPE {
			continue
		}
		for _, spec := range genDecl.Specs {
			typeSpec, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}
			if typeSpec.Name.Name == name {
				return typeSpec
			}
		}
	}
	return nil
}

func findMethodOn(file *ast.File, receiverType, methodName string) *ast.FuncDecl {
	for _, decl := range file.Decls {
		funcDecl, ok := decl.(*ast.FuncDecl)
		if !ok || funcDecl.Recv == nil || len(funcDecl.Recv.List) == 0 {
			continue
		}
		if funcDecl.Name.Name != methodName {
			continue
		}
		recvType := funcDecl.Recv.List[0].Type
		if star, ok := recvType.(*ast.StarExpr); ok {
			if ident, ok := star.X.(*ast.Ident); ok && ident.Name == receiverType {
				return funcDecl
			}
			continue
		}
		if ident, ok := recvType.(*ast.Ident); ok && ident.Name == receiverType {
			return funcDecl
		}
	}
	return nil
}

func findPackageFunc(file *ast.File, name string) *ast.FuncDecl {
	for _, decl := range file.Decls {
		funcDecl, ok := decl.(*ast.FuncDecl)
		if !ok || funcDecl.Recv != nil {
			continue
		}
		if funcDecl.Name.Name == name {
			return funcDecl
		}
	}
	return nil
}

// structFieldTag returns the raw (quoted) struct tag literal for the named
// field of the named struct type declared in file, or "" if absent.
func structFieldTag(file *ast.File, typeName, fieldName string) string {
	typeSpec := findPackageType(file, typeName)
	if typeSpec == nil {
		return ""
	}
	structType, ok := typeSpec.Type.(*ast.StructType)
	if !ok || structType.Fields == nil {
		return ""
	}
	for _, field := range structType.Fields.List {
		for _, name := range field.Names {
			if name.Name == fieldName {
				if field.Tag == nil {
					return ""
				}
				return field.Tag.Value
			}
		}
	}
	return ""
}

// TestU1_NonConformingSentinelDeclared asserts checkpoint_errors.go declares
// the ErrCheckpointNonConforming sentinel (U1, Q1).
func TestU1_NonConformingSentinelDeclared(t *testing.T) {
	file := parseCheckpointErrorsSource(t)
	spec := findPackageVar(file, "ErrCheckpointNonConforming")
	assert.NotNil(t, spec, "ErrCheckpointNonConforming is not declared in checkpoint_errors.go")
}

// TestU1_NonConformingErrorTypeDeclared asserts checkpoint_errors.go declares
// CheckpointNonConformingError{Fields []string} with Error() and Unwrap()
// error methods, mirroring CheckpointUnknownFieldError.
func TestU1_NonConformingErrorTypeDeclared(t *testing.T) {
	file := parseCheckpointErrorsSource(t)
	typeSpec := findPackageType(file, "CheckpointNonConformingError")
	if !assert.NotNil(t, typeSpec, "CheckpointNonConformingError is not declared in checkpoint_errors.go") {
		return
	}
	structType, ok := typeSpec.Type.(*ast.StructType)
	if !assert.True(t, ok, "CheckpointNonConformingError must be a struct type") {
		return
	}
	hasFields := false
	for _, field := range structType.Fields.List {
		arrayType, ok := field.Type.(*ast.ArrayType)
		if !ok || arrayType.Len != nil {
			continue
		}
		ident, ok := arrayType.Elt.(*ast.Ident)
		if !ok || ident.Name != "string" {
			continue
		}
		for _, name := range field.Names {
			if name.Name == "Fields" {
				hasFields = true
			}
		}
	}
	assert.True(t, hasFields, "CheckpointNonConformingError must declare a Fields []string field")
	assert.NotNil(t, findMethodOn(file, "CheckpointNonConformingError", "Error"),
		"CheckpointNonConformingError has no Error() method")
	assert.NotNil(t, findMethodOn(file, "CheckpointNonConformingError", "Unwrap"),
		"CheckpointNonConformingError has no Unwrap() method")
}

// TestU1_QuarantineIsRemedyDeclared asserts checkpoint_errors.go declares the
// exported QuarantineIsRemedy(err error) bool predicate (U1, Q1).
func TestU1_QuarantineIsRemedyDeclared(t *testing.T) {
	file := parseCheckpointErrorsSource(t)
	funcDecl := findPackageFunc(file, "QuarantineIsRemedy")
	if !assert.NotNil(t, funcDecl, "QuarantineIsRemedy is not declared in checkpoint_errors.go") {
		return
	}
	if assert.Len(t, funcDecl.Type.Params.List, 1, "QuarantineIsRemedy must take exactly one parameter") {
		paramType, ok := funcDecl.Type.Params.List[0].Type.(*ast.Ident)
		assert.True(t, ok && paramType.Name == "error", "QuarantineIsRemedy's parameter must be of type error")
	}
	if assert.Len(t, funcDecl.Type.Results.List, 1, "QuarantineIsRemedy must return exactly one value") {
		resultType, ok := funcDecl.Type.Results.List[0].Type.(*ast.Ident)
		assert.True(t, ok && resultType.Name == "bool", "QuarantineIsRemedy must return bool")
	}
}

// TestU1b_BoundedFieldPathSetDeclared asserts checkpoint_errors.go declares
// BoundedFieldPathSet with Paths, Truncated, OmittedPaths, and
// TruncatedPaths, each carrying the exact JSON tag with no omitempty
// (147-F / U1b, cycle-17 rewrite).
func TestU1b_BoundedFieldPathSetDeclared(t *testing.T) {
	file := parseCheckpointErrorsSource(t)
	wantTags := map[string]string{
		"Paths":          `"paths"`,
		"Truncated":      `"truncated"`,
		"OmittedPaths":   `"omitted_paths"`,
		"TruncatedPaths": `"truncated_paths"`,
	}
	for field, want := range wantTags {
		tag := structFieldTag(file, "BoundedFieldPathSet", field)
		if !assert.NotEmpty(t, tag, "BoundedFieldPathSet is missing declared field %q", field) {
			continue
		}
		assert.Contains(t, tag, want, "BoundedFieldPathSet.%s must be tagged json:%s", field, want)
		assert.NotContains(t, tag, "omitempty", "BoundedFieldPathSet.%s must not use omitempty", field)
	}
}

// TestU1b_BoundedFieldPathsMethodDeclared asserts checkpoint_errors.go
// declares BoundedFieldPaths() BoundedFieldPathSet on
// *CheckpointNonConformingError.
func TestU1b_BoundedFieldPathsMethodDeclared(t *testing.T) {
	file := parseCheckpointErrorsSource(t)
	methodDecl := findMethodOn(file, "CheckpointNonConformingError", "BoundedFieldPaths")
	if !assert.NotNil(t, methodDecl, "BoundedFieldPaths is not declared on *CheckpointNonConformingError") {
		return
	}
	assert.Empty(t, methodDecl.Type.Params.List, "BoundedFieldPaths must take no parameters")
	if assert.Len(t, methodDecl.Type.Results.List, 1, "BoundedFieldPaths must return exactly one value") {
		resultType, ok := methodDecl.Type.Results.List[0].Type.(*ast.Ident)
		assert.True(t, ok && resultType.Name == "BoundedFieldPathSet",
			"BoundedFieldPaths must return BoundedFieldPathSet")
	}
}

// TestU1b_BoundedFieldPathSetIsExported asserts BoundedFieldPathSet is a
// struct type (not an alias or interface), keeping the machine projection an
// exported concrete carrier.
func TestU1b_BoundedFieldPathSetIsExported(t *testing.T) {
	file := parseCheckpointErrorsSource(t)
	typeSpec := findPackageType(file, "BoundedFieldPathSet")
	if !assert.NotNil(t, typeSpec, "BoundedFieldPathSet is not declared in checkpoint_errors.go") {
		return
	}
	_, ok := typeSpec.Type.(*ast.StructType)
	assert.True(t, ok, "BoundedFieldPathSet must be a struct type")
}

// TestU1c_FieldPathsForDisplayDeclared asserts checkpoint_errors.go declares
// FieldPathsForDisplay() string on *CheckpointNonConformingError (147-F /
// U1c).
func TestU1c_FieldPathsForDisplayDeclared(t *testing.T) {
	file := parseCheckpointErrorsSource(t)
	methodDecl := findMethodOn(file, "CheckpointNonConformingError", "FieldPathsForDisplay")
	if !assert.NotNil(t, methodDecl, "FieldPathsForDisplay is not declared on *CheckpointNonConformingError") {
		return
	}
	assert.Empty(t, methodDecl.Type.Params.List, "FieldPathsForDisplay must take no parameters")
	if assert.Len(t, methodDecl.Type.Results.List, 1, "FieldPathsForDisplay must return exactly one value") {
		resultType, ok := methodDecl.Type.Results.List[0].Type.(*ast.Ident)
		assert.True(t, ok && resultType.Name == "string", "FieldPathsForDisplay must return string")
	}
}

// TestU1c_ErrorDelegatesToFieldPathsForDisplay asserts Error()'s body calls
// FieldPathsForDisplay, so the machine Error() message and the human
// rendering cannot drift apart.
func TestU1c_ErrorDelegatesToFieldPathsForDisplay(t *testing.T) {
	file := parseCheckpointErrorsSource(t)
	methodDecl := findMethodOn(file, "CheckpointNonConformingError", "Error")
	if !assert.NotNil(t, methodDecl, "CheckpointNonConformingError has no Error() method") {
		return
	}
	delegates := false
	ast.Inspect(methodDecl.Body, func(n ast.Node) bool {
		sel, ok := n.(*ast.SelectorExpr)
		if ok && sel.Sel.Name == "FieldPathsForDisplay" {
			delegates = true
		}
		return true
	})
	assert.True(t, delegates, "Error() must delegate to FieldPathsForDisplay rather than re-deriving its own rendering")
}

// TestU1cGuard_FieldPathsForDisplayIsOnlyQuotingSite asserts no other method
// on CheckpointNonConformingError calls strconv.Quote directly, keeping
// quoting/escaping isolated to the single human-facing rendering method
// (cycle-16 gate finding H4). This lands with the implementation: before
// FieldPathsForDisplay exists there is no Quote call anywhere to find, so
// this assertion cannot fail and is not part of the red harness.
func TestU1cGuard_FieldPathsForDisplayIsOnlyQuotingSite(t *testing.T) {
	file := parseCheckpointErrorsSource(t)
	for _, decl := range file.Decls {
		funcDecl, ok := decl.(*ast.FuncDecl)
		if !ok || funcDecl.Recv == nil || funcDecl.Name.Name == "FieldPathsForDisplay" || funcDecl.Name.Name == "Error" {
			continue
		}
		if funcDecl.Body == nil {
			continue
		}
		ast.Inspect(funcDecl.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if ok && sel.Sel.Name == "Quote" {
				assert.Fail(t, "only FieldPathsForDisplay may call strconv.Quote", "found in %s", funcDecl.Name.Name)
			}
			return true
		})
	}
}

// TestU1cGuard_QuotedPathJoin asserts Error() contains every offender path
// in quoted form.
func TestU1cGuard_QuotedPathJoin(t *testing.T) {
	err := &CheckpointNonConformingError{Fields: []string{"alpha_key", "zeta_key"}}
	msg := err.Error()
	assert.Contains(t, msg, `"alpha_key"`)
	assert.Contains(t, msg, `"zeta_key"`)
}

// TestU1cGuard_TruncationClauseRendersCounts asserts a truncated set renders
// the omitted and shortened counts in the human clause.
func TestU1cGuard_TruncationClauseRendersCounts(t *testing.T) {
	fields := make([]string, 21)
	for i := range fields {
		fields[i] = fmt.Sprintf("key_%02d", i)
	}
	err := &CheckpointNonConformingError{Fields: fields}
	display := err.FieldPathsForDisplay()
	assert.Contains(t, display, "5 more omitted")
}

// TestU1cGuard_ControlByteEscaped asserts a path containing a double quote
// and a newline is escaped rather than emitted verbatim.
func TestU1cGuard_ControlByteEscaped(t *testing.T) {
	err := &CheckpointNonConformingError{Fields: []string{"weird\"key\nvalue"}}
	display := err.FieldPathsForDisplay()
	assert.NotContains(t, display, "weird\"key\nvalue")
	assert.Contains(t, display, `\"`)
	assert.Contains(t, display, `\n`)
}

// TestU1Guard_ErrorsIsAsRecoverFields asserts errors.Is matches
// ErrCheckpointNonConforming through a wrapped CheckpointNonConformingError,
// and errors.As recovers the Fields slice.
func TestU1Guard_ErrorsIsAsRecoverFields(t *testing.T) {
	base := &CheckpointNonConformingError{Fields: []string{"extra_key", "progress.extra"}}
	wrapped := fmt.Errorf("disposition refused: %w", base)

	assert.True(t, errors.Is(wrapped, ErrCheckpointNonConforming))

	var recovered *CheckpointNonConformingError
	require.True(t, errors.As(wrapped, &recovered))
	assert.Equal(t, []string{"extra_key", "progress.extra"}, recovered.Fields)
}

// TestU1Guard_ErrorRendersFieldCount asserts Error() renders a non-empty
// message naming the offending field count.
func TestU1Guard_ErrorRendersFieldCount(t *testing.T) {
	err := &CheckpointNonConformingError{Fields: []string{"extra_key", "progress.extra"}}
	msg := err.Error()
	assert.NotEmpty(t, msg)
	assert.Contains(t, msg, "2")
	assert.Contains(t, msg, "extra_key")
	assert.Contains(t, msg, "progress.extra")
}

// TestU1Guard_QuarantineIsRemedyTruthTable asserts QuarantineIsRemedy is true
// for both ErrCheckpointUseQuarantine and ErrCheckpointNonConforming, and
// false for ErrCheckpointNotActive and nil.
func TestU1Guard_QuarantineIsRemedyTruthTable(t *testing.T) {
	assert.True(t, QuarantineIsRemedy(ErrCheckpointUseQuarantine))
	assert.True(t, QuarantineIsRemedy(ErrCheckpointNonConforming))
	assert.True(t, QuarantineIsRemedy(fmt.Errorf("wrapped: %w", ErrCheckpointNonConforming)))
	assert.False(t, QuarantineIsRemedy(ErrCheckpointNotActive))
	assert.False(t, QuarantineIsRemedy(nil))
}

// TestNormalizeSeamMalformedVerdict_WrapsCorruptAndInvalid is a regression
// test (found during 130-S adversarial review): RewriteCheckpointFile's own
// independent read can observe a checkpoint that became malformed or
// schema-invalid after a caller's earlier classification read passed
// (a between-read race), and its ParseCheckpoint/ValidateCheckpoint gate
// returns the raw, unwrapped verdict by contract. Without normalization, a
// caller checking QuarantineIsRemedy on the seam's returned error would miss
// this class. NormalizeSeamMalformedVerdict must wrap both ErrCheckpointCorrupt
// and ErrCheckpointInvalid with ErrCheckpointUseQuarantine (preserving the
// underlying cause via errors.Is/errors.As), leave every other error
// unchanged, and pass nil through as nil.
func TestNormalizeSeamMalformedVerdict_WrapsCorruptAndInvalid(t *testing.T) {
	corrupt := fmt.Errorf("%w: unexpected end of JSON input", ErrCheckpointCorrupt)
	got := NormalizeSeamMalformedVerdict(corrupt)
	require.Error(t, got)
	assert.ErrorIs(t, got, ErrCheckpointUseQuarantine)
	assert.ErrorIs(t, got, ErrCheckpointCorrupt, "the underlying cause must remain traversable")

	invalid := fmt.Errorf("%w: Status is required", ErrCheckpointInvalid)
	got = NormalizeSeamMalformedVerdict(invalid)
	require.Error(t, got)
	assert.ErrorIs(t, got, ErrCheckpointUseQuarantine)
	assert.ErrorIs(t, got, ErrCheckpointInvalid, "the underlying cause must remain traversable")

	unrelated := ErrCheckpointNotActive
	assert.Same(t, unrelated, NormalizeSeamMalformedVerdict(unrelated),
		"an unrelated error must pass through unchanged")

	nonConforming := &CheckpointNonConformingError{Fields: []string{"extra_key"}}
	assert.Same(t, error(nonConforming), NormalizeSeamMalformedVerdict(nonConforming),
		"a non-conforming verdict must not be treated as malformed")

	assert.NoError(t, NormalizeSeamMalformedVerdict(nil), "nil must pass through as nil")
}

// TestU1bGuard_UnderCapRoundTripsVerbatim asserts an under-cap field list
// round-trips verbatim and unquoted with no truncation metadata set.
func TestU1bGuard_UnderCapRoundTripsVerbatim(t *testing.T) {
	err := &CheckpointNonConformingError{Fields: []string{"zeta_key", "alpha_key", "alpha_key"}}
	set := err.BoundedFieldPaths()
	assert.Equal(t, []string{"alpha_key", "zeta_key"}, set.Paths)
	assert.False(t, set.Truncated)
	assert.Equal(t, 0, set.OmittedPaths)
	assert.Equal(t, 0, set.TruncatedPaths)
}

// TestU1bGuard_OverCapTruncatesWithMetadataNoSyntheticMarker asserts a
// 21-path list yields exactly 16 raw entries with Truncated: true,
// OmittedPaths: 5, and no synthetic marker element.
func TestU1bGuard_OverCapTruncatesWithMetadataNoSyntheticMarker(t *testing.T) {
	fields := make([]string, 21)
	for i := range fields {
		fields[i] = fmt.Sprintf("key_%02d", i)
	}
	err := &CheckpointNonConformingError{Fields: fields}
	set := err.BoundedFieldPaths()
	assert.Len(t, set.Paths, 16)
	assert.True(t, set.Truncated)
	assert.Equal(t, 5, set.OmittedPaths)
	for _, p := range set.Paths {
		assert.NotContains(t, p, "more")
		assert.NotContains(t, p, "+")
	}
}

// TestU1bGuard_MultiByteRuneBoundaryCutIsValidUTF8 asserts a path built from
// multi-byte runes that crosses the 128-byte cap is returned cut on a rune
// boundary, is valid UTF-8, and is counted in TruncatedPaths.
func TestU1bGuard_MultiByteRuneBoundaryCutIsValidUTF8(t *testing.T) {
	// U+00E9 ("é") encodes as 2 bytes in UTF-8; 70 repetitions is 140 bytes,
	// well past the 128-byte cap, and 128 is not a multiple of 2 away from 0
	// plus the ASCII prefix below, forcing a genuine boundary search.
	longPath := "prefix_" + strings.Repeat("\u00e9", 70)
	err := &CheckpointNonConformingError{Fields: []string{longPath}}
	set := err.BoundedFieldPaths()
	require.Len(t, set.Paths, 1)
	assert.True(t, utf8.ValidString(set.Paths[0]))
	assert.LessOrEqual(t, len(set.Paths[0]), 128)
	assert.True(t, set.Truncated)
	assert.Equal(t, 1, set.TruncatedPaths)
}
