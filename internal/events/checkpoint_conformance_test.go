package events

import (
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	backlogiterrors "github.com/softwaresalt/backlogit/internal/errors"
)

// TestU2_UnknownTopLevelKeysRefused is a source-shape harness (147-F / U2,
// cycle-31): checkpoint_conformance.go does not yet exist, so parsing it
// fails with a file-not-found error rather than a Go build error — the test
// file itself references no undeclared identifier and compiles cleanly. The
// assertion below fails via a normal test failure (require.NoError) rather
// than a compiler error, satisfying P-004's red phase. No stub file is
// committed ahead of this harness.
func TestU2_UnknownTopLevelKeysRefused(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "checkpoint_conformance.go", nil, parser.AllErrors)
	require.NoError(t, err, "checkpoint_conformance.go is not declared yet")

	funcDecl := findPackageFuncDecl(file, "CheckConformingTopLevelNamespace")
	if !assert.NotNil(t, funcDecl, "CheckConformingTopLevelNamespace is not declared in checkpoint_conformance.go") {
		return
	}
	if assert.Len(t, funcDecl.Type.Params.List, 1, "CheckConformingTopLevelNamespace must take exactly one parameter") {
		paramType, ok := funcDecl.Type.Params.List[0].Type.(*ast.ArrayType)
		assert.True(t, ok && paramType.Len == nil, "CheckConformingTopLevelNamespace's parameter must be []byte")
	}
	assert.Len(t, funcDecl.Type.Results.List, 1, "CheckConformingTopLevelNamespace must return exactly one value (error)")
}

// findPackageFuncDecl returns the package-level (non-method) func with the
// given name, or nil if absent.
func findPackageFuncDecl(file *ast.File, name string) *ast.FuncDecl {
	if file == nil {
		return nil
	}
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

// TestU2Guard_ConformingDocumentAccepted asserts a conforming V1 document
// returns nil.
func TestU2Guard_ConformingDocumentAccepted(t *testing.T) {
	doc := `{"schema_version":1,"agent":"ship","session_id":"s1","phase":"build","status":"active"}`
	assert.NoError(t, CheckConformingTopLevelNamespace([]byte(doc)))
}

// TestU2Guard_TwoUnknownTopLevelKeysRefused asserts a document with two
// unknown top-level keys returns the typed error with both keys sorted.
func TestU2Guard_TwoUnknownTopLevelKeysRefused(t *testing.T) {
	doc := `{"schema_version":1,"agent":"ship","session_id":"s1","phase":"build","zeta_key":"x","alpha_key":"y"}`
	err := CheckConformingTopLevelNamespace([]byte(doc))
	require.Error(t, err)
	var typed *backlogiterrors.CheckpointNonConformingError
	require.True(t, errors.As(err, &typed))
	assert.Equal(t, []string{"alpha_key", "zeta_key"}, typed.Fields)
}

// TestU2Guard_ReservedDispositionKeysAdmitted asserts a document carrying
// all four disposition* reserved keys with status:"abandoned" returns nil,
// proving reserved-key admission and the deliberate absence of a
// reserved-status-value check at the read boundary (U2 is not U4's
// create-boundary gate).
func TestU2Guard_ReservedDispositionKeysAdmitted(t *testing.T) {
	doc := `{"schema_version":1,"agent":"ship","session_id":"s1","phase":"build",` +
		`"status":"abandoned","disposition":"abandoned","disposition_reason":"stale",` +
		`"disposition_operator":"ship","disposition_at":"2026-08-24T00:00:00Z"}`
	assert.NoError(t, CheckConformingTopLevelNamespace([]byte(doc)))
}

// TestU2b_UnknownNestedProgressKeyRefused asserts an unknown key nested
// inside progress is refused, reported as "progress.<key>" (147-F / U2b).
// CheckConformingTopLevelNamespace already exists (U2 landed it), so this is
// a normal behavioural red: it fails via assertion, not a build error,
// because U2's flat top-level check never yet recurses into "progress".
func TestU2b_UnknownNestedProgressKeyRefused(t *testing.T) {
	doc := `{"schema_version":1,"agent":"ship","session_id":"s1","phase":"build",` +
		`"progress":{"unexpected_nested":"x"}}`
	err := CheckConformingTopLevelNamespace([]byte(doc))
	require.Error(t, err)
	var typed *backlogiterrors.CheckpointNonConformingError
	require.True(t, errors.As(err, &typed))
	assert.Equal(t, []string{"progress.unexpected_nested"}, typed.Fields)
}

// TestU2bGuard_UnmodeledContextKeysReturnNil is the permanent 146-F
// open-namespace regression guard: unmodeled context keys must never be
// swept into refusal.
func TestU2bGuard_UnmodeledContextKeysReturnNil(t *testing.T) {
	doc := `{"schema_version":1,"agent":"ship","session_id":"s1","phase":"build",` +
		`"context":{"anything_the_caller_wants":"x","nested":{"a":1}}}`
	assert.NoError(t, CheckConformingTopLevelNamespace([]byte(doc)))
}

// TestU2bGuard_NonObjectProgressReturnsNilWithoutPanicking asserts a
// non-object progress value (already governed by ParseCheckpoint) is not a
// conformance failure and does not panic.
func TestU2bGuard_NonObjectProgressReturnsNilWithoutPanicking(t *testing.T) {
	doc := `{"schema_version":1,"agent":"ship","session_id":"s1","phase":"build","progress":null}`
	assert.NotPanics(t, func() {
		assert.NoError(t, CheckConformingTopLevelNamespace([]byte(doc)))
	})
}

// TestU2e_ExactDuplicateNestedProgressKeyRefused asserts an exact duplicate
// nested progress key is non-conforming, reported as
// "duplicate:progress.<key>" (147-F / U2e).
func TestU2e_ExactDuplicateNestedProgressKeyRefused(t *testing.T) {
	doc := `{"schema_version":1,"agent":"ship","session_id":"s1","phase":"build",` +
		`"progress":{"decisions":["a"],"decisions":["b"]}}`
	err := CheckConformingTopLevelNamespace([]byte(doc))
	require.Error(t, err)
	var typed *backlogiterrors.CheckpointNonConformingError
	require.True(t, errors.As(err, &typed))
	assert.Contains(t, typed.Fields, "duplicate:progress.decisions")
}

// TestU2e_CaseVariantNestedProgressDuplicateRefused asserts a case-variant
// nested progress duplicate (tasks_completed + Tasks_Completed) is
// non-conforming.
func TestU2e_CaseVariantNestedProgressDuplicateRefused(t *testing.T) {
	doc := `{"schema_version":1,"agent":"ship","session_id":"s1","phase":"build",` +
		`"progress":{"tasks_completed":["a"],"Tasks_Completed":["b"]}}`
	err := CheckConformingTopLevelNamespace([]byte(doc))
	require.Error(t, err)
	var typed *backlogiterrors.CheckpointNonConformingError
	require.True(t, errors.As(err, &typed))
	assert.Contains(t, typed.Fields, "duplicate:progress.tasks_completed")
}

// TestU2eGuard_OneOccurrenceOfEachProgressKeyStaysConforming asserts a
// progress object with one occurrence of each key stays conforming and the
// create boundary's verdict on the same bytes is unchanged.
func TestU2eGuard_OneOccurrenceOfEachProgressKeyStaysConforming(t *testing.T) {
	doc := `{"schema_version":1,"agent":"ship","session_id":"s1","phase":"build",` +
		`"progress":{"tasks_completed":["a"],"tasks_remaining":["b"],"files_modified":["c"],"decisions":["d"]}}`
	assert.NoError(t, CheckConformingTopLevelNamespace([]byte(doc)))
	assert.NoError(t, checkClosedSchemaNamespace([]byte(doc)))
}

// TestU2c_ExactDuplicateTopLevelKeyRefused asserts an exact duplicate
// top-level key makes the document non-conforming, reported as
// "duplicate:<key>" (147-F / U2c).
func TestU2c_ExactDuplicateTopLevelKeyRefused(t *testing.T) {
	doc := `{"schema_version":1,"agent":"ship","session_id":"s1","phase":"build",` +
		`"status":"active","status":"resolved"}`
	err := CheckConformingTopLevelNamespace([]byte(doc))
	require.Error(t, err)
	var typed *backlogiterrors.CheckpointNonConformingError
	require.True(t, errors.As(err, &typed))
	assert.Contains(t, typed.Fields, "duplicate:status")
}

// TestU2c_CaseVariantDuplicateTopLevelKeyRefused asserts a case-fold-variant
// duplicate top-level key ("status" + "Status") is non-conforming.
func TestU2c_CaseVariantDuplicateTopLevelKeyRefused(t *testing.T) {
	doc := `{"schema_version":1,"agent":"ship","session_id":"s1","phase":"build",` +
		`"status":"active","Status":"active"}`
	err := CheckConformingTopLevelNamespace([]byte(doc))
	require.Error(t, err)
	var typed *backlogiterrors.CheckpointNonConformingError
	require.True(t, errors.As(err, &typed))
	assert.Contains(t, typed.Fields, "duplicate:status")
}

// TestU2cGuard_OneOccurrenceOfEveryModeledKeyStaysConforming asserts a
// document with one occurrence of every modeled key remains conforming.
func TestU2cGuard_OneOccurrenceOfEveryModeledKeyStaysConforming(t *testing.T) {
	doc := `{"schema_version":1,"agent":"ship","session_id":"s1","phase":"build","status":"active"}`
	assert.NoError(t, CheckConformingTopLevelNamespace([]byte(doc)))
}

// TestU2d_AllTopLevelKeysDerivedSetConsulted is a source-shape harness
// (147-F / U2d, cycle-31): checkpointV1AllTopLevelKeys does not yet exist,
// so referencing it directly would be a build error. This test inspects
// checkpoint_conformance.go's AST instead, so it compiles against the
// pre-delta tree and fails on assertions.
func TestU2d_AllTopLevelKeysDerivedSetConsulted(t *testing.T) {
	file := parseEventsSource(t, "checkpoint_conformance.go")
	varSpec := findPackageVarIn(file, "checkpointV1AllTopLevelKeys")
	if !assert.NotNil(t, varSpec, "checkpointV1AllTopLevelKeys is not declared in checkpoint_conformance.go") {
		return
	}
	funcDecl := findPackageFuncDecl(file, "CheckConformingTopLevelNamespace")
	if !assert.NotNil(t, funcDecl) {
		return
	}
	consults := false
	ast.Inspect(funcDecl.Body, func(n ast.Node) bool {
		if ident, ok := n.(*ast.Ident); ok && ident.Name == "checkpointV1AllTopLevelKeys" {
			consults = true
		}
		return true
	})
	assert.True(t, consults, "CheckConformingTopLevelNamespace must consult checkpointV1AllTopLevelKeys")
}

// TestU2dGuard_AllTopLevelKeysEqualsUnionOfTheTwoSets asserts
// checkpointV1AllTopLevelKeys equals checkpointV1TopLevelKeys UNION
// checkpointV1ReservedKeys, guarding drift in the hand-written reserved set
// rather than the reflected field set.
func TestU2dGuard_AllTopLevelKeysEqualsUnionOfTheTwoSets(t *testing.T) {
	union := map[string]struct{}{}
	for k := range checkpointV1TopLevelKeys {
		union[k] = struct{}{}
	}
	for k := range checkpointV1ReservedKeys {
		union[k] = struct{}{}
	}
	assert.Equal(t, union, checkpointV1AllTopLevelKeys)
}

// TestU2dGuard_NoTopLevelPreservationCarrier asserts CheckpointV1 declares
// no json:"-" map carrier (decision-anchored: revisit
// docs/decisions/2026-08-24-checkpoint-toplevel-key-disposition-deliberation.md
// before adding one).
func TestU2dGuard_NoTopLevelPreservationCarrier(t *testing.T) {
	typ := reflect.TypeOf(CheckpointV1{})
	for i := 0; i < typ.NumField(); i++ {
		tag := typ.Field(i).Tag.Get("json")
		assert.NotEqual(t, "-", strings.Split(tag, ",")[0],
			"CheckpointV1 must not declare a json:\"-\" carrier without revisiting the deliberation")
	}
}

// TestU2dGuard_EveryExportedFieldHasNonEmptyJSONTag closes the latent escape
// hatch: modeledJSONTagKeys skips untagged exported fields, so a future
// field added without a tag would silently escape the derived set.
func TestU2dGuard_EveryExportedFieldHasNonEmptyJSONTag(t *testing.T) {
	typ := reflect.TypeOf(CheckpointV1{})
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		if !field.IsExported() {
			continue
		}
		tag := field.Tag.Get("json")
		assert.NotEmpty(t, tag, "CheckpointV1.%s must carry a non-empty json tag", field.Name)
	}
}

// TestU2g_DuplicateExactContextMember asserts an exact-duplicate decoded
// context member — including an escape-equivalent spelling — is
// non-conforming, reported as duplicate:context.<key> (147-F / U2g).
func TestU2g_DuplicateExactContextMember(t *testing.T) {
	doc := `{"schema_version":1,"agent":"ship","session_id":"s1","phase":"build",` +
		`"context":{"foo":1,"\u0066oo":2}}`
	err := CheckConformingTopLevelNamespace([]byte(doc))
	require.Error(t, err)
	var typed *backlogiterrors.CheckpointNonConformingError
	require.True(t, errors.As(err, &typed))
	assert.Contains(t, typed.Fields, "duplicate:context.foo")
}

// TestU2g_DuplicateFoldVariantAliasingModeledField asserts a fold-variant
// pair aliasing a modeled context field (shipment_id + Shipment_Id) is
// non-conforming.
func TestU2g_DuplicateFoldVariantAliasingModeledField(t *testing.T) {
	doc := `{"schema_version":1,"agent":"ship","session_id":"s1","phase":"build",` +
		`"context":{"shipment_id":"130-S","Shipment_Id":"131-S"}}`
	err := CheckConformingTopLevelNamespace([]byte(doc))
	require.Error(t, err)
	var typed *backlogiterrors.CheckpointNonConformingError
	require.True(t, errors.As(err, &typed))
	assert.Contains(t, typed.Fields, "duplicate:context.shipment_id")
}

// TestU2gGuard_OpenNamespacePreserved asserts distinct unmodeled fold
// variants and unique extension keys stay conforming and survive the Extra
// round-trip (the open-namespace-preservation guard U2g must not narrow).
func TestU2gGuard_OpenNamespacePreserved(t *testing.T) {
	doc := `{"schema_version":1,"agent":"ship","session_id":"s1","phase":"build",` +
		`"context":{"foo":1,"Foo":2,"unique_ext":3}}`
	assert.NoError(t, CheckConformingTopLevelNamespace([]byte(doc)))

	cp, err := ParseCheckpoint([]byte(doc))
	require.NoError(t, err)
	require.Contains(t, cp.Context.Extra, "foo")
	require.Contains(t, cp.Context.Extra, "Foo")
	require.Contains(t, cp.Context.Extra, "unique_ext")
}

// TestU2h_LoneNonCanonicalContextSpellingDuplicateRefused asserts a document
// whose only context-routing member is spelled "Context" (no literal
// "context" sibling) and whose inner members carry an exact duplicate is
// non-conforming, reported as duplicate:context.<key> (147-F / U2h). Before
// this unit's fix, the U2g walk keys on the literal "context" spelling and
// never inspects a "Context" member.
func TestU2h_LoneNonCanonicalContextSpellingDuplicateRefused(t *testing.T) {
	doc := `{"schema_version":1,"agent":"ship","session_id":"s1","phase":"build",` +
		`"Context":{"foo":1,"\u0066oo":2}}`
	err := CheckConformingTopLevelNamespace([]byte(doc))
	require.Error(t, err)
	var typed *backlogiterrors.CheckpointNonConformingError
	require.True(t, errors.As(err, &typed))
	assert.Contains(t, typed.Fields, "duplicate:context.foo")
}

// TestU2hGuard_LoneNonCanonicalSpellingOpenNamespacePreserved asserts a
// document whose only context-routing member is spelled "CONTEXT" and whose
// inner members are all distinct unmodeled extensions stays conforming and
// every key survives the Extra round-trip.
func TestU2hGuard_LoneNonCanonicalSpellingOpenNamespacePreserved(t *testing.T) {
	doc := `{"schema_version":1,"agent":"ship","session_id":"s1","phase":"build",` +
		`"CONTEXT":{"foo":1,"bar":2}}`
	assert.NoError(t, CheckConformingTopLevelNamespace([]byte(doc)))

	cp, err := ParseCheckpoint([]byte(doc))
	require.NoError(t, err)
	require.Contains(t, cp.Context.Extra, "foo")
	require.Contains(t, cp.Context.Extra, "bar")
}
