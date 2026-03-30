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
//
// Worker: Implement JSONL append with O_APPEND and mutex protection.
func (w *EventWriter) AppendEvent(ctx context.Context, event Event) error {
	panic("not implemented: Worker: Implement event stream append")
}
