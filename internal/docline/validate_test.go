package docline

import (
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

func TestIsKnownDocType(t *testing.T) {
	assert.True(t, IsKnownDocType("decision"))
	assert.True(t, IsKnownDocType("learning"))
	assert.False(t, IsKnownDocType("bogus"))
	assert.False(t, IsKnownDocType(""))
	assert.Len(t, KnownDocTypes(), 11)
}
