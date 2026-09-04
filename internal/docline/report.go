package docline

// This file defines the pinned, serializable report shapes shared by the CLI
// adapter (065.007-T) and the MCP parity tools (065.008-T). Both surfaces
// marshal these exact types, so their JSON is structurally identical for the
// same scenario — same fields, values, and key order. The byte output differs
// only in formatting: the CLI pretty-prints with two-space indentation while
// the MCP emits compact JSON via json.Marshal.

// FindingReport is the serializable form of a lint Finding.
type FindingReport struct {
	File     string `json:"file"`
	Field    string `json:"field"`
	Rule     string `json:"rule"`
	Severity string `json:"severity"`
	Fix      string `json:"fix"`
}

// LintReport is the pinned lint result envelope. It always reports success
// (valid=false simply means violations exist); transport errors are reserved
// for invalid params / IO / parse failures. A per-file frontmatter decode
// failure (146.018-T / U8) is one such violation, not a transport error: it
// surfaces as a Finding with Rule "decode_error" (see RuleDecodeError) so the
// scan can report it and continue past the rest of the corpus.
type LintReport struct {
	Valid          bool            `json:"valid"`
	ViolationCount int             `json:"violation_count"`
	Findings       []FindingReport `json:"findings"`
}

// NewLintReport builds a LintReport from raw findings.
func NewLintReport(findings []Finding) LintReport {
	out := make([]FindingReport, 0, len(findings))
	for _, f := range findings {
		out = append(out, FindingReport{
			File:     f.File,
			Field:    f.Field,
			Rule:     f.Rule,
			Severity: string(f.Severity),
			Fix:      f.Fix,
		})
	}
	return LintReport{
		Valid:          len(findings) == 0,
		ViolationCount: len(findings),
		Findings:       out,
	}
}

// ChangeReport is the serializable form of a migration Change (metadata only;
// the full before/after bytes are intentionally omitted from the wire shape).
type ChangeReport struct {
	File             string `json:"file"`
	Action           string `json:"action"`
	BodyBytesChanged bool   `json:"body_bytes_changed"`
}

// MigrateReport is the pinned migration result envelope. Applied and Skipped
// are always-present arrays: they are [] (never null or absent) for both a
// dry-run plan (res == nil) and a zero-apply result (res != nil, empty sets).
// This is the always-an-array contract: downstream consumers must not
// distinguish null from empty — both encode "nothing happened" as [].
// Findings holds per-file frontmatter decode errors reported during PlanMigration
// (always-array: [] when there are none).
type MigrateReport struct {
	DryRun   bool            `json:"dry_run"`
	Changes  []ChangeReport  `json:"changes"`
	Applied  []string        `json:"applied"`
	Skipped  []string        `json:"skipped"`
	Findings []FindingReport `json:"findings"`
}

// NewMigrateReport builds a MigrateReport from a plan and optional apply result.
// Applied and Skipped are always initialised to non-nil empty slices so they
// marshal as [] rather than null — satisfying the always-an-array contract for
// both the dry-run (res == nil) and zero-apply (res != nil, empty results)
// cases. When res is non-nil and carries non-nil slices, those are used instead.
// Findings is similarly always-array: it is initialised from plan.Findings so a
// zero-findings plan marshals "findings":[] never null or absent.
func NewMigrateReport(plan MigrationPlan, res *Result, dryRun bool) MigrateReport {
	changes := make([]ChangeReport, 0, len(plan.Changes))
	for _, c := range plan.Changes {
		changes = append(changes, ChangeReport{
			File:             c.File,
			Action:           string(c.Action),
			BodyBytesChanged: c.BodyBytesChanged,
		})
	}
	findings := make([]FindingReport, 0, len(plan.Findings))
	for _, f := range plan.Findings {
		findings = append(findings, FindingReport{
			File:     f.File,
			Field:    f.Field,
			Rule:     f.Rule,
			Severity: string(f.Severity),
			Fix:      f.Fix,
		})
	}
	report := MigrateReport{
		DryRun:   dryRun,
		Changes:  changes,
		Applied:  []string{},
		Skipped:  []string{},
		Findings: findings,
	}
	if res != nil {
		if res.Applied != nil {
			report.Applied = res.Applied
		}
		if res.Skipped != nil {
			report.Skipped = res.Skipped
		}
	}
	return report
}
