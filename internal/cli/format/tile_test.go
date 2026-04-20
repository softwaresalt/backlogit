package format_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/cli/format"
)

// TestTileRenderer_ImplementsRenderer asserts that *TileRenderer satisfies Renderer.
func TestTileRenderer_ImplementsRenderer(t *testing.T) {
	var _ format.Renderer = format.NewTileRenderer(false)
}

// TestTileRenderer_ContainsPropertyValues asserts that tile output contains row field values.
func TestTileRenderer_ContainsPropertyValues(t *testing.T) {
	r := format.NewTileRenderer(false)
	var buf bytes.Buffer

	err := r.Render(&buf, testColumns, testRows[:1])
	require.NoError(t, err)

	out := buf.String()
	assert.Contains(t, out, "001-F", "tile output should contain row id value")
	assert.Contains(t, out, "Alpha feature", "tile output should contain row title value")
	assert.Contains(t, out, "active", "tile output should contain row status value")
}

// TestTileRenderer_PropertyValueFormat asserts the "Header: value" line format.
func TestTileRenderer_PropertyValueFormat(t *testing.T) {
	r := format.NewTileRenderer(false)
	var buf bytes.Buffer

	err := r.Render(&buf, testColumns, testRows[:1])
	require.NoError(t, err)

	out := buf.String()
	assert.Contains(t, out, "ID:", "tile output should contain 'ID:' property label")
	assert.Contains(t, out, "TITLE:", "tile output should contain 'TITLE:' property label")
	assert.Contains(t, out, "STATUS:", "tile output should contain 'STATUS:' property label")
}

// TestTileRenderer_BlocksSeparatedByBlankLine asserts blank lines between blocks.
func TestTileRenderer_BlocksSeparatedByBlankLine(t *testing.T) {
	r := format.NewTileRenderer(false)
	var buf bytes.Buffer

	err := r.Render(&buf, testColumns, testRows)
	require.NoError(t, err)

	out := buf.String()
	assert.True(t, strings.Contains(out, "\n\n"),
		"tile output with multiple rows should contain blank-line separators between blocks")
}

// TestTileRenderer_BoldFalse asserts no ANSI escape sequences when Bold is false.
func TestTileRenderer_BoldFalse(t *testing.T) {
	r := format.NewTileRenderer(false)
	var buf bytes.Buffer

	err := r.Render(&buf, testColumns, testRows[:1])
	require.NoError(t, err)

	out := buf.String()
	assert.NotContains(t, out, "\x1b[",
		"tile output with Bold=false must not contain ANSI escape sequences")
}

// TestTileRenderer_BoldTrue asserts that ANSI bold sequences appear when Bold is true.
func TestTileRenderer_BoldTrue(t *testing.T) {
	r := format.NewTileRenderer(true)
	var buf bytes.Buffer

	err := r.Render(&buf, testColumns, testRows[:1])
	require.NoError(t, err)

	out := buf.String()
	assert.Contains(t, out, "\x1b[",
		"tile output with Bold=true should contain ANSI bold escape sequence on first property")
}

// TestTileRenderer_SingleRow asserts that a single row produces no trailing blank line.
func TestTileRenderer_SingleRow(t *testing.T) {
	r := format.NewTileRenderer(false)
	var buf bytes.Buffer

	err := r.Render(&buf, testColumns, testRows[:1])
	require.NoError(t, err)

	out := buf.String()
	assert.False(t, strings.HasSuffix(out, "\n\n"),
		"single-row tile output should not have a trailing blank line")
}
