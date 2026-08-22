package events_test

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/events"
)

// objectKeyOrder reads a JSON object's top-level key names in the order they
// appear in raw, using a token stream rather than a map (which does not
// preserve order). raw must be a JSON object.
func objectKeyOrder(t *testing.T, raw []byte) []string {
	t.Helper()
	dec := json.NewDecoder(bytes.NewReader(raw))
	tok, err := dec.Token()
	require.NoError(t, err)
	delim, ok := tok.(json.Delim)
	require.True(t, ok, "expected object delimiter")
	require.Equal(t, "{", delim.String())

	var keys []string
	for dec.More() {
		keyTok, err := dec.Token()
		require.NoError(t, err)
		key, ok := keyTok.(string)
		require.True(t, ok, "expected string key")
		keys = append(keys, key)

		// Skip the value: decode it into a raw message so nested structures
		// are consumed without needing to know their shape.
		var v json.RawMessage
		require.NoError(t, dec.Decode(&v))
	}
	return keys
}

// TestCreateCheckpoint_OpenContextNamespace_FlatScalar is scenario 1 of
// 146.004-T (U1a): an unmodeled flat scalar key in context round-trips
// through CreateCheckpoint intact on disk. Asserted on the written bytes, not
// the returned struct, so a reader sharing the lossy type cannot false-green.
func TestCreateCheckpoint_OpenContextNamespace_FlatScalar(t *testing.T) {
	dir := t.TempDir()
	stateDump := `{"schema_version":1,"agent":"ship","session_id":"s1","phase":"build","context":{"shipment_id":"129-S","pr_number":372}}`

	result, err := events.CreateCheckpoint(context.Background(), dir, stateDump)
	require.NoError(t, err)

	raw, err := readCheckpointFile(t, result.Path)
	require.NoError(t, err)

	var doc map[string]any
	require.NoError(t, json.Unmarshal(raw, &doc))
	ctxAny, ok := doc["context"]
	require.True(t, ok, "written checkpoint must carry a context object")
	ctx, ok := ctxAny.(map[string]any)
	require.True(t, ok, "context must decode as an object")

	assert.Equal(t, "129-S", ctx["shipment_id"])
	prNumber, ok := ctx["pr_number"]
	require.True(t, ok, "unmodeled flat scalar key pr_number must survive the create round-trip")
	assert.Equal(t, float64(372), prNumber)
}

// TestCreateCheckpoint_OpenContextNamespace_NestedObject is scenario 2 of
// 146.004-T (U1a): an unmodeled nested object in context is preserved with its
// structure. Values are compared via map decode, and key ORDER is compared via
// a token stream, never a raw byte-substring match, because
// jsonutil.MarshalReadable compacts insignificant whitespace inside a
// json.RawMessage.
func TestCreateCheckpoint_OpenContextNamespace_NestedObject(t *testing.T) {
	dir := t.TempDir()
	stateDump := `{"schema_version":1,"agent":"ship","session_id":"s1","phase":"build","context":{"shipment_id":"129-S","review_gate":{"zeta":1,"alpha":2,"middle":{"nested":true}}}}`

	result, err := events.CreateCheckpoint(context.Background(), dir, stateDump)
	require.NoError(t, err)

	raw, err := readCheckpointFile(t, result.Path)
	require.NoError(t, err)

	var doc map[string]any
	require.NoError(t, json.Unmarshal(raw, &doc))
	ctx, ok := doc["context"].(map[string]any)
	require.True(t, ok, "context must decode as an object")

	gateAny, ok := ctx["review_gate"]
	require.True(t, ok, "unmodeled nested object key review_gate must survive the create round-trip")
	gate, ok := gateAny.(map[string]any)
	require.True(t, ok, "review_gate must decode as an object")
	assert.Equal(t, float64(1), gate["zeta"])
	assert.Equal(t, float64(2), gate["alpha"])
	middle, ok := gate["middle"].(map[string]any)
	require.True(t, ok, "nested middle object must survive")
	assert.Equal(t, true, middle["nested"])

	// Key order within the nested object must be preserved exactly as
	// supplied (zeta before alpha), proving Extra carries json.RawMessage
	// rather than reshaping through a decode/encode cycle that would
	// normalize order. Re-extract the exact raw bytes for review_gate from
	// the written file so the order check reads the persisted structure, not
	// a re-marshalled Go map (whose key order is unspecified).
	reviewGateRaw := extractObjectMember(t, raw, "context", "review_gate")
	assert.Equal(t, []string{"zeta", "alpha", "middle"}, objectKeyOrder(t, reviewGateRaw))
}

// TestCreateCheckpoint_ModeledFieldsAndFilters is scenario 3 of 146.004-T
// (U1a): the four modeled context fields still decode into their typed
// accessors, and a checkpoint written through CreateCheckpoint is still
// returned by ListCheckpoints under both a ShipmentID and a FeatureID filter.
// This is a regression guard and MUST be green before and after U2.
func TestCreateCheckpoint_ModeledFieldsAndFilters(t *testing.T) {
	dir := t.TempDir()
	stateDump := `{"schema_version":1,"agent":"ship","session_id":"s1","phase":"build","context":{"shipment_id":"129-S","feature_id":"146-F","branch":"feat/146-f-success-shaped-evidence-loss","task_ids":["146.001-T","146.002-T"]}}`

	result, err := events.CreateCheckpoint(context.Background(), dir, stateDump)
	require.NoError(t, err)

	raw, err := readCheckpointFile(t, result.Path)
	require.NoError(t, err)
	cp, err := events.ParseCheckpoint(raw)
	require.NoError(t, err)
	assert.Equal(t, "129-S", cp.Context.ShipmentID)
	assert.Equal(t, "146-F", cp.Context.FeatureID)
	assert.Equal(t, "feat/146-f-success-shaped-evidence-loss", cp.Context.Branch)
	assert.Equal(t, []string{"146.001-T", "146.002-T"}, cp.Context.TaskIDs)

	byShipment, err := events.ListCheckpoints(context.Background(), dir, events.CheckpointFilter{ShipmentID: "129-S"})
	require.NoError(t, err)
	assert.Len(t, byShipment, 1)

	byFeature, err := events.ListCheckpoints(context.Background(), dir, events.CheckpointFilter{FeatureID: "146-F"})
	require.NoError(t, err)
	assert.Len(t, byFeature, 1)
}
