package cli_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// U8c — CLI `checkpoint get` projects the conformance verdict (147-F /
// 147.027-T). CLI twin of U6c: `newCheckpointGetCmd` currently prints a
// literal "valid": true with no conformance field.

// TestU8c_ValidButNonConformingProjectsConformanceFields asserts a
// valid-but-non-conforming file reports valid:true, conforming:false,
// needs_quarantine:true, a remediation_intent naming verb "quarantine" and
// approval class "A4c", and a non_conforming_fields.paths list.
func TestU8c_ValidButNonConformingProjectsConformanceFields(t *testing.T) {
	root := setupCLIWorkspace(t)
	name := "checkpoint-u8c-nonconforming.json"
	writeCLICheckpoint(t, root, name,
		`{"schema_version":1,"agent":"ship","session_id":"s1","phase":"build","status":"active",`+
			`"created_at":"2026-08-01T00:00:00Z","updated_at":"2026-08-01T00:00:00Z","extra_key":"x"}`)

	out := runCLIStdout(t, root, "checkpoint", "get", name)
	var payload map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &payload))

	assert.Equal(t, true, payload["valid"])
	assert.Equal(t, false, payload["conforming"])
	assert.Equal(t, true, payload["needs_quarantine"])
	intent, ok := payload["remediation_intent"].(map[string]any)
	require.True(t, ok, "remediation_intent must be a structured object")
	assert.Equal(t, "quarantine", intent["verb"])
	assert.Equal(t, "A4c", intent["approval_class"])
	fields, ok := payload["non_conforming_fields"].(map[string]any)
	require.True(t, ok, "non_conforming_fields must be a structured object")
	paths, ok := fields["paths"].([]any)
	require.True(t, ok)
	assert.Contains(t, paths, "extra_key")
}

// TestU8c_ConformingProjectsConformingTrue asserts a conforming file reports
// conforming:true.
func TestU8c_ConformingProjectsConformingTrue(t *testing.T) {
	root := setupCLIWorkspace(t)
	name := "checkpoint-u8c-conforming.json"
	writeCLICheckpoint(t, root, name,
		`{"schema_version":1,"agent":"ship","session_id":"s1","phase":"build","status":"active",`+
			`"created_at":"2026-08-01T00:00:00Z","updated_at":"2026-08-01T00:00:00Z"}`)

	out := runCLIStdout(t, root, "checkpoint", "get", name)
	var payload map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &payload))
	assert.Equal(t, true, payload["conforming"])
}

// TestU8cGuard_SchemaInvalidStillExitsNonZero pins the shipped read
// contract: a schema-invalid file exits non-zero with the pre-existing
// validation-class refusal rather than a success payload.
func TestU8cGuard_SchemaInvalidStillExitsNonZero(t *testing.T) {
	root := setupCLIWorkspace(t)
	name := "checkpoint-u8c-schema-invalid.json"
	writeCLICheckpoint(t, root, name, `{"status":"active"}`)

	err := runCLIErr(t, root, "checkpoint", "get", name)
	require.Error(t, err)
}

// U8 (147.015-T) — CLI surfaces the resolve/abandon refusal as an actionable
// operator message: a schema-invalid legacy document names the required
// quarantine verb and the validation reason with no key list, while a
// valid-but-non-conforming document names the offending top-level keys in
// quoted, bounded form rendered from FieldPathsForDisplay(). No case here
// asserts a paste-runnable remediation command — that belongs to 147.039-T /
// U16.

// TestU8_ResolveSchemaInvalidNamesQuarantineNoKeyList asserts `checkpoint
// resolve` on a schema-invalid legacy document exits non-zero, names
// quarantine as the required verb, reports the validation reason, and prints
// no offending-key list (147-F / U8).
func TestU8_ResolveSchemaInvalidNamesQuarantineNoKeyList(t *testing.T) {
	root := setupCLIWorkspace(t)
	name := "checkpoint-u8-resolve-invalid.json"
	writeCLICheckpoint(t, root, name, `{"status":"active"}`)

	err := runCLIErr(t, root, "checkpoint", "resolve", name)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "required verb: quarantine",
		"must state the required verb explicitly, read from a RemediationIntent rather than incidental sentinel prose")
	assert.Contains(t, err.Error(), "validation", "must report the validation-class reason")
	assert.NotContains(t, err.Error(), `"`, "a schema-invalid refusal must not print any offending-key list")
}

// TestU8_ResolveNonConformingNamesOffendingKeys asserts `checkpoint resolve`
// on a valid-but-non-conforming document exits non-zero and names the
// offending top-level keys in quoted, bounded form (147-F / U8).
func TestU8_ResolveNonConformingNamesOffendingKeys(t *testing.T) {
	root := setupCLIWorkspace(t)
	name := "checkpoint-u8-resolve-nonconforming.json"
	writeCLICheckpoint(t, root, name,
		`{"schema_version":1,"agent":"ship","session_id":"s1","phase":"build","status":"active",`+
			`"created_at":"2026-08-01T00:00:00Z","updated_at":"2026-08-01T00:00:00Z","extra_key":"x"}`)

	err := runCLIErr(t, root, "checkpoint", "resolve", name)
	require.Error(t, err)
	assert.Contains(t, err.Error(), `"extra_key"`, "must name the offending key in quoted form")
	assert.Contains(t, err.Error(), "required verb: quarantine",
		"must state the required verb explicitly, read from a RemediationIntent")
}

// TestU8_AbandonNonConformingNamesOffendingKeys asserts `checkpoint abandon`
// on a valid-but-non-conforming document does likewise (147-F / U8).
func TestU8_AbandonNonConformingNamesOffendingKeys(t *testing.T) {
	root := setupCLIWorkspace(t)
	name := "checkpoint-u8-abandon-nonconforming.json"
	writeCLICheckpoint(t, root, name,
		`{"schema_version":1,"agent":"ship","session_id":"s1","phase":"build","status":"active",`+
			`"created_at":"2026-08-01T00:00:00Z","updated_at":"2026-08-01T00:00:00Z","extra_key":"x"}`)

	err := runCLIErr(t, root, "checkpoint", "abandon", name, "--reason", "non-conforming", "--operator", "tester@example.com")
	require.Error(t, err)
	assert.Contains(t, err.Error(), `"extra_key"`, "must name the offending key in quoted form")
	assert.Contains(t, err.Error(), "required verb: quarantine",
		"must state the required verb explicitly, read from a RemediationIntent")
}
