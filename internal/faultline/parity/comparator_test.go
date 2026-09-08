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
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/faultline"
	"github.com/softwaresalt/backlogit/internal/faultline/parity"
)

const trackedGatePayload = "166-F"

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

	// Collect every report_only dimension and the union of its declared fields.
	reportOnlyFields := map[string]struct{}{}
	sawReportOnly := false
	for _, d := range report.Dimensions {
		switch d.Status {
		case parity.StatusReportOnly:
			sawReportOnly = true
			require.True(t, d.ExpectedDivergence, "report_only dimension %q must flag expected divergence", d.Dimension)
			require.Equal(t, trackedGatePayload, d.TrackedDefect,
				"report_only dimension %q must pin the tracked defect", d.Dimension)
			for _, f := range d.DivergentFields {
				reportOnlyFields[f] = struct{}{}
			}
		case parity.StatusFail:
			t.Fatalf("dimension %q failed closed unexpectedly in the known-drift scenario: %v",
				d.Dimension, d.DivergentFields)
		}
	}
	require.True(t, sawReportOnly, "expected at least one report_only dimension for the known gate drift")

	// EXACT set equality on the declared field-path set.
	require.Equal(t, map[string]struct{}{"remediation": {}, "retry_after_ms": {}}, reportOnlyFields)

	// Evidence rolls up to report_only + tracked defect + exact field set.
	require.NotNil(t, report.Evidence)
	require.NoError(t, faultline.Validate(*report.Evidence))
	require.Equal(t, faultline.StatusReportOnly, report.Evidence.Status)
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

// TestU2ComparatorGetIntPresentNonFloat64TreatedAsAbsent guards against the
// F-C2 bug where getIntPresent returned (0, true) for any non-float64 value,
// masking the actual value. A string "5000" for retry_after_ms must be treated
// as absent (not (0, present)), so no spurious divergence is reported when the
// CLI side also has no retry_after_ms.
func TestU2ComparatorGetIntPresentNonFloat64TreatedAsAbsent(t *testing.T) {
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
	// With the fix, getIntPresent returns (0, false) so hasRetryAfter=false, matching CLI.
	// Before the fix, getIntPresent returned (0, true), causing a spurious divergence.
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

	// retry_after_ms with a non-float64 value must be treated as absent, so
	// the retryability dimension must NOT list retry_after_ms as a divergent field.
	d := findDimension(t, report, "retryability")
	for _, f := range d.DivergentFields {
		if f == "retry_after_ms" {
			t.Errorf("retry_after_ms: string value was treated as present (getIntPresent bug not fixed), want treated as absent")
		}
	}
}
