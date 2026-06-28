package mdfront

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The codec/fence corpus below is ported VERBATIM from internal/docline's
// codec_test.go (the canonical pre-refactor suite), MINUS the
// TestModelsArtifactSerialization_Unchanged case which exercises internal/models
// and stays in internal/docline (it does not belong in this leaf package).

func TestDecode_NoFrontmatter(t *testing.T) {
	raw := []byte("# Heading\n\nbody text\n")
	md, err := Decode(raw)
	require.NoError(t, err)
	assert.False(t, md.HasFrontmatter)
	assert.Equal(t, raw, md.Body, "body must be the whole document, byte-identical")
	assert.Empty(t, md.Frontmatter)
}

// TestDecode_NoFrontmatter_NilMapNoError locks the precise contract U4's
// rewriteArchivedFromField guard relies on: a fence-less document decodes with
// HasFrontmatter=false, a nil Frontmatter map, and NO error (unlike core's
// pre-refactor hard-error behavior). A regression here would let a naive caller
// nil-map-panic or synthesize a frontmatter block.
func TestDecode_NoFrontmatter_NilMapNoError(t *testing.T) {
	for name, raw := range map[string]string{
		"plain_body":       "# Heading\n\nbody text\n",
		"open_no_close":    "---\nnot really frontmatter\nstill body\n",
		"leading_hr_only":  "---\n",
		"empty":            "",
		"crlf_open_noclos": "---\r\nstray\r\n",
	} {
		t.Run(name, func(t *testing.T) {
			md, err := Decode([]byte(raw))
			require.NoError(t, err, "fence-less input must not error")
			assert.False(t, md.HasFrontmatter)
			assert.Nil(t, md.Frontmatter, "fence-less Decode must return a nil Frontmatter map")
			assert.Equal(t, []byte(raw), md.Body, "fence-less body must be the whole document")
		})
	}
}

func TestDecode_NoClosingFence_TreatedAsBody(t *testing.T) {
	// A leading --- with no closing fence must NOT be parsed as frontmatter.
	raw := []byte("---\nnot really frontmatter\nstill body\n")
	md, err := Decode(raw)
	require.NoError(t, err)
	assert.False(t, md.HasFrontmatter)
	assert.Equal(t, raw, md.Body)
}

func TestDecode_BodyStartingWithHorizontalRule(t *testing.T) {
	// The closing fence must be the FIRST --- after the opening fence; a later
	// --- horizontal rule in the body must be preserved, not misparsed.
	raw := []byte("---\ntitle: T\n---\n\n---\nan hr above this line\n")
	md, err := Decode(raw)
	require.NoError(t, err)
	require.True(t, md.HasFrontmatter)
	assert.Equal(t, "T", md.Frontmatter["title"])
	assert.Equal(t, []byte("\n---\nan hr above this line\n"), md.Body)
}

func TestDecode_CRLFBodyPreserved(t *testing.T) {
	raw := []byte("---\r\ntitle: Test\r\n---\r\nLine1\r\nLine2\r\n")
	md, err := Decode(raw)
	require.NoError(t, err)
	require.True(t, md.HasFrontmatter)
	assert.Equal(t, "Test", md.Frontmatter["title"])
	assert.Equal(t, []byte("Line1\r\nLine2\r\n"), md.Body, "CRLF body bytes must be preserved")

	// Round-trip: re-encode and re-decode; body bytes stay identical.
	enc, err := md.Encode()
	require.NoError(t, err)
	md2, err := Decode(enc)
	require.NoError(t, err)
	assert.Equal(t, md.Body, md2.Body)
}

func TestDecode_BlockScalarContainingFence(t *testing.T) {
	// A --- inside an indented block scalar is part of the YAML value and must
	// not be mistaken for the closing fence.
	raw := []byte("---\nnote: |\n  line a\n  ---\n  line b\n---\nbody\n")
	md, err := Decode(raw)
	require.NoError(t, err)
	require.True(t, md.HasFrontmatter)
	note, ok := md.Frontmatter["note"].(string)
	require.True(t, ok, "note must decode as a string scalar")
	assert.Contains(t, note, "---", "the fence inside the block scalar is part of the value")
	assert.Equal(t, []byte("body\n"), md.Body)
}

func TestDecode_NestedDoclineMapPreserved(t *testing.T) {
	raw := []byte("---\ntitle: T\ndocline:\n  tags:\n    - a\n    - b\n  severity: medium\n---\nbody\n")
	md, err := Decode(raw)
	require.NoError(t, err)
	require.True(t, md.HasFrontmatter)
	nested, ok := md.Frontmatter["docline"].(map[string]any)
	require.True(t, ok, "docline must decode as a nested map[string]any")
	assert.Equal(t, "medium", nested["severity"])
	assert.Len(t, nested["tags"], 2)
}

func TestEncode_SortedKeysStable(t *testing.T) {
	md := &Markdown{
		HasFrontmatter: true,
		Frontmatter:    map[string]any{"zebra": 1, "alpha": 2, "title": "t"},
		Body:           []byte("body\n"),
	}
	out1, err := md.Encode()
	require.NoError(t, err)
	out2, err := md.Encode()
	require.NoError(t, err)
	assert.Equal(t, out1, out2, "encoding must be deterministic")

	s := string(out1)
	assert.Less(t, requireIndexOf(t, s, "alpha"), requireIndexOf(t, s, "zebra"), "keys must be sorted")
	assert.Less(t, requireIndexOf(t, s, "title"), requireIndexOf(t, s, "zebra"), "keys must be sorted")
}

func TestEncode_NoFrontmatter_BodyOnly(t *testing.T) {
	md := &Markdown{HasFrontmatter: false, Body: []byte("# Heading\n")}
	out, err := md.Encode()
	require.NoError(t, err)
	assert.Equal(t, []byte("# Heading\n"), out, "empty frontmatter encodes to body only")
}

func TestEncode_InsertBlockWithoutDisturbingBody(t *testing.T) {
	md := &Markdown{
		HasFrontmatter: true,
		Frontmatter:    map[string]any{"title": "T"},
		Body:           []byte("# Heading\n\nbody\n"),
	}
	out, err := md.Encode()
	require.NoError(t, err)
	assert.Equal(t, "---\ntitle: T\n---\n# Heading\n\nbody\n", string(out))

	// The body bytes survive a decode of the encoded output unchanged.
	md2, err := Decode(out)
	require.NoError(t, err)
	assert.Equal(t, md.Body, md2.Body)
}

func TestRoundTrip_Idempotent(t *testing.T) {
	raw := []byte("---\ntitle: T\ndocline:\n  tags:\n    - a\n---\n\n# Body\n\ncontent\n")
	md, err := Decode(raw)
	require.NoError(t, err)
	out1, err := md.Encode()
	require.NoError(t, err)

	md2, err := Decode(out1)
	require.NoError(t, err)
	out2, err := md2.Encode()
	require.NoError(t, err)

	assert.Equal(t, out1, out2, "encode(decode(encode(x))) == encode(x)")
}

// TestGoldenByteEquality_VsPreRefactorDocline is the differential byte-equality
// gate: it asserts mdfront.Decode/Encode output is byte-identical to output
// captured from the PRE-refactor internal/docline codec over the corpus. The
// golden ENC strings below were captured by running the original docline codec
// on each input (see 068.001-T). If yaml.v3 marshaling or the fence logic ever
// drifts, this test fails before the drift can corrupt a doc or archive record.
func TestGoldenByteEquality_VsPreRefactorDocline(t *testing.T) {
	// Decode-then-Encode cases: raw input -> golden re-encoded output.
	decodeEncode := []struct {
		name       string
		raw        string
		goldenEnc  string
		goldenBody string
	}{
		{
			name:       "hr_body",
			raw:        "---\ntitle: T\n---\n\n---\nan hr above this line\n",
			goldenEnc:  "---\ntitle: T\n---\n\n---\nan hr above this line\n",
			goldenBody: "\n---\nan hr above this line\n",
		},
		{
			name:       "crlf",
			raw:        "---\r\ntitle: Test\r\n---\r\nLine1\r\nLine2\r\n",
			goldenEnc:  "---\ntitle: Test\n---\nLine1\r\nLine2\r\n",
			goldenBody: "Line1\r\nLine2\r\n",
		},
		{
			name:       "block_scalar",
			raw:        "---\nnote: |\n  line a\n  ---\n  line b\n---\nbody\n",
			goldenEnc:  "---\nnote: |\n    line a\n    ---\n    line b\n---\nbody\n",
			goldenBody: "body\n",
		},
		{
			name:       "nested",
			raw:        "---\ntitle: T\ndocline:\n  tags:\n    - a\n    - b\n  severity: medium\n---\nbody\n",
			goldenEnc:  "---\ndocline:\n    severity: medium\n    tags:\n        - a\n        - b\ntitle: T\n---\nbody\n",
			goldenBody: "body\n",
		},
		{
			name:       "roundtrip",
			raw:        "---\ntitle: T\ndocline:\n  tags:\n    - a\n---\n\n# Body\n\ncontent\n",
			goldenEnc:  "---\ndocline:\n    tags:\n        - a\ntitle: T\n---\n\n# Body\n\ncontent\n",
			goldenBody: "\n# Body\n\ncontent\n",
		},
	}
	for _, tc := range decodeEncode {
		t.Run(tc.name, func(t *testing.T) {
			md, err := Decode([]byte(tc.raw))
			require.NoError(t, err)
			assert.Equal(t, tc.goldenBody, string(md.Body), "decoded body must match pre-refactor bytes")
			enc, err := md.Encode()
			require.NoError(t, err)
			assert.Equal(t, tc.goldenEnc, string(enc), "re-encoded output must be byte-identical to pre-refactor docline")
		})
	}

	// Encode-only cases: constructed Markdown -> golden output.
	t.Run("insert", func(t *testing.T) {
		md := &Markdown{HasFrontmatter: true, Frontmatter: map[string]any{"title": "T"}, Body: []byte("# Heading\n\nbody\n")}
		enc, err := md.Encode()
		require.NoError(t, err)
		assert.Equal(t, "---\ntitle: T\n---\n# Heading\n\nbody\n", string(enc))
	})
	t.Run("sorted", func(t *testing.T) {
		md := &Markdown{HasFrontmatter: true, Frontmatter: map[string]any{"zebra": 1, "alpha": 2, "title": "t"}, Body: []byte("body\n")}
		enc, err := md.Encode()
		require.NoError(t, err)
		assert.Equal(t, "---\nalpha: 2\ntitle: t\nzebra: 1\n---\nbody\n", string(enc))
	})
}

// requireIndexOf returns the byte offset of needle in haystack, failing the test
// when needle is absent. Returning a sentinel (e.g. -1) would let an ordering
// assertion like assert.Less(indexOf(...), indexOf(...)) pass even when a key is
// missing, masking a real regression; failing fast keeps the test meaningful.
func requireIndexOf(t *testing.T, haystack, needle string) int {
	t.Helper()
	i := strings.Index(haystack, needle)
	require.GreaterOrEqualf(t, i, 0, "expected %q to be present in output", needle)
	return i
}
