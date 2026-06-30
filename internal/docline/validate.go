package docline

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// contentSHA256Pattern mirrors schemas/docline/base-frontmatter-v1.schema.json:
// content_sha256 must be empty or a 64-char lowercase/uppercase hex digest.
var contentSHA256Pattern = regexp.MustCompile(`^([a-fA-F0-9]{64})?$`)

// minLengthFields mirrors the v1 schema's minLength:1 + required surface.
// Behavior: requiredFields rejects absent/empty required keys; this min_length
// pass additionally governs minLength fields that are NOT required in the active
// profile. For those, a key that is PRESENT but blank (empty string OR
// whitespace-only) violates minLength:1, while an ABSENT optional key does not
// (070.003-T: empty-vs-absent parity, driven by BaseFrontmatter.present). When
// presence is unknown (a value built directly, not via FromMap), the historical
// whitespace-only-only behavior is preserved. Rejecting whitespace-only is a
// deliberate stricter-than-schema choice; JSON Schema minLength:1 counts
// whitespace as length 1.
var minLengthFields = []string{"title", "source", "ingested_at", "doc_type"}

// Violation is a single validation failure for one frontmatter field.
type Violation struct {
	Field string // the offending field name
	Rule  string // "required" | "unknown_doc_type" | "unknown_profile" | "min_length" | "pattern"
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
	if strings.TrimSpace(b.DocType) != "" && !IsKnownDocType(b.DocType) {
		vs = append(vs, Violation{
			Field: "doc_type",
			Rule:  "unknown_doc_type",
			Msg:   fmt.Sprintf("doc_type %q is not in the closed vocabulary", b.DocType),
		})
	}

	// Full schema constraints (base-frontmatter-v1): minLength on contract fields
	// and the content_sha256 hex pattern. A hand-rolled validator avoids a new
	// JSON-schema dependency for this tiny, fixed contract. Required fields are
	// already flagged blank by the required loop above, so min_length only applies
	// to fields NOT required in this profile to avoid double-reporting.
	required := make(map[string]bool, 4)
	for _, f := range requiredFields(profile) {
		required[f] = true
	}
	for _, f := range minLengthFields {
		if required[f] {
			continue
		}
		v := values[f]
		if strings.TrimSpace(v) != "" {
			continue // non-blank value satisfies minLength:1
		}
		// v is blank (empty string or whitespace-only). 070.003-T: decide using
		// key presence so an empty string is treated distinctly from an absent key.
		var violate bool
		if b.present != nil {
			// Presence is known (built via FromMap): a present minLength key that
			// is blank violates minLength:1; an absent optional key does not.
			violate = b.present[f]
		} else {
			// Presence unknown (value built directly): an empty string is
			// indistinguishable from an absent key, so preserve the historical
			// behavior of flagging only whitespace-only values.
			violate = v != ""
		}
		if violate {
			vs = append(vs, Violation{
				Field: f,
				Rule:  "min_length",
				Msg:   fmt.Sprintf("field %q must not be blank (minLength 1)", f),
			})
		}
	}
	if b.ContentSHA256 != "" && !contentSHA256Pattern.MatchString(b.ContentSHA256) {
		vs = append(vs, Violation{
			Field: "content_sha256",
			Rule:  "pattern",
			Msg:   "content_sha256 must be a 64-char hex digest",
		})
	}
	return vs
}

// Validate reports whether b satisfies the contract for the given profile.
// Violations are joined; "required" violations match ErrMissingRequiredField,
// "unknown_doc_type" violations match ErrUnknownDocType, "unknown_profile"
// violations match ErrUnknownProfile, and schema-constraint violations
// ("min_length"/"pattern") match ErrSchemaViolation via errors.Is. An
// unrecognized profile fails fast rather than silently validating against the
// authoring subset.
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
		case "min_length", "pattern":
			sentinel = ErrSchemaViolation
		}
		errs = append(errs, fmt.Errorf("docline.Validate: %s (field %q): %w", v.Msg, v.Field, sentinel))
	}
	return errors.Join(errs...)
}
