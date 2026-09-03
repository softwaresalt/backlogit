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

// TestCreateCheckpoint_U3_S1_TaskIDNotRejectedAsFalsePositive (153.003-T /
// S1 U3, adversarial-review F5 regression) asserts that ordinary task-ID
// references in context are NOT rejected. The unanchored "sk-" prefix is a
// substring of "task-" (t-a-s-k-hyphen), which is pervasive in this tool.
// After anchoring to `"sk-` (JSON string start), task-related content must
// not false-positive.
func TestCreateCheckpoint_U3_S1_TaskIDNotRejectedAsFalsePositive(t *testing.T) {
	dir := t.TempDir()

	// Contains common task-management vocabulary with "sk-" as a substring
	// of "task-" and other domain words.
	stateDump := `{"schema_version":1,"agent":"ship","session_id":"u3-s1-task-ids","phase":"build","status":"active","created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z","context":{"task_ids":["task-001","task-002"],"note":"risky-setting disk-check risk-averse"}}`

	result, err := events.CreateCheckpoint(context.Background(), dir, stateDump)

	require.NoError(t, err,
		"task IDs and domain vocabulary containing 'sk-' as a substring must NOT be rejected as false positives (F5 fix)")
	assert.NotEmpty(t, result.Path)
}

// TestCreateCheckpoint_U3_S1_UnicodeEscapeBypassRejected (153.003-T / S1 U3,
// adversarial-review Copilot finding #3) asserts that a secret encoded as a
// Unicode escape sequence (e.g. \u0067hp_ for ghp_) is detected and rejected.
// The raw-byte scan would miss this; the decoded-value pass catches it.
func TestCreateCheckpoint_U3_S1_UnicodeEscapeBypassRejected(t *testing.T) {
	dir := t.TempDir()

	// \u0067 = 'g', so \u0067hp_ decodes to ghp_ after JSON parsing.
	// Raw-byte scan sees "\u0067hp_" not "ghp_" — only the decoded pass catches this.
	stateDump := `{"schema_version":1,"agent":"ship","session_id":"u3-s1-escape","phase":"build","status":"active","created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z","context":{"token":"\u0067hp_1234567890abcdef1234567890abcdef12"}}`

	_, err := events.CreateCheckpoint(context.Background(), dir, stateDump)

	require.Error(t, err, "Unicode-escape-encoded secret must be rejected by the decoded-value pass")
	assert.True(t, errors.Is(err, backlogiterrors.ErrCheckpointStateDumpSecretDetected),
		"error must satisfy errors.Is(err, ErrCheckpointStateDumpSecretDetected), got: %v", err)

	assertNoCheckpointWritten(t, dir)
}

// TestCreateCheckpoint_U3_S1_EmbeddedSecretRejected (153.003-T / S1 U3,
// adversarial-review Copilot re-review finding) asserts that a secret
// embedded within a longer value (e.g. "Bearer ghp_abc") is also rejected.
// HasPrefix would miss this; the Contains-based decoded scan catches it.
func TestCreateCheckpoint_U3_S1_EmbeddedSecretRejected(t *testing.T) {
	dir := t.TempDir()

	// "Bearer ghp_..." pattern: secret is NOT at the start of the value.
	stateDump := `{"schema_version":1,"agent":"ship","session_id":"u3-s1-embedded","phase":"build","status":"active","created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z","context":{"auth":"Bearer ghp_1234567890abcdef1234567890abcdef12"}}`

	_, err := events.CreateCheckpoint(context.Background(), dir, stateDump)

	require.Error(t, err, "embedded secret (Bearer ghp_...) must be rejected")
	assert.True(t, errors.Is(err, backlogiterrors.ErrCheckpointStateDumpSecretDetected),
		"error must satisfy errors.Is(err, ErrCheckpointStateDumpSecretDetected), got: %v", err)

	assertNoCheckpointWritten(t, dir)
}

// TestCreateCheckpoint_U3_S1_BearerSkProjRejected asserts that a sk- prefix
// preceded by a space (word-boundary) in the decoded value is also rejected.
func TestCreateCheckpoint_U3_S1_BearerSkProjRejected(t *testing.T) {
	dir := t.TempDir()

	stateDump := `{"schema_version":1,"agent":"ship","session_id":"u3-s1-bearer-sk","phase":"build","status":"active","created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z","context":{"api_key":"key Bearer sk-proj-XXXXX1234567890abcdef"}}`

	_, err := events.CreateCheckpoint(context.Background(), dir, stateDump)

	require.Error(t, err, "sk- after a space (word boundary) must be rejected")
	assert.True(t, errors.Is(err, backlogiterrors.ErrCheckpointStateDumpSecretDetected),
		"error must satisfy errors.Is(err, ErrCheckpointStateDumpSecretDetected), got: %v", err)

	assertNoCheckpointWritten(t, dir)
}

// TestCreateCheckpoint_U3_S1_HoneyJarNotRejected (153.003-T / S1 U3,
// adversarial-review Copilot re-review PRRT_kwDORzozKM6fBPXn) asserts that
// common English words containing "eyJ" (like "honeyJar") are NOT rejected
// as JWT false positives. The word-boundary+length check gates eyJ detection.
func TestCreateCheckpoint_U3_S1_HoneyJarNotRejected(t *testing.T) {
	dir := t.TempDir()

	// "honeyJar" and "moneyJam" contain "eyJ" but should not trigger JWT detection.
	stateDump := `{"schema_version":1,"agent":"ship","session_id":"u3-s1-honeyjars","phase":"build","status":"active","created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z","context":{"items":["honeyJar","moneyJamRecipe"],"note":"coneyJunction"}}`

	result, err := events.CreateCheckpoint(context.Background(), dir, stateDump)

	require.NoError(t, err,
		"words containing 'eyJ' (honeyJar, moneyJam, etc.) must NOT be rejected as JWT false positives")
	assert.NotEmpty(t, result.Path)
}

// TestCreateCheckpoint_U3_S1_UnderscorePrecededGhpRejected (153.003-T / S1 U3,
// Copilot PRRT_kwDORzozKM6fEI53) asserts that a distinctive prefix like ghp_
// preceded by an underscore (e.g. "token_ghp_secret") IS still detected.
// Distinctive prefixes use strings.Contains (not word-boundary) so
// underscore-prefixed occurrences are not missed.
func TestCreateCheckpoint_U3_S1_UnderscorePrecededGhpRejected(t *testing.T) {
	dir := t.TempDir()

	// "token_ghp_..." has ghp_ preceded by underscore, which would be missed
	// by word-boundary matching but is caught by strings.Contains.
	stateDump := `{"schema_version":1,"agent":"ship","session_id":"u3-s1-underscore","phase":"build","status":"active","created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z","context":{"key":"token_ghp_1234567890abcdef1234567890abcdef12"}}`

	_, err := events.CreateCheckpoint(context.Background(), dir, stateDump)

	require.Error(t, err, "ghp_ preceded by underscore must be detected (distinctive prefix uses Contains)")
	assert.True(t, errors.Is(err, backlogiterrors.ErrCheckpointStateDumpSecretDetected),
		"error must satisfy errors.Is(err, ErrCheckpointStateDumpSecretDetected), got: %v", err)

	assertNoCheckpointWritten(t, dir)
}

// TestCreateCheckpoint_U3_S1_HyphenPrecededSkProjRejected (153.003-T / S1 U3,
// Copilot PRRT_kwDORzozKM6fDzYk) asserts that a sk- token preceded by a hyphen
// (e.g. "openai-key-sk-proj-...") IS detected. Hyphen is a boundary character,
// not a word character, so the key token starts properly after it.
func TestCreateCheckpoint_U3_S1_HyphenPrecededSkProjRejected(t *testing.T) {
	dir := t.TempDir()

	// "openai-key-sk-proj-..." has sk- preceded by a hyphen, not a letter.
	stateDump := `{"schema_version":1,"agent":"ship","session_id":"u3-s1-hyphen-sk","phase":"build","status":"active","created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z","context":{"api":"openai-key-sk-proj-XXXXXXXXXXXXX1234567890abcdef12345"}}`

	_, err := events.CreateCheckpoint(context.Background(), dir, stateDump)

	require.Error(t, err, "sk- preceded by a hyphen must be detected (hyphen is a boundary, not a word char)")
	assert.True(t, errors.Is(err, backlogiterrors.ErrCheckpointStateDumpSecretDetected),
		"error must satisfy errors.Is(err, ErrCheckpointStateDumpSecretDetected), got: %v", err)

	assertNoCheckpointWritten(t, dir)
}

// TestCreateCheckpoint_U3_S1_SGPrefixFalsePositiveNotRejected (153.003-T / S1 U3,
// Copilot PRRT_kwDORzozKM6fDJgC) asserts that "MSG.txt" and similar strings
// containing "SG." as an internal substring are NOT rejected (word-boundary fix).
func TestCreateCheckpoint_U3_S1_SGPrefixFalsePositiveNotRejected(t *testing.T) {
	dir := t.TempDir()

	stateDump := `{"schema_version":1,"agent":"ship","session_id":"u3-s1-msg","phase":"build","status":"active","created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z","context":{"files":["MSG.txt","messages.json","MESSAGING-API"]}}`

	result, err := events.CreateCheckpoint(context.Background(), dir, stateDump)

	require.NoError(t, err,
		"MSG.txt and other SG.-containing words must NOT be rejected (word-boundary fix)")
	assert.NotEmpty(t, result.Path)
}

// TestCreateCheckpoint_U3_S1_AKIAPrefixFalsePositiveNotRejected (153.003-T /
// S1 U3, Copilot PRRT_kwDORzozKM6fDJgC) asserts that "SLOVAKIA" and similar
// strings containing "AKIA" as an internal substring are NOT rejected.
func TestCreateCheckpoint_U3_S1_AKIAPrefixFalsePositiveNotRejected(t *testing.T) {
	dir := t.TempDir()

	stateDump := `{"schema_version":1,"agent":"ship","session_id":"u3-s1-slovakia","phase":"build","status":"active","created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z","context":{"country":"SLOVAKIA","note":"opaque-akiavariable"}}`

	result, err := events.CreateCheckpoint(context.Background(), dir, stateDump)

	require.NoError(t, err,
		"SLOVAKIA and other AKIA-containing words must NOT be rejected (word-boundary fix)")
	assert.NotEmpty(t, result.Path)
}

// TestCreateCheckpoint_U3_S1_DuplicateKeyEscapeRejected (153.003-T / S1 U3,
// Copilot PRRT_kwDORzozKM6fCnxo) asserts that a legacy dump with a
// Unicode-escaped secret in an EARLIER duplicate key occurrence is still
// rejected. json.Unmarshal into a map would collapse the first occurrence
// (the escape) to the safe second occurrence; token-stream scanning catches
// both in source order.
func TestCreateCheckpoint_U3_S1_DuplicateKeyEscapeRejected(t *testing.T) {
	dir := t.TempDir()

	// A legacy (non-V1) dump with duplicate "token" keys:
	// first occurrence contains \u0067hp_secret (decodes to ghp_secret),
	// second occurrence contains a safe value.
	// json.Unmarshal into a map would keep only "safe_value", missing the secret.
	stateDump := `{"data":"\u0067hp_SECRETVALUE1234567890abcdef1234", "data":"safe_value"}`

	_, err := events.CreateCheckpoint(context.Background(), dir, stateDump)

	require.Error(t, err, "legacy dump with Unicode-escaped secret in earlier duplicate key must be rejected")
	assert.True(t, errors.Is(err, backlogiterrors.ErrCheckpointStateDumpSecretDetected),
		"error must satisfy errors.Is(err, ErrCheckpointStateDumpSecretDetected), got: %v", err)

	assertNoCheckpointWritten(t, dir)
}

// TestCreateCheckpoint_U3_S1_FinegrainedPATRejected (153.003-T / S1 U3,
// Copilot 4th review PRRT_kwDORzozKM6fBihb) asserts that GitHub fine-grained
// PAT (github_pat_) and refresh token (ghr_) formats are also detected.
func TestCreateCheckpoint_U3_S1_FinegrainedPATRejected(t *testing.T) {
	dir := t.TempDir()

	stateDump := `{"schema_version":1,"agent":"ship","session_id":"u3-s1-fgpat","phase":"build","status":"active","created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z","context":{"token":"github_pat_XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX"}}`

	_, err := events.CreateCheckpoint(context.Background(), dir, stateDump)

	require.Error(t, err, "GitHub fine-grained PAT must be rejected")
	assert.True(t, errors.Is(err, backlogiterrors.ErrCheckpointStateDumpSecretDetected),
		"error must satisfy errors.Is(err, ErrCheckpointStateDumpSecretDetected), got: %v", err)

	assertNoCheckpointWritten(t, dir)
}

// TestCreateCheckpoint_U3_S1_RealJWTRejected asserts that a real JWT token
// (long eyJ-prefixed string with word boundary) IS rejected.
func TestCreateCheckpoint_U3_S1_RealJWTRejected(t *testing.T) {
	dir := t.TempDir()

	// A token starting with eyJ (JWT header) at a word boundary with enough length.
	stateDump := `{"schema_version":1,"agent":"ship","session_id":"u3-s1-jwt","phase":"build","status":"active","created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z","context":{"token":"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.signature"}}`

	_, err := events.CreateCheckpoint(context.Background(), dir, stateDump)

	require.Error(t, err, "real JWT token starting with eyJ must be rejected")
	assert.True(t, errors.Is(err, backlogiterrors.ErrCheckpointStateDumpSecretDetected),
		"error must satisfy errors.Is(err, ErrCheckpointStateDumpSecretDetected), got: %v", err)

	assertNoCheckpointWritten(t, dir)
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
