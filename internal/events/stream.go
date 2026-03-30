package events

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"
)

// Event represents a state change or comment in the event stream.
type Event struct {
	Timestamp time.Time      `json:"timestamp"`
	Actor     string         `json:"actor"`
	ItemID    string         `json:"item_id"`
	EventType string         `json:"event_type"`
	Delta     map[string]any `json:"delta"`
}

// EventWriter provides goroutine-safe append-only writes to events.jsonl.
type EventWriter struct {
	path string
	mu   sync.Mutex
}

// NewEventWriter creates an event writer for the given file path.
func NewEventWriter(path string) *EventWriter {
	return &EventWriter{path: path}
}

// AppendEvent marshals and appends an event to events.jsonl.
func (w *EventWriter) AppendEvent(_ context.Context, event Event) error {
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	f, err := os.OpenFile(w.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open events file: %w", err)
	}
	defer f.Close()
	_, err = fmt.Fprintf(f, "%s\n", data)
	return err
}
