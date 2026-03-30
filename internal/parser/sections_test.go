package parser_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/backlogit/backlogit/internal/parser"
)

// TASK-002.03.03: Implement section parser and writer.

func TestParseSections_Basic(t *testing.T) {
	// Arrange
	content := `# Title

## Description

<!-- BEGIN:description -->
This is the description content.
<!-- END:description -->

## Acceptance Criteria

<!-- BEGIN:acceptance-criteria -->
- Criterion 1
- Criterion 2
<!-- END:acceptance-criteria -->
`

	// Act
	sections, err := parser.ParseSections(content)

	// Assert
	require.NoError(t, err)
	assert.Len(t, sections, 2)
	assert.Contains(t, sections["description"], "description content")
	assert.Contains(t, sections["acceptance-criteria"], "Criterion 1")
}

func TestParseSections_EmptySection(t *testing.T) {
	// Arrange
	content := `<!-- BEGIN:notes -->
<!-- END:notes -->
`

	// Act
	sections, err := parser.ParseSections(content)

	// Assert
	require.NoError(t, err)
	assert.Contains(t, sections, "notes")
	assert.Empty(t, sections["notes"])
}

func TestParseSections_MissingEndTag(t *testing.T) {
	// Arrange
	content := `<!-- BEGIN:description -->
Content without closing tag
`

	// Act
	_, err := parser.ParseSections(content)

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "description")
}

func TestParseSections_NoSections(t *testing.T) {
	// Arrange
	content := `# Plain document with no sections`

	// Act
	sections, err := parser.ParseSections(content)

	// Assert
	require.NoError(t, err)
	assert.Empty(t, sections)
}

func TestWriteSections_ReplacesContent(t *testing.T) {
	// Arrange
	content := `# Doc

<!-- BEGIN:description -->
Old content
<!-- END:description -->

<!-- BEGIN:notes -->
Old notes
<!-- END:notes -->
`

	updates := map[string]string{
		"description": "New description content",
		"notes":       "Updated notes",
	}

	// Act
	result, err := parser.WriteSections(content, updates)

	// Assert
	require.NoError(t, err)
	assert.Contains(t, result, "New description content")
	assert.Contains(t, result, "Updated notes")
	assert.NotContains(t, result, "Old content")
	assert.NotContains(t, result, "Old notes")
}

func TestWriteSection_SingleUpdate(t *testing.T) {
	// Arrange
	content := `<!-- BEGIN:description -->
Original
<!-- END:description -->
`

	// Act
	result, err := parser.WriteSection(content, "description", "Replaced")

	// Assert
	require.NoError(t, err)
	assert.Contains(t, result, "Replaced")
	assert.NotContains(t, result, "Original")
}

func TestWriteSections_NonexistentSection(t *testing.T) {
	// Arrange
	content := `<!-- BEGIN:description -->
Content
<!-- END:description -->
`

	// Act
	_, err := parser.WriteSections(content, map[string]string{
		"nonexistent": "value",
	})

	// Assert
	require.Error(t, err)
}

func TestParseSections_PreservesWhitespace(t *testing.T) {
	// Arrange
	content := `<!-- BEGIN:code -->

    indented code block

<!-- END:code -->
`

	// Act
	sections, err := parser.ParseSections(content)

	// Assert
	require.NoError(t, err)
	assert.Contains(t, sections["code"], "    indented code block")
}

func TestWriteSections_RoundTrip(t *testing.T) {
	// Arrange
	content := `# Doc

<!-- BEGIN:description -->
Original description
<!-- END:description -->

<!-- BEGIN:notes -->
Original notes
<!-- END:notes -->
`
	// Act — parse, modify, write, re-parse
	sections, err := parser.ParseSections(content)
	require.NoError(t, err)

	sections["description"] = "Modified description"
	result, err := parser.WriteSections(content, sections)
	require.NoError(t, err)

	sections2, err := parser.ParseSections(result)
	require.NoError(t, err)

	// Assert
	assert.Equal(t, "Modified description", sections2["description"])
	assert.Equal(t, sections["notes"], sections2["notes"])
}
