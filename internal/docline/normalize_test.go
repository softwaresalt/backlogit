package docline

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fixedNow is a deterministic seed clock for ingested_at.
var fixedNow = time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC)

func normalizeOnce(t *testing.T, relPath, raw string) string {
	t.Helper()
	out, err := Normalize(relPath, []byte(raw), NormalizeOptions{Now: fixedNow})
	require.NoError(t, err)
	return string(out)
}

func TestNormalize_SetsRepoDerivedFields(t *testing.T) {
	t.Parallel()
	raw := "---\ntitle: My Decision\ntype: decision\n---\nBody line one.\n"
	got := normalizeOnce(t, "docs/decisions/x.md", raw)

	// Re-decode to inspect the structured frontmatter.
	md, err := Decode([]byte(got))
	require.NoError(t, err)
	require.True(t, md.HasFrontmatter)

	assert.Equal(t, "My Decision", md.Frontmatter["title"])
	assert.Equal(t, "decision", md.Frontmatter["doc_type"], "doc_type is path-derived")
	assert.Equal(t, "docs/decisions/x.md", md.Frontmatter["source"], "source is repo-relative POSIX path")
	assert.NotEmpty(t, md.Frontmatter["ingested_at"], "ingested_at seeded once")
	assert.Equal(t, "h1-h2-h3", md.Frontmatter["chunk_strategy"])
	assert.Equal(t, "1.0", md.Frontmatter["schema_version"])
}

func TestNormalize_FoldsLegacyTypeUnderDocline(t *testing.T) {
	t.Parallel()
	raw := "---\ntitle: Doc\ntype: legacy-kind\ntags: [a, b]\nseverity: high\n---\nBody.\n"
	got := normalizeOnce(t, "docs/compound/x.md", raw)

	md, err := Decode([]byte(got))
	require.NoError(t, err)

	// Path-derived doc_type supersedes the legacy `type`.
	assert.Equal(t, "learning", md.Frontmatter["doc_type"])

	// Legacy non-contract keys are MOVED under docline, never dropped.
	dl, ok := md.Frontmatter["docline"].(map[string]any)
	require.True(t, ok, "docline namespace present")
	assert.Equal(t, "legacy-kind", dl["type"], "legacy type preserved")
	assert.Contains(t, dl, "tags")
	assert.Equal(t, "high", dl["severity"])

	// No legacy key leaks back to the top level.
	for _, k := range []string{"type", "tags", "severity"} {
		_, present := md.Frontmatter[k]
		assert.Falsef(t, present, "top-level key %q must be folded away", k)
	}
}

func TestNormalize_ContractSurfaceHoldsOnlyContractKeys(t *testing.T) {
	t.Parallel()
	raw := "---\ntitle: Doc\ndescription: A doc\ngate_decision: approved\nconfidence: 0.9\n---\nBody.\n"
	got := normalizeOnce(t, "docs/reviews/x.md", raw)

	md, err := Decode([]byte(got))
	require.NoError(t, err)

	for k := range md.Frontmatter {
		assert.Truef(t, isContractField(k), "top-level key %q must be a contract field", k)
	}
}

func TestNormalize_Idempotent(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		relPath string
		raw     string
	}{
		{"with frontmatter", "docs/decisions/x.md", "---\ntitle: Doc\ntype: decision\ntags: [x]\ndate: 2026-06-22\n---\nBody bytes here.\n"},
		{"heterogeneous", "docs/reviews/y.md", "---\ntitle: Review\ngate_decision: pass\nseverity: low\nms.date: 2026-06-01\n---\n# Heading\n\nText.\n"},
		{"guide default", "README.md", "---\ntitle: Readme\n---\nProject readme.\n"},
		{"no frontmatter", "docs/research/z.md", "# Research\n\nNo frontmatter here.\n"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			first := normalizeOnce(t, tc.relPath, tc.raw)
			second := normalizeOnce(t, tc.relPath, first)
			assert.Equal(t, first, second, "normalize must be idempotent")
		})
	}
}

func TestNormalize_PreservesBodyBytes(t *testing.T) {
	t.Parallel()
	// Body contains a horizontal rule and CRLF that must survive untouched.
	body := "First paragraph.\r\n\r\n---\r\n\r\nAfter an HR.\r\n"
	raw := "---\ntitle: Doc\n---\n" + body
	got := normalizeOnce(t, "docs/cli-reference/x.md", raw)

	md, err := Decode([]byte(got))
	require.NoError(t, err)
	assert.Equal(t, body, string(md.Body), "body bytes preserved verbatim, including CRLF and HR")
}

func TestNormalize_SeedsIngestedAtOnceThenPreserves(t *testing.T) {
	t.Parallel()
	raw := "---\ntitle: Doc\n---\nBody.\n"
	first := normalizeOnce(t, "docs/decisions/x.md", raw)

	md1, err := Decode([]byte(first))
	require.NoError(t, err)
	seeded := md1.Frontmatter["ingested_at"]
	require.NotEmpty(t, seeded)

	// A later run with a DIFFERENT clock must NOT change the seeded value.
	laterNow := fixedNow.Add(48 * time.Hour)
	out2, err := Normalize("docs/decisions/x.md", []byte(first), NormalizeOptions{Now: laterNow})
	require.NoError(t, err)
	md2, err := Decode(out2)
	require.NoError(t, err)
	assert.Equal(t, seeded, md2.Frontmatter["ingested_at"], "ingested_at is seed-once")
}

func TestNormalize_RejectsZeroSeedTime(t *testing.T) {
	t.Parallel()
	// A doc that needs ingested_at seeded with a zero Now must fail fast rather
	// than writing a nonsense 0001-01-01T00:00:00Z timestamp.
	raw := "---\ntitle: Doc\n---\nBody.\n"
	_, err := Normalize("docs/decisions/x.md", []byte(raw), NormalizeOptions{})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrMissingSeedTime)

	// A doc that already carries ingested_at needs no seed, so a zero Now is
	// acceptable and normalization succeeds.
	seeded := normalizeOnce(t, "docs/decisions/x.md", raw)
	_, err = Normalize("docs/decisions/x.md", []byte(seeded), NormalizeOptions{})
	require.NoError(t, err)
}

func TestNormalize_NoFrontmatterCreatesContractBlock(t *testing.T) {
	t.Parallel()
	raw := "# Just a body\n\nNo frontmatter.\n"
	got := normalizeOnce(t, "docs/research/z.md", raw)
	assert.True(t, strings.HasPrefix(got, "---\n"), "frontmatter block prepended")

	md, err := Decode([]byte(got))
	require.NoError(t, err)
	assert.Equal(t, "research", md.Frontmatter["doc_type"])
	assert.Equal(t, "docs/research/z.md", md.Frontmatter["source"])
	assert.Equal(t, "# Just a body\n\nNo frontmatter.\n", string(md.Body))
}

func TestNormalize_PreservesScalarDoclineValue(t *testing.T) {
	t.Parallel()
	// A malformed/legacy scalar `docline:` value must be preserved (move,
	// never drop) rather than silently discarded by the map type assertion.
	raw := "---\ntitle: Doc\ndocline: legacy-scalar\n---\nBody.\n"
	got := normalizeOnce(t, "docs/decisions/x.md", raw)

	md, err := Decode([]byte(got))
	require.NoError(t, err)
	dl, ok := md.Frontmatter["docline"].(map[string]any)
	require.True(t, ok, "docline namespace materialized as a map")
	assert.Equal(t, "legacy-scalar", dl["docline"], "scalar docline value preserved under docline namespace")

	// And it must be idempotent.
	second := normalizeOnce(t, "docs/decisions/x.md", got)
	assert.Equal(t, got, second, "scalar-docline normalization is idempotent")
}

func TestNormalize_PreservesCollidingFoldedKeys(t *testing.T) {
	t.Parallel()
	// A top-level non-contract key that collides with an existing docline key
	// must not overwrite (drop) the nested value: both are preserved.
	raw := "---\ntitle: Doc\ntags: top-value\ndocline:\n  tags: nested-value\n---\nBody.\n"
	got := normalizeOnce(t, "docs/decisions/x.md", raw)

	md, err := Decode([]byte(got))
	require.NoError(t, err)
	dl, ok := md.Frontmatter["docline"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "nested-value", dl["tags"], "existing nested value retained")
	assert.Equal(t, "top-value", dl["tags_top1"], "colliding top-level value preserved, not dropped")

	// Idempotent: no top-level tags remains to re-collide on a second pass.
	second := normalizeOnce(t, "docs/decisions/x.md", got)
	assert.Equal(t, got, second, "collision-preserving normalization is idempotent")
}
