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

// MigrateReport is the pinned migration result envelope. Applied/Skipped are
// present only for an apply (they are nil for a dry-run plan).
type MigrateReport struct {
	DryRun  bool           `json:"dry_run"`
	Changes []ChangeReport `json:"changes"`
	Applied []string       `json:"applied,omitempty"`
	Skipped []string       `json:"skipped,omitempty"`
}

// NewMigrateReport builds a MigrateReport from a plan and optional apply result.
func NewMigrateReport(plan MigrationPlan, res *Result, dryRun bool) MigrateReport {
	changes := make([]ChangeReport, 0, len(plan.Changes))
	for _, c := range plan.Changes {
		changes = append(changes, ChangeReport{
			File:             c.File,
			Action:           string(c.Action),
			BodyBytesChanged: c.BodyBytesChanged,
		})
	}
	report := MigrateReport{DryRun: dryRun, Changes: changes}
	if res != nil {
		report.Applied = res.Applied
		report.Skipped = res.Skipped
	}
	return report
}
