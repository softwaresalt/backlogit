package parity

// This file implements U2: a dimension-aware cross-surface comparator that
// compares the three surface results ({CLI, MCP, Internal}) of one governed
// scenario, produces a per-dimension divergence report, and emits a validated
// EvidenceArtifact (NodeFamily=parity) conforming to the U4a contract.
//
// The comparator is applicability-aware: each dimension declares the surfaces
// it applies to, and comparison happens ONLY where applicable so a
// legitimately-absent surface never manufactures a false divergence. A KNOWN,
// tracked drift (the CLI --json gate payload omitting remediation and
// retry_after_ms that MCP emits) is classified as a report_only
// expected-divergence with EXACT (dimension, field-path) set equality; any
// unrelated or additional divergence fails closed.

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"time"

	"github.com/softwaresalt/backlogit/internal/faultline"
)

// Comparison status values for a single dimension. StatusPass/ReportOnly/Fail
// mirror the envelope statuses; StatusNotApplicable marks a dimension that does
// not apply to the current scenario (e.g. a read-only scenario has no durable
// post-state to compare) and NEVER contributes a false divergence.
const (
	// StatusPass indicates the applicable surfaces agree on the dimension.
	StatusPass = "pass"
	// StatusReportOnly indicates a known, tracked expected divergence.
	StatusReportOnly = "report_only"
	// StatusFail indicates an unexpected (or newly-introduced) divergence.
	StatusFail = "fail"
	// StatusNotApplicable indicates the dimension does not apply to the scenario.
	StatusNotApplicable = "not_applicable"
)

// TrackedDefectGatePayload is the canonical tracked-defect ID owning the known
// CLI --json gate-payload omission drift (remediation + retry_after_ms).
const TrackedDefectGatePayload = "156.007-T"

// producingCommit identifies the commit that produced comparator evidence. It
// only needs to be non-empty for the U4a envelope invariants.
const producingCommit = "c5ba5260"

// Dimension identifiers.
const (
	dimExitCode         = "exit_code"
	dimStructuredError  = "structured_error"
	dimDurablePostState = "durable_post_state"
	dimResponseShape    = "response_shape"
	dimRetryability     = "retryability"
	dimRemediation      = "remediation"
	dimSerialization    = "serialization"
	dimForceLever       = "force_lever"
)

// Surface indices into a [3]SurfaceResult ({CLI, MCP, Internal}).
const (
	surfaceCLI      = 0
	surfaceMCP      = 1
	surfaceInternal = 2
)

// surfaceNames maps a surface index to its governed identifier.
var surfaceNames = [3]string{"cli", "mcp", "internal"}

// DimensionResult holds the comparison result for one scenario dimension.
type DimensionResult struct {
	Dimension          string
	Status             string
	ApplicableSurfaces []string
	DivergentFields    []string
	ExpectedDivergence bool
	TrackedDefect      string
	Detail             string
}

// ComparisonReport is the full cross-surface comparison result plus the emitted
// and validated evidence artifact.
type ComparisonReport struct {
	ScenarioID string
	Dimensions []DimensionResult
	Evidence   *faultline.EvidenceArtifact
}

// driftDecl declares the EXACT divergent field-path set a dimension is allowed
// to drift on under a tracked defect. Any deviation (missing or additional
// field) fails closed.
type driftDecl struct {
	fields  []string
	tracked string
	detail  string
}

// knownDrifts is the registry of tracked, expected divergences. The CLI --json
// gate payload omits remediation (owned by the remediation dimension) and
// retry_after_ms (owned by the retryability dimension); both are pinned to the
// single tracked defect 156.007-T.
var knownDrifts = map[string]driftDecl{
	dimRemediation: {
		fields:  []string{"remediation"},
		tracked: TrackedDefectGatePayload,
		detail:  "CLI --json gate payload omits remediation emitted by MCP (tracked drift)",
	},
	dimRetryability: {
		fields:  []string{"retry_after_ms"},
		tracked: TrackedDefectGatePayload,
		detail:  "CLI --json gate payload omits retry_after_ms emitted by MCP (tracked drift)",
	},
}

// exitCodeClasses maps a CLI exit code to its COARSE equivalence class of MCP
// machine categories. The comparator asserts the MCP category is IN the class
// rather than exactly 1:1, because ExitCodeFor is coarse and the CLI payload
// carries no machine category.
var exitCodeClasses = map[int]map[string]struct{}{
	0: {"": {}, "success": {}, "ok": {}},
	6: {"blocked": {}, "block": {}, "dependency_blocked": {}, "block_family": {}},
	7: {"config": {}, "setup": {}, "configuration": {}},
	8: {"retryable": {}, "timeout": {}, "in_progress": {}},
	9: {"governance": {}, "policy": {}},
}

// surfaceProjection is the normalized, comparable projection of one surface's
// body. Presence flags let the comparator apply absent==false / omitempty
// normalization without conflating "absent" with a zero value.
type surfaceProjection struct {
	present        bool
	exitCode       int
	category       string // machine category (CLI: derived from exit bucket, MCP: error field)
	humanMessage   string // human message (CLI: error field, MCP: message field)
	remediation    string
	hasRemediation bool
	retryable      bool
	retryAfterMs   int
	hasRetryAfter  bool
	force          bool
	response       map[string]any
	responseKeys   []string
	post           *postProjection
}

// postProjection is the durable post-state timestamp projection.
type postProjection struct {
	createdAt     string
	updatedAt     string
	seedUpdatedAt string
}

// CompareResults compares three surface results ({CLI, MCP, Internal}) for one
// governed scenario and returns a dimension-aware divergence report plus a
// validated EvidenceArtifact.
func CompareResults(ctx context.Context, scenarioID string, results [3]SurfaceResult) (ComparisonReport, error) {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return ComparisonReport{}, fmt.Errorf("parity: comparison cancelled: %w", err)
		}
	}

	proj := [3]surfaceProjection{
		projectCLI(results[surfaceCLI]),
		projectMCP(results[surfaceMCP]),
		projectInternal(results[surfaceInternal]),
	}

	dims := []DimensionResult{
		compareExitCode(proj),
		compareStructuredError(proj),
		compareDurablePostState(proj),
		compareResponseShape(proj),
		compareRetryability(proj),
		compareRemediation(proj),
		compareSerialization(proj),
		compareForceLever(proj),
	}

	report := ComparisonReport{ScenarioID: scenarioID, Dimensions: dims}

	evidence, err := buildEvidence(scenarioID, dims)
	if err != nil {
		return report, fmt.Errorf("parity: emit evidence: %w", err)
	}
	report.Evidence = evidence
	return report, nil
}

// classifyDimension applies the exact-set-equality known-drift policy and
// returns the finished DimensionResult. A dimension with no applicable surfaces
// is not_applicable; a dimension whose divergent field set exactly matches its
// declared known drift is report_only; any other divergence fails closed.
func classifyDimension(dim string, divergent, applicable []string, detail string) DimensionResult {
	r := DimensionResult{
		Dimension:          dim,
		ApplicableSurfaces: applicable,
		Detail:             faultline.SanitizeDiagnostic(detail),
	}
	if len(applicable) == 0 {
		r.Status = StatusNotApplicable
		return r
	}
	divergent = dedupeSort(divergent)
	if len(divergent) == 0 {
		r.Status = StatusPass
		return r
	}
	if d, ok := knownDrifts[dim]; ok && sameStringSet(divergent, d.fields) {
		r.Status = StatusReportOnly
		r.DivergentFields = divergent
		r.ExpectedDivergence = true
		r.TrackedDefect = d.tracked
		if detail == "" {
			r.Detail = faultline.SanitizeDiagnostic(d.detail)
		}
		return r
	}
	r.Status = StatusFail
	r.DivergentFields = divergent
	return r
}

// compareExitCode checks the CLI-only exit code against the MCP machine
// category via the COARSE equivalence class (not exact 1:1). It is never folded
// into an expected divergence.
func compareExitCode(proj [3]surfaceProjection) DimensionResult {
	cli, mcp := proj[surfaceCLI], proj[surfaceMCP]
	if !cli.present || !mcp.present {
		return classifyDimension(dimExitCode, nil, nil, "")
	}
	applicable := []string{surfaceNames[surfaceCLI], surfaceNames[surfaceMCP]}
	var divergent []string
	if !inExitClass(cli.exitCode, mcp.category) {
		divergent = append(divergent, "exit_code_class")
	}
	detail := fmt.Sprintf("cli exit %d vs mcp category %q", cli.exitCode, mcp.category)
	return classifyDimension(dimExitCode, divergent, applicable, detail)
}

// compareStructuredError normalizes the CLI/MCP structured error data via
// explicit field mapping: CLI error(human) maps to MCP message, and the
// CLI-derived category maps to MCP error (class-consistent, not literal).
func compareStructuredError(proj [3]surfaceProjection) DimensionResult {
	cli, mcp := proj[surfaceCLI], proj[surfaceMCP]
	if !cli.present || !mcp.present {
		return classifyDimension(dimStructuredError, nil, nil, "")
	}
	applicable := []string{surfaceNames[surfaceCLI], surfaceNames[surfaceMCP]}
	var divergent []string
	if cli.humanMessage != mcp.humanMessage {
		divergent = append(divergent, "message")
	}
	if !inExitClass(cli.exitCode, mcp.category) {
		divergent = append(divergent, "category")
	}
	return classifyDimension(dimStructuredError, divergent, applicable, "")
}

// compareDurablePostState compares the durable post-state through a
// deterministic semantic projection: timestamps are asserted PRESENT, VALID,
// updated_at BUMPED vs seed, and updated_at >= created_at. It is not_applicable
// only for genuinely read-only scenarios (no surface captured a post-state).
func compareDurablePostState(proj [3]surfaceProjection) DimensionResult {
	var applicable []string
	var divergent []string
	anyPostState := false
	for i := range proj {
		p := proj[i]
		if !p.present || p.post == nil {
			continue
		}
		anyPostState = true
		applicable = append(applicable, surfaceNames[i])
		for _, f := range validatePostState(p.post) {
			divergent = append(divergent, fmt.Sprintf("%s.%s", surfaceNames[i], f))
			// Also surface the bare field so callers can assert on it directly.
			divergent = append(divergent, f)
		}
	}
	if !anyPostState {
		return classifyDimension(dimDurablePostState, nil, nil, "")
	}
	return classifyDimension(dimDurablePostState, divergent, applicable, "")
}

// validatePostState returns the field names that violate the timestamp
// semantic projection for one surface's post-state.
func validatePostState(p *postProjection) []string {
	var bad []string
	created, createdOK := parseTime(p.createdAt)
	updated, updatedOK := parseTime(p.updatedAt)
	if !createdOK {
		bad = append(bad, "created_at")
	}
	if !updatedOK {
		bad = append(bad, "updated_at")
		return bad
	}
	if createdOK && updated.Before(created) {
		bad = append(bad, "updated_at")
	}
	// updated_at MUST be bumped vs the seed baseline: a surface that fails to
	// bump it (under-persist) is caught rather than normalized away.
	if p.seedUpdatedAt != "" && p.updatedAt == p.seedUpdatedAt {
		bad = append(bad, "updated_at")
	}
	return bad
}

// compareResponseShape compares the normalized business response across all
// present surfaces via field mapping (timestamps stripped), not literal
// equality.
func compareResponseShape(proj [3]surfaceProjection) DimensionResult {
	var applicable []string
	var present []surfaceProjection
	for i := range proj {
		if proj[i].present {
			applicable = append(applicable, surfaceNames[i])
			present = append(present, proj[i])
		}
	}
	if len(present) < 2 {
		return classifyDimension(dimResponseShape, nil, applicable, "")
	}
	base := normalizeResponse(present[0].response)
	divergentSet := map[string]struct{}{}
	for _, p := range present[1:] {
		other := normalizeResponse(p.response)
		for k := range unionKeys(base, other) {
			if !reflect.DeepEqual(base[k], other[k]) {
				divergentSet[k] = struct{}{}
			}
		}
	}
	return classifyDimension(dimResponseShape, keys(divergentSet), applicable, "")
}

// compareRetryability compares retryability across all three surfaces with
// absent==false normalization (CLI omitempty vs MCP always-emit), and the
// CLI/MCP retry_after_ms field (tracked drift lives here).
func compareRetryability(proj [3]surfaceProjection) DimensionResult {
	var applicable []string
	var present []surfaceProjection
	for i := range proj {
		if proj[i].present {
			applicable = append(applicable, surfaceNames[i])
			present = append(present, proj[i])
		}
	}
	if len(present) == 0 {
		return classifyDimension(dimRetryability, nil, nil, "")
	}
	var divergent []string
	// retryable: normalized absent==false across all present surfaces.
	base := present[0].retryable
	for _, p := range present[1:] {
		if p.retryable != base {
			divergent = append(divergent, "retryable")
			break
		}
	}
	// retry_after_ms compared between CLI and MCP only.
	cli, mcp := proj[surfaceCLI], proj[surfaceMCP]
	if cli.present && mcp.present {
		if cli.hasRetryAfter != mcp.hasRetryAfter || cli.retryAfterMs != mcp.retryAfterMs {
			divergent = append(divergent, "retry_after_ms")
		}
	}
	return classifyDimension(dimRetryability, divergent, applicable, "")
}

// compareRemediation compares the remediation string across CLI --json + MCP
// only; the internal surface has no remediation and is not_applicable. The
// tracked drift (CLI omission) lives here.
func compareRemediation(proj [3]surfaceProjection) DimensionResult {
	cli, mcp := proj[surfaceCLI], proj[surfaceMCP]
	if !cli.present || !mcp.present {
		return classifyDimension(dimRemediation, nil, nil, "")
	}
	applicable := []string{surfaceNames[surfaceCLI], surfaceNames[surfaceMCP]}
	var divergent []string
	if cli.hasRemediation != mcp.hasRemediation || cli.remediation != mcp.remediation {
		divergent = append(divergent, "remediation")
	}
	return classifyDimension(dimRemediation, divergent, applicable, "")
}

// compareSerialization captures the emitted-field policy (omitempty vs
// always-emit) explicitly by comparing the set of business response keys
// emitted by each present surface.
func compareSerialization(proj [3]surfaceProjection) DimensionResult {
	var applicable []string
	var present []surfaceProjection
	for i := range proj {
		if proj[i].present {
			applicable = append(applicable, surfaceNames[i])
			present = append(present, proj[i])
		}
	}
	if len(present) < 2 {
		return classifyDimension(dimSerialization, nil, applicable, "")
	}
	base := stringSet(present[0].responseKeys)
	divergentSet := map[string]struct{}{}
	for _, p := range present[1:] {
		other := stringSet(p.responseKeys)
		for k := range mergeSets(base, other) {
			_, inBase := base[k]
			_, inOther := other[k]
			if inBase != inOther {
				divergentSet[k] = struct{}{}
			}
		}
	}
	return classifyDimension(dimSerialization, keys(divergentSet), applicable, "")
}

// compareForceLever compares the force lever symmetrically (present/absent
// normalized to false) across all three surfaces.
func compareForceLever(proj [3]surfaceProjection) DimensionResult {
	var applicable []string
	var present []surfaceProjection
	for i := range proj {
		if proj[i].present {
			applicable = append(applicable, surfaceNames[i])
			present = append(present, proj[i])
		}
	}
	if len(present) == 0 {
		return classifyDimension(dimForceLever, nil, nil, "")
	}
	var divergent []string
	base := present[0].force
	for _, p := range present[1:] {
		if p.force != base {
			divergent = append(divergent, "force")
			break
		}
	}
	return classifyDimension(dimForceLever, divergent, applicable, "")
}

// buildEvidence emits an aggregate ParityEvidence envelope for the report and
// validates it against the U4a contract by round-tripping through
// DecodeAndValidate.
func buildEvidence(scenarioID string, dims []DimensionResult) (*faultline.EvidenceArtifact, error) {
	surfaceSet := map[string]struct{}{}
	for _, d := range dims {
		for _, s := range d.ApplicableSurfaces {
			surfaceSet[s] = struct{}{}
		}
	}
	surfaces := dedupeSort(keys(surfaceSet))
	if len(surfaces) == 0 {
		return nil, fmt.Errorf("no applicable surfaces to attest")
	}

	failFields := map[string]struct{}{}
	reportFields := map[string]struct{}{}
	anyFail, anyReport := false, false
	tracked := ""
	for _, d := range dims {
		switch d.Status {
		case StatusFail:
			anyFail = true
			for _, f := range d.DivergentFields {
				failFields[f] = struct{}{}
			}
		case StatusReportOnly:
			anyReport = true
			tracked = d.TrackedDefect
			for _, f := range d.DivergentFields {
				reportFields[f] = struct{}{}
			}
		}
	}

	var status, detail string
	var divergent []string
	var expected bool
	switch {
	case anyFail:
		status = faultline.StatusFail
		divergent = dedupeSort(keys(failFields))
		detail = "unexpected cross-surface divergence"
		tracked = ""
	case anyReport:
		status = faultline.StatusReportOnly
		divergent = dedupeSort(keys(reportFields))
		expected = true
		detail = "known tracked cross-surface drift (report only)"
	default:
		status = faultline.StatusPass
		detail = "all applicable surfaces agree"
	}

	payload := faultline.ParityEvidence{
		ScenarioID:         scenarioID,
		Dimension:          "aggregate",
		Surfaces:           surfaces,
		DivergentFields:    divergent,
		ExpectedDivergence: expected,
		TrackedDefect:      tracked,
		Detail:             faultline.SanitizeDiagnostic(detail),
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal parity payload: %w", err)
	}
	art := faultline.EvidenceArtifact{
		SchemaVersion:     faultline.EvidenceSchemaVersion,
		ProducingTask:     "156.002-T",
		ProducingCommit:   producingCommit,
		NodeFamily:        faultline.NodeFamilyParity,
		Applicability:     surfaces,
		ValidatorIdentity: "parity-comparator",
		VerifiedEvidence:  raw,
		Status:            status,
	}
	data, err := json.Marshal(art)
	if err != nil {
		return nil, fmt.Errorf("marshal parity envelope: %w", err)
	}
	decoded, _, err := faultline.DecodeAndValidate(data)
	if err != nil {
		return nil, fmt.Errorf("validate parity envelope: %w", err)
	}
	return &decoded, nil
}

// --- projection extraction ------------------------------------------------

// projectCLI extracts the normalized projection from the CLI surface result:
// the human message lives in the `error` field, and the machine category is
// DERIVED from the exit-code bucket (the CLI payload carries no category).
func projectCLI(res SurfaceResult) surfaceProjection {
	m, ok := parseBody(res.Body)
	p := surfaceProjection{present: ok, exitCode: res.ExitCode}
	if !ok {
		return p
	}
	p.humanMessage = getString(m, "error")
	p.category = deriveCLICategory(res.ExitCode)
	p.remediation, p.hasRemediation = getStringPresent(m, "remediation")
	p.retryable = getBool(m, "retryable")
	p.retryAfterMs, p.hasRetryAfter = getIntPresent(m, "retry_after_ms")
	p.force = getBool(m, "force")
	fillResponse(&p, m)
	return p
}

// projectMCP extracts the normalized projection from the MCP surface result:
// the machine category lives in `error` and the human text in `message`.
func projectMCP(res SurfaceResult) surfaceProjection {
	m, ok := parseBody(res.Body)
	p := surfaceProjection{present: ok, exitCode: res.ExitCode}
	if !ok {
		return p
	}
	p.category = getString(m, "error")
	p.humanMessage = getString(m, "message")
	p.remediation, p.hasRemediation = getStringPresent(m, "remediation")
	p.retryable = getBool(m, "retryable")
	p.retryAfterMs, p.hasRetryAfter = getIntPresent(m, "retry_after_ms")
	p.force = getBool(m, "force")
	fillResponse(&p, m)
	return p
}

// projectInternal extracts the normalized projection from the internal surface
// result: it has no structured error, category, or remediation.
func projectInternal(res SurfaceResult) surfaceProjection {
	m, ok := parseBody(res.Body)
	p := surfaceProjection{present: ok, exitCode: res.ExitCode}
	if !ok {
		return p
	}
	p.retryable = getBool(m, "retryable")
	p.force = getBool(m, "force")
	fillResponse(&p, m)
	return p
}

// fillResponse populates the response projection and post-state from a body.
func fillResponse(p *surfaceProjection, m map[string]any) {
	if resp, ok := m["response"].(map[string]any); ok {
		p.response = resp
		p.responseKeys = keys(mapKeySet(resp))
	}
	if ps, ok := m["post_state"].(map[string]any); ok {
		p.post = &postProjection{
			createdAt:     getString(ps, "created_at"),
			updatedAt:     getString(ps, "updated_at"),
			seedUpdatedAt: getString(ps, "seed_updated_at"),
		}
	}
}

// deriveCLICategory maps a CLI exit code to a canonical representative category
// for its coarse equivalence class. exit 0 yields the empty (success) category.
func deriveCLICategory(exit int) string {
	switch exit {
	case 6:
		return "blocked"
	case 7:
		return "config"
	case 8:
		return "retryable"
	case 9:
		return "governance"
	default:
		return ""
	}
}

// inExitClass reports whether category is IN the coarse equivalence class of
// the CLI exit code (not an exact 1:1 match).
func inExitClass(exit int, category string) bool {
	class, ok := exitCodeClasses[exit]
	if !ok {
		return category == ""
	}
	_, in := class[category]
	return in
}

// --- small helpers --------------------------------------------------------

// parseBody unmarshals a JSON object body, returning ok=false for empty or
// non-object bodies.
func parseBody(b []byte) (map[string]any, bool) {
	if len(b) == 0 {
		return nil, false
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, false
	}
	return m, true
}

// normalizeResponse returns a copy of resp with volatile timestamp fields
// stripped so business content is compared via field mapping, not raw equality.
func normalizeResponse(resp map[string]any) map[string]any {
	out := make(map[string]any, len(resp))
	for k, v := range resp {
		switch k {
		case "created_at", "updated_at", "seed_updated_at":
			continue
		default:
			out[k] = v
		}
	}
	return out
}

// parseTime parses an RFC3339 timestamp, reporting validity.
func parseTime(s string) (time.Time, bool) {
	if s == "" {
		return time.Time{}, false
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}

// getString returns the string value at key, or "" if absent or non-string.
func getString(m map[string]any, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

// getStringPresent returns the string value at key and whether the key exists.
func getStringPresent(m map[string]any, key string) (string, bool) {
	v, ok := m[key]
	if !ok {
		return "", false
	}
	s, _ := v.(string)
	return s, true
}

// getBool returns the bool value at key, defaulting to false (absent==false).
func getBool(m map[string]any, key string) bool {
	if v, ok := m[key].(bool); ok {
		return v
	}
	return false
}

// getIntPresent returns the int value at key and whether the key exists. JSON
// numbers decode as float64.
func getIntPresent(m map[string]any, key string) (int, bool) {
	v, ok := m[key]
	if !ok {
		return 0, false
	}
	if f, isFloat := v.(float64); isFloat {
		return int(f), true
	}
	return 0, true
}

// dedupeSort returns a sorted, duplicate-free copy of xs.
func dedupeSort(xs []string) []string {
	if len(xs) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(xs))
	out := make([]string, 0, len(xs))
	for _, x := range xs {
		if _, ok := seen[x]; ok {
			continue
		}
		seen[x] = struct{}{}
		out = append(out, x)
	}
	sort.Strings(out)
	return out
}

// sameStringSet reports whether a and b hold the same distinct elements.
func sameStringSet(a, b []string) bool {
	sa := stringSet(a)
	sb := stringSet(b)
	if len(sa) != len(sb) {
		return false
	}
	for x := range sa {
		if _, ok := sb[x]; !ok {
			return false
		}
	}
	return true
}

// stringSet returns the distinct-element set of xs.
func stringSet(xs []string) map[string]struct{} {
	s := make(map[string]struct{}, len(xs))
	for _, x := range xs {
		s[x] = struct{}{}
	}
	return s
}

// mapKeySet returns the key set of m.
func mapKeySet(m map[string]any) map[string]struct{} {
	s := make(map[string]struct{}, len(m))
	for k := range m {
		s[k] = struct{}{}
	}
	return s
}

// keys returns the keys of a set as a slice.
func keys(s map[string]struct{}) []string {
	out := make([]string, 0, len(s))
	for k := range s {
		out = append(out, k)
	}
	return out
}

// unionKeys returns the union key set of two maps.
func unionKeys(a, b map[string]any) map[string]struct{} {
	s := make(map[string]struct{}, len(a)+len(b))
	for k := range a {
		s[k] = struct{}{}
	}
	for k := range b {
		s[k] = struct{}{}
	}
	return s
}

// mergeSets returns the union of two string sets.
func mergeSets(a, b map[string]struct{}) map[string]struct{} {
	s := make(map[string]struct{}, len(a)+len(b))
	for k := range a {
		s[k] = struct{}{}
	}
	for k := range b {
		s[k] = struct{}{}
	}
	return s
}
