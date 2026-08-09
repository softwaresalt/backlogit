package gate

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/softwaresalt/backlogit/internal/canonical"
	bkerrors "github.com/softwaresalt/backlogit/internal/errors"
)

// FormalReviewEntry is one attributed reviewer's decision within a
// FormalReport. Persona and Decision must both be non-blank for the entry to
// count as valid attributed-review evidence.
type FormalReviewEntry struct {
	Persona  string `json:"persona"`
	Decision string `json:"decision"`
}

// FormalReport is the schema-validated payload required for formal gate
// admission (106-F F1/U5). Unlike the permissive GateReport ParseReport
// accepts (which tolerates empty or non-JSON stdout on an exit-0 pass), a
// FormalReport additionally requires at least one attributed reviewer entry —
// evidence that a complete, attributed formal review actually occurred,
// rather than merely that the gate broker ran and exited zero.
type FormalReport struct {
	Reviewers []FormalReviewEntry `json:"reviewers"`
}

// ValidateFormalReport parses and validates stdout against the formal report
// schema. It rejects (wrapping ErrFormalReportInvalid) an empty stdout,
// non-JSON stdout, a report with zero reviewer entries, or any reviewer entry
// missing a non-blank persona or decision. Unknown fields (e.g. the ordinary
// repeated_failure field also present in the same JSON payload) are ignored,
// not rejected, so this validator layers on top of the existing report shape
// rather than replacing it.
func ValidateFormalReport(stdout []byte) (*FormalReport, error) {
	if len(stdout) == 0 {
		return nil, fmt.Errorf("%w: stdout is empty", bkerrors.ErrFormalReportInvalid)
	}
	var r FormalReport
	if err := json.Unmarshal(stdout, &r); err != nil {
		return nil, fmt.Errorf("%w: stdout is not valid JSON: %v", bkerrors.ErrFormalReportInvalid, err)
	}
	if len(r.Reviewers) == 0 {
		return nil, fmt.Errorf("%w: no attributed reviewer entries", bkerrors.ErrFormalReportInvalid)
	}
	for i, rv := range r.Reviewers {
		if strings.TrimSpace(rv.Persona) == "" {
			return nil, fmt.Errorf("%w: reviewers[%d] missing persona", bkerrors.ErrFormalReportInvalid, i)
		}
		if strings.TrimSpace(rv.Decision) == "" {
			return nil, fmt.Errorf("%w: reviewers[%d] missing decision", bkerrors.ErrFormalReportInvalid, i)
		}
	}
	return &r, nil
}

// canonicalMap converts a validated FormalReport into the map[string]any shape
// internal/canonical.Canonicalize accepts.
func (r FormalReport) canonicalMap() map[string]any {
	reviewers := make([]any, len(r.Reviewers))
	for i, rv := range r.Reviewers {
		reviewers[i] = map[string]any{
			"persona":  rv.Persona,
			"decision": rv.Decision,
		}
	}
	return map[string]any{"reviewers": reviewers}
}

// FormalReportDigest returns internal/canonical.Hash of the VALIDATED report
// — never of the raw stdout bytes — so semantically identical reports
// (reordered fields, incidental whitespace) always digest identically, and a
// report is only ever digested after it has passed ValidateFormalReport.
func FormalReportDigest(r FormalReport) (string, error) {
	digest, err := canonical.Hash(r.canonicalMap())
	if err != nil {
		return "", fmt.Errorf("hash formal report: %w", err)
	}
	return digest, nil
}
