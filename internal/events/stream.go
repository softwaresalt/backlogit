package events

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// EventEstimateHistory records size/provenance mutation audit entries.
const EventEstimateHistory = "estimate_history"

// Event represents a state change or comment in the event stream.
type Event struct {
	Timestamp time.Time      `json:"timestamp"`
	Actor     string         `json:"actor"`
	ItemID    string         `json:"item_id"`
	EventType string         `json:"event_type"`
	Delta     map[string]any `json:"delta"`
	CommitSHA string         `json:"commit_sha,omitempty"`
}

// EventWriter provides goroutine-safe append-only writes to per-item JSONL log files.
type EventWriter struct {
	logsDir string
	mu      sync.Mutex
}

// NewEventWriter creates an event writer for the given logs directory.
func NewEventWriter(logsDir string) *EventWriter {
	return &EventWriter{logsDir: logsDir}
}

// LogPathForItem returns the JSONL log path for a work item ID.
func LogPathForItem(logsDir, itemID string) string {
	return filepath.Join(logsDir, itemID+".jsonl")
}

// AppendEvent marshals and appends an event to the item's JSONL log file.
func (w *EventWriter) AppendEvent(_ context.Context, event Event) error {
	if event.ItemID == "" {
		return fmt.Errorf("append event: item_id is required")
	}
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if err := os.MkdirAll(w.logsDir, 0o755); err != nil {
		return fmt.Errorf("create logs dir: %w", err)
	}
	path := LogPathForItem(w.logsDir, event.ItemID)
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open item log file: %w", err)
	}
	defer f.Close()
	_, err = fmt.Fprintf(f, "%s\n", data)
	return err
}
