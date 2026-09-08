package parity

// This file implements U3: the recurring-failure corpus for the cross-surface
// golden parity harness (156.003-T). It seeds TYPED scenarios for the known
// recurring failure modes (typed-error → exit 1 collapse, domainError mapping
// drift, retryability disagreement, omitempty array loss) plus the ONE known
// tracked drift (the CLI --json gate payload omitting remediation +
// retry_after_ms that MCP emits), which is classified report_only under EXACT
// (dimension, field-path) set equality and pinned to a durable tracked defect.
//
// The corpus is simulation-based: each scenario hand-crafts the three surface
// results ({CLI, MCP, Internal}) that model the real surface output shapes and
// drives them through CompareResults so the comparator's classification is
// exercised directly. End-to-end surface execution (full CLI/MCP/Internal
// invocation via *Driver) is a later producer obligation; the *Driver argument
// is threaded through the Scenario.Run signature so that upgrade is additive.
//
// TrackedDefect resolution is a HARNESS responsibility (not the leaf
// internal/faultline package, which only validates syntax). ResolveTrackedDefect
// walks the workspace and rejects any ID that is nonexistent or terminal
// (archived/done/shipped/…), so a tracked defect can only anchor a report_only
// divergence while it is a CURRENT, NON-TERMINAL backlog item.

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/softwaresalt/backlogit/internal/faultline"
	"github.com/softwaresalt/backlogit/internal/models"
)

// Scenario describes a named parity test scenario in the recurring-failure
// corpus. Run executes the scenario and returns the three surface results;
// Validate asserts the comparator's classification of those results.
type Scenario struct {
	// Name identifies the scenario in test output.
	Name string
	// Description states which recurring failure this scenario seeds.
	Description string
	// Run executes the scenario and returns [3]SurfaceResult ({CLI, MCP,
	// Internal}). The *Driver is threaded through for a later end-to-end
	// upgrade; the current simulation-based corpus hand-crafts the results and
	// may be called with a nil driver.
	Run func(t *testing.T, d *Driver) [3]SurfaceResult
	// Validate checks the comparison report produced from Run's results.
	Validate func(t *testing.T, report ComparisonReport)
}

// nonTerminalStatuses is the set of workspace statuses under which a backlog
// item is still a CURRENT owner able to anchor a report_only tracked drift. Any
// status outside this set (done, accepted, rejected, archived, shipped,
// abandoned) is terminal and rejects the tracked-defect resolution.
var nonTerminalStatuses = map[string]struct{}{
	string(models.StatusQueued):  {},
	string(models.StatusActive):  {},
	string(models.StatusBlocked): {},
	string(models.StatusReview):  {},
}

// ErrTrackedDefectNotFound is returned when a tracked-defect ID resolves to no
// artifact in the workspace (e.g. a nonexistent ID such as 999-F).
var ErrTrackedDefectNotFound = errors.New("parity: tracked defect not found in workspace")

// ErrTrackedDefectTerminal is returned when a tracked-defect ID resolves to a
// TERMINAL artifact (archived/done/shipped/…) that can no longer own a
// report_only tracked drift.
var ErrTrackedDefectTerminal = errors.New("parity: tracked defect is terminal")

// LocateBacklogitDir walks up from startDir until it finds a directory that
// contains a `.backlogit` storage root, returning the absolute path to that
// `.backlogit` directory. It is how the harness anchors tracked-defect
// resolution to the CURRENT workspace rather than a cloned fixture.
func LocateBacklogitDir(startDir string) (string, error) {
	dir, err := filepath.Abs(startDir)
	if err != nil {
		return "", fmt.Errorf("parity: resolve start dir: %w", err)
	}
	for {
		candidate := filepath.Join(dir, ".backlogit")
		if info, statErr := os.Stat(candidate); statErr == nil && info.IsDir() {
			return candidate, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("parity: no .backlogit workspace above %q: %w", startDir, os.ErrNotExist)
		}
		dir = parent
	}
}

// ResolveTrackedDefect resolves a tracked-defect ID against the workspace
// rooted at backlogitDir and returns nil ONLY when the ID names a CURRENT,
// NON-TERMINAL backlog item. A nonexistent ID yields ErrTrackedDefectNotFound;
// a terminal (archived/done/shipped/…) ID yields ErrTrackedDefectTerminal.
//
// Resolution is filesystem-based (no SQLite mutation): it walks the storage
// root — including status-relocated directories such as archive — for the
// Markdown artifact whose frontmatter `id` matches, then inspects its `status`.
func ResolveTrackedDefect(backlogitDir, id string) error {
	status, found, err := lookupStatus(backlogitDir, id)
	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("%w: %s", ErrTrackedDefectNotFound, id)
	}
	if _, ok := nonTerminalStatuses[status]; !ok {
		return fmt.Errorf("%w: %s is %q", ErrTrackedDefectTerminal, id, status)
	}
	return nil
}

// lookupStatus walks backlogitDir for the Markdown artifact whose frontmatter
// `id` equals id, returning its `status`, whether it was found, and any walk
// error. Non-Markdown files and unparsable Markdown are skipped, so log/journal
// files never masquerade as artifacts.
func lookupStatus(backlogitDir, id string) (string, bool, error) {
	var status string
	found := false
	walkErr := filepath.WalkDir(backlogitDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || filepath.Ext(path) != ".md" {
			return nil
		}
		raw, readErr := os.ReadFile(path) //nolint:gosec // path is a workspace-owned artifact under backlogitDir.
		if readErr != nil {
			return nil // skip unreadable files, continue walking
		}
		fm, _, parseErr := models.ParseFrontmatter(string(raw))
		if parseErr != nil || fm == nil {
			return nil
		}
		if gotID, _ := fm["id"].(string); gotID == id {
			status, _ = fm["status"].(string)
			found = true
			return filepath.SkipAll
		}
		return nil
	})
	if walkErr != nil {
		return "", false, fmt.Errorf("parity: walk workspace for %s: %w", id, walkErr)
	}
	return status, found, nil
}

// SeedCorpus returns the full recurring-failure corpus in a deterministic order.
func SeedCorpus() []Scenario {
	return []Scenario{
		TypedErrorCollapseScenario(),
		DomainErrorDriftScenario(),
		RetryabilityDisagreementScenario(),
		OmitemptyArrayLossScenario(),
		KnownGatePayloadDriftScenario(),
	}
}

// bumpedPostState is a shared, VALID + bumped durable post-state projection so
// the durable_post_state dimension passes and never manufactures a false
// divergence in a corpus scenario that is not about post-state.
func bumpedPostState() map[string]any {
	return map[string]any{
		"created_at":      "2026-01-01T00:00:00Z",
		"updated_at":      "2026-01-02T00:00:00Z",
		"seed_updated_at": "2026-01-01T00:00:00Z",
	}
}

// marshalBody encodes a surface body map to JSON, failing the test on error.
func marshalBody(t *testing.T, m map[string]any) []byte {
	t.Helper()
	raw, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("parity: marshal surface body: %v", err)
	}
	return raw
}

// dimensionByName returns the named dimension result from a report.
func dimensionByName(t *testing.T, report ComparisonReport, name string) DimensionResult {
	t.Helper()
	for _, d := range report.Dimensions {
		if d.Dimension == name {
			return d
		}
	}
	t.Fatalf("parity: dimension %q not present in report", name)
	return DimensionResult{}
}

// requireDimensionStatus asserts a dimension has the expected comparison status.
func requireDimensionStatus(t *testing.T, report ComparisonReport, name, want string) {
	t.Helper()
	got := dimensionByName(t, report, name)
	if got.Status != want {
		t.Fatalf("parity: dimension %q status = %q, want %q (fields=%v)",
			name, got.Status, want, got.DivergentFields)
	}
}

// requireEvidenceStatus asserts the emitted evidence envelope rolls up to the
// expected status.
func requireEvidenceStatus(t *testing.T, report ComparisonReport, want string) {
	t.Helper()
	if report.Evidence == nil {
		t.Fatalf("parity: report has no evidence artifact")
	}
	if err := faultline.Validate(*report.Evidence); err != nil {
		t.Fatalf("parity: evidence failed U4a validation: %v", err)
	}
	if report.Evidence.Status != want {
		t.Fatalf("parity: evidence status = %q, want %q", report.Evidence.Status, want)
	}
}

// TypedErrorCollapseScenario seeds the typed-error → exit 1 collapse: a typed
// gate error that SHOULD surface a governed exit (6/7/8/9) collapses to the
// generic exit 1 on the CLI while MCP still emits the typed block category.
// Seeded as FAIL (unexpected divergence).
func TypedErrorCollapseScenario() Scenario {
	return Scenario{
		Name:        "typed-error-exit-1-collapse",
		Description: "typed gate error collapses to CLI exit 1 instead of the governed 6/7/8/9 class",
		Run: func(t *testing.T, _ *Driver) [3]SurfaceResult {
			t.Helper()
			response := map[string]any{"id": "T-9", "status": "blocked"}
			cli := marshalBody(t, map[string]any{
				"error":      "task is blocked by an open dependency",
				"force":      false,
				"response":   response,
				"post_state": bumpedPostState(),
			})
			mcp := marshalBody(t, map[string]any{
				"error":      "blocked",
				"message":    "task is blocked by an open dependency",
				"force":      false,
				"response":   response,
				"post_state": bumpedPostState(),
			})
			internal := marshalBody(t, map[string]any{
				"force":      false,
				"response":   response,
				"post_state": bumpedPostState(),
			})
			return [3]SurfaceResult{
				{ExitCode: 1, Body: cli, PostStatePath: "/cli"}, // collapsed exit
				{ExitCode: 6, Body: mcp, PostStatePath: "/mcp"}, // typed block category
				{ExitCode: 0, Body: internal, PostStatePath: "/internal"},
			}
		},
		Validate: func(t *testing.T, report ComparisonReport) {
			t.Helper()
			requireDimensionStatus(t, report, dimExitCode, StatusFail)
			requireEvidenceStatus(t, report, faultline.StatusFail)
		},
	}
}

// DomainErrorDriftScenario seeds a domainError mapping drift: the same domain
// error is mapped to a DIFFERENT human message across CLI vs MCP while the exit
// class agrees. Seeded as FAIL (structured_error divergence) with exit_code
// still passing, distinguishing it from the typed-error collapse.
func DomainErrorDriftScenario() Scenario {
	return Scenario{
		Name:        "domain-error-mapping-drift",
		Description: "one domain error maps to divergent structured-error messages across CLI vs MCP",
		Run: func(t *testing.T, _ *Driver) [3]SurfaceResult {
			t.Helper()
			response := map[string]any{"id": "T-4", "status": "blocked"}
			cli := marshalBody(t, map[string]any{
				"error":      "policy denied: owner approval required",
				"force":      false,
				"response":   response,
				"post_state": bumpedPostState(),
			})
			mcp := marshalBody(t, map[string]any{
				"error":      "governance",
				"message":    "blocked by governance rule GR-7", // divergent mapping
				"force":      false,
				"response":   response,
				"post_state": bumpedPostState(),
			})
			internal := marshalBody(t, map[string]any{
				"force":      false,
				"response":   response,
				"post_state": bumpedPostState(),
			})
			return [3]SurfaceResult{
				{ExitCode: 9, Body: cli, PostStatePath: "/cli"}, // governance class
				{ExitCode: 9, Body: mcp, PostStatePath: "/mcp"}, // MCP category in class 9
				{ExitCode: 0, Body: internal, PostStatePath: "/internal"},
			}
		},
		Validate: func(t *testing.T, report ComparisonReport) {
			t.Helper()
			// Exit class agrees; the mapping drift surfaces in structured_error.
			requireDimensionStatus(t, report, dimExitCode, StatusPass)
			requireDimensionStatus(t, report, dimStructuredError, StatusFail)
			requireEvidenceStatus(t, report, faultline.StatusFail)
		},
	}
}

// RetryabilityDisagreementScenario seeds a retryability disagreement: one
// surface reports retryable=true while another reports retryable=false for the
// same transient failure. Seeded as FAIL.
func RetryabilityDisagreementScenario() Scenario {
	return Scenario{
		Name:        "retryability-disagreement",
		Description: "CLI and MCP disagree on retryable for the same transient failure",
		Run: func(t *testing.T, _ *Driver) [3]SurfaceResult {
			t.Helper()
			response := map[string]any{"id": "T-2", "status": "blocked"}
			cli := marshalBody(t, map[string]any{
				"error":      "transient timeout",
				"retryable":  true,
				"force":      false,
				"response":   response,
				"post_state": bumpedPostState(),
			})
			mcp := marshalBody(t, map[string]any{
				"error":      "retryable",
				"message":    "transient timeout",
				"retryable":  false, // disagreement
				"force":      false,
				"response":   response,
				"post_state": bumpedPostState(),
			})
			internal := marshalBody(t, map[string]any{
				"retryable":  false,
				"force":      false,
				"response":   response,
				"post_state": bumpedPostState(),
			})
			return [3]SurfaceResult{
				{ExitCode: 8, Body: cli, PostStatePath: "/cli"},
				{ExitCode: 8, Body: mcp, PostStatePath: "/mcp"},
				{ExitCode: 0, Body: internal, PostStatePath: "/internal"},
			}
		},
		Validate: func(t *testing.T, report ComparisonReport) {
			t.Helper()
			d := dimensionByName(t, report, dimRetryability)
			if d.Status != StatusFail {
				t.Fatalf("parity: retryability status = %q, want fail (fields=%v)", d.Status, d.DivergentFields)
			}
			if !containsField(d.DivergentFields, "retryable") {
				t.Fatalf("parity: retryability divergent fields = %v, want to include retryable", d.DivergentFields)
			}
			requireEvidenceStatus(t, report, faultline.StatusFail)
		},
	}
}

// OmitemptyArrayLossScenario seeds an omitempty array loss: the CLI surface
// emits an empty array field (labels: []) that MCP/Internal drop under
// omitempty, so the emitted-field set diverges. Seeded as FAIL in the
// serialization dimension.
func OmitemptyArrayLossScenario() Scenario {
	return Scenario{
		Name:        "omitempty-array-loss",
		Description: "one surface omits an empty array field others emit (serialization drift)",
		Run: func(t *testing.T, _ *Driver) [3]SurfaceResult {
			t.Helper()
			withLabels := map[string]any{"id": "F-1", "status": "active", "labels": []any{}}
			withoutLabels := map[string]any{"id": "F-1", "status": "active"}
			cli := marshalBody(t, map[string]any{
				"force":      false,
				"response":   withLabels,
				"post_state": bumpedPostState(),
			})
			mcp := marshalBody(t, map[string]any{
				"retryable":  false,
				"force":      false,
				"response":   withoutLabels, // omitempty dropped the empty array
				"post_state": bumpedPostState(),
			})
			internal := marshalBody(t, map[string]any{
				"force":      false,
				"response":   withoutLabels,
				"post_state": bumpedPostState(),
			})
			return [3]SurfaceResult{
				{ExitCode: 0, Body: cli, PostStatePath: "/cli"},
				{ExitCode: 0, Body: mcp, PostStatePath: "/mcp"},
				{ExitCode: 0, Body: internal, PostStatePath: "/internal"},
			}
		},
		Validate: func(t *testing.T, report ComparisonReport) {
			t.Helper()
			d := dimensionByName(t, report, dimSerialization)
			if d.Status != StatusFail {
				t.Fatalf("parity: serialization status = %q, want fail (fields=%v)", d.Status, d.DivergentFields)
			}
			if !containsField(d.DivergentFields, "labels") {
				t.Fatalf("parity: serialization divergent fields = %v, want to include labels", d.DivergentFields)
			}
			requireEvidenceStatus(t, report, faultline.StatusFail)
		},
	}
}

// KnownGatePayloadDriftScenario seeds the ONE known, tracked drift: the CLI
// --json gate-error payload OMITS remediation and retry_after_ms that MCP
// emits. It is classified report_only (NOT fail) under EXACT (dimension,
// field-path) set equality = {remediation, retry_after_ms}, pinned to the
// tracked defect 166-F. A NEW or ADDITIONAL diverging field fails closed.
func KnownGatePayloadDriftScenario() Scenario {
	return Scenario{
		Name:        "known-gate-payload-drift",
		Description: "CLI --json gate payload omits remediation + retry_after_ms MCP emits (tracked 166-F)",
		Run: func(t *testing.T, _ *Driver) [3]SurfaceResult {
			t.Helper()
			response := map[string]any{"id": "T-9", "status": "blocked"}
			cli := marshalBody(t, map[string]any{
				"error":      "task is blocked by an open dependency",
				"retryable":  true,
				"force":      false,
				"response":   response,
				"post_state": bumpedPostState(),
			})
			mcp := marshalBody(t, map[string]any{
				"error":          "blocked",
				"message":        "task is blocked by an open dependency",
				"remediation":    "resolve the blocking dependency then retry",
				"retry_after_ms": 5000,
				"retryable":      true,
				"force":          false,
				"response":       response,
				"post_state":     bumpedPostState(),
			})
			internal := marshalBody(t, map[string]any{
				"retryable":  true,
				"force":      false,
				"response":   response,
				"post_state": bumpedPostState(),
			})
			return [3]SurfaceResult{
				{ExitCode: 6, Body: cli, PostStatePath: "/cli"},
				{ExitCode: 6, Body: mcp, PostStatePath: "/mcp"},
				{ExitCode: 0, Body: internal, PostStatePath: "/internal"},
			}
		},
		Validate: func(t *testing.T, report ComparisonReport) {
			t.Helper()
			reportOnlyFields := map[string]struct{}{}
			sawReportOnly := false
			for _, d := range report.Dimensions {
				switch d.Status {
				case StatusReportOnly:
					sawReportOnly = true
					if !d.ExpectedDivergence {
						t.Fatalf("parity: report_only dimension %q must flag expected divergence", d.Dimension)
					}
					if d.TrackedDefect != TrackedDefectGatePayload {
						t.Fatalf("parity: report_only dimension %q tracked defect = %q, want %q",
							d.Dimension, d.TrackedDefect, TrackedDefectGatePayload)
					}
					for _, f := range d.DivergentFields {
						reportOnlyFields[f] = struct{}{}
					}
				case StatusFail:
					t.Fatalf("parity: dimension %q failed closed in the known-drift scenario: %v",
						d.Dimension, d.DivergentFields)
				}
			}
			if !sawReportOnly {
				t.Fatalf("parity: expected at least one report_only dimension for the known gate drift")
			}
			// EXACT set equality on the declared field-path set.
			want := map[string]struct{}{"remediation": {}, "retry_after_ms": {}}
			if !sameFieldSet(reportOnlyFields, want) {
				t.Fatalf("parity: report_only field set = %v, want %v", keysOf(reportOnlyFields), keysOf(want))
			}
			requireEvidenceStatus(t, report, faultline.StatusReportOnly)
		},
	}
}

// containsField reports whether xs contains f.
func containsField(xs []string, f string) bool {
	for _, x := range xs {
		if x == f {
			return true
		}
	}
	return false
}

// sameFieldSet reports set equality of two string sets.
func sameFieldSet(a, b map[string]struct{}) bool {
	if len(a) != len(b) {
		return false
	}
	for k := range a {
		if _, ok := b[k]; !ok {
			return false
		}
	}
	return true
}

// keysOf returns the keys of a set for diagnostics.
func keysOf(m map[string]struct{}) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
