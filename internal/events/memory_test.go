package events_test

import (
	"context"
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
	cpPath, err := events.CreateCheckpoint(context.Background(), dir, `{"state": "test"}`)

	// Assert
	require.NoError(t, err)
	assert.FileExists(t, cpPath)
}
