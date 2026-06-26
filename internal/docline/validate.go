package docline

import (
	"errors"
	"fmt"
	"strings"
)

// Violation is a single validation failure for one frontmatter field.
type Violation struct {
	Field string // the offending field name
	Rule  string // the rule that failed: "required" | "unknown_doc_type" | "unknown_profile"
	Msg   string // an actionable human-readable message
}

// ValidateFields returns the structured validation violations for b under the
// given profile: required-field presence (per profile) plus closed-vocabulary
// membership for doc_type. An unrecognized profile yields a single
// "unknown_profile" violation so direct callers cannot silently fall back to
// the weaker authoring rules. An empty slice means valid.
func ValidateFields(b BaseFrontmatter, profile Profile) []Violation {
	if !isKnownProfile(profile) {
		return []Violation{{
			Field: "profile",
			Rule:  "unknown_profile",
			Msg:   fmt.Sprintf("unknown profile %q (want authoring or ingestion)", profile),
		}}
	}

	values := map[string]string{
		"title":       b.Title,
		"source":      b.Source,
		"ingested_at": b.IngestedAt,
		"doc_type":    b.DocType,
	}

	var vs []Violation
	for _, f := range requiredFields(profile) {
		if strings.TrimSpace(values[f]) == "" {
			vs = append(vs, Violation{
				Field: f,
				Rule:  "required",
				Msg:   fmt.Sprintf("required field %q is missing or empty", f),
			})
		}
	}
	if b.DocType != "" && !IsKnownDocType(b.DocType) {
		vs = append(vs, Violation{
			Field: "doc_type",
			Rule:  "unknown_doc_type",
			Msg:   fmt.Sprintf("doc_type %q is not in the closed vocabulary", b.DocType),
		})
	}
	return vs
}

// Validate reports whether b satisfies the contract for the given profile.
// Violations are joined; "required" violations match ErrMissingRequiredField,
// "unknown_doc_type" violations match ErrUnknownDocType, and "unknown_profile"
// violations match ErrUnknownProfile via errors.Is. An unrecognized profile
// fails fast rather than silently validating against the authoring subset.
func Validate(b BaseFrontmatter, profile Profile) error {
	vs := ValidateFields(b, profile)
	if len(vs) == 0 {
		return nil
	}
	errs := make([]error, 0, len(vs))
	for _, v := range vs {
		sentinel := ErrMissingRequiredField
		switch v.Rule {
		case "unknown_doc_type":
			sentinel = ErrUnknownDocType
		case "unknown_profile":
			sentinel = ErrUnknownProfile
		}
		errs = append(errs, fmt.Errorf("docline.Validate: %s (field %q): %w", v.Msg, v.Field, sentinel))
	}
	return errors.Join(errs...)
}
