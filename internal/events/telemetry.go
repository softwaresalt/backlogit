package events

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"
)

// TelemetryEntry represents an agent telemetry log entry.
type TelemetryEntry struct {
	Timestamp time.Time      `json:"timestamp"`
	EventType string         `json:"event_type"`
	Payload   map[string]any `json:"payload"`
}

// TelemetryWriter provides goroutine-safe writes to telemetry.jsonl.
type TelemetryWriter struct {
	path string
	mu   sync.Mutex
}

// NewTelemetryWriter creates a telemetry writer.
func NewTelemetryWriter(path string) *TelemetryWriter {
	return &TelemetryWriter{path: path}
}

// LogTelemetry appends a telemetry entry to telemetry.jsonl.
func (w *TelemetryWriter) LogTelemetry(_ context.Context, entry TelemetryEntry) error {
	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now()
	}
	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("marshal telemetry: %w", err)
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	f, err := os.OpenFile(w.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open telemetry file: %w", err)
	}
	defer f.Close()
	_, err = fmt.Fprintf(f, "%s\n", data)
	return err
}
