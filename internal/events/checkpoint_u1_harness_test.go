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

// TestCreateCheckpoint_U1_MalformedJSON_TruncatedV1Shape (148.005-T / U1)
// asserts that a truncated V1-shaped payload (syntactically invalid JSON)
// returns a typed *CheckpointMalformedInputError via errors.As and leaves
// no checkpoint file on disk.
func TestCreateCheckpoint_U1_MalformedJSON_TruncatedV1Shape(t *testing.T) {
	dir := t.TempDir()
	// Truncated mid-value — clearly V1-shaped but syntactically invalid
	stateDump := `{"schema_version":1,"agent":"ship","session_id":"s1","phase":"bu`

	_, err := events.CreateCheckpoint(context.Background(), dir, stateDump)

	require.Error(t, err, "truncated V1-shaped payload must be rejected")
	require.True(t, errors.Is(err, backlogiterrors.ErrCheckpointMalformedInput),
		"error must satisfy errors.Is(err, ErrCheckpointMalformedInput), got: %v", err)

	var typed *backlogiterrors.CheckpointMalformedInputError
	require.True(t, errors.As(err, &typed),
		"error must be recoverable via errors.As as *CheckpointMalformedInputError")

	assertNoCheckpointWritten(t, dir)
}

// TestCreateCheckpoint_U1_MalformedJSON_EmptyString asserts that an empty
// string state_dump is rejected as malformed before any V1 probe.
func TestCreateCheckpoint_U1_MalformedJSON_EmptyString(t *testing.T) {
	dir := t.TempDir()

	_, err := events.CreateCheckpoint(context.Background(), dir, "")

	require.Error(t, err)
	require.True(t, errors.Is(err, backlogiterrors.ErrCheckpointMalformedInput),
		"empty string must be rejected as malformed JSON")
	assertNoCheckpointWritten(t, dir)
}

// TestCreateCheckpoint_U1_MalformedJSON_BareOpenBrace asserts that a bare
// open brace (unclosed JSON object) is rejected.
func TestCreateCheckpoint_U1_MalformedJSON_BareOpenBrace(t *testing.T) {
	dir := t.TempDir()

	_, err := events.CreateCheckpoint(context.Background(), dir, "{")

	require.Error(t, err)
	require.True(t, errors.Is(err, backlogiterrors.ErrCheckpointMalformedInput))
	assertNoCheckpointWritten(t, dir)
}

// TestCreateCheckpoint_U1_ValidLegacyJSON_PassesThrough asserts that valid
// non-V1 JSON still flows through the legacy path unchanged after U1.
// Preserves backward compatibility.
func TestCreateCheckpoint_U1_ValidLegacyJSON_PassesThrough(t *testing.T) {
	dir := t.TempDir()
	stateDump := `{"state": "legacy-test", "some_key": "value"}`

	result, err := events.CreateCheckpoint(context.Background(), dir, stateDump)

	require.NoError(t, err, "valid legacy JSON must succeed after U1")
	assert.FileExists(t, result.Path)
}

// TestCreateCheckpoint_U1_ValidV1JSON_UnchangedBehavior asserts that a
// complete valid V1 payload behaves exactly as before U1.
func TestCreateCheckpoint_U1_ValidV1JSON_UnchangedBehavior(t *testing.T) {
	dir := t.TempDir()
	stateDump := `{"schema_version":1,"agent":"ship","session_id":"s1","phase":"build"}`

	result, err := events.CreateCheckpoint(context.Background(), dir, stateDump)

	require.NoError(t, err, "valid V1 JSON must succeed after U1")
	assert.FileExists(t, result.Path)
}

// TestCreateCheckpoint_U1_MalformedErrorHasNoRawPayload asserts the error
// message does not include the raw state_dump bytes (Constitution III:
// checkpoint context may contain sensitive data).
func TestCreateCheckpoint_U1_MalformedErrorHasNoRawPayload(t *testing.T) {
	dir := t.TempDir()
	stateDump := `{"secret_key":"super_secret_value_here","schema_version":1,`

	_, err := events.CreateCheckpoint(context.Background(), dir, stateDump)

	require.Error(t, err)
	require.True(t, errors.Is(err, backlogiterrors.ErrCheckpointMalformedInput))

	errMsg := err.Error()
	assert.NotContains(t, errMsg, "super_secret_value_here",
		"error message must not include raw payload bytes")
}
