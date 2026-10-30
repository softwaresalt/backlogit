package parity_test

// 156.002-T (U2): cross-surface comparator + divergence report harness.
//
// These tests drive parity.CompareResults with hand-crafted three-surface
// results ({CLI, MCP, Internal}) that model real surface output shapes, and
// assert the dimension-aware comparator:
//   1. reports every applicable dimension as pass for identical behavior;
//   2. detects and FAILS an injected divergence in each applicable dimension;
//   3. classifies the KNOWN gate-payload drift (CLI --json omits remediation +
//      retry_after_ms that MCP emits) as report_only with the exact field-path
//      set {remediation, retry_after_ms} pinned to TrackedDefect 166-F;
//   4. FAILS CLOSED on a new/unrelated divergence layered on the known drift;
//   5. emits an EvidenceArtifact that validates against the U4a contract.

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/cli"
	corerrors "github.com/softwaresalt/backlogit/internal/errors"
	"github.com/softwaresalt/backlogit/internal/faultline"
	"github.com/softwaresalt/backlogit/internal/mcp"
	"github.com/softwaresalt/backlogit/internal/faultline/parity"
)

// surfaceBody is a convenience builder for a JSON surface body.
type surfaceBody map[string]any

func body(t *testing.T, b surfaceBody) []byte {
	t.Helper()
	raw, err := json.Marshal(b)
	require.NoError(t, err)
	return raw
}

// identicalPostState is a shared, bumped post-state projection.
func identicalPostState() surfaceBody {
	return surfaceBody{
		"created_at":      "2026-01-01T00:00:00Z",
		"updated_at":      "2026-01-02T00:00:00Z",
		"seed_updated_at": "2026-01-01T00:00:00Z",
	}
}

// identicalResults builds three semantically identical MUTATING surface results
// so every dimension is applicable and should pass.
func identicalResults(t *testing.T) [3]parity.SurfaceResult {
	t.Helper()
	response := surfaceBody{"id": "F-1", "title": "Seed feature", "status": "active"}

	cli := body(t, surfaceBody{
		"response":   response,
		"force":      false,
		"post_state": identicalPostState(),
	})
	mcp := body(t, surfaceBody{
		"response":   response,
		"retryable":  false,
		"force":      false,
		"post_state": identicalPostState(),
	})
	internal := body(t, surfaceBody{
		"response":   response,
		"force":      false,
		"post_state": identicalPostState(),
	})
	return [3]parity.SurfaceResult{
		{ExitCode: 0, Body: cli, PostStatePath: "/cli"},
		{ExitCode: 0, Body: mcp, PostStatePath: "/mcp"},
		{ExitCode: 0, Body: internal, PostStatePath: "/internal"},
	}
}

// gateDriftResults models the KNOWN gate-payload drift: a block-family gate
// where MCP emits remediation + retry_after_ms and the CLI --json payload omits
// both. Every other dimension agrees.
func gateDriftResults(t *testing.T) [3]parity.SurfaceResult {
	t.Helper()
	response := surfaceBody{"id": "T-9", "status": "blocked"}

	cli := body(t, surfaceBody{
		"error":      "task is blocked by an open dependency",
		"retryable":  true,
		"force":      false,
		"response":   response,
		"post_state": identicalPostState(),
	})
	mcp := body(t, surfaceBody{
		"error":          "blocked",
		"message":        "task is blocked by an open dependency",
		"remediation":    "resolve the blocking dependency then retry",
		"retry_after_ms": 5000,
		"retryable":      true,
		"force":          false,
		"response":       response,
		"post_state":     identicalPostState(),
	})
	internal := body(t, surfaceBody{
		"retryable":  true,
		"force":      false,
		"response":   response,
		"post_state": identicalPostState(),
	})
	return [3]parity.SurfaceResult{
		{ExitCode: 6, Body: cli, PostStatePath: "/cli"},
		{ExitCode: 6, Body: mcp, PostStatePath: "/mcp"},
		{ExitCode: 0, Body: internal, PostStatePath: "/internal"},
	}
}

func findDimension(t *testing.T, report parity.ComparisonReport, name string) parity.DimensionResult {
	t.Helper()
	for _, d := range report.Dimensions {
		if d.Dimension == name {
			return d
		}
	}
	t.Fatalf("dimension %q not present in report", name)
	return parity.DimensionResult{}
}

func TestU2ComparatorIdenticalAllPass(t *testing.T) {
	report, err := parity.CompareResults(context.Background(), "identical-mutation", identicalResults(t))
	require.NoError(t, err)
	require.NotEmpty(t, report.Dimensions)

	for _, d := range report.Dimensions {
		require.Containsf(t, []string{parity.StatusPass, parity.StatusNotApplicable}, d.Status,
			"dimension %q must be pass/not_applicable for identical results, got %q (%v)",
			d.Dimension, d.Status, d.DivergentFields)
		require.False(t, d.ExpectedDivergence, "dimension %q must not flag expected divergence", d.Dimension)
	}

	require.NotNil(t, report.Evidence)
	require.NoError(t, faultline.Validate(*report.Evidence))
	require.Equal(t, faultline.StatusPass, report.Evidence.Status)
}

func TestU2ComparatorInjectedDivergenceFails(t *testing.T) {
	t.Run("response_shape", func(t *testing.T) {
		results := identicalResults(t)
		results[0].Body = body(t, surfaceBody{
			"response":   surfaceBody{"id": "F-1", "title": "DIVERGENT title", "status": "active"},
			"force":      false,
			"post_state": identicalPostState(),
		})
		report, err := parity.CompareResults(context.Background(), "diverge-response", results)
		require.NoError(t, err)
		require.Equal(t, parity.StatusFail, findDimension(t, report, "response_shape").Status)
		require.Equal(t, faultline.StatusFail, report.Evidence.Status)
	})

	t.Run("durable_post_state_not_bumped", func(t *testing.T) {
		results := identicalResults(t)
		// CLI fails to bump updated_at (equal to seed) => under-persist caught.
		results[0].Body = body(t, surfaceBody{
			"response": surfaceBody{"id": "F-1", "title": "Seed feature", "status": "active"},
			"force":    false,
			"post_state": surfaceBody{
				"created_at":      "2026-01-01T00:00:00Z",
				"updated_at":      "2026-01-01T00:00:00Z",
				"seed_updated_at": "2026-01-01T00:00:00Z",
			},
		})
		report, err := parity.CompareResults(context.Background(), "diverge-poststate", results)
		require.NoError(t, err)
		d := findDimension(t, report, "durable_post_state")
		require.Equal(t, parity.StatusFail, d.Status)
		require.Contains(t, d.DivergentFields, "updated_at")
	})

	t.Run("force_lever", func(t *testing.T) {
		results := identicalResults(t)
		results[1].Body = body(t, surfaceBody{
			"response":   surfaceBody{"id": "F-1", "title": "Seed feature", "status": "active"},
			"force":      true,
			"post_state": identicalPostState(),
		})
		report, err := parity.CompareResults(context.Background(), "diverge-force", results)
		require.NoError(t, err)
		require.Equal(t, parity.StatusFail, findDimension(t, report, "force_lever").Status)
	})

	t.Run("exit_code_class", func(t *testing.T) {
		results := gateDriftResults(t)
		// MCP reports a governance category that is NOT in the CLI exit-6 block class.
		results[1].Body = body(t, surfaceBody{
			"error":          "governance",
			"message":        "task is blocked by an open dependency",
			"remediation":    "resolve the blocking dependency then retry",
			"retry_after_ms": 5000,
			"retryable":      true,
			"force":          false,
			"response":       surfaceBody{"id": "T-9", "status": "blocked"},
			"post_state":     identicalPostState(),
		})
		report, err := parity.CompareResults(context.Background(), "diverge-exit", results)
		require.NoError(t, err)
		require.Equal(t, parity.StatusFail, findDimension(t, report, "exit_code").Status)
	})
}

func TestU2ComparatorExpectedGateDrift(t *testing.T) {
	report, err := parity.CompareResults(context.Background(), "gate-block-json", gateDriftResults(t))
	require.NoError(t, err)

	// Gate-payload drift now fails closed: no report_only dimensions remain.
	failFields := map[string]struct{}{}
	for _, d := range report.Dimensions {
		require.NotEqual(t, parity.StatusReportOnly, d.Status,
			"dimension %q must not be report_only", d.Dimension)
		if d.Status == parity.StatusFail {
			require.False(t, d.ExpectedDivergence, "failed dimension %q must not flag expected divergence", d.Dimension)
			require.Empty(t, d.TrackedDefect, "failed dimension %q must not pin a tracked defect", d.Dimension)
			for _, f := range d.DivergentFields {
				failFields[f] = struct{}{}
			}
		}
	}
	require.Equal(t, parity.StatusFail, findDimension(t, report, "remediation").Status)
	require.Equal(t, parity.StatusFail, findDimension(t, report, "retryability").Status)

	// EXACT set equality on the divergent field-path union.
	require.Equal(t, map[string]struct{}{"remediation": {}, "retry_after_ms": {}}, failFields)

	require.NotNil(t, report.Evidence)
	require.NoError(t, faultline.Validate(*report.Evidence))
	require.Equal(t, faultline.StatusFail, report.Evidence.Status)
}

func TestU2ComparatorNewDivergenceFailsClosed(t *testing.T) {
	results := gateDriftResults(t)
	// Layer an UNRELATED divergence (force lever) on top of the known drift.
	results[2].Body = body(t, surfaceBody{
		"retryable":  true,
		"force":      true, // internal disagrees on the force lever
		"response":   surfaceBody{"id": "T-9", "status": "blocked"},
		"post_state": identicalPostState(),
	})
	report, err := parity.CompareResults(context.Background(), "gate-block-json-plus-drift", results)
	require.NoError(t, err)

	require.Equal(t, parity.StatusFail, findDimension(t, report, "force_lever").Status)
	require.Equal(t, faultline.StatusFail, report.Evidence.Status)
	require.False(t, report.Evidence.Status == faultline.StatusReportOnly)
}

func TestU2ComparatorEvidenceValidates(t *testing.T) {
	report, err := parity.CompareResults(context.Background(), "identical-mutation", identicalResults(t))
	require.NoError(t, err)
	require.NotNil(t, report.Evidence)

	// Round-trips through the U4a decode+validate contract.
	raw, err := json.Marshal(*report.Evidence)
	require.NoError(t, err)
	decoded, fp, err := faultline.DecodeAndValidate(raw)
	require.NoError(t, err)
	require.Equal(t, faultline.NodeFamilyParity, decoded.NodeFamily)
	_, ok := fp.(*faultline.ParityEvidence)
	require.True(t, ok, "evidence payload must be *ParityEvidence")
}

// TestU2ComparatorGetIntPresentNonFloat64TypeMismatch guards the numeric type
// -mismatch policy (supersedes the earlier F-C2 "treat as absent" behavior). A
// string "5000" for retry_after_ms is present-but-wrong-type, so getIntPresent
// reports it as a type mismatch and the retryability dimension fails CLOSED via
// the distinct "retry_after_ms:type_mismatch" marker rather than silently
// masking the malformed value as absent or as a zero value.
func TestU2ComparatorGetIntPresentNonFloat64TypeMismatch(t *testing.T) {
	response := surfaceBody{"id": "T-9", "status": "blocked"}
	post := identicalPostState()

	// CLI: no retry_after_ms field.
	cli := body(t, surfaceBody{
		"error":      "task is blocked by an open dependency",
		"retryable":  true,
		"force":      false,
		"response":   response,
		"post_state": post,
	})
	// MCP: retry_after_ms as a string "5000" (type mismatch — not a JSON number).
	mcp := body(t, surfaceBody{
		"error":          "blocked",
		"message":        "task is blocked by an open dependency",
		"retry_after_ms": "5000",
		"retryable":      true,
		"force":          false,
		"response":       response,
		"post_state":     post,
	})
	internal := body(t, surfaceBody{
		"retryable":  true,
		"force":      false,
		"response":   response,
		"post_state": post,
	})
	results := [3]parity.SurfaceResult{
		{ExitCode: 6, Body: cli, PostStatePath: "/cli"},
		{ExitCode: 6, Body: mcp, PostStatePath: "/mcp"},
		{ExitCode: 0, Body: internal, PostStatePath: "/internal"},
	}

	report, err := parity.CompareResults(context.Background(), "string-retry-after-ms", results)
	require.NoError(t, err)

	// The malformed numeric value must fail closed as a distinct type-mismatch
	// marker, and must NOT be folded into the tracked-drift "retry_after_ms"
	// field (which would masquerade as an expected divergence).
	d := findDimension(t, report, "retryability")
	require.Equal(t, parity.StatusFail, d.Status)
	require.Contains(t, d.DivergentFields, "retry_after_ms:type_mismatch")
	require.NotContains(t, d.DivergentFields, "retry_after_ms")
}

// arrayBody marshals a top-level JSON array surface body (e.g. the `list`
// command shape).
func arrayBody(t *testing.T, items []any) []byte {
	t.Helper()
	raw, err := json.Marshal(items)
	require.NoError(t, err)
	return raw
}

// TestU2ComparatorArrayBodyIdenticalPass verifies that surfaces whose body is a
// top-level JSON array are treated as PRESENT and compared, so three identical
// array bodies produce a parity pass rather than being treated as absent (which
// previously yielded "no applicable surfaces to attest").
func TestU2ComparatorArrayBodyIdenticalPass(t *testing.T) {
	items := []any{
		map[string]any{"id": "F-1", "title": "Alpha", "status": "active"},
		map[string]any{"id": "F-2", "title": "Beta", "status": "queued"},
	}
	b := arrayBody(t, items)
	results := [3]parity.SurfaceResult{
		{ExitCode: 0, Body: b, PostStatePath: "/cli"},
		{ExitCode: 0, Body: b, PostStatePath: "/mcp"},
		{ExitCode: 0, Body: b, PostStatePath: "/internal"},
	}

	report, err := parity.CompareResults(context.Background(), "list-array", results)
	require.NoError(t, err)
	require.NotNil(t, report.Evidence)

	for _, d := range report.Dimensions {
		require.Containsf(t, []string{parity.StatusPass, parity.StatusNotApplicable}, d.Status,
			"dimension %q must be pass/not_applicable for identical array bodies, got %q (%v)",
			d.Dimension, d.Status, d.DivergentFields)
	}
	require.Equal(t, faultline.StatusPass, report.Evidence.Status)
}

// TestU2ComparatorArrayBodyDivergesFailsClosed verifies that DIFFERING array
// bodies fail the response-shape dimension closed.
func TestU2ComparatorArrayBodyDivergesFailsClosed(t *testing.T) {
	base := arrayBody(t, []any{map[string]any{"id": "F-1", "status": "active"}})
	other := arrayBody(t, []any{map[string]any{"id": "F-1", "status": "DIVERGENT"}})
	results := [3]parity.SurfaceResult{
		{ExitCode: 0, Body: base, PostStatePath: "/cli"},
		{ExitCode: 0, Body: other, PostStatePath: "/mcp"},
		{ExitCode: 0, Body: base, PostStatePath: "/internal"},
	}

	report, err := parity.CompareResults(context.Background(), "list-array-diverge", results)
	require.NoError(t, err)
	require.Equal(t, parity.StatusFail, findDimension(t, report, "response_shape").Status)
	require.Equal(t, faultline.StatusFail, report.Evidence.Status)
}

// TestU2ComparatorErroredSurfaceFailsClosed verifies that a surface whose runner
// failed (SurfaceResult.Err != nil) is treated as "present but failed" and fails
// its applicable dimensions closed rather than being silently omitted (which
// would let a runner failure masquerade as a parity pass).
func TestU2ComparatorErroredSurfaceFailsClosed(t *testing.T) {
	results := identicalResults(t)
	// MCP would otherwise agree, but its runner failed.
	results[1].Err = errors.New("mcp runner failed to execute scenario")

	report, err := parity.CompareResults(context.Background(), "errored-surface", results)
	require.NoError(t, err)
	require.NotNil(t, report.Evidence)
	require.Equal(t, faultline.StatusFail, report.Evidence.Status,
		"an errored surface must fail closed, not pass")
}

// TestU2ComparatorBoolTypeMismatchFailsClosed verifies that a present-but-wrong
// -type boolean (e.g. retryable: "false") is a type mismatch that fails closed
// rather than being normalized to false and indistinguishable from absent.
func TestU2ComparatorBoolTypeMismatchFailsClosed(t *testing.T) {
	results := identicalResults(t)
	results[1].Body = body(t, surfaceBody{
		"response":   surfaceBody{"id": "F-1", "title": "Seed feature", "status": "active"},
		"retryable":  "false", // string, not bool: type mismatch
		"force":      false,
		"post_state": identicalPostState(),
	})

	report, err := parity.CompareResults(context.Background(), "bool-type-mismatch", results)
	require.NoError(t, err)
	d := findDimension(t, report, "retryability")
	require.Equal(t, parity.StatusFail, d.Status)
	require.Contains(t, d.DivergentFields, "retryable:type_mismatch")
}

// TestU2ComparatorNullBoolIsTypeMismatch verifies that an explicit JSON null
// value for `retryable` is treated as present-with-a-type-mismatch (fail
// closed), NOT as an absent key that normalizes to false. A missing key and a
// null value are semantically distinct: null IS present, just with the wrong
// type.
func TestU2ComparatorNullBoolIsTypeMismatch(t *testing.T) {
	results := identicalResults(t)
	results[1].Body = body(t, surfaceBody{
		"response":   surfaceBody{"id": "F-1", "title": "Seed feature", "status": "active"},
		"retryable":  nil, // JSON null: present-but-wrong-type, not absent
		"force":      false,
		"post_state": identicalPostState(),
	})

	report, err := parity.CompareResults(context.Background(), "null-bool-type-mismatch", results)
	require.NoError(t, err)
	d := findDimension(t, report, "retryability")
	require.Equal(t, parity.StatusFail, d.Status)
	require.Contains(t, d.DivergentFields, "retryable:type_mismatch")
}

// realGateErrorResults drives the REAL CLI renderer/exit mapper and the REAL MCP
// gate-error mapper for one GateError and returns the {CLI, MCP, Internal}
// surface results (internal intentionally empty: not a gate-error surface).
func realGateErrorResults(t *testing.T, ge *corerrors.GateError) [3]parity.SurfaceResult {
	t.Helper()
	cliJSON, err := cli.RenderGateErrorJSON(ge.ItemID, ge)
	require.NoError(t, err)
	exitErr := cli.GateExitError(ge)
	require.NotNil(t, exitErr)

	res, handled := mcp.GateErrorResult(ge, "done")
	require.True(t, handled)
	require.NotEmpty(t, res.Content)
	tc, ok := res.Content[0].(mcplib.TextContent)
	require.True(t, ok, "expected TextContent, got %T", res.Content[0])

	return [3]parity.SurfaceResult{
		{ExitCode: cli.ExitCodeFor(exitErr), Body: []byte(cliJSON)},
		{ExitCode: 1, Body: []byte(tc.Text)},
		{},
	}
}

// TestU2ComparatorRealGateErrorParityAllClasses verifies the real CLI and MCP
// gate-error payloads agree across every applicable dimension for all GateError
// classes, and that the comparator fails closed when a divergence is injected.
func TestU2ComparatorRealGateErrorParityAllClasses(t *testing.T) {
	cases := []struct {
		name  string
		class string
		retry int
	}{
		{"setup", "setup", 0},
		{"config", "config", 0},
		{"timeout_retry_after", "timeout", 1500},
		{"timeout_no_retry_after", "timeout", 0},
		{"in_progress_retry_after", "in_progress", 2000},
		{"in_progress_no_retry_after", "in_progress", 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ge := &corerrors.GateError{Class: tc.class, ItemID: "001.001-T", Message: "gate " + tc.class, RetryAfterMs: tc.retry}

			results := realGateErrorResults(t, ge)
			report, err := parity.CompareResults(context.Background(), "real-gate-error-"+tc.name, results)
			require.NoError(t, err)
			for _, d := range report.Dimensions {
				require.Containsf(t, []string{parity.StatusPass, parity.StatusNotApplicable}, d.Status,
					"dimension %q must pass for class %q, got %q (%v)", d.Dimension, tc.name, d.Status, d.DivergentFields)
			}
			require.Equal(t, faultline.StatusPass, report.Evidence.Status)

			t.Run("injected_remediation_mismatch_fails", func(t *testing.T) {
				if tc.class != "setup" && tc.class != "config" {
					t.Skip("remediation is only emitted for setup/config")
				}
				bad := realGateErrorResults(t, ge)
				bad[0].Body = injectField(t, bad[0].Body, "remediation", "DIVERGENT remediation")
				rep, err := parity.CompareResults(context.Background(), "inject-remediation-"+tc.name, bad)
				require.NoError(t, err)
				require.Equal(t, parity.StatusFail, findDimension(t, rep, "remediation").Status)
			})

			t.Run("injected_retry_after_mismatch_fails", func(t *testing.T) {
				bad := realGateErrorResults(t, ge)
				bad[0].Body = injectField(t, bad[0].Body, "retry_after_ms", 99999)
				rep, err := parity.CompareResults(context.Background(), "inject-retry-"+tc.name, bad)
				require.NoError(t, err)
				require.Equal(t, parity.StatusFail, findDimension(t, rep, "retryability").Status)
			})
		})
	}
}

// injectField sets key=value in a JSON object body and re-marshals it.
func injectField(t *testing.T, raw []byte, key string, value any) []byte {
	t.Helper()
	var m map[string]any
	require.NoError(t, json.Unmarshal(raw, &m))
	m[key] = value
	out, err := json.Marshal(m)
	require.NoError(t, err)
	return out
}
