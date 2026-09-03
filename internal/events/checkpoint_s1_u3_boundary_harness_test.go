package events_test

// 153.003-T / S1 U3 — Checkpoint-context write-boundary size guard +
// fail-closed secret scan.
//
// RED harnesses: these tests assert that CreateCheckpoint rejects an
// oversized state dump and a state dump containing a heuristically detected
// secret pattern. Before the fix there is no size guard or secret scan, so
// both writes succeed — the assertions fail (RED). After the fix, the
// fail-closed guards are in place and both writes are rejected (GREEN).

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	backlogiterrors "github.com/softwaresalt/backlogit/internal/errors"
	"github.com/softwaresalt/backlogit/internal/events"
)

// TestCreateCheckpoint_U3_S1_OversizedContextRejected (153.003-T / S1 U3)
// asserts that a state_dump exceeding the size limit is rejected fail-closed.
// RED: before the fix there is no size guard and the oversized dump is written
// successfully — the error assertion fails. After the fix the guard rejects
// it before any write (GREEN).
func TestCreateCheckpoint_U3_S1_OversizedContextRejected(t *testing.T) {
	dir := t.TempDir()

	// Build a state dump that exceeds the allowed maximum. Fill the context
	// value with enough data to push the total payload over the limit.
	// Using a large context string ensures this is a V1-path rejection, not a
	// legacy-path one (the legacy path is also covered by the guard, but the
	// V1 path is the primary concern for session checkpoints).
	bigValue := strings.Repeat("a", 200*1024) // 200 KiB inside the JSON value
	stateDump := `{"schema_version":1,"agent":"ship","session_id":"u3-s1-oversize","phase":"build","status":"active","created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z","context":{"notes":"` + bigValue + `"}}`

	_, err := events.CreateCheckpoint(context.Background(), dir, stateDump)

	require.Error(t, err, "oversized state_dump must be rejected fail-closed")
	assert.True(t, errors.Is(err, backlogiterrors.ErrCheckpointStateDumpTooLarge),
		"error must satisfy errors.Is(err, ErrCheckpointStateDumpTooLarge), got: %v", err)

	assertNoCheckpointWritten(t, dir)
}

// TestCreateCheckpoint_U3_S1_SecretPatternRejected asserts that a state_dump
// containing a heuristically detected secret pattern (a known API-key prefix)
// is rejected fail-closed. RED: before the fix there is no secret scan and
// the dump is written — the error assertion fails. After the fix the guard
// rejects it (GREEN).
func TestCreateCheckpoint_U3_S1_SecretPatternRejected(t *testing.T) {
	dir := t.TempDir()

	// Embed a pattern matching a GitHub Personal Access Token prefix.
	stateDump := `{"schema_version":1,"agent":"ship","session_id":"u3-s1-secret","phase":"build","status":"active","created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z","context":{"token":"ghp_1234567890abcdef1234567890abcdef12345678"}}`

	_, err := events.CreateCheckpoint(context.Background(), dir, stateDump)

	require.Error(t, err, "state_dump containing a secret pattern must be rejected fail-closed")
	assert.True(t, errors.Is(err, backlogiterrors.ErrCheckpointStateDumpSecretDetected),
		"error must satisfy errors.Is(err, ErrCheckpointStateDumpSecretDetected), got: %v", err)

	assertNoCheckpointWritten(t, dir)
}

// TestCreateCheckpoint_U3_S1_AWSKeyRejected asserts that an AWS access key
// ID pattern (AKIA prefix) is also rejected.
func TestCreateCheckpoint_U3_S1_AWSKeyRejected(t *testing.T) {
	dir := t.TempDir()

	stateDump := `{"schema_version":1,"agent":"ship","session_id":"u3-s1-aws","phase":"build","status":"active","created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z","context":{"cred":"AKIAIOSFODNN7EXAMPLE"}}`

	_, err := events.CreateCheckpoint(context.Background(), dir, stateDump)

	require.Error(t, err, "state_dump containing an AWS key prefix must be rejected")
	assert.True(t, errors.Is(err, backlogiterrors.ErrCheckpointStateDumpSecretDetected),
		"error must satisfy errors.Is(err, ErrCheckpointStateDumpSecretDetected), got: %v", err)

	assertNoCheckpointWritten(t, dir)
}

// TestCreateCheckpoint_U3_S1_PEMKeyRejected asserts that PEM-encoded key
// material is rejected.
func TestCreateCheckpoint_U3_S1_PEMKeyRejected(t *testing.T) {
	dir := t.TempDir()

	stateDump := `{"schema_version":1,"agent":"ship","session_id":"u3-s1-pem","phase":"build","status":"active","created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z","context":{"key":"-----BEGIN RSA PRIVATE KEY-----"}}`

	_, err := events.CreateCheckpoint(context.Background(), dir, stateDump)

	require.Error(t, err, "state_dump containing PEM key material must be rejected")
	assert.True(t, errors.Is(err, backlogiterrors.ErrCheckpointStateDumpSecretDetected),
		"error must satisfy errors.Is(err, ErrCheckpointStateDumpSecretDetected), got: %v", err)

	assertNoCheckpointWritten(t, dir)
}

// TestCreateCheckpoint_U3_S1_NormalContextAccepted asserts that a legitimate
// state_dump (under the size limit, no secret patterns) is accepted.
func TestCreateCheckpoint_U3_S1_NormalContextAccepted(t *testing.T) {
	dir := t.TempDir()

	stateDump := `{"schema_version":1,"agent":"ship","session_id":"u3-s1-normal","phase":"build","status":"active","created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z","context":{"shipment_id":"135-S","feature_id":"153-F"}}`

	result, err := events.CreateCheckpoint(context.Background(), dir, stateDump)

	require.NoError(t, err, "a normal state_dump must be accepted after U3")
	assert.NotEmpty(t, result.Path)
}

// TestCreateCheckpoint_U3_S1_ErrorDoesNotLeakPayload asserts that the error
// message for an oversized or secret-containing dump does not include the raw
// payload (Constitution III: checkpoint context may contain sensitive data).
func TestCreateCheckpoint_U3_S1_ErrorDoesNotLeakPayload(t *testing.T) {
	dir := t.TempDir()

	// A dump with a detectable secret AND a distinctive value we must not see in the error.
	stateDump := `{"schema_version":1,"agent":"ship","session_id":"leak-test","phase":"build","status":"active","created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z","context":{"token":"ghp_VERYSECRETTOKEN12345678901234567890"}}`

	_, err := events.CreateCheckpoint(context.Background(), dir, stateDump)

	require.Error(t, err)
	assert.True(t, errors.Is(err, backlogiterrors.ErrCheckpointStateDumpSecretDetected))

	errMsg := err.Error()
	assert.NotContains(t, errMsg, "VERYSECRETTOKEN",
		"error message must not contain raw payload bytes (Constitution III)")
	assert.NotContains(t, errMsg, "ghp_",
		"error message must not contain the secret prefix that triggered detection")
}
