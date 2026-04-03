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
	writer := events.NewEventWriter(dir)
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
	assert.FileExists(t, filepath.Join(dir, "T001.jsonl"))
}

func TestTailEvents_FiltersByItemID(t *testing.T) {
	// Arrange
	dir := t.TempDir()

	// Act
	result, err := events.TailEvents(context.Background(), dir, "T001", 5)

	// Assert
	require.NoError(t, err)
	assert.Empty(t, result)
}
