package events_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	backlogiterrors "github.com/softwaresalt/backlogit/internal/errors"
	"github.com/softwaresalt/backlogit/internal/events"
)

// TestCreateCheckpoint_U2_DuplicateContextKey_ExactCase (148.001-T / U2)
// asserts that a V1 payload whose context object contains exact-duplicate
// member names is rejected with a typed *CheckpointDuplicateContextKeyError
// and writes no file.
func TestCreateCheckpoint_U2_DuplicateContextKey_ExactCase(t *testing.T) {
	dir := t.TempDir()
	// context with two identical "shipment_id" keys
	stateDump := `{"schema_version":1,"agent":"ship","session_id":"s1","phase":"build",` +
		`"context":{"shipment_id":"001-S","shipment_id":"002-S"}}`

	_, err := events.CreateCheckpoint(context.Background(), dir, stateDump)

	require.Error(t, err, "duplicate context key must be rejected")
	require.True(t, errors.Is(err, backlogiterrors.ErrCheckpointDuplicateContextKey),
		"error must satisfy errors.Is(err, ErrCheckpointDuplicateContextKey), got: %v", err)

	var typed *backlogiterrors.CheckpointDuplicateContextKeyError
	require.True(t, errors.As(err, &typed),
		"error must be recoverable via errors.As as *CheckpointDuplicateContextKeyError")
	assert.NotEmpty(t, typed.Keys, "duplicate key names must be reported")

	assertNoCheckpointWritten(t, dir)
}

// TestCreateCheckpoint_U2_DuplicateContextKey_CaseFoldAlias asserts that
// case-fold-aliased context member names (e.g. shipment_id / Shipment_Id)
// are detected and rejected, matching the Unicode simple case folding
// encoding/json uses for field matching.
func TestCreateCheckpoint_U2_DuplicateContextKey_CaseFoldAlias(t *testing.T) {
	dir := t.TempDir()
	stateDump := `{"schema_version":1,"agent":"ship","session_id":"s1","phase":"build",` +
		`"context":{"shipment_id":"001-S","Shipment_Id":"002-S"}}`

	_, err := events.CreateCheckpoint(context.Background(), dir, stateDump)

	require.Error(t, err, "case-fold-aliased context key must be rejected")
	require.True(t, errors.Is(err, backlogiterrors.ErrCheckpointDuplicateContextKey),
		"error must satisfy errors.Is(err, ErrCheckpointDuplicateContextKey), got: %v", err)
	assertNoCheckpointWritten(t, dir)
}

// TestCreateCheckpoint_U2_UniqueContextKeys_Accepted asserts that a V1
// payload with distinct context keys (including unmodeled extra keys) is
// accepted by the new gate.
func TestCreateCheckpoint_U2_UniqueContextKeys_Accepted(t *testing.T) {
	dir := t.TempDir()
	stateDump := `{"schema_version":1,"agent":"ship","session_id":"s1","phase":"build",` +
		`"context":{"shipment_id":"001-S","feature_id":"002-F","extra_key":"value"}}`

	result, err := events.CreateCheckpoint(context.Background(), dir, stateDump)

	require.NoError(t, err, "unique context keys must be accepted")
	assert.FileExists(t, result.Path)
}

// TestCreateCheckpoint_U2_NilContextKeys_Accepted asserts that an empty or
// absent context object is accepted by the new gate.
func TestCreateCheckpoint_U2_NilContextKeys_Accepted(t *testing.T) {
	dir := t.TempDir()
	stateDump := `{"schema_version":1,"agent":"ship","session_id":"s1","phase":"build"}`

	result, err := events.CreateCheckpoint(context.Background(), dir, stateDump)

	require.NoError(t, err, "absent context must be accepted")
	assert.FileExists(t, result.Path)
}

// TestCreateCheckpoint_U2_DirtyLastContextEntry_Rejected (regression for FINDING-2
// of 148-F adversarial review) asserts that duplicate context keys in a LATER
// fold-equivalent "Context" entry are detected even when an earlier "context"
// entry is clean. Without the all-entry scan fix, only the first entry would be
// inspected and the duplicate in "Context" would bypass U2.
func TestCreateCheckpoint_U2_DirtyLastContextEntry_Rejected(t *testing.T) {
	dir := t.TempDir()
	// "context" (first) is clean; "Context" (last, fold-matches) has duplicate shipment_id.
	// encoding/json uses last-wins, so cp.Context is populated from "Context".
	stateDump := `{"schema_version":1,"agent":"ship","session_id":"s1","phase":"build",` +
		`"context":{"shipment_id":"001-S"},` +
		`"Context":{"shipment_id":"001-S","shipment_id":"002-S"}}`

	_, err := events.CreateCheckpoint(context.Background(), dir, stateDump)

	require.Error(t, err, "duplicate key in last-wins Context entry must be rejected")
	require.True(t, errors.Is(err, backlogiterrors.ErrCheckpointDuplicateContextKey),
		"error must satisfy errors.Is(err, ErrCheckpointDuplicateContextKey), got: %v", err)
	assertNoCheckpointWritten(t, dir)
}
