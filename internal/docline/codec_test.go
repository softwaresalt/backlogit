package docline

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/models"
)

func TestDecode_NoFrontmatter(t *testing.T) {
	raw := []byte("# Heading\n\nbody text\n")
	md, err := Decode(raw)
	require.NoError(t, err)
	assert.False(t, md.HasFrontmatter)
	assert.Equal(t, raw, md.Body, "body must be the whole document, byte-identical")
	assert.Empty(t, md.Frontmatter)
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
	assert.Less(t, indexOf(s, "alpha"), indexOf(s, "zebra"), "keys must be sorted")
	assert.Less(t, indexOf(s, "title"), indexOf(s, "zebra"), "keys must be sorted")
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

// TestModelsArtifactSerialization_Unchanged freezes the existing backlog
// artifact serialization behavior so the new docline codec is proven not to
// have disturbed it (contract test required by the plan).
func TestModelsArtifactSerialization_Unchanged(t *testing.T) {
	fields := map[string]any{"id": "001-F", "title": "X", "status": "queued"}
	got := models.SerializeFrontmatter(fields, "Body\n")
	want := "---\nid: 001-F\nstatus: queued\ntitle: X\n---\n\nBody\n"
	assert.Equal(t, want, got)
}

// indexOf is a tiny helper to avoid pulling strings into assertions inline.
func indexOf(haystack, needle string) int {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}
