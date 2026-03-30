package events_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/backlogit/backlogit/internal/events"
)

func TestEventWriter_AppendEvent(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	path := filepath.Join(dir, "events.jsonl")
	writer := events.NewEventWriter(path)
	event := events.Event{
		Actor:     "test-agent",
		ItemID:    "T001",
		EventType: "state_change",
		Delta:     map[string]any{"status": "done"},
	}

	// Act
	err := writer.AppendEvent(context.Background(), event)

	// Assert
	require.NoError(t, err)
	assert.FileExists(t, path)
}

func TestTailEvents_FiltersByItemID(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	path := filepath.Join(dir, "events.jsonl")

	// Act
	result, err := events.TailEvents(context.Background(), path, "T001", 5)

	// Assert
	require.NoError(t, err)
	assert.Empty(t, result)
}
