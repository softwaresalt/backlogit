package mcp

// 136-F/U11: backlogit_abandon_checkpoint and backlogit_quarantine_checkpoint
// MCP tool handler tests. Both tools require an explicit, non-empty operator
// parameter — it is never inferred on the MCP surface — and route through the
// shared core.AbandonCheckpoint / core.QuarantineCheckpoint verbs used by the
// CLI fallback.

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	mcplib "github.com/mark3labs/mcp-go/mcp"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	backlogiterrors "github.com/softwaresalt/backlogit/internal/errors"
)

func writeCheckpointFileMCP(t *testing.T, root, filename, body string) {
	t.Helper()
	dir := filepath.Join(root, ".backlogit", "checkpoints")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, filename), []byte(body), 0o644))
}

func TestHandleAbandonCheckpoint_HappyPath(t *testing.T) {
	s, ws := setupBugFixServer(t)
	ctx := context.Background()
	writeCheckpointFileMCP(t, ws.RootPath, "checkpoint-mcp-abandon.json",
		`{"schema_version":1,"agent":"ship","session_id":"s1","phase":"build","status":"active","created_at":"2026-08-01T00:00:00Z","updated_at":"2026-08-01T00:00:00Z"}`)

	request := mcplib.CallToolRequest{}
	request.Params.Name = "backlogit_abandon_checkpoint"
	request.Params.Arguments = map[string]any{
		"filename": "checkpoint-mcp-abandon.json",
		"reason":   "superseded",
		"operator": "tester@example.com",
	}

	result, err := s.handleAbandonCheckpoint(ctx, request)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.False(t, result.IsError, "abandon must succeed for a valid target")
	tc, ok := result.Content[0].(mcplib.TextContent)
	require.True(t, ok)
	var resp map[string]any
	require.NoError(t, json.Unmarshal([]byte(tc.Text), &resp))
	assert.Equal(t, "abandoned", resp["disposition"])
}

func TestHandleAbandonCheckpoint_MissingOperator(t *testing.T) {
	s, ws := setupBugFixServer(t)
	ctx := context.Background()
	writeCheckpointFileMCP(t, ws.RootPath, "checkpoint-mcp-abandon2.json",
		`{"schema_version":1,"agent":"ship","session_id":"s1","phase":"build","status":"active","created_at":"2026-08-01T00:00:00Z","updated_at":"2026-08-01T00:00:00Z"}`)

	request := mcplib.CallToolRequest{}
	request.Params.Name = "backlogit_abandon_checkpoint"
	request.Params.Arguments = map[string]any{
		"filename": "checkpoint-mcp-abandon2.json",
		"reason":   "superseded",
	}

	result, err := s.handleAbandonCheckpoint(ctx, request)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError, "abandon must fail when operator is not supplied")
}

func TestHandleAbandonCheckpoint_MalformedNamesQuarantine(t *testing.T) {
	s, ws := setupBugFixServer(t)
	ctx := context.Background()
	writeCheckpointFileMCP(t, ws.RootPath, "checkpoint-mcp-malformed.json", "not-json{")

	request := mcplib.CallToolRequest{}
	request.Params.Name = "backlogit_abandon_checkpoint"
	request.Params.Arguments = map[string]any{
		"filename": "checkpoint-mcp-malformed.json",
		"reason":   "x",
		"operator": "tester@example.com",
	}

	result, err := s.handleAbandonCheckpoint(ctx, request)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.True(t, result.IsError)
	tc, ok := result.Content[0].(mcplib.TextContent)
	require.True(t, ok)
	var resp map[string]any
	require.NoError(t, json.Unmarshal([]byte(tc.Text), &resp))
	assert.Equal(t, "checkpoint_use_quarantine", resp["code"])
	assert.False(t, resp["retryable"].(bool))
}

func TestHandleQuarantineCheckpoint_HappyPath(t *testing.T) {
	s, ws := setupBugFixServer(t)
	ctx := context.Background()
	writeCheckpointFileMCP(t, ws.RootPath, "checkpoint-mcp-quarantine.json", "not-json{")

	request := mcplib.CallToolRequest{}
	request.Params.Name = "backlogit_quarantine_checkpoint"
	request.Params.Arguments = map[string]any{
		"filename": "checkpoint-mcp-quarantine.json",
		"reason":   "corrupt",
		"operator": "tester@example.com",
	}

	result, err := s.handleQuarantineCheckpoint(ctx, request)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.False(t, result.IsError, "quarantine must succeed for a malformed target")
	tc, ok := result.Content[0].(mcplib.TextContent)
	require.True(t, ok)
	var resp map[string]any
	require.NoError(t, json.Unmarshal([]byte(tc.Text), &resp))
	assert.Equal(t, "quarantined", resp["disposition"])

	destPath := filepath.Join(ws.RootPath, ".backlogit", "archive", "checkpoints", "checkpoint-mcp-quarantine.json")
	assert.FileExists(t, destPath)
}

func TestHandleQuarantineCheckpoint_ValidNamesAbandon(t *testing.T) {
	s, ws := setupBugFixServer(t)
	ctx := context.Background()
	writeCheckpointFileMCP(t, ws.RootPath, "checkpoint-mcp-valid.json",
		`{"schema_version":1,"agent":"ship","session_id":"s1","phase":"build","status":"active","created_at":"2026-08-01T00:00:00Z","updated_at":"2026-08-01T00:00:00Z"}`)

	request := mcplib.CallToolRequest{}
	request.Params.Name = "backlogit_quarantine_checkpoint"
	request.Params.Arguments = map[string]any{
		"filename": "checkpoint-mcp-valid.json",
		"reason":   "x",
		"operator": "tester@example.com",
	}

	result, err := s.handleQuarantineCheckpoint(ctx, request)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.True(t, result.IsError)
	tc, ok := result.Content[0].(mcplib.TextContent)
	require.True(t, ok)
	var resp map[string]any
	require.NoError(t, json.Unmarshal([]byte(tc.Text), &resp))
	assert.Equal(t, "checkpoint_use_abandon", resp["code"])
}

func TestHandleQuarantineCheckpoint_MissingOperator(t *testing.T) {
	s, ws := setupBugFixServer(t)
	ctx := context.Background()
	writeCheckpointFileMCP(t, ws.RootPath, "checkpoint-mcp-quarantine2.json", "not-json{")

	request := mcplib.CallToolRequest{}
	request.Params.Name = "backlogit_quarantine_checkpoint"
	request.Params.Arguments = map[string]any{
		"filename": "checkpoint-mcp-quarantine2.json",
		"reason":   "corrupt",
	}

	result, err := s.handleQuarantineCheckpoint(ctx, request)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError, "quarantine must fail when operator is not supplied")
}

// TestHandleAbandonCheckpoint_WhitespaceOnlyOperatorRejected asserts CLI/MCP
// parity: the CLI trims and rejects a whitespace-only operator, so the MCP
// surface must reject it too rather than persisting a blank audit identity.
func TestHandleAbandonCheckpoint_WhitespaceOnlyOperatorRejected(t *testing.T) {
	s, ws := setupBugFixServer(t)
	ctx := context.Background()
	writeCheckpointFileMCP(t, ws.RootPath, "checkpoint-mcp-ws-operator.json",
		`{"schema_version":1,"agent":"ship","session_id":"s1","phase":"build","status":"active","created_at":"2026-08-01T00:00:00Z","updated_at":"2026-08-01T00:00:00Z"}`)

	request := mcplib.CallToolRequest{}
	request.Params.Name = "backlogit_abandon_checkpoint"
	request.Params.Arguments = map[string]any{
		"filename": "checkpoint-mcp-ws-operator.json",
		"reason":   "superseded",
		"operator": "   ",
	}

	result, err := s.handleAbandonCheckpoint(ctx, request)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError, "abandon must reject a whitespace-only operator")
}

// TestHandleAbandonCheckpoint_WhitespaceOnlyReasonRejected mirrors the
// operator case for reason.
func TestHandleAbandonCheckpoint_WhitespaceOnlyReasonRejected(t *testing.T) {
	s, ws := setupBugFixServer(t)
	ctx := context.Background()
	writeCheckpointFileMCP(t, ws.RootPath, "checkpoint-mcp-ws-reason.json",
		`{"schema_version":1,"agent":"ship","session_id":"s1","phase":"build","status":"active","created_at":"2026-08-01T00:00:00Z","updated_at":"2026-08-01T00:00:00Z"}`)

	request := mcplib.CallToolRequest{}
	request.Params.Name = "backlogit_abandon_checkpoint"
	request.Params.Arguments = map[string]any{
		"filename": "checkpoint-mcp-ws-reason.json",
		"reason":   "   ",
		"operator": "tester@example.com",
	}

	result, err := s.handleAbandonCheckpoint(ctx, request)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError, "abandon must reject a whitespace-only reason")
}

// TestHandleQuarantineCheckpoint_WhitespaceOnlyOperatorRejected mirrors the
// abandon case for quarantine.
func TestHandleQuarantineCheckpoint_WhitespaceOnlyOperatorRejected(t *testing.T) {
	s, ws := setupBugFixServer(t)
	ctx := context.Background()
	writeCheckpointFileMCP(t, ws.RootPath, "checkpoint-mcp-ws-quarantine.json", "not-json{")

	request := mcplib.CallToolRequest{}
	request.Params.Name = "backlogit_quarantine_checkpoint"
	request.Params.Arguments = map[string]any{
		"filename": "checkpoint-mcp-ws-quarantine.json",
		"reason":   "corrupt",
		"operator": "\t\n",
	}

	result, err := s.handleQuarantineCheckpoint(ctx, request)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError, "quarantine must reject a whitespace-only operator")
}

// TestU7e_DomainErrorMapsCannotResolveAbandoned asserts domainError maps a
// realistically wrapped ErrCheckpointCannotResolveAbandoned to
// validation_failed rather than falling through to the internal default
// (147-F / U7e).
func TestU7e_DomainErrorMapsCannotResolveAbandoned(t *testing.T) {
	wrapped := fmt.Errorf("resolve %s: %w", "checkpoint-abandoned.json", backlogiterrors.ErrCheckpointCannotResolveAbandoned)
	result := domainError("resolve checkpoint", wrapped)
	require.NotNil(t, result)
	require.True(t, result.IsError)
	tc, ok := result.Content[0].(mcplib.TextContent)
	require.True(t, ok)
	var resp map[string]any
	require.NoError(t, json.Unmarshal([]byte(tc.Text), &resp))
	assert.Equal(t, "validation_failed", resp["error"])
}

// TestU6c_ValidButNonConformingProjectsConformanceFields asserts
// handleGetCheckpoint on a valid-but-non-conforming file returns valid:true,
// conforming:false, needs_quarantine:true, a remediation_intent object
// naming verb "quarantine" and approval_class "A4c", and a
// non_conforming_fields.paths array matching the events result (147-F /
// U6c).
func TestU6c_ValidButNonConformingProjectsConformanceFields(t *testing.T) {
	s, ws := setupBugFixServer(t)
	ctx := context.Background()
	name := "checkpoint-u6c-nonconforming.json"
	writeCheckpointFileMCP(t, ws.RootPath, name,
		`{"schema_version":1,"agent":"ship","session_id":"s1","phase":"build","status":"active",`+
			`"created_at":"2026-08-01T00:00:00Z","updated_at":"2026-08-01T00:00:00Z","extra_key":"x"}`)

	request := mcplib.CallToolRequest{}
	request.Params.Name = "backlogit_get_checkpoint"
	request.Params.Arguments = map[string]any{"filename": name}

	result, err := s.handleGetCheckpoint(ctx, request)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.False(t, result.IsError)
	tc, ok := result.Content[0].(mcplib.TextContent)
	require.True(t, ok)
	var resp map[string]any
	require.NoError(t, json.Unmarshal([]byte(tc.Text), &resp))

	assert.Equal(t, true, resp["valid"])
	assert.Equal(t, false, resp["conforming"])
	assert.Equal(t, true, resp["needs_quarantine"])
	intent, ok := resp["remediation_intent"].(map[string]any)
	require.True(t, ok, "remediation_intent must be a structured object")
	assert.Equal(t, "quarantine", intent["verb"])
	assert.Equal(t, "A4c", intent["approval_class"])
	fields, ok := resp["non_conforming_fields"].(map[string]any)
	require.True(t, ok, "non_conforming_fields must be a structured object")
	paths, ok := fields["paths"].([]any)
	require.True(t, ok)
	assert.Contains(t, paths, "extra_key")
}

// TestU6c_ConformingProjectsConformingTrueWithNullIntentAndEmptyPaths
// asserts a conforming file returns conforming:true, remediation_intent:
// null, and non_conforming_fields.paths: [] read through a .([]any) type
// assertion so an absent or null key fails.
func TestU6c_ConformingProjectsConformingTrueWithNullIntentAndEmptyPaths(t *testing.T) {
	s, ws := setupBugFixServer(t)
	ctx := context.Background()
	name := "checkpoint-u6c-conforming.json"
	writeCheckpointFileMCP(t, ws.RootPath, name,
		`{"schema_version":1,"agent":"ship","session_id":"s1","phase":"build","status":"active",`+
			`"created_at":"2026-08-01T00:00:00Z","updated_at":"2026-08-01T00:00:00Z"}`)

	request := mcplib.CallToolRequest{}
	request.Params.Name = "backlogit_get_checkpoint"
	request.Params.Arguments = map[string]any{"filename": name}

	result, err := s.handleGetCheckpoint(ctx, request)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.False(t, result.IsError)
	tc, ok := result.Content[0].(mcplib.TextContent)
	require.True(t, ok)
	var resp map[string]any
	require.NoError(t, json.Unmarshal([]byte(tc.Text), &resp))

	assert.Equal(t, true, resp["conforming"])
	assert.Nil(t, resp["remediation_intent"])
	fields, ok := resp["non_conforming_fields"].(map[string]any)
	require.True(t, ok, "non_conforming_fields must be present even for a conforming document")
	paths, ok := fields["paths"].([]any)
	require.True(t, ok, "non_conforming_fields.paths must be an array, not absent or null")
	assert.Empty(t, paths)
}

// TestU6cGuard_SchemaInvalidStillReturnsValidationFailed pins the shipped
// read contract: a schema-invalid file returns the pre-existing
// validation_failed refusal rather than a success payload.
func TestU6cGuard_SchemaInvalidStillReturnsValidationFailed(t *testing.T) {
	s, ws := setupBugFixServer(t)
	ctx := context.Background()
	name := "checkpoint-u6c-schema-invalid.json"
	writeCheckpointFileMCP(t, ws.RootPath, name, `{"status":"active"}`)

	request := mcplib.CallToolRequest{}
	request.Params.Name = "backlogit_get_checkpoint"
	request.Params.Arguments = map[string]any{"filename": name}

	result, err := s.handleGetCheckpoint(ctx, request)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.True(t, result.IsError)
	tc, ok := result.Content[0].(mcplib.TextContent)
	require.True(t, ok)
	var resp map[string]any
	require.NoError(t, json.Unmarshal([]byte(tc.Text), &resp))
	assert.Equal(t, "validation_failed", resp["error"])
}

// TestU7_CheckpointDispositionErrorMapsNonConforming asserts
// checkpointDispositionError returns code checkpoint_non_conforming for
// ErrCheckpointNonConforming, and that its pre-existing
// ErrCheckpointNotFound -> NotFound case still fires unchanged (147-F /
// U7).
func TestU7_CheckpointDispositionErrorMapsNonConforming(t *testing.T) {
	nonConforming := &backlogiterrors.CheckpointNonConformingError{Fields: []string{"extra_key"}}
	result := checkpointDispositionError("abandon checkpoint", "checkpoint-u7.json", nonConforming)
	require.NotNil(t, result)
	require.True(t, result.IsError)
	tc, ok := result.Content[0].(mcplib.TextContent)
	require.True(t, ok)
	var resp map[string]any
	require.NoError(t, json.Unmarshal([]byte(tc.Text), &resp))
	assert.Equal(t, "checkpoint_non_conforming", resp["code"])

	notFoundResult := checkpointDispositionError("abandon checkpoint", "missing.json", backlogiterrors.ErrCheckpointNotFound)
	require.NotNil(t, notFoundResult)
	require.True(t, notFoundResult.IsError)
	tc2, ok := notFoundResult.Content[0].(mcplib.TextContent)
	require.True(t, ok)
	var resp2 map[string]any
	require.NoError(t, json.Unmarshal([]byte(tc2.Text), &resp2))
	assert.Equal(t, "not_found", resp2["error"])
}

// TestU7_HandleAbandonNonConformingReturnsUnknownFields asserts
// handleAbandonCheckpoint on a non-conforming target returns
// checkpoint_non_conforming with a populated unknown_fields array (raw,
// unquoted entries via a .([]any) type assertion), the three truncation
// scalars all false/zero, and a remediation string naming
// backlogit_abandon_checkpoint as the originating verb (147-F / U7).
func TestU7_HandleAbandonNonConformingReturnsUnknownFields(t *testing.T) {
	s, ws := setupBugFixServer(t)
	ctx := context.Background()
	name := "checkpoint-u7-nonconforming.json"
	writeCheckpointFileMCP(t, ws.RootPath, name,
		`{"schema_version":1,"agent":"ship","session_id":"s1","phase":"build","status":"active",`+
			`"created_at":"2026-08-01T00:00:00Z","updated_at":"2026-08-01T00:00:00Z","extra_key":"x"}`)

	request := mcplib.CallToolRequest{}
	request.Params.Name = "backlogit_abandon_checkpoint"
	request.Params.Arguments = map[string]any{
		"filename": name,
		"reason":   "non-conforming",
		"operator": "tester@example.com",
	}

	result, err := s.handleAbandonCheckpoint(ctx, request)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.True(t, result.IsError)
	tc, ok := result.Content[0].(mcplib.TextContent)
	require.True(t, ok)
	var resp map[string]any
	require.NoError(t, json.Unmarshal([]byte(tc.Text), &resp))

	assert.Equal(t, "checkpoint_non_conforming", resp["code"])
	fields, ok := resp["unknown_fields"].([]any)
	require.True(t, ok, "unknown_fields must be a raw array read through .([]any)")
	assert.Contains(t, fields, "extra_key")
	for _, f := range fields {
		s, isString := f.(string)
		require.True(t, isString)
		assert.NotContains(t, s, `"`, "unknown_fields entries must be raw, never quoted")
	}
	assert.Equal(t, false, resp["unknown_fields_truncated"])
	assert.Equal(t, float64(0), resp["unknown_fields_omitted"])
	assert.Equal(t, float64(0), resp["unknown_fields_shortened"])
	remediation, ok := resp["remediation"].(string)
	require.True(t, ok)
	assert.Contains(t, remediation, "backlogit_abandon_checkpoint",
		"remediation must name the originating verb derived from op, not a hardcoded verb")
}

// TestU7_ConformingRefusalStillReportsAllFourUnknownFieldsKeys asserts a
// disposition refusal unrelated to non-conformance (e.g.
// checkpoint_use_abandon) still reports unknown_fields: [] and all three
// truncation scalars present, rather than omitting any of the four keys
// (147-F / U7: none of the four fields carry omitempty).
func TestU7_ConformingRefusalStillReportsAllFourUnknownFieldsKeys(t *testing.T) {
	s, ws := setupBugFixServer(t)
	ctx := context.Background()
	name := "checkpoint-u7-conforming-refusal.json"
	writeCheckpointFileMCP(t, ws.RootPath, name,
		`{"schema_version":1,"agent":"ship","session_id":"s1","phase":"build","status":"active",`+
			`"created_at":"2026-08-01T00:00:00Z","updated_at":"2026-08-01T00:00:00Z"}`)

	request := mcplib.CallToolRequest{}
	request.Params.Name = "backlogit_quarantine_checkpoint"
	request.Params.Arguments = map[string]any{
		"filename": name,
		"reason":   "x",
		"operator": "tester@example.com",
	}

	result, err := s.handleQuarantineCheckpoint(ctx, request)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.True(t, result.IsError, "a conforming active document is refused by quarantine (use abandon)")
	tc, ok := result.Content[0].(mcplib.TextContent)
	require.True(t, ok)

	var raw map[string]json.RawMessage
	require.NoError(t, json.Unmarshal([]byte(tc.Text), &raw))
	for _, key := range []string{"unknown_fields", "unknown_fields_truncated", "unknown_fields_omitted", "unknown_fields_shortened"} {
		_, present := raw[key]
		assert.True(t, present, "key %q must be present even when the refusal is not non-conformance-related", key)
	}

	var resp map[string]any
	require.NoError(t, json.Unmarshal([]byte(tc.Text), &resp))
	assert.Equal(t, "checkpoint_use_abandon", resp["code"])
	fields, ok := resp["unknown_fields"].([]any)
	require.True(t, ok)
	assert.Empty(t, fields)
	assert.Equal(t, false, resp["unknown_fields_truncated"])
	assert.Equal(t, float64(0), resp["unknown_fields_omitted"])
	assert.Equal(t, float64(0), resp["unknown_fields_shortened"])
}

// TestU7d_ResolveSchemaInvalidRoutesThroughDispositionShape asserts
// handleResolveCheckpoint on a schema-invalid legacy document returns
// code:checkpoint_use_quarantine with a populated filename and a
// remediation string naming backlogit_resolve_checkpoint — not
// backlogit_abandon_checkpoint — as the originating verb, and that the
// payload is not the generic "internal" shape (147-F / U7d).
func TestU7d_ResolveSchemaInvalidRoutesThroughDispositionShape(t *testing.T) {
	s, ws := setupBugFixServer(t)
	ctx := context.Background()
	name := "checkpoint-u7d-invalid.json"
	writeCheckpointFileMCP(t, ws.RootPath, name, `{"status":"active"}`)

	request := mcplib.CallToolRequest{}
	request.Params.Name = "backlogit_resolve_checkpoint"
	request.Params.Arguments = map[string]any{"filename": name}

	result, err := s.handleResolveCheckpoint(ctx, request)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.True(t, result.IsError)
	tc, ok := result.Content[0].(mcplib.TextContent)
	require.True(t, ok)
	var resp map[string]any
	require.NoError(t, json.Unmarshal([]byte(tc.Text), &resp))

	assert.NotEqual(t, "internal", resp["error"])
	assert.Equal(t, "checkpoint_use_quarantine", resp["code"])
	assert.Equal(t, name, resp["filename"])
	remediation, ok := resp["remediation"].(string)
	require.True(t, ok)
	assert.Contains(t, remediation, "backlogit_resolve_checkpoint")
	assert.NotContains(t, remediation, "backlogit_abandon_checkpoint")
}

// TestU7d_ResolveNonConformingRoutesThroughDispositionShape asserts
// handleResolveCheckpoint on a valid-but-non-conforming document returns
// code:checkpoint_non_conforming with a non-empty unknown_fields and a
// remediation string naming backlogit_resolve_checkpoint as the
// originating verb (147-F / U7d).
func TestU7d_ResolveNonConformingRoutesThroughDispositionShape(t *testing.T) {
	s, ws := setupBugFixServer(t)
	ctx := context.Background()
	name := "checkpoint-u7d-nonconforming.json"
	writeCheckpointFileMCP(t, ws.RootPath, name,
		`{"schema_version":1,"agent":"ship","session_id":"s1","phase":"build","status":"active",`+
			`"created_at":"2026-08-01T00:00:00Z","updated_at":"2026-08-01T00:00:00Z","extra_key":"x"}`)

	request := mcplib.CallToolRequest{}
	request.Params.Name = "backlogit_resolve_checkpoint"
	request.Params.Arguments = map[string]any{"filename": name}

	result, err := s.handleResolveCheckpoint(ctx, request)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.True(t, result.IsError)
	tc, ok := result.Content[0].(mcplib.TextContent)
	require.True(t, ok)
	var resp map[string]any
	require.NoError(t, json.Unmarshal([]byte(tc.Text), &resp))

	assert.Equal(t, "checkpoint_non_conforming", resp["code"])
	fields, ok := resp["unknown_fields"].([]any)
	require.True(t, ok)
	assert.NotEmpty(t, fields)
	remediation, ok := resp["remediation"].(string)
	require.True(t, ok)
	assert.Contains(t, remediation, "backlogit_resolve_checkpoint")
}

// TestU7dGuard_ResolveMissingFileStillReturnsNotFound pins case 3: a
// missing file still returns the pre-existing not-found refusal, not
// internal, after the routing change (147-F / U7d).
func TestU7dGuard_ResolveMissingFileStillReturnsNotFound(t *testing.T) {
	s, _ := setupBugFixServer(t)
	ctx := context.Background()

	request := mcplib.CallToolRequest{}
	request.Params.Name = "backlogit_resolve_checkpoint"
	request.Params.Arguments = map[string]any{"filename": "checkpoint-u7d-missing.json"}

	result, err := s.handleResolveCheckpoint(ctx, request)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.True(t, result.IsError)
	tc, ok := result.Content[0].(mcplib.TextContent)
	require.True(t, ok)
	var resp map[string]any
	require.NoError(t, json.Unmarshal([]byte(tc.Text), &resp))
	assert.Equal(t, "not_found", resp["error"])
}
