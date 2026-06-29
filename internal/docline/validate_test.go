package docline

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFromMap_AppliesDefaults(t *testing.T) {
	b := FromMap(map[string]any{
		"title":    "T",
		"source":   "docs/x.md",
		"doc_type": "guide",
	})
	assert.Equal(t, "T", b.Title)
	assert.Equal(t, "docs/x.md", b.Source)
	assert.Equal(t, "guide", b.DocType)
	assert.Equal(t, DefaultChunkStrategy, b.ChunkStrategy, "chunk_strategy default applied")
	assert.Equal(t, DefaultSchemaVersion, b.SchemaVersion, "schema_version default applied")
	assert.Empty(t, b.ContentSHA256, "pipeline-owned field not fabricated")
	assert.Empty(t, b.SourcePath, "pipeline-owned field not fabricated")
}

func TestFromMap_PreservesNestedDocline(t *testing.T) {
	b := FromMap(map[string]any{
		"title":    "T",
		"source":   "docs/x.md",
		"doc_type": "guide",
		"docline":  map[string]any{"tags": []any{"a", "b"}, "severity": "low"},
	})
	require.NotNil(t, b.Docline)
	assert.Equal(t, "low", b.Docline["severity"])
}

func TestToMap_RoundTripContractFields(t *testing.T) {
	in := map[string]any{
		"title":       "T",
		"source":      "docs/x.md",
		"ingested_at": "2026-06-22T00:00:00Z",
		"doc_type":    "decision",
		"description": "d",
		"docline":     map[string]any{"tags": []any{"a"}},
	}
	out := FromMap(in).ToMap()
	assert.Equal(t, "T", out["title"])
	assert.Equal(t, "docs/x.md", out["source"])
	assert.Equal(t, "2026-06-22T00:00:00Z", out["ingested_at"])
	assert.Equal(t, "decision", out["doc_type"])
	assert.Equal(t, "d", out["description"])
	assert.Equal(t, DefaultChunkStrategy, out["chunk_strategy"])
	assert.Equal(t, DefaultSchemaVersion, out["schema_version"])
	assert.Contains(t, out, "docline")
	// Pipeline-owned empty fields are not emitted onto the repo surface.
	assert.NotContains(t, out, "content_sha256")
	assert.NotContains(t, out, "source_path")
}

func TestToMap_OmitsEmptyDocline(t *testing.T) {
	out := FromMap(map[string]any{
		"title":    "T",
		"source":   "docs/x.md",
		"doc_type": "guide",
	}).ToMap()
	assert.NotContains(t, out, "docline", "empty docline namespace must not be emitted")
}

func TestValidate_AuthoringProfile_Valid(t *testing.T) {
	b := FromMap(map[string]any{
		"title":    "T",
		"source":   "docs/x.md",
		"doc_type": "guide",
	})
	assert.NoError(t, Validate(b, ProfileAuthoring))
}

func TestValidate_MissingRequiredField(t *testing.T) {
	tests := []struct {
		name    string
		fm      map[string]any
		profile Profile
		field   string
	}{
		{"missing title", map[string]any{"source": "docs/x.md", "doc_type": "guide"}, ProfileAuthoring, "title"},
		{"missing source", map[string]any{"title": "T", "doc_type": "guide"}, ProfileAuthoring, "source"},
		{"missing doc_type", map[string]any{"title": "T", "source": "docs/x.md"}, ProfileAuthoring, "doc_type"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			b := FromMap(tc.fm)
			err := Validate(b, tc.profile)
			require.Error(t, err)
			assert.ErrorIs(t, err, ErrMissingRequiredField)
			assert.Contains(t, err.Error(), tc.field, "error must name the missing field")
		})
	}
}

func TestValidate_UnknownDocType(t *testing.T) {
	b := FromMap(map[string]any{
		"title":    "T",
		"source":   "docs/x.md",
		"doc_type": "bogus",
	})
	err := Validate(b, ProfileAuthoring)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrUnknownDocType)
}

func TestValidate_IngestionProfile(t *testing.T) {
	// Authored-only file is missing ingested_at -> fails ingestion profile.
	authored := FromMap(map[string]any{
		"title":    "T",
		"source":   "docs/x.md",
		"doc_type": "guide",
	})
	err := Validate(authored, ProfileIngestion)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrMissingRequiredField)
	assert.Contains(t, err.Error(), "ingested_at")

	// Fully populated file passes the ingestion profile.
	full := FromMap(map[string]any{
		"title":       "T",
		"source":      "docs/x.md",
		"ingested_at": "2026-06-22T00:00:00Z",
		"doc_type":    "guide",
	})
	assert.NoError(t, Validate(full, ProfileIngestion))
}

func TestValidate_RejectsUnknownProfile(t *testing.T) {
	// A fully valid authoring doc still fails fast when the profile itself is a
	// typo, rather than silently validating against the authoring subset.
	b := FromMap(map[string]any{
		"title":    "T",
		"source":   "docs/x.md",
		"doc_type": "guide",
	})
	err := Validate(b, Profile("authroing"))
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrUnknownProfile)

	// The empty profile is accepted as the authoring default (mirrors ParseProfile).
	assert.NoError(t, Validate(b, Profile("")))
}

func TestValidateFields_RejectsUnknownProfile(t *testing.T) {
	// ValidateFields is exported; a misspelled profile must surface a single
	// unknown_profile violation rather than silently applying authoring rules.
	b := FromMap(map[string]any{
		"title":    "T",
		"source":   "docs/x.md",
		"doc_type": "guide",
	})
	vs := ValidateFields(b, Profile("authroing"))
	require.Len(t, vs, 1)
	assert.Equal(t, "unknown_profile", vs[0].Rule)
	assert.Equal(t, "profile", vs[0].Field)

	// Known profiles (including the empty default) do not emit a profile violation.
	assert.Empty(t, ValidateFields(b, Profile("")))
	assert.Empty(t, ValidateFields(b, ProfileAuthoring))
}

// TestValidateFields_SchemaPatternAndMinLength verifies 069.003-T: ValidateFields
// enforces the base-frontmatter v1 schema beyond presence — content_sha256 must
// match the 64-hex pattern, required fields must satisfy minLength, and a valid
// fully-populated record passes with no violations.
func TestValidateFields_SchemaPatternAndMinLength(t *testing.T) {
	// Bad content_sha256: present but not a 64-char hex digest.
	bad := FromMap(map[string]any{
		"title":          "T",
		"source":         "docs/x.md",
		"ingested_at":    "2026-06-22T00:00:00Z",
		"doc_type":       "guide",
		"content_sha256": "not-a-hash",
	})
	vs := ValidateFields(bad, ProfileIngestion)
	var hasPattern bool
	for _, v := range vs {
		if v.Field == "content_sha256" && v.Rule == "pattern" {
			hasPattern = true
		}
	}
	assert.True(t, hasPattern, "malformed content_sha256 must report a pattern violation")

	// Empty content_sha256 (pipeline-owned, not fabricated) is allowed.
	emptyOK := FromMap(map[string]any{
		"title":       "T",
		"source":      "docs/x.md",
		"ingested_at": "2026-06-22T00:00:00Z",
		"doc_type":    "guide",
	})
	assert.Empty(t, ValidateFields(emptyOK, ProfileIngestion))

	// Valid 64-hex content_sha256 passes.
	valid := FromMap(map[string]any{
		"title":          "T",
		"source":         "docs/x.md",
		"ingested_at":    "2026-06-22T00:00:00Z",
		"doc_type":       "guide",
		"content_sha256": strings.Repeat("a", 64),
	})
	assert.Empty(t, ValidateFields(valid, ProfileIngestion))
}

// TestValidateFields_MinLengthNonRequired verifies the min_length rule fires
// for a contract field that is present-but-blank yet NOT required in the profile
// (ingested_at under authoring), where the required loop does not already cover it.
func TestValidateFields_MinLengthNonRequired(t *testing.T) {
	b := FromMap(map[string]any{
		"title":       "T",
		"source":      "docs/x.md",
		"doc_type":    "guide",
		"ingested_at": "   ",
	})
	vs := ValidateFields(b, ProfileAuthoring)
	var hasMinLen bool
	for _, v := range vs {
		if v.Field == "ingested_at" && v.Rule == "min_length" {
			hasMinLen = true
		}
		assert.NotEqual(t, "required", v.Rule, "ingested_at is not required under authoring; must not double-report")
	}
	assert.True(t, hasMinLen, "blank non-required ingested_at must report a min_length violation under authoring")
}

// TestValidate_SchemaViolationSentinel locks in that Validate() maps schema
// constraint failures (pattern/min_length) to ErrSchemaViolation, not to
// ErrMissingRequiredField.
func TestValidate_SchemaViolationSentinel(t *testing.T) {
	b := FromMap(map[string]any{
		"title":          "T",
		"source":         "docs/x.md",
		"ingested_at":    "2026-06-22T00:00:00Z",
		"doc_type":       "guide",
		"content_sha256": "not-a-hash",
	})
	err := Validate(b, ProfileIngestion)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrSchemaViolation)
	assert.NotErrorIs(t, err, ErrMissingRequiredField)
}

// TestValidateFields_BlankDocTypeNoDoubleReport pins the fix: a whitespace-only
// doc_type reports only a "required" violation, not also "unknown_doc_type".
func TestValidateFields_BlankDocTypeNoDoubleReport(t *testing.T) {
	b := FromMap(map[string]any{
		"title":       "T",
		"source":      "docs/x.md",
		"ingested_at": "2026-06-22T00:00:00Z",
		"doc_type":    "   ",
	})
	vs := ValidateFields(b, ProfileAuthoring)
	for _, v := range vs {
		assert.NotEqual(t, "unknown_doc_type", v.Rule, "blank doc_type must not be flagged unknown_doc_type")
	}
}

func TestIsKnownDocType(t *testing.T) {
	assert.True(t, IsKnownDocType("decision"))
	assert.True(t, IsKnownDocType("learning"))
	assert.False(t, IsKnownDocType("bogus"))
	assert.False(t, IsKnownDocType(""))
	assert.Len(t, KnownDocTypes(), 11)
}
