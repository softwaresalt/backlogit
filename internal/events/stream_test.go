package events_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/events"
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

func TestEventWriter_AppendEvent_WithCommitSHA(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	writer := events.NewEventWriter(dir)
	event := events.Event{
		Actor:     "test-agent",
		ItemID:    "T002",
		EventType: "state_change",
		Delta:     map[string]any{"status": "done"},
		CommitSHA: "abc123def456",
	}

	// Act
	err := writer.AppendEvent(context.Background(), event)

	// Assert
	require.NoError(t, err)
	logPath := filepath.Join(dir, "T002.jsonl")
	assert.FileExists(t, logPath)

	// Read back and verify CommitSHA is present in JSON
	raw, readErr := os.ReadFile(logPath)
	require.NoError(t, readErr)
	var parsed events.Event
	require.NoError(t, json.Unmarshal(raw, &parsed))
	assert.Equal(t, "abc123def456", parsed.CommitSHA)
}

func TestEventWriter_AppendEvent_WithoutCommitSHA_OmitsField(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	writer := events.NewEventWriter(dir)
	event := events.Event{
		Actor:     "test-agent",
		ItemID:    "T003",
		EventType: "state_change",
		Delta:     map[string]any{"status": "active"},
	}

	// Act
	err := writer.AppendEvent(context.Background(), event)

	// Assert
	require.NoError(t, err)
	logPath := filepath.Join(dir, "T003.jsonl")
	raw, readErr := os.ReadFile(logPath)
	require.NoError(t, readErr)

	// Verify commit_sha is omitted from JSON output
	assert.NotContains(t, string(raw), "commit_sha")
}

func TestEvent_UnmarshalWithoutCommitSHA_Succeeds(t *testing.T) {
	// Simulate an existing JSONL entry that lacks the commit_sha field
	jsonData := `{"timestamp":"2026-01-01T00:00:00Z","actor":"agent","item_id":"T004","event_type":"comment","delta":{"text":"hello"}}`

	var event events.Event
	err := json.Unmarshal([]byte(jsonData), &event)

	require.NoError(t, err)
	assert.Equal(t, "T004", event.ItemID)
	assert.Equal(t, "", event.CommitSHA) // zero value for missing field
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
