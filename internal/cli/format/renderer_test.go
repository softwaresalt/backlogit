package format_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/cli/format"
)

var testColumns = []format.Column{
	{Key: "id", Header: "ID"},
	{Key: "title", Header: "TITLE"},
	{Key: "status", Header: "STATUS"},
}

var testRows = []map[string]any{
	{"id": "001-F", "title": "Alpha feature", "status": "active"},
	{"id": "002-T", "title": "Beta task", "status": "queued"},
	{"id": "003-T", "title": "Gamma task", "status": "done"},
}

// TestFormatConstants asserts that Format type constants have expected string values.
func TestFormatConstants(t *testing.T) {
	assert.Equal(t, format.Format("table"), format.FormatTable)
	assert.Equal(t, format.Format("json"), format.FormatJSON)
	assert.Equal(t, format.Format("tile"), format.FormatTile)
}

// TestTableRenderer_ImplementsRenderer asserts that *TableRenderer satisfies Renderer.
func TestTableRenderer_ImplementsRenderer(t *testing.T) {
	var _ format.Renderer = format.NewTableRenderer()
}

// TestTableRenderer_ContainsHeaders asserts that table output contains column headings.
func TestTableRenderer_ContainsHeaders(t *testing.T) {
	r := format.NewTableRenderer()
	var buf bytes.Buffer

	err := r.Render(&buf, testColumns, testRows)
	require.NoError(t, err)

	out := buf.String()
	assert.Contains(t, out, "ID", "table output should contain ID header")
	assert.Contains(t, out, "TITLE", "table output should contain TITLE header")
	assert.Contains(t, out, "STATUS", "table output should contain STATUS header")
}

// TestTableRenderer_ContainsRowValues asserts that all row values appear in table output.
func TestTableRenderer_ContainsRowValues(t *testing.T) {
	r := format.NewTableRenderer()
	var buf bytes.Buffer

	err := r.Render(&buf, testColumns, testRows)
	require.NoError(t, err)

	out := buf.String()
	for _, row := range testRows {
		assert.Contains(t, out, row["id"], "row id should appear in table output")
		assert.Contains(t, out, row["title"], "row title should appear in table output")
	}
}

// TestTableRenderer_RowCountMatchesInput asserts the table has a row for each input.
func TestTableRenderer_RowCountMatchesInput(t *testing.T) {
	r := format.NewTableRenderer()
	var buf bytes.Buffer

	err := r.Render(&buf, testColumns, testRows)
	require.NoError(t, err)

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	// header + len(testRows) data rows
	assert.Equal(t, len(testRows)+1, len(lines), "table should have header plus one line per row")
}

// TestJSONRenderer_ImplementsRenderer asserts that *JSONRenderer satisfies Renderer.
func TestJSONRenderer_ImplementsRenderer(t *testing.T) {
	var _ format.Renderer = format.NewJSONRenderer()
}

// TestJSONRenderer_ValidJSONArray asserts that JSONRenderer produces a JSON array.
func TestJSONRenderer_ValidJSONArray(t *testing.T) {
	r := format.NewJSONRenderer()
	var buf bytes.Buffer

	err := r.Render(&buf, testColumns, testRows)
	require.NoError(t, err)

	out := strings.TrimSpace(buf.String())
	assert.True(t, strings.HasPrefix(out, "["), "JSON output should start with [")
	assert.True(t, strings.HasSuffix(out, "]"), "JSON output should end with ]")
}

// TestJSONRenderer_ContainsFieldValues asserts that JSON output includes expected values.
func TestJSONRenderer_ContainsFieldValues(t *testing.T) {
	r := format.NewJSONRenderer()
	var buf bytes.Buffer

	err := r.Render(&buf, testColumns, testRows)
	require.NoError(t, err)

	out := buf.String()
	assert.Contains(t, out, "001-F", "JSON output should contain first row id")
	assert.Contains(t, out, "Alpha feature", "JSON output should contain first row title")
	assert.Contains(t, out, "active", "JSON output should contain first row status")
}

// TestJSONRenderer_EmptyRows asserts that an empty input produces an empty JSON array.
func TestJSONRenderer_EmptyRows(t *testing.T) {
	r := format.NewJSONRenderer()
	var buf bytes.Buffer

	err := r.Render(&buf, testColumns, []map[string]any{})
	require.NoError(t, err)

	out := strings.TrimSpace(buf.String())
	assert.Equal(t, "[]", out, "empty input should produce []")
}
