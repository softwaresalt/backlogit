package parser_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/parser"
)

func TestParseMarkdownFile_Valid(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	content := "---\nid: T001\ntitle: Test\nstatus: queued\nartifact_type: task\n---\n\nDescription"
	path := filepath.Join(dir, "T001.md")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))

	// Act
	artifact, body, err := parser.ParseMarkdownFile(path)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "T001", artifact.ID)
	assert.Contains(t, body, "Description")
}

func TestParseMarkdownFile_MissingFile(t *testing.T) {
	// Act
	_, _, err := parser.ParseMarkdownFile("/nonexistent/file.md")

	// Assert
	assert.Error(t, err)
}
