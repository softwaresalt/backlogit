package stash

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// T004 / ST019: Round-trip JSONL serialization preserves all fields.
func TestJSONL_RoundTrip(t *testing.T) {
	// Arrange
	entries := []Entry{
		{ID: "AAAA1111", Priority: "high", Kind: "feature", Text: "Add shipment type", DeliberationID: "DL001"},
		{ID: "BBBB2222", Priority: "medium", Kind: "bug", Text: "Fix stash parsing"},
		{ID: "CCCC3333", Priority: "low", Kind: "task", Text: "Update docs"},
	}

	// Act
	var buf bytes.Buffer
	err := WriteJSONL(&buf, entries)
	require.NoError(t, err)

	recovered, err := ReadJSONL(&buf)
	require.NoError(t, err)

	// Assert
	require.Len(t, recovered, 3)
	assert.Equal(t, entries[0].ID, recovered[0].ID)
	assert.Equal(t, entries[0].Priority, recovered[0].Priority)
	assert.Equal(t, entries[0].Kind, recovered[0].Kind)
	assert.Equal(t, entries[0].Text, recovered[0].Text)
	assert.Equal(t, entries[0].DeliberationID, recovered[0].DeliberationID)
	assert.Equal(t, entries[1].ID, recovered[1].ID)
	assert.Equal(t, entries[2].ID, recovered[2].ID)
}

// T004 / ST019: WriteJSONL produces one line per entry.
func TestWriteJSONL_OneLinePerEntry(t *testing.T) {
	// Arrange
	entries := []Entry{
		{ID: "AAAA1111", Kind: "task", Text: "First"},
		{ID: "BBBB2222", Kind: "bug", Text: "Second"},
	}

	// Act
	var buf bytes.Buffer
	err := WriteJSONL(&buf, entries)
	require.NoError(t, err)

	// Assert
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	assert.Len(t, lines, 2, "each entry must produce exactly one line")
}

// T004 / ST019: ReadJSONL skips empty lines.
func TestReadJSONL_SkipsEmptyLines(t *testing.T) {
	// Arrange
	jsonl := `{"id":"AAAA1111","kind":"task","text":"First"}

{"id":"BBBB2222","kind":"bug","text":"Second"}
`

	// Act
	entries, err := ReadJSONL(strings.NewReader(jsonl))

	// Assert
	require.NoError(t, err)
	assert.Len(t, entries, 2)
}

// T004 / ST019: ReadJSONL returns error on malformed JSON.
func TestReadJSONL_MalformedLine(t *testing.T) {
	// Arrange
	jsonl := `{"id":"AAAA1111","kind":"task","text":"Valid"}
not valid json
`

	// Act
	_, err := ReadJSONL(strings.NewReader(jsonl))

	// Assert
	require.Error(t, err, "malformed line must produce an error")
}

// T004 / ST020: Migrate .stash.md to .stash.jsonl with correct entry count.
func TestMigrateStashMDToJSONL_Success(t *testing.T) {
	// Arrange
	tmpDir := t.TempDir()
	srcPath := filepath.Join(tmpDir, ".stash.md")
	dstPath := filepath.Join(tmpDir, "stash.jsonl")

	stashContent := `---
stash_version: "1"
---

- id: AAAA1111
  priority: high
  kind: feature
  text: "Add shipment type"
  deliberation: DL001

- id: BBBB2222
  priority: medium
  kind: bug
  text: "Fix stash parsing"
`
	require.NoError(t, os.WriteFile(srcPath, []byte(stashContent), 0o644))

	// Act
	count, err := MigrateStashMDToJSONL(srcPath, dstPath)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, 2, count, "two entries should be migrated")

	// Verify JSONL file exists and can be read back
	f, err := os.Open(dstPath)
	require.NoError(t, err)
	defer f.Close()

	recovered, err := ReadJSONL(f)
	require.NoError(t, err)
	assert.Len(t, recovered, 2)
	assert.Equal(t, "AAAA1111", recovered[0].ID)
	assert.Equal(t, "BBBB2222", recovered[1].ID)
}

// T004 / ST020: Migration with no entries produces empty JSONL file.
func TestMigrateStashMDToJSONL_EmptyStash(t *testing.T) {
	// Arrange
	tmpDir := t.TempDir()
	srcPath := filepath.Join(tmpDir, ".stash.md")
	dstPath := filepath.Join(tmpDir, "stash.jsonl")

	stashContent := `---
stash_version: "1"
---
`
	require.NoError(t, os.WriteFile(srcPath, []byte(stashContent), 0o644))

	// Act
	count, err := MigrateStashMDToJSONL(srcPath, dstPath)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

// T004 / ST020: Migration uses atomic write (dstPath should not exist partially).
func TestMigrateStashMDToJSONL_AtomicWrite(t *testing.T) {
	// Arrange
	tmpDir := t.TempDir()
	srcPath := filepath.Join(tmpDir, ".stash.md")
	dstPath := filepath.Join(tmpDir, "stash.jsonl")

	stashContent := `---
stash_version: "1"
---

- id: AAAA1111
  priority: high
  kind: feature
  text: "Atomic test"
`
	require.NoError(t, os.WriteFile(srcPath, []byte(stashContent), 0o644))

	// Act
	_, err := MigrateStashMDToJSONL(srcPath, dstPath)

	// Assert
	require.NoError(t, err)

	// Verify the file is complete and readable
	data, err := os.ReadFile(dstPath)
	require.NoError(t, err)
	assert.True(t, len(data) > 0, "JSONL file should not be empty")
	assert.True(t, strings.HasSuffix(strings.TrimSpace(string(data)), "}"),
		"last line should end with a JSON object closing brace")
}
