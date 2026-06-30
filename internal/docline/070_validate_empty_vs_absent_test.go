package docline

// 070.003-T harness: ValidateFields must distinguish "key present but empty
// string" from "key absent" for non-required minLength fields. JSON Schema
// minLength:1 is violated by a present-but-empty value but says nothing about an
// absent optional key. The struct previously collapsed both to "" at the
// BaseFrontmatter level, so a present-but-empty value escaped the min_length
// check. RED until FromMap threads presence and ValidateFields consults it.

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func hasMinLengthViolation(vs []Violation, field string) bool {
	for _, v := range vs {
		if v.Field == field && v.Rule == "min_length" {
			return true
		}
	}
	return false
}

// TestValidateFields_PresentButEmptyMinLength is the core parity case: a
// non-required minLength field that is PRESENT with an empty string violates
// minLength:1 and must be flagged.
func TestValidateFields_PresentButEmptyMinLength(t *testing.T) {
	b := FromMap(map[string]any{
		"title":       "T",
		"source":      "docs/x.md",
		"doc_type":    "guide",
		"ingested_at": "", // present, empty -> violates minLength:1
	})
	vs := ValidateFields(b, ProfileAuthoring)
	assert.True(t, hasMinLengthViolation(vs, "ingested_at"),
		"a present-but-empty non-required minLength field must report a min_length violation")
}

// TestValidateFields_AbsentMinLengthNotFlagged is the counterpart: an ABSENT
// optional minLength key carries no value to be too short, so it must NOT be
// flagged (only required-field presence governs absence).
func TestValidateFields_AbsentMinLengthNotFlagged(t *testing.T) {
	b := FromMap(map[string]any{
		"title":    "T",
		"source":   "docs/x.md",
		"doc_type": "guide",
		// ingested_at intentionally absent
	})
	vs := ValidateFields(b, ProfileAuthoring)
	assert.False(t, hasMinLengthViolation(vs, "ingested_at"),
		"an absent optional minLength key must not report a min_length violation")
}

// TestValidateFields_WhitespaceOnlyStillFlagged pins the preserved behavior: a
// present whitespace-only value remains a min_length violation.
func TestValidateFields_WhitespaceOnlyStillFlagged(t *testing.T) {
	b := FromMap(map[string]any{
		"title":       "T",
		"source":      "docs/x.md",
		"doc_type":    "guide",
		"ingested_at": "   ",
	})
	vs := ValidateFields(b, ProfileAuthoring)
	assert.True(t, hasMinLengthViolation(vs, "ingested_at"),
		"a present whitespace-only minLength field must still report a min_length violation")
}

// TestValidateFields_StructLiteralPresenceUnknown locks in the fallback for
// BaseFrontmatter values built directly (not via FromMap), where key presence is
// unknowable: an empty string cannot be distinguished from an absent key, so the
// historical whitespace-only-only behavior is preserved (empty not flagged,
// whitespace-only flagged).
func TestValidateFields_StructLiteralPresenceUnknown(t *testing.T) {
	empty := BaseFrontmatter{Title: "T", Source: "docs/x.md", DocType: "guide", IngestedAt: ""}
	assert.False(t, hasMinLengthViolation(ValidateFields(empty, ProfileAuthoring), "ingested_at"),
		"struct-literal empty string (presence unknown) preserves prior behavior: not flagged")

	ws := BaseFrontmatter{Title: "T", Source: "docs/x.md", DocType: "guide", IngestedAt: "   "}
	assert.True(t, hasMinLengthViolation(ValidateFields(ws, ProfileAuthoring), "ingested_at"),
		"struct-literal whitespace-only is still flagged regardless of presence tracking")
}
