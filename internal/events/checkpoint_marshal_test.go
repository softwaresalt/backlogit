package events_test

import (
	"context"
	"encoding/json"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/events"
)

// buildContextWithExtra returns a CheckpointContext value (not a pointer)
// carrying a populated Extra map, constructed directly rather than via
// json.Unmarshal, so scenario 1 exercises the marshal side only.
func buildContextWithExtra() events.CheckpointContext {
	return events.CheckpointContext{
		ShipmentID: "129-S",
		Extra:      map[string]json.RawMessage{"pr_number": json.RawMessage(`372`)},
	}
}

// TestCheckpointContext_MarshalJSON_NonAddressableValue is scenario 1 of
// 146.005-T (U1b): a non-addressable CheckpointContext value is marshalled
// directly — as a value copy and inside a map[string]any — and must still
// emit its unmodeled Extra keys. A pointer-receiver MarshalJSON would be
// silently skipped by encoding/json in both cases, because a function return
// value and a map value are never addressable.
func TestCheckpointContext_MarshalJSON_NonAddressableValue(t *testing.T) {
	// Direct value marshal: the argument is a non-addressable rvalue.
	b, err := json.Marshal(buildContextWithExtra())
	require.NoError(t, err)
	assert.Contains(t, string(b), `"pr_number":372`, "unmodeled Extra key must survive marshalling a non-addressable value directly")

	// Inside a map[string]any: map values are never addressable.
	wrapped := map[string]any{"context": buildContextWithExtra()}
	b2, err := json.Marshal(wrapped)
	require.NoError(t, err)
	assert.Contains(t, string(b2), `"pr_number":372`, "unmodeled Extra key must survive marshalling a non-addressable value nested inside a map")
}

// TestCheckpointContext_MarshalJSON_ModeledKeyCollision is scenario 2 of
// 146.005-T (U1b): an Extra key colliding with a modeled JSON tag must emit
// exactly one occurrence of that key, with the modeled field winning, and no
// key literally named "Extra" may ever appear in the emitted context object.
//
// Fixture construction: the base value is decoded from literal JSON bytes via
// json.Unmarshal, per the file's fixture-construction rule. The one permitted
// carve-out is used here: the collision is injected into Extra directly after
// decode, because a correct UnmarshalJSON routes a modeled key into its
// modeled field and makes this state unreachable through the decode path
// itself.
//
// This scenario is green before 146.006-T (U2) only because Extra carries
// json:"-" today and is therefore invisible to encoding/json; it becomes
// load-bearing once U2 gives the carrier real marshal/unmarshal behavior.
func TestCheckpointContext_MarshalJSON_ModeledKeyCollision(t *testing.T) {
	var ctx events.CheckpointContext
	require.NoError(t, json.Unmarshal([]byte(`{"shipment_id":"modeled-value","note":"unmodeled"}`), &ctx))

	if ctx.Extra == nil {
		ctx.Extra = map[string]json.RawMessage{}
	}
	// Carve-out: inject a modeled-key collision directly into Extra, since a
	// correct decode path can never produce this state itself.
	ctx.Extra["shipment_id"] = json.RawMessage(`"extra-value"`)

	b, err := json.Marshal(ctx)
	require.NoError(t, err)
	s := string(b)

	assert.Equal(t, 1, strings.Count(s, `"shipment_id"`), "shipment_id must appear exactly once in the emitted context")
	assert.Contains(t, s, `"shipment_id":"modeled-value"`, "the modeled field must win the collision")
	assert.NotContains(t, s, `"extra-value"`, "the colliding Extra value must be skipped, never emitted")
	assert.NotContains(t, s, `"Extra"`, `no literal "Extra" key may ever appear in the emitted context object`)
}

// TestModeledContextKeys_MatchesLiteralExpectation is scenario 3 of 146.005-T
// (U1b): reflects over CheckpointContext's json tags and compares the result
// against a hard-coded literal list. It must NOT reference any
// production-derived key set, so it compiles at 146.001-T (U0a) and cannot
// pass vacuously against a derivation that forgets to strip ",omitempty".
func TestModeledContextKeys_MatchesLiteralExpectation(t *testing.T) {
	want := []string{"branch", "feature_id", "shipment_id", "task_ids"}
	sort.Strings(want)

	typ := reflect.TypeOf(events.CheckpointContext{})
	var got []string
	for i := 0; i < typ.NumField(); i++ {
		f := typ.Field(i)
		if f.PkgPath != "" {
			// unexported field
			continue
		}
		tag := f.Tag.Get("json")
		if tag == "-" || tag == "" {
			continue
		}
		name := strings.Split(tag, ",")[0]
		if name == "" {
			continue
		}
		got = append(got, name)
	}
	sort.Strings(got)

	require.NotEmpty(t, got)
	assert.Equal(t, want, got)
}

// TestCreateCheckpoint_ContextHTMLEscapeGuard is scenario 4 of 146.005-T
// (U1b): a modeled field (branch) and an unmodeled Extra value each carrying
// "a > b && b < c" must land on disk unescaped. This mirrors the existing
// top-level escaping guard (TestCreateCheckpoint_V1NoHTMLEscape), which
// cannot reach the context object's nested values.
func TestCreateCheckpoint_ContextHTMLEscapeGuard(t *testing.T) {
	dir := t.TempDir()
	const danger = "a > b && b < c"
	stateDump := `{"schema_version":1,"agent":"ship","session_id":"s1","phase":"build","context":{"branch":"` + danger + `","note":"` + danger + `"}}`

	result, err := events.CreateCheckpoint(context.Background(), dir, stateDump)
	require.NoError(t, err)

	raw, err := readCheckpointFile(t, result.Path)
	require.NoError(t, err)
	s := string(raw)

	assert.NotContains(t, s, `\u003e`)
	assert.NotContains(t, s, `\u003c`)
	assert.NotContains(t, s, `\u0026`)

	var doc map[string]any
	require.NoError(t, json.Unmarshal(raw, &doc))
	ctx, ok := doc["context"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, danger, ctx["branch"], "modeled field branch must round-trip unescaped")
	assert.Equal(t, danger, ctx["note"], "unmodeled Extra value note must survive and round-trip unescaped")
}

// TestCreateCheckpoint_ContextKeyInjectionGuard is scenario 5 of 146.005-T
// (U1b): two Extra keys decoded from literal-JSON fixtures and re-emitted —
// one containing a double quote, a backslash, and a newline, one containing
// "a > b && b < c" — must round-trip with their exact literal key text and
// original value, shipment_id must be unchanged, and the raw bytes must never
// contain \u0026. This fails any emit() that splices raw key text into a
// buffer and any emit() that encodes keys with escape-enabled json.Marshal.
func TestCreateCheckpoint_ContextKeyInjectionGuard(t *testing.T) {
	dir := t.TempDir()

	injectionKey := "foo\",\"shipment_id\":\"pwned\n"
	escapeKey := "a > b && b < c"

	contextObj := map[string]any{
		"shipment_id": "129-S",
		injectionKey:  "value1",
		escapeKey:     "value2",
	}
	dump := map[string]any{
		"schema_version": 1,
		"agent":          "ship",
		"session_id":     "s1",
		"phase":          "build",
		"context":        contextObj,
	}
	dumpBytes, err := json.Marshal(dump)
	require.NoError(t, err)

	result, err := events.CreateCheckpoint(context.Background(), dir, string(dumpBytes))
	require.NoError(t, err)

	raw, err := readCheckpointFile(t, result.Path)
	require.NoError(t, err)
	s := string(raw)
	assert.NotContains(t, s, `\u0026`)

	var doc map[string]any
	require.NoError(t, json.Unmarshal(raw, &doc))
	ctx, ok := doc["context"].(map[string]any)
	require.True(t, ok)

	assert.Equal(t, "129-S", ctx["shipment_id"], "shipment_id must be unchanged")
	assert.Equal(t, "value1", ctx[injectionKey], "the exact literal injection key must survive with its original value")
	assert.Equal(t, "value2", ctx[escapeKey], "the exact literal escape-guard key must survive with its original value")
}
