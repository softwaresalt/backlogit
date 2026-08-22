package events_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/events"
)

func TestSaveMemory_CreatesFile(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	path := filepath.Join(dir, "memories.json")

	// Act
	err := events.SaveMemory(context.Background(), path, "test-key", "test summary")

	// Assert
	require.NoError(t, err)
	assert.FileExists(t, path)
}

func TestCreateCheckpoint_WritesFile(t *testing.T) {
	// Arrange
	dir := t.TempDir()

	// Act
	result, err := events.CreateCheckpoint(context.Background(), dir, `{"state": "test"}`)

	// Assert
	require.NoError(t, err)
	assert.FileExists(t, result.Path)
}

// TestCreateCheckpoint_V1NoHTMLEscape is a regression test for the checkpoint
// JSON readability fix (137-F): V1 checkpoint creation must not HTML-escape
// <, >, and & in the rewritten JSON bytes.
func TestCreateCheckpoint_V1NoHTMLEscape(t *testing.T) {
	dir := t.TempDir()
	stateDump := `{"schema_version":1,"agent":"ship","session_id":"s1","phase":"build","resume_hint":"a > b && b < c"}`

	cpPath, err := events.CreateCheckpoint(context.Background(), dir, stateDump)
	require.NoError(t, err)

	data, err := os.ReadFile(cpPath.Path)
	require.NoError(t, err)
	s := string(data)

	assert.Contains(t, s, "a > b && b < c")
	assert.NotContains(t, s, `\u003e`)
	assert.NotContains(t, s, `\u003c`)
	assert.NotContains(t, s, `\u0026`)
}
